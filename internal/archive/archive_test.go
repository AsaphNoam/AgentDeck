package archive

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentdeck/agentdeck/internal/state"
)

// TestEmptyArchiveMarshalsResultsArray guards the J8/S1 blocker: an empty archive
// must serialize results as [] (not null), or the dashboard crashes on
// results.length/.map. This path (Search with no query -> list) does not need
// FTS5, so the test runs on the shipped no-FTS5 fallback build too.
func TestEmptyArchiveMarshalsResultsArray(t *testing.T) {
	st, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	defer st.Close()

	resp, err := New(st.DB()).Search(Query{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if resp.Results == nil {
		t.Fatal("Results is nil; want non-nil empty slice")
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"results":[]`) {
		t.Fatalf("results did not marshal to []; got %s", b)
	}
}
