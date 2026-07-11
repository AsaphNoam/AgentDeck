package configsource

import (
	"context"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	// sweepInterval is the stat/fingerprint safety net that catches missed
	// fsnotify events (§2.5).
	sweepInterval = 30 * time.Second
	// watchDebounce coalesces a burst of writes into one re-resolve (§2.5).
	watchDebounce = 250 * time.Millisecond
)

// Watch runs the filesystem watcher plus periodic sweep until ctx is cancelled;
// run it in a goroutine. When fsnotify is unavailable it degrades to the sweep
// alone so launch freshness still self-heals. It never writes source files or
// config-sources.json, so it cannot create a feedback loop.
func (m *Manager) Watch(ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		m.sweepLoop(ctx, nil)
		return
	}
	defer watcher.Close()
	m.sweepLoop(ctx, watcher)
}

func (m *Manager) sweepLoop(ctx context.Context, watcher *fsnotify.Watcher) {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()
	m.refreshWatches(watcher)

	var events <-chan fsnotify.Event
	var errs <-chan error
	if watcher != nil {
		events = watcher.Events
		errs = watcher.Errors
	}
	for {
		select {
		case <-ctx.Done():
			m.stopDebounces()
			return
		case ev := <-events:
			m.onFSEvent(ev)
		case <-errs:
			// A watcher error just means we lean on the sweep; ignore.
		case <-ticker.C:
			m.Sweep(ctx)
			m.refreshWatches(watcher)
		}
	}
}

// refreshWatches registers any newly-relevant parent directory with fsnotify.
// Add is only called for dirs not already watched so re-registration is cheap.
func (m *Manager) refreshWatches(watcher *fsnotify.Watcher) {
	if watcher == nil {
		return
	}
	m.mu.RLock()
	dirs := map[string]bool{}
	for _, gen := range m.gens {
		for _, d := range gen.watchDirs {
			dirs[d] = true
		}
	}
	m.mu.RUnlock()

	m.watchMu.Lock()
	defer m.watchMu.Unlock()
	for d := range dirs {
		if m.watched[d] {
			continue
		}
		if err := watcher.Add(d); err == nil {
			m.watched[d] = true
		}
	}
}

// onFSEvent schedules a debounced re-resolve for every generation whose watched
// directories include the event's directory.
func (m *Manager) onFSEvent(ev fsnotify.Event) {
	dir := filepath.Dir(ev.Name)
	m.mu.RLock()
	var keys []genKey
	for k, gen := range m.gens {
		if gen.watchSet[dir] || gen.watchSet[ev.Name] {
			keys = append(keys, k)
		}
	}
	m.mu.RUnlock()
	for _, k := range keys {
		m.scheduleRefresh(k)
	}
}

func (m *Manager) scheduleRefresh(k genKey) {
	m.watchMu.Lock()
	defer m.watchMu.Unlock()
	if t := m.debounces[k]; t != nil {
		t.Stop()
	}
	m.debounces[k] = time.AfterFunc(watchDebounce, func() {
		m.refreshOne(context.Background(), k)
	})
}

func (m *Manager) stopDebounces() {
	m.watchMu.Lock()
	defer m.watchMu.Unlock()
	for k, t := range m.debounces {
		t.Stop()
		delete(m.debounces, k)
	}
}

// Sweep re-resolves every live generation. Used by the periodic ticker and by
// tests that want a deterministic freshness pass.
func (m *Manager) Sweep(ctx context.Context) {
	m.mu.RLock()
	keys := make([]genKey, 0, len(m.gens))
	for k := range m.gens {
		keys = append(keys, k)
	}
	m.mu.RUnlock()
	for _, k := range keys {
		m.refreshOne(ctx, k)
	}
}

// refreshOne re-resolves a single generation from its retained project and the
// currently-persisted binding. A failed resolve marks the generation stale
// (display-only) rather than dropping the last-known-good view.
func (m *Manager) refreshOne(ctx context.Context, k genKey) {
	m.mu.RLock()
	gen := m.gens[k]
	m.mu.RUnlock()
	if gen == nil {
		return
	}
	b, err := m.binding(k.backendID)
	if err != nil {
		// Binding removed since it was committed: forget the generation so the
		// watcher stops re-resolving a source the user detached.
		m.forget(k)
		return
	}
	resolver, err := m.resolverFor(b.Provider)
	if err != nil {
		m.markStale(k.backendID, k.projectID, err)
		return
	}
	eff, rep, err := resolveStable(ctx, resolver, b, gen.project)
	if err != nil {
		m.markStale(k.backendID, k.projectID, err)
		return
	}
	m.commit(k.backendID, k.projectID, gen.project, b, eff, rep)
}

func (m *Manager) forget(k genKey) {
	m.mu.Lock()
	delete(m.gens, k)
	m.mu.Unlock()
}
