package state

import (
	"errors"
	"testing"
)

// TT-05: whichever durable claim wins excludes the other plane. A task
// reservation is already assigned work even before its first provider frame.
func TestThinkTankAndTaskAdmissionExcludeEachOther(t *testing.T) {
	for _, roomFirst := range []bool{false, true} {
		t.Run(map[bool]string{false: "task first", true: "room first"}[roomFirst], func(t *testing.T) {
			st, _ := newTestStore(t)
			d := ttCreate(t, st, false, ttMember("a", 2), ttMember("b", 2))
			task := newTask(t, st, "proj-a", "assigned work")
			begin := func() (ThinkTankAttempt, error) {
				return st.BeginThinkTankAttempt(ThinkTankBegin{RoomID: d.Room.RoomID, Revision: d.Room.Revision,
					AgentID: "a", Turn: ThinkTankTurnDiscussion, Generation: "g", TurnID: "t"})
			}
			reserve := func() bool {
				_, ok, err := st.AdmitReadyTask(task.TaskID, TaskReservation{AgentID: "a", Generation: "g", AttemptID: "start", Claim: ClaimBorrowed}, 10)
				if err != nil {
					t.Fatal(err)
				}
				return ok
			}
			if roomFirst {
				if _, err := begin(); err != nil {
					t.Fatal(err)
				}
				if reserve() {
					t.Fatal("task claimed an active room actor")
				}
			} else {
				if !reserve() {
					t.Fatal("task not reserved")
				}
				if _, err := begin(); !errors.Is(err, ErrThinkTankConflict) {
					t.Fatalf("room admission = %v", err)
				}
				if got := mustTT(t, st, d.Room.RoomID); got.Active != nil {
					t.Fatal("refused room created attempt")
				}
			}
		})
	}
}

// TT-04: the in-flight launch claim survives End, prevents deletion, and
// revokes every later slot even when a worker retains a stale setup snapshot.
func TestThinkTankSetupClaimsFenceEndAndDeletion(t *testing.T) {
	st, _ := newTestStore(t)
	a, b := ttMember("a", 1), ttMember("b", 1)
	a.SetupConfig, b.SetupConfig = `{"project":"p"}`, `{"project":"p"}`
	d := ttCreate(t, st, false, a, b)
	room := d.Room.RoomID
	if _, err := st.ClaimThinkTankMemberSetup(room, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.EndThinkTank(room); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteThinkTank(room); !errors.Is(err, ErrThinkTankConflict) {
		t.Fatalf("delete during launch = %v", err)
	}
	if _, err := st.ClaimThinkTankMemberSetup(room, "b"); !errors.Is(err, ErrThinkTankConflict) {
		t.Fatalf("later launch = %v", err)
	}
	if _, err := st.MarkThinkTankMemberSetup(room, "a", "Created ordinary agent", ""); err != nil {
		t.Fatal(err)
	}
	if got := mustTT(t, st, room).Members[1].SetupState; got != ThinkTankSetupAbandoned {
		t.Fatalf("unstarted slot = %s", got)
	}
	if err := st.DeleteThinkTank(room); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimThinkTankMemberSetup(room, "b"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted launch = %v", err)
	}
}

func TestThinkTankSetupClaimFencedOnRestart(t *testing.T) {
	st, _ := newTestStore(t)
	a := ttMember("a", 1)
	a.SetupConfig = `{"project":"p"}`
	d := ttCreate(t, st, false, a, ttMember("b", 1))
	if _, err := st.ClaimThinkTankMemberSetup(d.Room.RoomID, "a"); err != nil {
		t.Fatal(err)
	}
	if err := st.RecoverThinkTanks(); err != nil {
		t.Fatal(err)
	}
	d = mustTT(t, st, d.Room.RoomID)
	if d.Members[0].SetupState != ThinkTankSetupFailed || d.Room.Control != ThinkTankPaused {
		t.Fatalf("recovery = %+v", d)
	}
	if err := st.DeleteThinkTank(d.Room.RoomID); err != nil {
		t.Fatal(err)
	}
}
