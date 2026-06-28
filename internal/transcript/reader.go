package transcript

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/agentdeck/agentdeck/internal/runtime"
)

const maxRecordSize = 8 * 1024 * 1024

// ReadOptions controls transcript replay.
type ReadOptions struct {
	SinceSeq    int64
	IncludeMeta bool
}

// Reader replays one transcript.ndjson file.
type Reader struct {
	path string
}

func NewReader(path string) *Reader {
	return &Reader{path: path}
}

func ReadFile(home, agentID string, opts ReadOptions) ([]runtime.Event, error) {
	return NewReader(transcriptPath(home, agentID)).ReadAll(opts)
}

func (r *Reader) ReadAll(opts ReadOptions) ([]runtime.Event, error) {
	f, err := os.Open(r.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []runtime.Event{}, nil
		}
		return nil, fmt.Errorf("transcript: open read: %w", err)
	}
	defer f.Close()
	return readAll(f, opts)
}

func readAll(rd io.Reader, opts ReadOptions) ([]runtime.Event, error) {
	sc := bufio.NewScanner(rd)
	sc.Buffer(make([]byte, 64*1024), maxRecordSize)
	var out []runtime.Event
	for sc.Scan() {
		line := append([]byte(nil), sc.Bytes()...)
		if len(line) == 0 {
			continue
		}
		var ev runtime.Event
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		if !opts.IncludeMeta && (ev.Seq == 0 || ev.Type == runtime.EvSessionMeta) {
			continue
		}
		if opts.SinceSeq > 0 && ev.Seq <= opts.SinceSeq {
			continue
		}
		out = append(out, ev)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("transcript: scan: %w", err)
	}
	return out, nil
}

func recoverMaxSeq(path string) (maxSeq int64, existed bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("transcript: open recover: %w", err)
	}
	defer f.Close()
	events, err := readAll(f, ReadOptions{IncludeMeta: true})
	if err != nil {
		return 0, true, err
	}
	for _, ev := range events {
		if ev.Seq > maxSeq {
			maxSeq = ev.Seq
		}
	}
	return maxSeq, true, nil
}
