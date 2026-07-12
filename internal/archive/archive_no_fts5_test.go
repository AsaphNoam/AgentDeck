//go:build !sqlite_fts5

package archive

import (
	"testing"

	"github.com/agentdeck/agentdeck/internal/state"
)

// TestSearchFallbackFiltersMetadata guards usability J8: when FTS5 is not available,
// search must use a fallback that filters by metadata fields and returns clear results
// (not raw errors or stale rows). This test runs only on the untagged (non-FTS5) build
// because with FTS5, the FTS5 search path is used instead of the fallback.
func TestSearchFallbackFiltersMetadata(t *testing.T) {
	st, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	defer st.Close()

	db := st.DB()
	// Insert two sessions with distinct names/roles for filtering
	if _, err := db.Exec(`
INSERT INTO sessions(agent_id, name, role, project, backend, model, interface, cwd, system_prompt, created_at, updated_at, grp)
VALUES ('a1', 'Alpha Agent', 'reviewer', 'project-alpha', 'claude', 'sonnet', 'chat', '/tmp/a1', '', '2026-07-01T10:00:00Z', '2026-07-01T11:00:00Z', '')
`); err != nil {
		t.Fatalf("insert session for a1: %v", err)
	}

	if _, err := db.Exec(`
INSERT INTO sessions(agent_id, name, role, project, backend, model, interface, cwd, system_prompt, created_at, updated_at, grp)
VALUES ('a2', 'Beta Agent', 'implementer', 'project-beta', 'claude', 'sonnet', 'chat', '/tmp/a2', '', '2026-07-01T10:00:00Z', '2026-07-01T12:00:00Z', '')
`); err != nil {
		t.Fatalf("insert session for a2: %v", err)
	}

	archive := New(db)

	// Search for "reviewer" (should match only the first session)
	resp, err := archive.Search(Query{Q: "reviewer", Limit: 50})
	if err != nil {
		t.Fatalf("Search for 'reviewer': %v", err)
	}

	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result for 'reviewer', got %d", len(resp.Results))
	}
	if resp.Results[0].AgentID != "a1" {
		t.Fatalf("expected agent_id='a1', got '%s'", resp.Results[0].AgentID)
	}

	// Search for "project-beta" (should match only the second session)
	resp, err = archive.Search(Query{Q: "project-beta", Limit: 50})
	if err != nil {
		t.Fatalf("Search for 'project-beta': %v", err)
	}

	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result for 'project-beta', got %d", len(resp.Results))
	}
	if resp.Results[0].AgentID != "a2" {
		t.Fatalf("expected agent_id='a2', got '%s'", resp.Results[0].AgentID)
	}

	// Search for "Alpha" (should match the first session's name)
	resp, err = archive.Search(Query{Q: "Alpha", Limit: 50})
	if err != nil {
		t.Fatalf("Search for 'Alpha': %v", err)
	}

	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result for 'Alpha', got %d", len(resp.Results))
	}
	if resp.Results[0].AgentID != "a1" {
		t.Fatalf("expected agent_id='a1', got '%s'", resp.Results[0].AgentID)
	}
}
