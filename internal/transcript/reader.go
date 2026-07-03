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
	br := bufio.NewReaderSize(rd, 64*1024)
	var out []runtime.Event
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
					out = append(out, ev)
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("transcript: scan: %w", err)
		}
	}
	return out, nil
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
