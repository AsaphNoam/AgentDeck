package configsource

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/config"
)

// recorder captures published updates in a thread-safe way (watch fires them
// from timer goroutines).
type recorder struct {
	mu      sync.Mutex
	updates []Update
}

func (r *recorder) publish(u Update) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updates = append(r.updates, u)
}

func (r *recorder) snapshot() []Update {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Update{}, r.updates...)
}

// managerFixture builds a Manager over a Claude source tree plus a distinct
// AgentDeck home holding config-sources.json and the mirror cache.
func managerFixture(t *testing.T) (*Manager, *config.Store, *recorder, string, config.Project) {
	t.Helper()
	userHome, project := claudeFixture(t)
	root := filepath.Join(userHome, ".claude")
	writeClaudeTestFile(t, filepath.Join(root, "settings.json"), `{"model":"user-model","env":{"ANTHROPIC_API_KEY":"user-secret"}}`)

	agentHome := t.TempDir()
	store := config.NewWithHome(agentHome)
	rec := &recorder{}
	m := NewManager(store, map[string]Resolver{
		ProviderClaude: NewClaudeResolver(userHome),
	}, rec.publish)
	return m, store, rec, root, config.Project{Cwd: project}
}

func writeBinding(t *testing.T, store *config.Store, backendID string, b Binding) {
	t.Helper()
	c := config.ConfigSources{Version: 1, Sources: map[string]config.SourceBinding{backendID: b}}
	if err := store.WriteConfigSources(c); err != nil {
		t.Fatalf("write binding: %v", err)
	}
}

func TestResolveFreshNoBinding(t *testing.T) {
	m, _, _, _, project := managerFixture(t)
	_, _, _, err := m.ResolveFresh(context.Background(), "claude", "proj", project)
	if !errors.Is(err, ErrNoBinding) {
		t.Fatalf("err = %v, want ErrNoBinding", err)
	}
}

func TestResolveFreshResolvesAndCaches(t *testing.T) {
	m, store, rec, root, project := managerFixture(t)
	writeBinding(t, store, "claude", Binding{
		Provider: ProviderClaude, Mode: ModeLinked, Root: root,
		Claims: []string{"launch_defaults"}, Approved: []string{root, project.Cwd},
	})
	eff, _, b, err := m.ResolveFresh(context.Background(), "claude", "proj", project)
	if err != nil {
		t.Fatalf("ResolveFresh: %v", err)
	}
	if eff.Model == nil || *eff.Model != "user-model" {
		t.Fatalf("model = %v, want user-model", eff.Model)
	}
	if b.Provider != ProviderClaude {
		t.Fatalf("binding provider = %q", b.Provider)
	}
	// Cached returns the committed generation.
	cachedEff, _, health, stale, ok := m.Cached("claude", "proj")
	if !ok || stale || health != HealthOK || cachedEff.Model == nil || *cachedEff.Model != "user-model" {
		t.Fatalf("Cached = %+v health=%q stale=%v ok=%v", cachedEff.Model, health, stale, ok)
	}
	// A publish with the new generation must have fired.
	ups := rec.snapshot()
	if len(ups) == 0 || ups[len(ups)-1].Health != HealthOK {
		t.Fatalf("updates = %+v", ups)
	}
}

func TestResolveFreshInvalidSourceBlocksButKeepsLastKnownGood(t *testing.T) {
	m, store, rec, root, project := managerFixture(t)
	writeBinding(t, store, "claude", Binding{
		Provider: ProviderClaude, Mode: ModeLinked, Root: root,
		Claims: []string{"launch_defaults"}, Approved: []string{root, project.Cwd},
	})
	if _, _, _, err := m.ResolveFresh(context.Background(), "claude", "proj", project); err != nil {
		t.Fatalf("initial ResolveFresh: %v", err)
	}
	// Corrupt the source: fresh resolve must fail and never launch from stale.
	writeClaudeTestFile(t, filepath.Join(root, "settings.json"), `{bad json`)
	_, _, _, err := m.ResolveFresh(context.Background(), "claude", "proj", project)
	if !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("err = %v, want ErrInvalidSource", err)
	}
	// The last-known-good is retained for display, flagged stale + invalid.
	_, _, health, stale, ok := m.Cached("claude", "proj")
	if !ok || !stale || health != HealthSourceInvalid {
		t.Fatalf("Cached after invalid: health=%q stale=%v ok=%v", health, stale, ok)
	}
	ups := rec.snapshot()
	last := ups[len(ups)-1]
	if !last.Stale || last.Health != HealthSourceInvalid {
		t.Fatalf("stale update = %+v", last)
	}
}

func TestMirroredCacheIsRedactedAndOwnerOnly(t *testing.T) {
	m, store, _, root, project := managerFixture(t)
	writeBinding(t, store, "claude", Binding{
		Provider: ProviderClaude, Mode: ModeMirrored, Root: root,
		Claims: []string{"launch_defaults"}, Approved: []string{root, project.Cwd},
	})
	if _, _, _, err := m.ResolveFresh(context.Background(), "claude", "proj", project); err != nil {
		t.Fatalf("ResolveFresh: %v", err)
	}
	cacheFile := filepath.Join(store.Home(), "cache", "config-sources", "claude", "proj.json")
	info, err := os.Stat(cacheFile)
	if err != nil {
		t.Fatalf("stat cache: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("cache perm = %v, want 0600", info.Mode().Perm())
	}
	if dirInfo, err := os.Stat(filepath.Dir(cacheFile)); err != nil || dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("cache dir perm = %v err=%v", dirInfo.Mode().Perm(), err)
	}
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "user-secret") {
		t.Fatalf("mirror cache leaked a secret value:\n%s", data)
	}
	// The env key NAME may appear as metadata, but never the value.
	if !strings.Contains(string(data), "ANTHROPIC_API_KEY") {
		t.Errorf("expected redacted env key metadata in cache")
	}
}

func TestPreviewTokenRoundTrip(t *testing.T) {
	m, _, _, root, project := managerFixture(t)
	b := Binding{Provider: ProviderClaude, Mode: ModeLinked, Root: root, Claims: []string{"launch_defaults"}}
	overrideModel := "override-model"
	_, _, token, expires, err := m.Preview(context.Background(), b, "proj", project)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if token == "" || !expires.After(time.Now()) {
		t.Fatalf("token=%q expires=%v", token, expires)
	}
	bound, projectID, _, err := m.ConsumeBind(context.Background(), token, config.SourceOverrides{Model: &overrideModel})
	if err != nil {
		t.Fatalf("ConsumeBind: %v", err)
	}
	if projectID != "proj" {
		t.Fatalf("projectID = %q, want proj", projectID)
	}
	// The frozen binding carries the discovered approved roots + the override.
	if len(bound.Approved) == 0 || bound.Overrides.Model == nil || *bound.Overrides.Model != "override-model" {
		t.Fatalf("bound = %+v", bound)
	}
	if !containsString(bound.Approved, root) {
		t.Fatalf("approved roots %v missing root %s", bound.Approved, root)
	}
	// A spent token cannot be reused.
	if _, _, _, err := m.ConsumeBind(context.Background(), token, config.SourceOverrides{}); !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("reused token err = %v, want ErrApprovalRequired", err)
	}
}

func TestPreviewTokenTOCTOURejected(t *testing.T) {
	m, _, _, root, project := managerFixture(t)
	b := Binding{Provider: ProviderClaude, Mode: ModeLinked, Root: root, Claims: []string{"launch_defaults"}}
	_, _, token, _, err := m.Preview(context.Background(), b, "proj", project)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	// The source changes between preview and bind.
	writeClaudeTestFile(t, filepath.Join(root, "settings.json"), `{"model":"changed-model"}`)
	if _, _, _, err := m.ConsumeBind(context.Background(), token, config.SourceOverrides{}); !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("TOCTOU err = %v, want ErrSourceChanged", err)
	}
}

func TestPreviewTokenExpiry(t *testing.T) {
	m, _, _, root, project := managerFixture(t)
	now := time.Now()
	m.now = func() time.Time { return now }
	b := Binding{Provider: ProviderClaude, Mode: ModeLinked, Root: root, Claims: []string{"launch_defaults"}}
	_, _, token, _, err := m.Preview(context.Background(), b, "proj", project)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	now = now.Add(previewTokenTTL + time.Second)
	if _, _, _, err := m.ConsumeBind(context.Background(), token, config.SourceOverrides{}); !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("expired token err = %v, want ErrApprovalRequired", err)
	}
}

func TestConsumeBindUnknownToken(t *testing.T) {
	m, _, _, _, _ := managerFixture(t)
	if _, _, _, err := m.ConsumeBind(context.Background(), "nope", config.SourceOverrides{}); !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("unknown token err = %v, want ErrApprovalRequired", err)
	}
}

// TestSweepRecoversMissedEvent proves the periodic sweep re-resolves a generation
// after an external write that fsnotify may have dropped.
func TestSweepRecoversMissedEvent(t *testing.T) {
	m, store, _, root, project := managerFixture(t)
	writeBinding(t, store, "claude", Binding{
		Provider: ProviderClaude, Mode: ModeLinked, Root: root,
		Claims: []string{"launch_defaults"}, Approved: []string{root, project.Cwd},
	})
	if _, _, _, err := m.ResolveFresh(context.Background(), "claude", "proj", project); err != nil {
		t.Fatalf("ResolveFresh: %v", err)
	}
	writeClaudeTestFile(t, filepath.Join(root, "settings.json"), `{"model":"swept-model"}`)
	m.Sweep(context.Background())
	eff, _, _, _, ok := m.Cached("claude", "proj")
	if !ok || eff.Model == nil || *eff.Model != "swept-model" {
		t.Fatalf("after sweep model = %v, want swept-model", eff.Model)
	}
}

// TestWatchReresolvesOnFilesystemEvent exercises the real fsnotify path end to
// end: a write to a watched file triggers a debounced re-resolve.
func TestWatchReresolvesOnFilesystemEvent(t *testing.T) {
	m, store, _, root, project := managerFixture(t)
	writeBinding(t, store, "claude", Binding{
		Provider: ProviderClaude, Mode: ModeLinked, Root: root,
		Claims: []string{"launch_defaults"}, Approved: []string{root, project.Cwd},
	})
	if _, _, _, err := m.ResolveFresh(context.Background(), "claude", "proj", project); err != nil {
		t.Fatalf("ResolveFresh: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.Watch(ctx)
	// Give the watcher a moment to register the parent dir before writing.
	time.Sleep(150 * time.Millisecond)
	writeClaudeTestFile(t, filepath.Join(root, "settings.json"), `{"model":"watched-model"}`)

	deadline := time.After(3 * time.Second)
	for {
		eff, _, _, _, ok := m.Cached("claude", "proj")
		if ok && eff.Model != nil && *eff.Model == "watched-model" {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("watch did not re-resolve within deadline; model=%v", eff.Model)
		case <-time.After(50 * time.Millisecond):
		}
	}
}
