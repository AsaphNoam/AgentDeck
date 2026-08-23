package contextref

import (
	"encoding/base64"
	"strings"
	"unicode/utf8"
)

// pageWriter turns a deterministic text stream into one bounded page. The
// renderer always writes the whole source from the beginning; the writer
// discards everything before the cursor offset and stops collecting once the
// page is full, so memory stays bounded by MaxPageBytes no matter how long the
// underlying transcript is.
type pageWriter struct {
	skip     int
	limit    int
	buf      strings.Builder
	dropped  int
	overflow bool
	total    int  // bytes the source produced, used to bound a supplied offset
	split    bool // the supplied offset landed inside a multibyte rune
}

func newPageWriter(offset, limit int) *pageWriter {
	return &pageWriter{skip: offset, limit: limit}
}

func (w *pageWriter) write(s string) {
	if s == "" {
		return
	}
	w.total += len(s)
	if w.dropped < w.skip {
		need := w.skip - w.dropped
		if len(s) <= need {
			w.dropped += len(s)
			return
		}
		w.dropped = w.skip
		if !utf8.RuneStart(s[need]) {
			// Issued offsets are always rune boundaries, so this one was altered.
			w.split = true
		}
		s = s[need:]
	}
	room := w.limit - w.buf.Len()
	if room <= 0 {
		w.overflow = true
		return
	}
	if len(s) <= room {
		w.buf.WriteString(s)
		return
	}
	// Cut back to the last rune boundary that fits, so every page is valid UTF-8
	// and the next cursor lands on a boundary too.
	cut := room
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	w.buf.WriteString(s[:cut])
	w.overflow = true
}

// validOffset reports whether the offset the caller supplied actually names a
// position this source has: inside it, and on a rune boundary. Cursors are
// opaque and every issued one has content after it, so an offset that splits a
// rune or reaches past the end was forged or corrupted and must fail rather
// than return replacement characters or a misleading empty completion
// (FS-15.R9/R11, TS-04.R28, TS-05.R16, INV §8/§11).
func (w *pageWriter) validOffset() bool {
	if w.skip == 0 {
		return true
	}
	return !w.split && w.skip < w.total
}

// page returns the collected text, the absolute offset to resume from, and
// whether the source was exhausted.
func (w *pageWriter) page() (text string, nextOffset int, complete bool) {
	text = w.buf.String()
	return text, w.skip + len(text), !w.overflow
}

// encodeCursor binds a resume offset to the reference it belongs to. A cursor
// confers no authority and embeds no source content: it is validated against
// the separately supplied reference id only after authorization succeeds
// (TS-05.R16).
func encodeCursor(refID string, offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(refID + "\x00" + itoa(offset)))
}

func decodeCursor(refID, cursor string) (int, *Error) {
	if cursor == "" {
		return 0, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, failf(CodeInvalidCursor, "Cursor is malformed.")
	}
	boundRef, rest, ok := strings.Cut(string(raw), "\x00")
	if !ok || boundRef != refID {
		return 0, failf(CodeInvalidCursor, "Cursor does not belong to this context reference.")
	}
	offset, ok := atoi(rest)
	if !ok || offset < 0 {
		return 0, failf(CodeInvalidCursor, "Cursor is malformed.")
	}
	return offset, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func atoi(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
		n = n*10 + int(s[i]-'0')
		if n > 1<<40 {
			return 0, false
		}
	}
	return n, true
}

func encodeOpaque(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

func decodeOpaque(s string) (string, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return "", false
	}
	return string(raw), true
}
