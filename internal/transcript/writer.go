package transcript

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/agentdeck/agentdeck/internal/runtime"
)

const (
	dirSessions = "sessions"
	fileLog     = "transcript.ndjson"
)

// Writer appends normalized runtime events to one agent's durable transcript log.
type Writer struct {
	f       *os.File
	path    string
	agentID string
	nextSeq int64
	mu      sync.Mutex
}

// Open opens sessions/{agentID}/transcript.ndjson under home in append mode.
// If the log is new and meta is non-nil, it writes the seq:0 session_meta record.
func Open(home, agentID string, meta *runtime.SessionMetaData) (*Writer, error) {
	if agentID == "" {
		return nil, fmt.Errorf("transcript: agent id is required")
	}
	dir := filepath.Join(home, dirSessions, agentID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("transcript: mkdir: %w", err)
	}
	path := filepath.Join(dir, fileLog)
	maxSeq, existed, err := recoverMaxSeq(path)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("transcript: open: %w", err)
	}
	w := &Writer{f: f, path: path, agentID: agentID, nextSeq: maxSeq + 1}
	if !existed && meta != nil {
		if err := w.appendMeta(*meta); err != nil {
			_ = f.Close()
			return nil, err
		}
	}
	return w, nil
}

func (w *Writer) appendMeta(meta runtime.SessionMetaData) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("transcript: marshal session_meta: %w", err)
	}
	ev := runtime.Event{
		AgentID: w.agentID,
		Seq:     0,
		Type:    runtime.EvSessionMeta,
		Data:    data,
		Ts:      time.Now().UTC().Format(time.RFC3339),
	}
	return w.appendLocked(ev)
}

// Path returns the transcript log path.
func (w *Writer) Path() string { return w.path }

// NextSeq returns the next event sequence discovered from the existing log.
func (w *Writer) NextSeq() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.nextSeq
}

// Append writes one complete JSON record plus its trailing newline with one Write.
func (w *Writer) Append(ev runtime.Event) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if ev.AgentID == "" {
		ev.AgentID = w.agentID
	}
	if ev.AgentID != w.agentID {
		return fmt.Errorf("transcript: event agent_id %q does not match writer %q", ev.AgentID, w.agentID)
	}
	if ev.Seq == 0 && ev.Type != runtime.EvSessionMeta {
		ev.Seq = w.nextSeq
	}
	if ev.Ts == "" {
		ev.Ts = time.Now().UTC().Format(time.RFC3339)
	}
	if err := w.appendLocked(ev); err != nil {
		return err
	}
	if ev.Seq >= w.nextSeq {
		w.nextSeq = ev.Seq + 1
	}
	if ev.Type == runtime.EvTurnEnd || ev.Type == runtime.EvError {
		return w.syncLocked()
	}
	return nil
}

func (w *Writer) appendLocked(ev runtime.Event) error {
	if w.f == nil {
		return fmt.Errorf("transcript: writer is closed")
	}
	b, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("transcript: marshal event: %w", err)
	}
	b = append(b, '\n')
	if _, err := w.f.Write(b); err != nil {
		return fmt.Errorf("transcript: append: %w", err)
	}
	return nil
}

// Sync flushes the log to disk.
func (w *Writer) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.syncLocked()
}

func (w *Writer) syncLocked() error {
	if w.f == nil {
		return nil
	}
	if err := w.f.Sync(); err != nil {
		return fmt.Errorf("transcript: sync: %w", err)
	}
	return nil
}

// Close syncs and closes the log.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	errSync := w.syncLocked()
	errClose := w.f.Close()
	w.f = nil
	if errSync != nil {
		return fmt.Errorf("transcript: sync close: %w", errSync)
	}
	if errClose != nil {
		return fmt.Errorf("transcript: close: %w", errClose)
	}
	return nil
}

func transcriptPath(home, agentID string) string {
	return filepath.Join(home, dirSessions, agentID, fileLog)
}
