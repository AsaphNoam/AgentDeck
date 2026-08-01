package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/agentdeck/agentdeck/internal/bus"
	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/configsource"
	"github.com/agentdeck/agentdeck/internal/runtime"
)

// newConfigSourceManager builds the federation SourceManager over the real user
// home so the resolvers read the CLI's native config, publishing changes onto
// the SSE bus as config_source_update events.
func newConfigSourceManager(store *config.Store, eventBus *bus.Bus) *configsource.Manager {
	userHome, _ := os.UserHomeDir()
	resolvers := map[string]configsource.Resolver{
		configsource.ProviderClaude: configsource.NewClaudeResolver(userHome),
		configsource.ProviderCodex: configsource.NewCodexResolver(configsource.CodexOptions{
			UserHome: userHome, CodexHome: os.Getenv("CODEX_HOME"),
		}),
	}
	publish := func(u configsource.Update) {
		bid := u.BackendID
		eventBus.Publish("config_source_update", &bid, u)
	}
	return configsource.NewManager(store, resolvers, publish)
}

// configSourceBindingView is the redacted binding + health returned by GET.
type configSourceBindingView struct {
	BackendID  string                 `json:"backend_id"`
	Provider   string                 `json:"provider"`
	Mode       string                 `json:"mode"`
	Root       string                 `json:"root"`
	Profile    string                 `json:"profile,omitempty"`
	Claims     []string               `json:"claims"`
	Overrides  config.SourceOverrides `json:"overrides,omitempty"`
	Approved   []string               `json:"approved_roots"`
	Health     string                 `json:"health,omitempty"`
	Stale      bool                   `json:"stale"`
	Generation int                    `json:"generation,omitempty"`
}

type configSourcesResponse struct {
	Bindings   []configSourceBindingView `json:"bindings"`
	Candidates []configsource.Candidate  `json:"candidates"`
}

// handleGetConfigSources implements GET /api/config-sources?project=<id>. It
// returns discovery candidates for the federation-capable backends plus the
// active bindings and their display health. It never returns secrets.
func (s *Server) handleGetConfigSources(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project")
	project, _ := s.readProjectOptional(projectID)

	backends, err := s.readBackendsOrDefault()
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, "read backends: "+err.Error()))
		return
	}
	sources, err := s.readConfigSources()
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, "read config sources: "+err.Error()))
		return
	}

	resp := configSourcesResponse{Bindings: []configSourceBindingView{}, Candidates: []configsource.Candidate{}}
	seenProvider := map[string]bool{}
	for backendID, backend := range backends.Backends {
		provider, ok := config.ProviderForBackendType(backend.Type)
		if !ok {
			continue
		}
		if !seenProvider[provider] {
			seenProvider[provider] = true
			if candidates, derr := s.sourceMgr.Discover(r.Context(), provider, project); derr == nil {
				resp.Candidates = append(resp.Candidates, candidates...)
			}
		}
		binding, bound := sources.Sources[backendID]
		if !bound {
			continue
		}
		health, stale, gen, _ := s.sourceMgr.Status(backendID, projectID)
		resp.Bindings = append(resp.Bindings, configSourceBindingView{
			BackendID: backendID, Provider: binding.Provider, Mode: binding.Mode,
			Root: binding.Root, Profile: binding.Profile, Claims: binding.Claims,
			Overrides: binding.Overrides, Approved: binding.Approved,
			Health: health, Stale: stale, Generation: gen,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

type previewRequest struct {
	Provider string   `json:"provider"`
	Root     string   `json:"root"` // "auto" | absolute path
	Profile  string   `json:"profile"`
	Mode     string   `json:"mode"`
	Claims   []string `json:"claims"`
	Project  string   `json:"project"`
}

type previewResponse struct {
	PreviewToken string                 `json:"preview_token"`
	ExpiresAt    time.Time              `json:"expires_at"`
	Effective    configsource.Effective `json:"effective"`
	Report       configsource.Report    `json:"report"`
}

// handlePreviewConfigSource implements POST /api/config-sources/preview. It
// resolves read-only and mints a preview token the client echoes back on PUT.
func (s *Server) handlePreviewConfigSource(w http.ResponseWriter, r *http.Request) {
	var req previewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apiError(runtime.CodeInvalidField, "invalid JSON body"))
		return
	}
	if req.Provider != configsource.ProviderClaude && req.Provider != configsource.ProviderCodex {
		writeAPIError(w, apiError(runtime.CodeInvalidField, "provider must be claude-code or codex"))
		return
	}
	if req.Mode != configsource.ModeLinked && req.Mode != configsource.ModeMirrored {
		writeAPIError(w, apiError(runtime.CodeInvalidField, "mode must be linked or mirrored"))
		return
	}
	// An omitted project previews the provider's user-level source only
	// (TS-03.R22). The persisted binding is backend-global either way: a later
	// launch still resolves against its own actual project.
	var project config.Project
	if req.Project != "" {
		var perr error
		if project, perr = s.configStore.ReadProject(req.Project); perr != nil {
			writeAPIError(w, apiError(runtime.CodeInvalidField, "unknown project: "+req.Project))
			return
		}
	}

	root := req.Root
	if root == "" || root == "auto" {
		candidates, derr := s.sourceMgr.Discover(r.Context(), req.Provider, project)
		if derr != nil || len(candidates) == 0 || candidates[0].Root == "" {
			writeAPIError(w, apiError(runtime.CodeSourceNotFound, "no native configuration found for "+req.Provider))
			return
		}
		root = candidates[0].Root
	}

	binding := configsource.Binding{
		Provider: req.Provider, Mode: req.Mode, Root: root,
		Profile: req.Profile, Claims: req.Claims,
	}
	effective, report, token, expires, err := s.sourceMgr.Preview(r.Context(), binding, req.Project, project)
	if err != nil {
		writeAPIError(w, sourceAPIError(err))
		return
	}
	writeJSON(w, http.StatusOK, previewResponse{
		PreviewToken: token, ExpiresAt: expires, Effective: effective, Report: report,
	})
}

type bindRequest struct {
	PreviewToken string                 `json:"preview_token"`
	Overrides    config.SourceOverrides `json:"overrides"`
	// EnableModelSync opts the bind into FS-09.R47's target-only model import.
	// Existing preview/override/compatibility callers omit it and keep the
	// source-only path byte-for-byte.
	EnableModelSync bool `json:"enable_model_sync"`
}

// bindResponse is the bind body: the redacted binding, plus the non-secret
// model-sync outcome when the caller enabled it (TS-03.R22).
type bindResponse struct {
	configSourceBindingView
	ModelSyncEnabled bool `json:"model_sync_enabled,omitempty"`
	ModelsAdded      int  `json:"models_added,omitempty"`
}

// handleBindConfigSource implements PUT /api/config-sources/{backend_id}. It
// rebuilds the binding from the preview token (never trusting client paths after
// preview), validates it against the backend, persists, and resolves fresh so
// the generation + SSE reflect the new binding.
func (s *Server) handleBindConfigSource(w http.ResponseWriter, r *http.Request) {
	backendID := r.PathValue("backend_id")
	var req bindRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apiError(runtime.CodeInvalidField, "invalid JSON body"))
		return
	}
	view, added, aerr := s.bindConfigSource(r.Context(), backendID, req)
	if aerr != nil {
		writeAPIError(w, aerr)
		return
	}
	writeJSON(w, http.StatusOK, bindResponse{
		configSourceBindingView: view,
		ModelSyncEnabled:        req.EnableModelSync,
		ModelsAdded:             added,
	})
}

// bindConfigSource consumes a preview token and persists its binding, returning
// the redacted binding view and how many models an enabled bind imported. It is
// the one bind seam: the PUT route and the item-scoped create's orchestrated
// connection (TS-03.R23) both go through it, so consent, TOCTOU re-checking, and
// persistence ordering cannot drift between them.
//
// Persistence order for an enabled bind is deliberate (TS-07.R17): the merged
// catalog is written first and the source manifest second, so an interruption
// can only leave an *unbound* backend carrying add-only models — never a binding
// that claims a connection the catalog does not reflect. The generation and its
// SSE are installed only after both writes succeed.
func (s *Server) bindConfigSource(ctx context.Context, backendID string, req bindRequest) (configSourceBindingView, int, *runtime.APIError) {
	backends, err := s.readBackendsOrDefault()
	if err != nil {
		return configSourceBindingView{}, 0, apiError(runtime.CodeInternal, "read backends: "+err.Error())
	}
	backend, ok := backends.Backends[backendID]
	if !ok {
		return configSourceBindingView{}, 0, apiError(runtime.CodeSourceNotFound, "unknown backend: "+backendID)
	}
	// Validate the named backend before spending consent (FS-08.R5).
	provider, supported := config.ProviderForBackendType(backend.Type)
	if !supported {
		return configSourceBindingView{}, 0, apiError(runtime.CodeInvalidField, "backend does not support configuration sources")
	}
	binding, projectID, project, err := s.sourceMgr.ConsumeBind(ctx, req.PreviewToken, req.Overrides)
	if err != nil {
		return configSourceBindingView{}, 0, sourceAPIError(err)
	}
	if provider != binding.Provider {
		return configSourceBindingView{}, 0, apiError(runtime.CodeInvalidField, "backend does not support this provider")
	}

	added, aerr := s.persistBinding(backendID, binding, req.EnableModelSync)
	if aerr != nil {
		return configSourceBindingView{}, 0, aerr
	}

	// Populate the generation + emit SSE. A resolve failure here does not undo the
	// persisted binding; it surfaces as stale/invalid health the UI can repair.
	_, _, _, _ = s.sourceMgr.ResolveFresh(ctx, backendID, projectID, project)
	health, stale, gen, _ := s.sourceMgr.Status(backendID, projectID)
	return configSourceBindingView{
		BackendID: backendID, Provider: binding.Provider, Mode: binding.Mode,
		Root: binding.Root, Profile: binding.Profile, Claims: binding.Claims,
		Overrides: binding.Overrides, Approved: binding.Approved,
		Health: health, Stale: stale, Generation: gen,
	}, added, nil
}

// persistBinding writes the (optionally model-merged) catalog and the source
// manifest under the shared catalog lock, restoring the catalog preimage when
// the manifest write returns an error. It returns the number of models imported.
func (s *Server) persistBinding(backendID string, binding config.SourceBinding, enableModelSync bool) (int, *runtime.APIError) {
	s.catalogMu.Lock()
	defer s.catalogMu.Unlock()

	added := 0
	var preimage config.BackendsConfig
	backends, aerr := s.readBackendsForWrite()
	if aerr != nil {
		return 0, aerr
	}
	if enableModelSync {
		// Re-read under the lock: the catalog may have changed since the
		// pre-consent check, and the preimage must be an independent document
		// (the merge mutates the target's model map in place).
		preimage, aerr = s.readBackendsForWrite()
		if aerr != nil {
			return 0, aerr
		}
		if _, still := backends.Backends[backendID]; !still {
			return 0, apiError(runtime.CodeSourceNotFound, "unknown backend: "+backendID)
		}
		added = config.ImportConfiguredModels(&backends, backendID)
		if verr := config.ValidateBackendsConfig(&backends); verr != nil {
			return 0, apiError(runtime.CodeSourceInvalid, "imported models are not a valid catalog")
		}
		if werr := s.configStore.WriteBackends(backends); werr != nil {
			return 0, apiError(runtime.CodeInternal, "write backends: "+werr.Error())
		}
		s.invalidateOnboardingCache()
	}

	sources, serr := s.readConfigSources()
	if serr != nil {
		return 0, s.restoreCatalog(preimage, enableModelSync,
			apiError(runtime.CodeInternal, "read config sources: "+serr.Error()))
	}
	sources.Sources[backendID] = binding
	if verr := config.ValidateConfigSources(&sources, backends); verr != nil {
		return 0, s.restoreCatalog(preimage, enableModelSync,
			apiError(runtime.CodeSourceInvalid, "binding is not valid for this backend"))
	}
	if werr := s.configStore.WriteConfigSources(sources); werr != nil {
		return 0, s.restoreCatalog(preimage, enableModelSync,
			apiError(runtime.CodeInternal, "write config sources: "+werr.Error()))
	}
	return added, nil
}

// restoreCatalog attempts to undo an enabled bind's catalog write after the
// source manifest failed, so a failed connection does not silently keep the
// autosync flag it turned on. A failed restoration is the accepted add-only
// unbound residue of TS-07.R17, not a second reported failure: the backend stays
// valid, no entry was overwritten, and the stable id makes retry converge.
func (s *Server) restoreCatalog(preimage config.BackendsConfig, attempted bool, cause *runtime.APIError) *runtime.APIError {
	if attempted {
		if err := s.configStore.WriteBackends(preimage); err != nil {
			s.log.Error("config source: catalog restoration failed", "err", err)
		}
		s.invalidateOnboardingCache()
	}
	return cause
}

// handleRefreshConfigSource implements POST /api/config-sources/{backend_id}/refresh.
// It re-resolves synchronously (the same path launch uses) so a manual refresh
// cannot serve stale cache.
func (s *Server) handleRefreshConfigSource(w http.ResponseWriter, r *http.Request) {
	backendID := r.PathValue("backend_id")
	projectID := r.URL.Query().Get("project")
	// An omitted project refreshes the user-level view of a backend-global
	// binding (TS-03.R22); an explicit one keeps the project-aware behavior.
	var project config.Project
	if projectID != "" {
		var perr error
		if project, perr = s.configStore.ReadProject(projectID); perr != nil {
			writeAPIError(w, apiError(runtime.CodeInvalidField, "unknown project: "+projectID))
			return
		}
	}
	effective, _, binding, rerr := s.sourceMgr.ResolveFresh(r.Context(), backendID, projectID, project)
	if rerr != nil {
		writeAPIError(w, sourceAPIError(rerr))
		return
	}
	health, stale, gen, _ := s.sourceMgr.Status(backendID, projectID)
	writeJSON(w, http.StatusOK, struct {
		Binding   configSourceBindingView `json:"binding"`
		Effective configsource.Effective  `json:"effective"`
	}{
		Binding: configSourceBindingView{
			BackendID: backendID, Provider: binding.Provider, Mode: binding.Mode,
			Root: binding.Root, Profile: binding.Profile, Claims: binding.Claims,
			Overrides: binding.Overrides, Approved: binding.Approved,
			Health: health, Stale: stale, Generation: gen,
		},
		Effective: effective,
	})
}

// handleDeleteConfigSource implements DELETE /api/config-sources/{backend_id}.
// detach=false unbinds the source. detach=true (materialize under AgentDeck
// ownership) is gated: no Claude/Codex asset has a verified launch-injection copy
// path yet, so materialization is not implemented and returns 501 rather than
// silently dropping the copy promise.
func (s *Server) handleDeleteConfigSource(w http.ResponseWriter, r *http.Request) {
	backendID := r.PathValue("backend_id")
	detach := r.URL.Query().Get("detach") == "true"

	sources, err := s.readConfigSources()
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, "read config sources: "+err.Error()))
		return
	}
	if _, ok := sources.Sources[backendID]; !ok {
		writeAPIError(w, apiError(runtime.CodeSourceNotFound, "no configuration source bound to "+backendID))
		return
	}
	if detach {
		writeAPIError(w, apiError(runtime.CodeNotImplemented,
			"detached import is not yet available: no verified launch-injection path exists for Claude/Codex setup assets"))
		return
	}
	delete(sources.Sources, backendID)
	if werr := s.configStore.WriteConfigSources(sources); werr != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, "write config sources: "+werr.Error()))
		return
	}
	s.sourceMgr.ForgetBackend(backendID)
	w.WriteHeader(http.StatusNoContent)
}

// readConfigSources reads config-sources.json, treating a missing document as an
// empty (normalized) manifest so callers can amend and write it back.
func (s *Server) readConfigSources() (config.ConfigSources, error) {
	sources, err := s.configStore.ReadConfigSources()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			return config.ConfigSources{Version: 1, Sources: map[string]config.SourceBinding{}}, nil
		}
		return config.ConfigSources{}, err
	}
	if sources.Version == 0 {
		sources.Version = 1
	}
	if sources.Sources == nil {
		sources.Sources = map[string]config.SourceBinding{}
	}
	return sources, nil
}

// pruneIncompatibleConfigSources removes bindings whose backend was deleted or
// retargeted to another provider. A binding has no valid meaning without the
// backend it names; keeping it would block later binds when the manifest is
// validated as a whole.
func (s *Server) pruneIncompatibleConfigSources(backends config.BackendsConfig) error {
	sources, err := s.readConfigSources()
	if err != nil {
		return err
	}

	removed := make([]string, 0)
	for backendID, binding := range sources.Sources {
		backend, exists := backends.Backends[backendID]
		provider, supported := config.ProviderForBackendType(backend.Type)
		if exists && supported && binding.Provider == provider {
			continue
		}
		delete(sources.Sources, backendID)
		removed = append(removed, backendID)
	}
	if len(removed) == 0 {
		return nil
	}
	if err := s.configStore.WriteConfigSources(sources); err != nil {
		return err
	}
	for _, backendID := range removed {
		s.sourceMgr.ForgetBackend(backendID)
	}
	return nil
}

// readBackendsForWrite reads the catalog for a read-modify-write. Unlike
// readBackendsOrDefault it never substitutes the seeded catalog for an
// unreadable one: an item create or a model merge that fell back to the default
// document would overwrite a hand-broken backends.json with four invented
// backends (INV §7). A missing document is an empty catalog, which is the state
// a first create legitimately amends.
func (s *Server) readBackendsForWrite() (config.BackendsConfig, *runtime.APIError) {
	backends, err := s.configStore.ReadBackends()
	switch {
	case err == nil:
		if backends.Backends == nil {
			backends.Backends = map[string]config.Backend{}
		}
		return backends, nil
	case errors.Is(err, config.ErrNotFound):
		return config.BackendsConfig{Version: 2, Backends: map[string]config.Backend{}}, nil
	case errors.Is(err, config.ErrCorrupt):
		return config.BackendsConfig{}, apiError(runtime.CodeInternal,
			"the backend catalog is unreadable; repair backends.json before changing it")
	default:
		return config.BackendsConfig{}, apiError(runtime.CodeInternal, "read backends: "+err.Error())
	}
}

func (s *Server) readBackendsOrDefault() (config.BackendsConfig, error) {
	backends, err := s.configStore.ReadBackends()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) || errors.Is(err, config.ErrCorrupt) {
			return config.DefaultBackends(), nil
		}
		return config.BackendsConfig{}, err
	}
	return backends, nil
}

// readProjectOptional resolves a project by id, returning a zero Project when the
// id is empty or unknown (GET tolerates a missing project for user-layer preview).
func (s *Server) readProjectOptional(projectID string) (config.Project, bool) {
	if projectID == "" {
		return config.Project{}, false
	}
	project, err := s.configStore.ReadProject(projectID)
	if err != nil {
		return config.Project{}, false
	}
	return project, true
}

// launchConfigDoc is the redacted, versioned federation launch object frozen into
// sessions.launch_config_json (§2.5). It records the binding, the model AgentDeck
// requested vs what the source resolved, the source generation + fingerprints, and
// whether the model was left to native resolution. It carries no secret values:
// the effective model/effort/provider are display-safe and fingerprints are only
// path + hash + size.
type launchConfigDoc struct {
	Version         int                        `json:"version"`
	Binding         launchConfigBinding        `json:"binding"`
	RequestedModel  string                     `json:"requested_model"`
	Resolved        launchConfigResolved       `json:"resolved"`
	Generation      string                     `json:"generation"`
	Fingerprints    []configsource.Fingerprint `json:"fingerprints"`
	NativeInherited bool                       `json:"native_inherited"`
}

type launchConfigBinding struct {
	BackendID string `json:"backend_id"`
	Provider  string `json:"provider"`
	Profile   string `json:"profile,omitempty"`
	Mode      string `json:"mode"`
}

type launchConfigResolved struct {
	Model    *string `json:"model,omitempty"`
	Effort   *string `json:"effort,omitempty"`
	Provider *string `json:"provider,omitempty"`
}

// federationModel carries a bound source's model-composition decision back to the
// launch composer (§2.3/§2.4). Exactly one of the intents applies per launch:
//   - inherit: the user chose no model and the binding has no override, so the
//     model flag is OMITTED over ACP and the CLI resolves its own native model.
//   - override != nil: the binding carries an AgentDeck source override, applied
//     when the user chose no explicit model.
//   - neither: an explicit launch model was chosen and wins over the source.
type federationModel struct {
	inherit        bool
	override       *string
	effortOverride *string
}

// composeFederation resolves a backend's active source binding fresh at launch and
// returns the frozen launch-config JSON plus the source's model-composition
// decision. It returns (nil, nil, nil) when the backend is not federation-capable
// or has no binding (a plain launch). A stale/invalid/unapproved source returns an
// APIError so the launch is blocked, never composed from cache (§2.5).
//
// Model composition (§2.4) layers explicit launch choice / source override above
// native resolution: when neither an explicit model nor an override applies the
// model flag is omitted so the CLI applies its own native config via cwd/home
// pass-through, rather than AgentDeck forcing its backend default over ACP. The
// resolved high-level model is still recorded as redacted provenance.
func (s *Server) composeFederation(ctx context.Context, backendID string, req launchRequest, backend config.Backend, project config.Project, requestedModelID string) (json.RawMessage, *federationModel, *runtime.APIError) {
	if s.sourceMgr == nil {
		return nil, nil, nil
	}
	if _, ok := config.ProviderForBackendType(backend.Type); !ok {
		return nil, nil, nil
	}
	eff, rep, binding, err := s.sourceMgr.ResolveFresh(ctx, backendID, req.Project, project)
	if errors.Is(err, configsource.ErrNoBinding) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, sourceAPIError(err)
	}
	// Reserved messaging-MCP collision preflight (§2.4): AgentDeck injects an MCP
	// server with the reserved id messagingMCPName. If the native config already
	// declares that exact id, injecting ours would shadow/duplicate it, so block
	// the launch with 409 source_conflict rather than silently colliding.
	for _, name := range eff.MCPServers {
		if name == messagingMCPName {
			return nil, nil, apiError(runtime.CodeSourceConflict,
				"native configuration already declares the reserved MCP server id "+messagingMCPName)
		}
	}
	inherited := req.Model == "" && binding.Overrides.Model == nil
	doc := launchConfigDoc{
		Version:         1,
		Binding:         launchConfigBinding{BackendID: backendID, Provider: binding.Provider, Profile: binding.Profile, Mode: binding.Mode},
		RequestedModel:  requestedModelID,
		Resolved:        launchConfigResolved{Model: eff.Model, Effort: eff.Effort, Provider: eff.Provider},
		Generation:      rep.SourceDigest,
		Fingerprints:    rep.Fingerprints,
		NativeInherited: inherited,
	}
	data, err := json.Marshal(doc)
	if err != nil {
		return nil, nil, apiError(runtime.CodeInternal, "marshal launch config: "+err.Error())
	}
	fed := &federationModel{inherit: inherited}
	if req.Model == "" && binding.Overrides.Model != nil {
		fed.override = binding.Overrides.Model
	}
	if req.Effort == "" && binding.Overrides.Effort != nil {
		fed.effortOverride = binding.Overrides.Effort
	}
	return data, fed, nil
}

// frozenModelInherited reports whether a frozen federation launch object marks the
// model as native-inherited — i.e. launch composition omitted the model over ACP so
// the CLI resolves its own. Resume and same-identity switch honor this so a launch
// that deferred to native config does not silently regain an AgentDeck default. A
// missing/malformed doc means "not inherited" (send the model normally).
func frozenModelInherited(launchConfig json.RawMessage) bool {
	if len(launchConfig) == 0 {
		return false
	}
	var doc launchConfigDoc
	if err := json.Unmarshal(launchConfig, &doc); err != nil {
		return false
	}
	return doc.NativeInherited
}

// sourceAPIError maps a configsource error to the §2.7 API envelope. Resolver
// errors already carry only path + error-class (no secret values or raw source),
// so their message is safe to surface.
func sourceAPIError(err error) *runtime.APIError {
	switch {
	case errors.Is(err, configsource.ErrNoBinding):
		return apiError(runtime.CodeSourceNotFound, "no configuration source bound")
	case errors.Is(err, configsource.ErrSourceChanged):
		return apiError(runtime.CodeSourceChanged, "source changed since preview; re-preview and try again")
	case errors.Is(err, configsource.ErrApprovalRequired):
		return apiError(runtime.CodeApprovalRequired, err.Error())
	case errors.Is(err, configsource.ErrInvalidSource):
		return apiError(runtime.CodeSourceInvalid, err.Error())
	case errors.Is(err, os.ErrNotExist):
		return apiError(runtime.CodeSourceNotFound, "configuration source not found")
	default:
		return apiError(runtime.CodeInternal, "config source error")
	}
}
