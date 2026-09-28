package server

import (
	"context"
	goruntime "runtime"
	"time"
)

// keepAwakeRecheck bounds how long a missed event can leave the assertion
// stale; bus events usually trigger the recheck sooner.
const (
	keepAwakeRecheck  = 15 * time.Second
	keepAwakeDebounce = 500 * time.Millisecond
)

// workActive is the keep-awake predicate: any agent busy, any pending
// permission request, or any pipeline run queued or running (TS-13.R13).
func (s *Server) workActive() bool {
	for _, a := range s.eventBus.Snapshot() {
		if !a.Removed && !a.Archived && a.State == "busy" {
			return true
		}
	}
	s.permissionMu.Lock()
	pending := len(s.permissionTools) > 0
	s.permissionMu.Unlock()
	if pending {
		return true
	}
	runs, err := s.stateStore.ListActivePipelineRuns()
	if err != nil {
		s.log.Warn("keep-awake: list runs", "err", err)
		return false
	}
	for _, r := range runs {
		if r.State == "queued" || r.State == "running" {
			return true
		}
	}
	return false
}

// nudgeKeepAwake asks the owner loop to re-evaluate soon (setting changed).
func (s *Server) nudgeKeepAwake() {
	select {
	case s.keepAwakeNudge <- struct{}{}:
	default:
	}
}

// runKeepAwake is the one owner of the sleep assertion. It re-evaluates on bus
// activity (debounced), on nudges, and periodically, and releases the
// assertion when ctx ends (FS-20.R22).
func (s *Server) runKeepAwake(ctx context.Context) {
	defer s.keepAwake.Close()
	if goruntime.GOOS != "darwin" {
		return
	}
	events, unsub := s.eventBus.Subscribe()
	defer unsub()
	ticker := time.NewTicker(keepAwakeRecheck)
	defer ticker.Stop()
	var debounce <-chan time.Time
	evaluate := func() {
		want := false
		if cfg, err := s.configStore.ReadConfig(); err == nil && cfg.KeepAwake {
			want = s.workActive()
		}
		if err := s.keepAwake.Set(want); err != nil {
			s.log.Warn("keep-awake: take assertion", "err", err)
		}
	}
	evaluate()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-events:
			if !ok {
				return
			}
			if debounce == nil {
				debounce = time.After(keepAwakeDebounce)
			}
		case <-debounce:
			debounce = nil
			evaluate()
		case <-s.keepAwakeNudge:
			evaluate()
		case <-ticker.C:
			evaluate()
		}
	}
}
