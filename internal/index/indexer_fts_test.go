//go:build sqlite_fts5

package index

import "testing"

func TestIndexerFTSMatch(t *testing.T) {
	st, _ := openTestDB(t)
	indexFixture(t, st.DB())

	var agentID string
	if err := st.DB().QueryRow(`SELECT agent_id FROM sessions_fts WHERE sessions_fts MATCH ?`, `"distinctive quartz"`).Scan(&agentID); err != nil {
		t.Fatalf("fts search: %v", err)
	}
	if agentID != "a_index" {
		t.Fatalf("fts agent = %q, want a_index", agentID)
	}
}
