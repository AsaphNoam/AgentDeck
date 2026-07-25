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
	out := make([]runtime.Event, 0)
	if err := r.ForEach(opts, func(ev runtime.Event) error {
		out = append(out, ev)
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// ForEach streams transcript events without retaining the complete session in
// memory. The same parser and filtering rules back ReadAll and index replays.
func (r *Reader) ForEach(opts ReadOptions, visit func(runtime.Event) error) error {
	f, err := os.Open(r.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("transcript: open read: %w", err)
	}
	defer f.Close()
	return forEach(f, opts, visit)
}

func readAll(rd io.Reader, opts ReadOptions) ([]runtime.Event, error) {
	br := bufio.NewReaderSize(rd, 64*1024)
	out := make([]runtime.Event, 0)
	err := forEachBuffered(br, opts, func(ev runtime.Event) error {
		out = append(out, ev)
		return nil
	})
	return out, err
}

func forEach(rd io.Reader, opts ReadOptions, visit func(runtime.Event) error) error {
	return forEachBuffered(bufio.NewReaderSize(rd, 64*1024), opts, visit)
}

func forEachBuffered(br *bufio.Reader, opts ReadOptions, visit func(runtime.Event) error) error {
	for {
		line, oversized, err := readLine(br)
		if !oversized && len(line) > 0 {
			var ev runtime.Event
			if json.Unmarshal(line, &ev) == nil {
				keep := true
				if !opts.IncludeMeta && (ev.Seq == 0 || ev.Type == runtime.EvSessionMeta) {
					keep = false
				}
				if opts.SinceSeq > 0 && ev.Seq <= opts.SinceSeq {
					keep = false
				}
				if keep {
					if visitErr := visit(ev); visitErr != nil {
						return fmt.Errorf("transcript: consume event seq %d: %w", ev.Seq, visitErr)
					}
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("transcript: scan: %w", err)
		}
	}
	return nil
}

// readLine reads one newline-terminated record from br. A record whose length
// exceeds maxRecordSize is skipped: oversized is set true, its bytes are
// discarded, but the stream is still consumed up to the next newline so the
// reader stays aligned to the following record. The returned line has any
// trailing '\n' stripped. err is nil for a normal line, io.EOF at end of
// stream (possibly with a final unterminated fragment), or a real read error.
func readLine(br *bufio.Reader) (line []byte, oversized bool, err error) {
	var buf []byte
	for {
		frag, e := br.ReadSlice('\n')
		if !oversized {
			if len(buf)+len(frag) > maxRecordSize {
				oversized = true
				buf = nil
			} else {
				buf = append(buf, frag...)
			}
		}
		if e == bufio.ErrBufferFull {
			continue
		}
		if e != nil {
			// io.EOF or a real read error terminates this record.
			return trimNewline(buf), oversized, e
		}
		return trimNewline(buf), oversized, nil
	}
}

func trimNewline(b []byte) []byte {
	if n := len(b); n > 0 && b[n-1] == '\n' {
		return b[:n-1]
	}
	return b
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
	err = forEach(f, ReadOptions{IncludeMeta: true}, func(ev runtime.Event) error {
		if ev.Seq > maxSeq {
			maxSeq = ev.Seq
		}
		return nil
	})
	if err != nil {
		return 0, true, err
	}
	return maxSeq, true, nil
}
