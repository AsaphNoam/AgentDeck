package state

import (
	"errors"
	"testing"
)

func TestClassifySessionsFTSMissingModule(t *testing.T) {
	mode, err := classifySessionsFTS("CREATE VIRTUAL TABLE sessions_fts USING fts5(content)", errors.New("no such module: fts5"))
	if err != nil || mode != SessionsFTSUnavailable {
		t.Fatalf("mode = %q err = %v", mode, err)
	}
}
