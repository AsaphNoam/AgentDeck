package transcript

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/agentdeck/agentdeck/internal/runtime"
)

// CloneCompletedPrefix is the sole clone builder (TS-02.R35): the source's
// durable events through its last root turn_end, without the source's session
// metadata or annotation records, which belong to the source alone. The clone's
// own session metadata comes from its normal launch path. A zero boundary means
// the source has no completed turn to fork from.
func CloneCompletedPrefix(home, sourceAgentID string) ([]runtime.Event, int64, error) {
	events, err := ReadFile(home, sourceAgentID, ReadOptions{})
	if err != nil {
		return nil, 0, err
	}
	var boundary int64
	for _, ev := range events {
		if ev.Type == runtime.EvTurnEnd && ev.ActivityID == "" {
			boundary = ev.Seq
		}
	}
	if boundary == 0 {
		return nil, 0, nil
	}
	prefix := make([]runtime.Event, 0, len(events))
	for _, ev := range events {
		if ev.Seq <= 0 || ev.Seq > boundary {
			continue
		}
		if ev.Type == runtime.EvSessionMeta || ev.Type == runtime.EvAnnotation {
			continue
		}
		prefix = append(prefix, ev)
	}
	return prefix, boundary, nil
}

// Discard closes the log and removes it. It is the rollback for a clone whose
// launch fails after its copied prefix was written (TS-02.R35, INV §15).
func (w *Writer) Discard() error {
	errClose := w.Close()
	if err := os.Remove(w.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("transcript: discard: %w", err)
	}
	_ = os.Remove(filepath.Dir(w.path)) // only when empty
	return errClose
}
