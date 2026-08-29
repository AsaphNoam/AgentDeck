package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/agentdeck/agentdeck/internal/runtime"
)

const reconcileInterval = 30 * time.Second
const reconcileDebounce = 250 * time.Millisecond

func (s *Server) startReconciliationSweep(ctx context.Context) {
	go func() {
		sessionsDir := filepath.Join(s.configStore.Home(), "sessions")
		_ = os.MkdirAll(sessionsDir, 0o700)
		s.reconcileSessionsOnce(time.Now().Add(-reconcileInterval))
		s.pruneStaleRunning()

		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			s.log.Warn("reconcile: fsnotify unavailable", "err", err)
			s.reconcileSessionsTimer(ctx)
			return
		}
		defer watcher.Close()
		if err := watcher.Add(sessionsDir); err != nil {
			s.log.Warn("reconcile: watch sessions", "err", err)
		}

		ticker := time.NewTicker(reconcileInterval)
		defer ticker.Stop()
		var debounce *time.Timer
		var debounceC <-chan time.Time
		pending := make(map[string]struct{})
		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-watcher.Events:
				if ev.Has(fsnotify.Create) || ev.Has(fsnotify.Write) {
					if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
						_ = watcher.Add(ev.Name)
					} else {
						pending[ev.Name] = struct{}{}
					}
					if debounce == nil {
						debounce = time.NewTimer(reconcileDebounce)
					} else {
						if !debounce.Stop() {
							select {
							case <-debounce.C:
							default:
							}
						}
						debounce.Reset(reconcileDebounce)
					}
					debounceC = debounce.C
				}
			case <-debounceC:
				staleBefore := time.Now().Add(-reconcileInterval)
				for path := range pending {
					s.reconcileSessionPath(path, staleBefore)
					delete(pending, path)
				}
				s.pruneStaleRunning()
				debounceC = nil
			case err := <-watcher.Errors:
				s.log.Warn("reconcile: watcher error", "err", err)
			case <-ticker.C:
				s.reconcileSessionsOnce(time.Now().Add(-reconcileInterval))
				s.pruneStaleRunning()
			}
		}
	}()
}

func (s *Server) reconcileSessionPath(path string, staleBefore time.Time) bool {
	sessionsDir := filepath.Join(s.configStore.Home(), "sessions")
	agentID := agentIDFromSessionPath(sessionsDir, path)
	if agentID == "" {
		return false
	}
	preview := lastAssistantPreview(path)
	if _, ok, err := s.stateMgr.ApplyStaleCorrection(agentID, preview, staleBefore); err != nil {
		s.log.Debug("reconcile: correction skipped", "agent", agentID, "err", err)
		return false
	} else {
		return ok
	}
}

func (s *Server) reconcileSessionsTimer(ctx context.Context) {
	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.reconcileSessionsOnce(time.Now().Add(-reconcileInterval))
			s.pruneStaleRunning()
		}
	}
}

func (s *Server) reconcileSessionsOnce(staleBefore time.Time) int {
	sessionsDir := filepath.Join(s.configStore.Home(), "sessions")
	applied := 0
	_ = filepath.WalkDir(sessionsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if s.reconcileSessionPath(path, staleBefore) {
			applied++
		}
		return nil
	})
	return applied
}

func agentIDFromSessionPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return ""
	}
	parts := strings.Split(rel, string(filepath.Separator))
	for _, part := range parts {
		if strings.HasPrefix(part, "a_") {
			id := strings.TrimSuffix(part, filepath.Ext(part))
			if id != "" {
				return id
			}
		}
	}
	return ""
}

// lastAssistantPreview derives a bounded, human-readable status preview from the
// assistant's text in the last turn of the NDJSON transcript. Each line is a
// normalized runtime.Event; assistant_text deltas are concatenated and reset at
// each turn boundary, so the result is the final assistant message (clipped to
// the last detailPreviewLimit runes) rather than a raw event envelope (§6.4/§13
// "last output line"). The tail is kept, not the head, so a card preview shows
// the most recently produced text of a long or still-streaming message — the
// same end the live client-side preview (`ui/src/store/transcriptStore.ts`)
// keeps, so a reload or reconcile sweep never flips which end of the message a
// card shows (INV §2). Returns "" when the transcript carries no assistant text, which
// leaves the existing status detail untouched (a non-text envelope is never a
// meaningful preview and must not clobber a healthy card — the BLOCKING fix).
func lastAssistantPreview(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var completed, current string
	for _, raw := range strings.Split(string(b), "\n") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var ev runtime.Event
		if json.Unmarshal([]byte(raw), &ev) != nil {
			continue
		}
		switch ev.Type {
		case runtime.EvAssistantText:
			var d runtime.AssistantTextData
			if json.Unmarshal(ev.Data, &d) == nil {
				current += d.Delta
			}
		case runtime.EvTurnEnd:
			if strings.TrimSpace(current) != "" {
				completed = current
			}
			current = ""
		}
	}
	preview := current
	if strings.TrimSpace(preview) == "" {
		preview = completed
	}
	return clipPreview(strings.TrimSpace(preview), detailPreviewLimit)
}

// detailPreviewLimit bounds a reconciled status-detail preview (mirrors the
// chat runtime's clip(tail, 120) crash-path convention).
const detailPreviewLimit = 120

// clipPreview keeps at most the last n runes of s, so a status detail written
// from a transcript never grows unbounded, never splits a multi-byte rune
// (which would produce invalid UTF-8 in the status JSON), and shows the most
// recently produced text rather than a fixed prefix that never advances while
// a message is still streaming.
func clipPreview(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[len(r)-n:])
}

func (s *Server) pruneStaleRunning() int {
	rows, err := s.stateStore.ListRunning()
	if err != nil {
		s.log.Debug("liveness: list running", "err", err)
		return 0
	}
	pruned := 0
	for _, row := range rows {
		if pidAlive(row.PID) {
			continue
		}
		if err := s.stateStore.DeleteRunning(row.AgentID); err != nil {
			s.log.Debug("liveness: delete running", "agent", row.AgentID, "err", err)
			continue
		}
		if st, err := s.stateStore.ReadStatus(row.AgentID); err == nil {
			st.State = "done"
			st.Detail = "process exited"
			st.LastTrace = "Stop"
			st.BusySince = nil
			_ = s.stateStore.WriteStatus(st)
		}
		if _, err := s.stateMgr.Touch(row.AgentID); err != nil {
			s.log.Debug("liveness: touch", "agent", row.AgentID, "err", err)
		}
		pruned++
	}
	return pruned
}

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}
