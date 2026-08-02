package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/configsource"
	"github.com/agentdeck/agentdeck/internal/runtime"
)

// Item-scoped backend creation (TS-03.R23, FS-04.R40). POST /api/backends adds
// exactly one canonical starter backend to the durable catalog. It deliberately
// never accepts the browser's whole-catalog Settings draft, so creating a backend
// cannot commit — or discard — unrelated unsaved edits; PUT /api/backends remains
// the complete-document editing operation.

// sourceConnectClaims is the standard claim set the normal connection requests.
// The regular flow chooses no root, profile, project, claim set, or binding mode
// (TS-07.R16); this list matches what the Settings panel has always previewed.
var sourceConnectClaims = []string{"launch_defaults", "model_catalog", "setup"}

type createBackendRequest struct {
	BackendID string `json:"backend_id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	// ConnectNativeConfiguration performs FS-08.R34's project-free Linked
	// connection as part of the same visible action.
	ConnectNativeConfiguration bool `json:"connect_native_configuration"`
}

// connectionResult reports the optional connection half of a create. "connected"
// carries the ordinary redacted binding; "unbound" carries a safe, retryable
// error and claims no binding, generation, or source event.
type connectionResult struct {
	Status           string                   `json:"status"`
	Binding          *configSourceBindingView `json:"binding,omitempty"`
	ModelSyncEnabled bool                     `json:"model_sync_enabled,omitempty"`
	ModelsAdded      int                      `json:"models_added,omitempty"`
	Error            *runtime.APIError        `json:"error,omitempty"`
}

type createBackendResponse struct {
	BackendID  string            `json:"backend_id"`
	Backend    config.Backend    `json:"backend"`
	Connection *connectionResult `json:"connection,omitempty"`
}

// handleCreateBackend implements POST /api/backends.
func (s *Server) handleCreateBackend(w http.ResponseWriter, r *http.Request) {
	var req createBackendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, fieldErrors(config.FieldError{
			Field: "", Code: "bad_request", Message: "malformed JSON",
		}))
		return
	}
	req.Name = strings.TrimSpace(req.Name)

	if !config.ValidSlug(req.BackendID) {
		writeValidationError(w, fieldErrors(config.FieldError{
			Field: "backend_id", Code: "invalid_slug",
			Message: "backend id must be lowercase letters, digits, and hyphens",
		}))
		return
	}
	if req.Name == "" {
		writeValidationError(w, fieldErrors(config.FieldError{
			Field: "name", Code: "required", Message: "display name is required",
		}))
		return
	}
	starter, known := config.StarterBackend(req.Type)
	if !known {
		writeValidationError(w, fieldErrors(config.FieldError{
			Field: "type", Code: "unknown_type", Message: "unknown backend type: " + req.Type,
		}))
		return
	}
	provider, federated := config.ProviderForBackendType(req.Type)
	if req.ConnectNativeConfiguration && !federated {
		// Rejected before anything is written: OpenCode and OpenHands have no
		// native configuration source to connect (FS-08.R1).
		writeValidationError(w, fieldErrors(config.FieldError{
			Field: "connect_native_configuration", Code: "unsupported",
			Message: req.Type + " has no native configuration to connect",
		}))
		return
	}

	backend, created, aerr := s.createBackend(req.BackendID, req.Name, starter)
	if aerr != nil {
		writeAPIError(w, aerr)
		return
	}
	s.invalidateOnboardingCache()

	resp := createBackendResponse{BackendID: req.BackendID, Backend: backend}
	if req.ConnectNativeConfiguration {
		// Creation already succeeded and is durable. A connection failure from
		// here leaves the valid backend saved and unbound rather than
		// compensating away the successful create (TS-07.R18).
		resp.Connection = s.connectNativeConfiguration(r.Context(), req.BackendID, provider)
	}
	// The editor merges this one backend into its local draft, so return the
	// validator for the final catalog too. That draft can then save its own
	// unrelated edits without being mistaken for a stale whole-catalog PUT.
	if refreshed, err := s.configStore.ReadBackends(); err == nil {
		resp.Backend = refreshed.Backends[req.BackendID]
		w.Header().Set("ETag", backendCatalogETag(refreshed))
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, resp)
}

// createBackend inserts one starter backend into the durable catalog under the
// shared catalog lock and reports whether it was newly created. Replaying the
// same id/name/type is idempotent — including after an earlier connection
// attempt already enabled autosync or added models — so a retried create never
// resets a partially connected backend.
func (s *Server) createBackend(backendID, name string, starter config.Backend) (config.Backend, bool, *runtime.APIError) {
	s.catalogMu.Lock()
	defer s.catalogMu.Unlock()

	backends, aerr := s.readBackendsForWrite()
	if aerr != nil {
		return config.Backend{}, false, aerr
	}
	if existing, ok := backends.Backends[backendID]; ok {
		if existing.Type != starter.Type || existing.Name != name {
			return config.Backend{}, false, apiError(runtime.CodeBackendExists,
				"a different backend already uses the id "+backendID)
		}
		return existing, false, nil
	}

	starter.Name = name
	// An empty catalog makes the new entry default; otherwise the existing
	// default is preserved.
	starter.Default = len(backends.Backends) == 0
	backends.Backends[backendID] = starter
	if verr := config.ValidateBackendsConfig(&backends); verr != nil {
		return config.Backend{}, false, apiError(runtime.CodeValidation, "backend is not a valid catalog entry")
	}
	if werr := s.configStore.WriteBackends(backends); werr != nil {
		return config.Backend{}, false, s.internalSourceError("backend create: write catalog", werr)
	}
	return backends.Backends[backendID], true, nil
}

// connectNativeConfiguration performs the normal project-free Linked connection
// server-side: standard auto-root discovery, a read-only preview, and its
// matching bind with the model import enabled (TS-07.R16). It never returns an
// error to the caller — a failure is reported as an unbound, retryable
// connection result beside the saved backend.
func (s *Server) connectNativeConfiguration(ctx context.Context, backendID, provider string) *connectionResult {
	candidates, err := s.sourceMgr.Discover(ctx, provider, config.Project{})
	if err != nil || len(candidates) == 0 || candidates[0].Root == "" {
		return unboundConnection(apiError(runtime.CodeSourceNotFound,
			"no native configuration found for "+provider))
	}
	binding := configsource.Binding{
		Provider: provider, Mode: configsource.ModeLinked,
		Root: candidates[0].Root, Claims: sourceConnectClaims,
	}
	_, _, token, _, err := s.sourceMgr.Preview(ctx, binding, "", config.Project{})
	if err != nil {
		return unboundConnection(sourceAPIError(err))
	}
	view, added, aerr := s.bindConfigSource(ctx, backendID, bindRequest{
		PreviewToken: token, EnableModelSync: true,
	})
	if aerr != nil {
		return unboundConnection(aerr)
	}
	return &connectionResult{
		Status: "connected", Binding: &view,
		ModelSyncEnabled: true, ModelsAdded: added,
	}
}

func unboundConnection(cause *runtime.APIError) *connectionResult {
	return &connectionResult{Status: "unbound", Error: cause}
}

// fieldErrors wraps single field errors in the shared validation envelope.
func fieldErrors(errs ...config.FieldError) *config.ValidationErrors {
	return &config.ValidationErrors{Errors: errs}
}

// invalidateOnboardingCache drops the cached credential-check result after any
// catalog mutation: env and model contents may have changed (INV §8).
func (s *Server) invalidateOnboardingCache() {
	s.onboardingCacheMu.Lock()
	s.onboardingCache = nil
	s.onboardingCacheMu.Unlock()
}
