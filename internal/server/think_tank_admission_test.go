package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AsaphNoam/Chuck/internal/state"
)

// TT-06: retained agent identity does not make a missing or unreadable project
// eligible. The room gives recovery/End guidance without a provider prompt.
func TestThinkTankMissingAndUnreadableProjectHold(t *testing.T) {
	for _, mode := range []string{"missing", "corrupt", "read error"} {
		t.Run(mode, func(t *testing.T) {
			srv, promptLog, ids, _ := thinkTankTestServer(t)
			room := createTestRoom(t, srv, strings.Split(ids, ","), 2)
			path := filepath.Join(srv.configStore.Home(), "projects", "tmpproj.json")
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if mode == "corrupt" {
				if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
					t.Fatal(err)
				}
			} else if mode == "read error" {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			srv.progressThinkTanks(context.Background())
			d, err := srv.stateStore.ReadThinkTank(room)
			if err != nil {
				t.Fatal(err)
			}
			if d.Active != nil || !strings.Contains(d.Room.Hold, "project is missing or unreadable") || promptCount(t, promptLog) != 0 {
				t.Fatalf("eligibility = %+v, prompts=%d", d, promptCount(t, promptLog))
			}
			if ended, err := srv.stateStore.EndThinkTank(room); err != nil || ended.Room.Phase != state.ThinkTankPhaseEnded {
				t.Fatalf("End = %+v %v", ended.Room, err)
			}
		})
	}
}

// TT-05: act on an opportunity selected before archive/task eligibility changed.
// Final admission must refuse it and leave both durable rows and wire untouched.
func TestThinkTankFinalAdmissionRechecksEligibility(t *testing.T) {
	for _, mode := range []string{"agent archive", "project archive", "task reservation"} {
		t.Run(mode, func(t *testing.T) {
			srv, promptLog, ids, _ := thinkTankTestServer(t)
			agents := strings.Split(ids, ",")
			room := createTestRoom(t, srv, agents, 2)
			d, _ := srv.stateStore.ReadThinkTank(room)
			next, ok := state.NextThinkTankOpportunity(d)
			if !ok || srv.thinkTankIneligible(next.AgentID) != "" {
				t.Fatal("initial opportunity ineligible")
			}
			switch mode {
			case "agent archive":
				if err := srv.stateStore.SetAgentsArchived([]string{next.AgentID}, true); err != nil {
					t.Fatal(err)
				}
			case "project archive":
				p, _ := srv.configStore.ReadProject("tmpproj")
				p.Archived = true
				if err := srv.configStore.WriteProject("tmpproj", p); err != nil {
					t.Fatal(err)
				}
			case "task reservation":
				task, err := srv.stateStore.CreateTask(state.Task{TaskID: "tk_reserved", Project: "tmpproj", DisplayName: "Assigned", Instruction: "work", TargetKind: state.TargetAgent, TargetAgentID: next.AgentID, CreatedByKind: "person"})
				if err != nil {
					t.Fatal(err)
				}
				if _, ok, err := srv.stateStore.AdmitReadyTask(task.TaskID, state.TaskReservation{AgentID: next.AgentID, Generation: srv.registry.Generation(next.AgentID), AttemptID: "start", Claim: state.ClaimBorrowed}, 10); err != nil || !ok {
					t.Fatalf("reserve = %v %v", ok, err)
				}
			}
			if srv.startThinkTankTurn(context.Background(), d, next) {
				t.Fatal("stale opportunity started")
			}
			got, _ := srv.stateStore.ReadThinkTank(room)
			if got.Active != nil || promptCount(t, promptLog) != 0 {
				t.Fatalf("admitted: %+v, prompts=%d", got, promptCount(t, promptLog))
			}
		})
	}
}

// TT-04: End during slow setup retains only the already claimed ordinary launch;
// deletion waits for that claim and the stale worker cannot launch later slots.
func TestThinkTankSlowSetupEndDeletion(t *testing.T) {
	srv, _, ids, _ := thinkTankTestServer(t)
	existing := strings.Split(ids, ",")[0]
	newDump, release := filepath.Join(t.TempDir(), "new.json"), filepath.Join(t.TempDir(), "release")
	t.Setenv("FAKEACP_NEW_DUMP", newDump)
	t.Setenv("FAKEACP_NEW_HOLD_FILE", release)
	config := `{"role":"impl","project":"tmpproj","interface":"chat"}`
	d, err := srv.stateStore.CreateThinkTank(state.ThinkTankCreate{CommandID: "setup", Goal: "Decide", OriginProject: "tmpproj", Members: []state.ThinkTankMember{
		{AgentID: existing, AgentName: "Existing", Project: "tmpproj", Cap: 1},
		{AgentID: "a_setup1", AgentName: "First", Project: "tmpproj", Cap: 1, SetupConfig: config},
		{AgentID: "a_setup2", AgentName: "Later", Project: "tmpproj", Cap: 1, SetupConfig: config},
	}})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { defer close(done); srv.launchThinkTankSetup(context.Background(), d) }()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(newDump); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("setup never reached provider")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := srv.stateStore.EndThinkTank(d.Room.RoomID); err != nil {
		t.Fatal(err)
	}
	if err := srv.stateStore.DeleteThinkTank(d.Room.RoomID); !errors.Is(err, state.ErrThinkTankConflict) {
		t.Fatalf("delete in-flight launch = %v", err)
	}
	if err := os.WriteFile(release, []byte("go"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("setup launch stuck")
	}
	if _, err := srv.stateStore.ReadAgent("a_setup2"); !errors.Is(err, state.ErrNotFound) {
		t.Fatalf("later provider launched: %v", err)
	}
	if err := srv.stateStore.DeleteThinkTank(d.Room.RoomID); err != nil {
		t.Fatal(err)
	}
	srv.launchThinkTankSetup(context.Background(), d)
	if _, err := srv.stateStore.ReadAgent("a_setup2"); !errors.Is(err, state.ErrNotFound) {
		t.Fatalf("deleted room launched provider: %v", err)
	}
}
