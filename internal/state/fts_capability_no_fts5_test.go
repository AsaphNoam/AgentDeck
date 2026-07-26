//go:build !sqlite_fts5

package state

import "testing"

func TestDetectSessionsFTSFallbackBuild(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	mode, err := DetectSessionsFTS(st.DB())
	if err != nil || mode != SessionsFTSFallback {
		t.Fatalf("mode = %q err = %v", mode, err)
	}
}
