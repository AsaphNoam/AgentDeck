package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/AsaphNoam/Chuck/internal/backend/credcheck"
	"github.com/AsaphNoam/Chuck/internal/backend/providerexec"
	"github.com/AsaphNoam/Chuck/internal/config"
	"github.com/AsaphNoam/Chuck/internal/runtime"
)

// providerRuntime is one backend/model's next-start provider metadata
// (TS-03.R52). Version and CheckedAt appear only for a fresh observation of the
// same executable; an available path with no version is unverified, not
// incompatible.
type providerRuntime struct {
	Source    string `json:"source"`
	State     string `json:"state"`
	Path      string `json:"path,omitempty"`
	Version   string `json:"version,omitempty"`
	CheckedAt string `json:"checked_at,omitempty"`
}

// providerObservation is a version a Refresh provider accepted for one
// executable identity.
type providerObservation struct {
	identity  string
	version   string
	checkedAt time.Time
}

// providerObservationTTL bounds how long a GET reuses an observation (TS-04.R72).
const providerObservationTTL = 60 * time.Second

// providerRefreshSlots caps concurrent refreshes process-wide, with no waiting
// queue (TS-03.R53).
var providerRefreshSlots = make(chan struct{}, 2)

func providerObsKey(backendID, modelID string) string { return backendID + "\x00" + modelID }

// executableIdentity names the selection and the file it reaches, so an
// observation is never reused after a symlink retarget or in-place update.
func executableIdentity(sel providerexec.Selection) string {
	id := sel.Source + "\x00" + sel.Path
	if target, err := filepath.EvalSymlinks(sel.Path); err == nil {
		id += "\x00" + target
		if info, err := os.Stat(target); err == nil {
			id += fmt.Sprintf("\x00%d\x00%d", info.Size(), info.ModTime().UnixNano())
		}
	}
	return id
}

func (s *Server) discardProviderObservations() {
	s.providerObsMu.Lock()
	s.providerObs = nil
	s.providerObsMu.Unlock()
}

func (s *Server) publishProviderObservation(backendID, modelID string, obs providerObservation) {
	s.providerObsMu.Lock()
	if s.providerObs == nil {
		s.providerObs = map[string]providerObservation{}
	}
	s.providerObs[providerObsKey(backendID, modelID)] = obs
	s.providerObsMu.Unlock()
}

// providerRuntimeFor projects one selection, attaching a fresh matching
// observation. It only resolves and stats: no provider command runs.
func (s *Server) providerRuntimeFor(backendID, modelID string, sel providerexec.Selection) providerRuntime {
	out := providerRuntime{Source: sel.Source, State: sel.State, Path: sel.Path}
	if !sel.Available() {
		return out
	}
	s.providerObsMu.Lock()
	obs, ok := s.providerObs[providerObsKey(backendID, modelID)]
	s.providerObsMu.Unlock()
	if ok && time.Since(obs.checkedAt) <= providerObservationTTL && obs.identity == executableIdentity(sel) {
		out.Version, out.CheckedAt = obs.version, obs.checkedAt.UTC().Format(time.RFC3339)
	}
	return out
}

// providerRuntimes is the GET/PUT projection for every Claude/Codex
// backend/model. Empty maps are {} (INV §11).
func (s *Server) providerRuntimes(b config.BackendsConfig) map[string]map[string]providerRuntime {
	out := map[string]map[string]providerRuntime{}
	for backendID, be := range b.Backends {
		if _, ok := providerexec.ForBackendType(be.Type); !ok {
			continue
		}
		models := map[string]providerRuntime{}
		for modelID, model := range be.Models {
			sel, _ := providerexec.ForBackend(be, model)
			models[modelID] = s.providerRuntimeFor(backendID, modelID, sel)
		}
		out[backendID] = models
	}
	return out
}

type refreshCatalogResult struct {
	Status     string `json:"status"` // added | unchanged | disabled | unavailable
	AddedCount int    `json:"added_count"`
}

type refreshProviderResponse struct {
	Runtime     providerRuntime      `json:"runtime"`
	Credentials credcheck.CredResult `json:"credentials"`
	Catalog     refreshCatalogResult `json:"catalog"`
}

// handleRefreshProvider implements POST /api/backends/{id}/refresh-provider
// (FS-09.R72, TS-03.R53): recheck one saved backend/model's selected provider,
// its readiness and, when autosync is on, the add-only local model import.
// Provider execution happens outside the catalog lock; under it the original
// catalog is revalidated before anything is written or published.
func (s *Server) handleRefreshProvider(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ModelID string `json:"model_id"`
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4<<10))
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "request body too large or unreadable"))
		return
	}
	if len(raw) > 0 {
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			writeAPIError(w, apiError(runtime.CodeValidation, "body may only contain model_id"))
			return
		}
	}

	backendID := r.PathValue("id")
	current, err := s.configStore.ReadBackends()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) || errors.Is(err, config.ErrCorrupt) {
			current = config.DefaultBackends()
		} else {
			writeAPIError(w, apiError(runtime.CodeInternal, "internal error"))
			return
		}
	}
	be, ok := current.Backends[backendID]
	if !ok {
		writeAPIError(w, apiError(runtime.CodeNotFound, "unknown backend: "+backendID))
		return
	}
	modelID := req.ModelID
	if modelID == "" {
		modelID = be.DefaultModel
	}
	model, ok := be.Models[modelID]
	if !ok {
		writeAPIError(w, apiError(runtime.CodeNotFound, "unknown model: "+modelID))
		return
	}
	if _, ok := providerexec.ForBackendType(be.Type); !ok {
		writeAPIError(w, apiError(runtime.CodeValidation, "provider refresh is available for Claude and Codex backends"))
		return
	}
	original := backendCatalogETag(current)
	if r.Header.Get("If-Match") != original {
		writeAPIError(w, apiError(runtime.CodeBackendCatalogChanged, "backend catalog changed; reload it before refreshing"))
		return
	}
	select {
	case providerRefreshSlots <- struct{}{}:
		defer func() { <-providerRefreshSlots }()
	default:
		writeAPIError(w, apiError(runtime.CodeProviderCheckBusy, "provider checks are already running; retry in a moment"))
		return
	}

	// Bounded checks, outside the lock, sharing readiness's deadline.
	ctx, cancel := context.WithTimeout(r.Context(), credcheck.DefaultTimeout)
	defer cancel()
	sel, _ := providerexec.ForBackend(be, model)
	obs := providerObservation{identity: executableIdentity(sel), checkedAt: time.Now()}
	if sel.Available() {
		obs.version = providerexec.ProbeVersion(ctx, sel.Path, composeEnv(os.Environ(), be.Env, model.Env))
	}
	credentials := s.credCheck(ctx, be, model, credcheck.MergeEnv(be.Env, model.Env))

	s.catalogMu.Lock()
	latest, err := s.configStore.ReadBackends()
	if errors.Is(err, config.ErrNotFound) || errors.Is(err, config.ErrCorrupt) {
		latest, err = config.DefaultBackends(), nil
	}
	if err != nil || backendCatalogETag(latest) != original {
		s.catalogMu.Unlock()
		writeAPIError(w, apiError(runtime.CodeBackendCatalogChanged, "backend catalog changed during the check; reload and retry"))
		return
	}
	catalog := refreshCatalogResult{Status: "disabled"}
	enabled, available, added := config.RefreshBackendModels(&latest, backendID)
	switch {
	case !enabled:
	case !available:
		catalog.Status = "unavailable"
	case added == 0:
		catalog.Status = "unchanged"
	default:
		if err := s.configStore.WriteBackends(latest); err != nil {
			s.catalogMu.Unlock()
			s.log.Error("backends: refresh import", "err", err)
			writeAPIError(w, apiError(runtime.CodeInternal, "internal error"))
			return
		}
		catalog = refreshCatalogResult{Status: "added", AddedCount: added}
	}
	if added > 0 {
		s.invalidateOnboardingCache() // a catalog write: also discards observations
	} else {
		s.onboardingCacheMu.Lock()
		s.onboardingCache = nil // readiness was rechecked
		s.onboardingCacheMu.Unlock()
	}
	if obs.version != "" {
		s.publishProviderObservation(backendID, modelID, obs)
	}
	etag := backendCatalogETag(latest)
	s.catalogMu.Unlock()

	w.Header().Set("ETag", etag)
	writeJSON(w, http.StatusOK, refreshProviderResponse{
		Runtime:     s.providerRuntimeFor(backendID, modelID, sel),
		Credentials: credentials,
		Catalog:     catalog,
	})
}
