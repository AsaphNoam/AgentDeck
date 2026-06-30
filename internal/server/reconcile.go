package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

const reconcileInterval = 30 * time.Second

func (s *Server) startReconciliationSweep(ctx context.Context) {
	go func() {
		sessionsDir := filepath.Join(s.configStore.Home(), "sessions")
		_ = os.MkdirAll(sessionsDir, 0o755)
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
		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-watcher.Events:
				if ev.Has(fsnotify.Create) || ev.Has(fsnotify.Write) {
					if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
						_ = watcher.Add(ev.Name)
					}
					s.reconcileSessionsOnce(time.Now().Add(-reconcileInterval))
					s.pruneStaleRunning()
				}
			case err := <-watcher.Errors:
				s.log.Warn("reconcile: watcher error", "err", err)
			case <-ticker.C:
				s.reconcileSessionsOnce(time.Now().Add(-reconcileInterval))
				s.pruneStaleRunning()
			}
		}
	}()
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
		agentID := agentIDFromSessionPath(sessionsDir, path)
		if agentID == "" {
			return nil
		}
		line := lastNonEmptyLine(path)
		if _, ok, err := s.stateMgr.ApplyStaleCorrection(agentID, line, staleBefore); err != nil {
			s.log.Debug("reconcile: correction skipped", "agent", agentID, "err", err)
		} else if ok {
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

func lastNonEmptyLine(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(b), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}
	return ""
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
