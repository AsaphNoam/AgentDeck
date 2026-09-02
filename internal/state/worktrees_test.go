package state

import (
	"errors"
	"strings"
	"testing"
)

// TS-12.R2/§3: an ownership row round-trips, and its absence is the ordinary
// answer for an external checkout rather than an error condition.
func TestProjectWorktreeRoundTrip(t *testing.T) {
	st, _ := newTestStore(t)

	if _, err := st.ReadProjectWorktree("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadProjectWorktree(absent) = %v, want ErrNotFound", err)
	}

	want := ProjectWorktree{
		Project:      "fork-a",
		RepoPath:     "/repo",
		Branch:       "agentdeck/fork-a",
		CheckoutPath: "/home/worktrees/fork-a",
	}
	if err := st.InsertProjectWorktree(want); err != nil {
		t.Fatalf("InsertProjectWorktree: %v", err)
	}
	got, err := st.ReadProjectWorktree("fork-a")
	if err != nil {
		t.Fatalf("ReadProjectWorktree: %v", err)
	}
	if got.RepoPath != want.RepoPath || got.Branch != want.Branch || got.CheckoutPath != want.CheckoutPath {
		t.Fatalf("row = %+v, want %+v", got, want)
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("created_at was not stamped")
	}
	if got.SetupOK != nil {
		t.Fatalf("setup_ok = %v, want NULL until the first setup run", *got.SetupOK)
	}

	list, err := st.ListProjectWorktrees()
	if err != nil {
		t.Fatalf("ListProjectWorktrees: %v", err)
	}
	if len(list) != 1 || list["fork-a"].Branch != want.Branch {
		t.Fatalf("list = %+v", list)
	}

	if err := st.DeleteProjectWorktree("fork-a"); err != nil {
		t.Fatalf("DeleteProjectWorktree: %v", err)
	}
	if _, err := st.ReadProjectWorktree("fork-a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("read after delete = %v, want ErrNotFound", err)
	}
	// A missing row is tolerated: deletion runs after the checkout is gone and
	// may be retried (TS-12.R7).
	if err := st.DeleteProjectWorktree("fork-a"); err != nil {
		t.Fatalf("DeleteProjectWorktree(absent): %v", err)
	}
}

// The second insert for one project must conflict rather than re-point an
// existing ownership record at a different checkout.
func TestInsertProjectWorktreeRefusesDuplicate(t *testing.T) {
	st, _ := newTestStore(t)
	row := ProjectWorktree{Project: "fork-a", RepoPath: "/repo", Branch: "b", CheckoutPath: "/one"}
	if err := st.InsertProjectWorktree(row); err != nil {
		t.Fatalf("InsertProjectWorktree: %v", err)
	}
	row.CheckoutPath = "/two"
	if err := st.InsertProjectWorktree(row); err == nil {
		t.Fatal("duplicate insert succeeded")
	}
}

// TS-12.R5: the setup result and its bounded output tail survive on the row, so
// the warning outlives the request that produced it.
func TestRecordProjectWorktreeSetup(t *testing.T) {
	st, _ := newTestStore(t)
	if err := st.InsertProjectWorktree(ProjectWorktree{
		Project: "fork-a", RepoPath: "/repo", Branch: "b", CheckoutPath: "/c",
	}); err != nil {
		t.Fatalf("InsertProjectWorktree: %v", err)
	}

	huge := strings.Repeat("x", setupOutputLimit+500) + "TAIL"
	if err := st.RecordProjectWorktreeSetup("fork-a", false, huge); err != nil {
		t.Fatalf("RecordProjectWorktreeSetup: %v", err)
	}
	got, err := st.ReadProjectWorktree("fork-a")
	if err != nil {
		t.Fatalf("ReadProjectWorktree: %v", err)
	}
	if got.SetupOK == nil || *got.SetupOK {
		t.Fatalf("setup_ok = %v, want false", got.SetupOK)
	}
	if got.SetupAt == nil {
		t.Fatal("setup_at was not stamped")
	}
	if len(got.SetupOutput) != setupOutputLimit {
		t.Fatalf("output length = %d, want %d", len(got.SetupOutput), setupOutputLimit)
	}
	// The tail is kept, not the head: the failure reason is at the end.
	if !strings.HasSuffix(got.SetupOutput, "TAIL") {
		t.Fatal("stored output dropped the tail")
	}

	if err := st.RecordProjectWorktreeSetup("absent", true, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RecordProjectWorktreeSetup(absent) = %v, want ErrNotFound", err)
	}
}
