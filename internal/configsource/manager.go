package configsource

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/agentdeck/agentdeck/internal/config"
)

// Health values reported over SSE / in the discovery response. A binding that
// resolves cleanly is "ok"; a source that fails freshness is invalid or needs
// re-approval; a display-only last-known-good is "stale".
const (
	HealthOK               = "ok"
	HealthSourceInvalid    = "source_invalid"
	HealthApprovalRequired = "approval_required"
	HealthStale            = "stale"

	// previewTokenTTL bounds how long a preview token authorizes a bind (§2.6).
	previewTokenTTL = 10 * time.Minute
)

// ErrSourceChanged is returned when a bind's preview token no longer matches the
// live source fingerprints (a TOCTOU write between preview and PUT). It maps to
// 409 source_changed.
var ErrSourceChanged = errors.New("config source: changed since preview")

// ErrNoBinding is returned by ResolveFresh when a backend has no active source
// binding — the caller falls back to backends.json defaults, not an error path.
var ErrNoBinding = errors.New("config source: no binding")

// Update is the SSE payload published when a binding's effective view changes.
type Update struct {
	BackendID  string   `json:"backend_id"`
	ProjectID  string   `json:"project_id"`
	Generation int      `json:"generation"`
	Health     string   `json:"health"`
	Changed    []string `json:"changed"`
	Stale      bool     `json:"stale"`
}

// generation is an immutable resolved view for one (backend, project). Once
// published it is never mutated; a refresh swaps in a new pointer.
type generation struct {
	id         int
	backendID  string
	projectID  string
	project    config.Project // retained so watch/sweep can re-resolve
	binding    Binding
	effective  Effective
	report     Report
	health     string
	stale      bool
	resolvedAt time.Time
	watchDirs  []string        // parent dirs to register with fsnotify
	watchSet   map[string]bool // membership test for fsnotify events
}

type genKey struct {
	backendID string
	projectID string
}

// previewToken authorizes a single bind, tying the client's later PUT to the
// exact provider/root/profile/fingerprints the server showed during preview.
type previewToken struct {
	provider string
	root     string
	profile  string
	digest   string // report.SourceDigest at preview time
	expires  time.Time
}

// Manager owns config-source resolution for the server: immutable per-binding
// generations, the mirrored last-known-good cache, preview-token consent, and
// (via watch.go) filesystem watching plus a periodic sweep. All resolution flows
// through here so launch freshness, SSE and caching share one code path.
type Manager struct {
	store     *config.Store
	resolvers map[string]Resolver // provider -> resolver
	cacheDir  string
	publish   func(Update)
	now       func() time.Time

	mu     sync.RWMutex
	gens   map[genKey]*generation
	nextID int
	tokens map[string]*previewToken

	// watch state (watch.go)
	watchMu   sync.Mutex
	watched   map[string]bool // parent dirs currently registered with fsnotify
	debounces map[genKey]*time.Timer
}

// NewManager constructs a Manager. resolvers is keyed by provider
// (ProviderClaude/ProviderCodex). publish may be nil (no SSE). The cache lives
// under {home}/cache/config-sources.
func NewManager(store *config.Store, resolvers map[string]Resolver, publish func(Update)) *Manager {
	if publish == nil {
		publish = func(Update) {}
	}
	return &Manager{
		store:     store,
		resolvers: resolvers,
		cacheDir:  filepath.Join(store.Home(), "cache", "config-sources"),
		publish:   publish,
		now:       time.Now,
		gens:      map[genKey]*generation{},
		tokens:    map[string]*previewToken{},
		watched:   map[string]bool{},
		debounces: map[genKey]*time.Timer{},
	}
}

// binding reads the persisted binding for a backend. A missing document or an
// unbound backend yields ErrNoBinding so callers fall through to defaults.
func (m *Manager) binding(backendID string) (Binding, error) {
	sources, err := m.store.ReadConfigSources()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			return Binding{}, ErrNoBinding
		}
		return Binding{}, err
	}
	b, ok := sources.Sources[backendID]
	if !ok {
		return Binding{}, ErrNoBinding
	}
	return b, nil
}

func (m *Manager) resolverFor(provider string) (Resolver, error) {
	r, ok := m.resolvers[provider]
	if !ok {
		return nil, fmt.Errorf("%w: no resolver for provider %q", ErrInvalidSource, provider)
	}
	return r, nil
}

// ResolveFresh is the launch correctness boundary (§2.5): it re-reads the source
// synchronously, never serving stale cache. It returns ErrNoBinding when the
// backend is unbound (caller uses backends.json), ErrApprovalRequired /
// ErrInvalidSource on a source that must not launch, and otherwise the freshly
// resolved effective view plus the binding whose fields it owns.
func (m *Manager) ResolveFresh(ctx context.Context, backendID, projectID string, project config.Project) (Effective, Report, Binding, error) {
	b, err := m.binding(backendID)
	if err != nil {
		return Effective{}, Report{}, Binding{}, err
	}
	resolver, err := m.resolverFor(b.Provider)
	if err != nil {
		return Effective{}, Report{}, b, err
	}
	eff, rep, err := resolveStable(ctx, resolver, b, project)
	if err != nil {
		// Keep last-known-good for display but mark it stale; the launch still
		// fails so we never compose args from a stale generation.
		m.markStale(backendID, projectID, err)
		return Effective{}, Report{}, b, err
	}
	m.commit(backendID, projectID, project, b, eff, rep)
	return eff, rep, b, nil
}

// Preview resolves read-only for the given (not-yet-persisted) binding and mints
// a preview token the caller must echo back on PUT. It never touches the stored
// generation or cache.
func (m *Manager) Preview(ctx context.Context, b Binding, project config.Project) (Effective, Report, string, time.Time, error) {
	resolver, err := m.resolverFor(b.Provider)
	if err != nil {
		return Effective{}, Report{}, "", time.Time{}, err
	}
	eff, rep, err := resolver.Preview(ctx, b, project)
	if err != nil {
		return eff, rep, "", time.Time{}, err
	}
	token, expires := m.mintToken(b, rep.SourceDigest)
	return eff, rep, token, expires, nil
}

// ConsumePreviewToken validates and one-time-consumes a preview token against the
// binding being persisted. The provider/root/profile must match what was
// previewed, the token must be unexpired, and (TOCTOU guard) the live source
// digest must still equal the previewed digest.
func (m *Manager) ConsumePreviewToken(ctx context.Context, token string, b Binding, project config.Project) error {
	m.mu.Lock()
	pt, ok := m.tokens[token]
	if ok {
		delete(m.tokens, token)
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("%w: unknown or spent preview token", ErrApprovalRequired)
	}
	if m.now().After(pt.expires) {
		return fmt.Errorf("%w: preview token expired", ErrApprovalRequired)
	}
	if pt.provider != b.Provider || pt.root != b.Root || pt.profile != b.Profile {
		return fmt.Errorf("%w: preview token does not match binding", ErrApprovalRequired)
	}
	resolver, err := m.resolverFor(b.Provider)
	if err != nil {
		return err
	}
	_, rep, err := resolver.Preview(ctx, b, project)
	if err != nil {
		return err
	}
	if rep.SourceDigest != pt.digest {
		return ErrSourceChanged
	}
	return nil
}

func (m *Manager) mintToken(b Binding, digest string) (string, time.Time) {
	var raw [24]byte
	_, _ = rand.Read(raw[:])
	token := hex.EncodeToString(raw[:])
	expires := m.now().Add(previewTokenTTL)
	m.mu.Lock()
	m.pruneTokensLocked()
	m.tokens[token] = &previewToken{provider: b.Provider, root: b.Root, profile: b.Profile, digest: digest, expires: expires}
	m.mu.Unlock()
	return token, expires
}

func (m *Manager) pruneTokensLocked() {
	now := m.now()
	for k, t := range m.tokens {
		if now.After(t.expires) {
			delete(m.tokens, k)
		}
	}
}

// commit installs a fresh generation and publishes an update if the effective
// view changed. Mirrored bindings also write the redacted cache.
func (m *Manager) commit(backendID, projectID string, project config.Project, b Binding, eff Effective, rep Report) {
	dirs, set := watchDirsFor(rep)
	m.mu.Lock()
	key := genKey{backendID, projectID}
	prev := m.gens[key]
	m.nextID++
	gen := &generation{
		id: m.nextID, backendID: backendID, projectID: projectID, project: project,
		binding: b, effective: eff, report: rep, health: HealthOK, resolvedAt: m.now(),
		watchDirs: dirs, watchSet: set,
	}
	m.gens[key] = gen
	m.mu.Unlock()

	if b.Mode == ModeMirrored {
		if err := m.writeCache(gen); err != nil {
			// Cache is compatibility state only; a failure never blocks resolution.
			gen.report.Warnings = append(gen.report.Warnings, "mirror cache write failed")
		}
	}
	m.publish(Update{
		BackendID: backendID, ProjectID: projectID, Generation: gen.id,
		Health: HealthOK, Changed: changedFields(prev, gen), Stale: false,
	})
}

// markStale flags the existing generation stale (display-only) after a failed
// fresh resolve and publishes the corresponding health.
func (m *Manager) markStale(backendID, projectID string, cause error) {
	health := HealthSourceInvalid
	if errors.Is(cause, ErrApprovalRequired) {
		health = HealthApprovalRequired
	}
	m.mu.Lock()
	key := genKey{backendID, projectID}
	genID := 0
	if prev := m.gens[key]; prev != nil {
		// Copy-on-write: never mutate the published pointer.
		stale := *prev
		stale.stale = true
		stale.health = health
		m.gens[key] = &stale
		genID = stale.id
	}
	m.mu.Unlock()
	m.publish(Update{
		BackendID: backendID, ProjectID: projectID, Generation: genID,
		Health: health, Changed: nil, Stale: true,
	})
}

// Cached returns the last-known-good generation for display (may be stale). The
// bool is false when nothing has been resolved yet.
func (m *Manager) Cached(backendID, projectID string) (Effective, Report, string, bool, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	gen := m.gens[genKey{backendID, projectID}]
	if gen == nil {
		return Effective{}, Report{}, "", false, false
	}
	return gen.effective, gen.report, gen.health, gen.stale, true
}

// writeCache atomically writes the redacted normalized cache for a mirrored
// binding. The Effective/Report objects are already redacted by the resolver.
func (m *Manager) writeCache(gen *generation) error {
	dir := filepath.Join(m.cacheDir, gen.backendID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	payload := struct {
		Version    int       `json:"version"`
		BackendID  string    `json:"backend_id"`
		ProjectID  string    `json:"project_id"`
		Generation int       `json:"generation"`
		ResolvedAt time.Time `json:"resolved_at"`
		Effective  Effective `json:"effective"`
		Report     Report    `json:"report"`
	}{1, gen.backendID, gen.projectID, gen.id, gen.resolvedAt, gen.effective, gen.report}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	final := filepath.Join(dir, cacheFileName(gen.projectID))
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

// watchDirsFor collects the parent directories of every file the resolver read
// plus the approved roots, so fsnotify covers create/rename of not-yet-existing
// files (e.g. a settings.local.json that appears later).
func watchDirsFor(rep Report) ([]string, map[string]bool) {
	set := map[string]bool{}
	for _, fp := range rep.Fingerprints {
		set[filepath.Dir(fp.Path)] = true
	}
	for _, root := range rep.ApprovedRoots {
		set[root] = true
	}
	dirs := make([]string, 0, len(set))
	for d := range set {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	return dirs, set
}

func cacheFileName(projectID string) string {
	if projectID == "" {
		return "_.json"
	}
	// Project ids are validated slugs elsewhere; guard the path just in case.
	return filepath.Base(projectID) + ".json"
}

// resolveStable resolves twice and compares the source digest so a concurrent
// write during the read cannot yield a mixed-file view (§2.5). If the two reads
// disagree the tree changed mid-read; one retry returns generation N or N+1.
func resolveStable(ctx context.Context, resolver Resolver, b Binding, project config.Project) (Effective, Report, error) {
	eff1, rep1, err := resolver.Resolve(ctx, b, project)
	if err != nil {
		return eff1, rep1, err
	}
	eff2, rep2, err := resolver.Resolve(ctx, b, project)
	if err != nil {
		return eff2, rep2, err
	}
	if rep1.SourceDigest == rep2.SourceDigest {
		return eff2, rep2, nil
	}
	return resolver.Resolve(ctx, b, project)
}

// changedFields diffs two generations' high-level effective view for the SSE
// "changed" list. A nil previous means every present field is new.
func changedFields(prev, cur *generation) []string {
	changed := []string{}
	var pe *Effective
	if prev != nil {
		pe = &prev.effective
	}
	if !strEq(effModel(pe), effModel(&cur.effective)) {
		changed = append(changed, "model")
	}
	if !strEq(effEffort(pe), effEffort(&cur.effective)) {
		changed = append(changed, "effort")
	}
	if prevAssetDigest(pe) != prevAssetDigest(&cur.effective) {
		changed = append(changed, "setup")
	}
	sort.Strings(changed)
	return changed
}

func effModel(e *Effective) *string {
	if e == nil {
		return nil
	}
	return e.Model
}

func effEffort(e *Effective) *string {
	if e == nil {
		return nil
	}
	return e.Effort
}

func prevAssetDigest(e *Effective) string {
	if e == nil {
		return ""
	}
	paths := make([]string, 0, len(e.Assets))
	for _, a := range e.Assets {
		paths = append(paths, a.Path+":"+a.SHA256)
	}
	sort.Strings(paths)
	return fmt.Sprint(paths)
}

func strEq(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
