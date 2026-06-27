package runtime

import (
	"log/slog"
	"syscall"

	"github.com/agentdeck/agentdeck/internal/state"
)

// ReconcileStale is called on server start. It scans the running rows for stale
// entries (a pid that is no longer alive) — left behind by a crashed prior
// server run — deletes those running rows, and marks their status rows `error`
// so Phase 2 doesn't show ghost cards (techspec §8.5). Full session resume is
// Phase 4; this is cleanup only.
func ReconcileStale(s *state.Store) error {
	running, err := s.ListRunning()
	if err != nil {
		return err
	}
	for _, r := range running {
		if pidAlive(r.PID) {
			continue
		}
		slog.Info("runtime: reconciling stale running row", "agent", r.AgentID, "pid", r.PID)
		if err := s.DeleteRunning(r.AgentID); err != nil {
			return err
		}
		st, err := s.ReadStatus(r.AgentID)
		if err != nil {
			st = state.Status{AgentID: r.AgentID}
		}
		st.State = "error"
		st.Detail = "stale session reconciled on startup"
		st.LastTrace = "Error"
		st.BusySince = nil
		if err := s.WriteStatus(st); err != nil {
			return err
		}
	}
	return nil
}

// pidAlive reports whether a process group / pid is still alive. signal 0 does no
// signalling but performs the existence + permission check.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}
