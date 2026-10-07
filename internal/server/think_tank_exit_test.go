package server

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/AsaphNoam/Chuck/internal/state"
)

// TT-01: requested exits suppress provider turn_end, so the lifecycle hook
// must retire the running attempt and capture itself, without charging it.
func TestThinkTankRequestedExitSettlesActiveWork(t *testing.T) {
	for _, phase := range []string{"participant", "closing", "judge"} {
		for _, action := range []string{"stop", "archive", "delete"} {
			t.Run(phase+"/"+action, func(t *testing.T) {
				srv, _, ids, hold := thinkTankTestServer(t)
				agents := strings.Split(ids, ",")
				room := ""
				if phase == "judge" {
					d, err := srv.stateStore.CreateThinkTank(state.ThinkTankCreate{
						CommandID: "judge-exit", Goal: "Decide", OriginProject: "tmpproj",
						JudgeConfig: `{"role":"impl","project":"tmpproj","interface":"chat"}`,
						Members: []state.ThinkTankMember{
							{AgentID: agents[0], AgentName: "A", Project: "tmpproj", Cap: 2},
							{AgentID: agents[1], AgentName: "B", Project: "tmpproj", Cap: 2},
						},
					})
					if err != nil {
						t.Fatal(err)
					}
					room = d.Room.RoomID
					if _, err := srv.stateStore.EndThinkTank(room); err != nil {
						t.Fatal(err)
					}
					srv.progressThinkTanks(context.Background())
					srv.progressThinkTanks(context.Background())
				} else {
					room = createTestRoom(t, srv, agents, 2)
					srv.progressThinkTanks(context.Background())
					if phase == "closing" {
						a := waitActiveAttempt(t, srv, room, agents[0])
						actAsAgent(t, srv, a, state.ThinkTankLeave, "")
						d, _ := srv.stateStore.ReadThinkTank(room)
						releaseTurn(t, hold, srv, room, d.Room.Revision)
						srv.progressThinkTanks(context.Background())
					}
				}
				d := waitRoom(t, srv, room, func(d state.ThinkTankDetail) bool { return d.Active != nil })
				a := *d.Active
				wantTurn := map[string]string{"participant": state.ThinkTankTurnDiscussion, "closing": state.ThinkTankTurnClosing, "judge": state.ThinkTankTurnJudge}[phase]
				if a.Turn != wantTurn {
					t.Fatalf("turn = %s, want %s", a.Turn, wantTurn)
				}
				completed := map[string]int{}
				for _, m := range d.Members {
					completed[m.AgentID] = m.Completed
				}
				pathAction := action
				if action == "delete" {
					pathAction = "stop"
				}
				rec := doJSON(t, srv.routes(), http.MethodPost, "/api/sessions/"+a.AgentID+"/"+pathAction, `{}`)
				if rec.Code != http.StatusOK {
					t.Fatalf("%s: %d %s", action, rec.Code, rec.Body.String())
				}
				if action == "delete" {
					if err := srv.stateStore.DeleteAgent(a.AgentID); err != nil {
						t.Fatal(err)
					}
				}
				d = waitRoom(t, srv, room, func(d state.ThinkTankDetail) bool { return d.Active == nil })
				found := false
				for _, attempt := range d.Attempts {
					if attempt.AttemptID == a.AttemptID {
						found = true
						if attempt.State != state.ThinkTankAttemptFailed {
							t.Fatalf("attempt state = %s", attempt.State)
						}
					}
				}
				if !found {
					t.Fatal("attempt disappeared")
				}
				for _, m := range d.Members {
					if m.Completed != completed[m.AgentID] {
						t.Fatalf("exit charged %s", m.AgentID)
					}
				}
				srv.thinkTankCaptureMu.Lock()
				capture := srv.thinkTankCaptures[a.AgentID]
				srv.thinkTankCaptureMu.Unlock()
				if capture != nil {
					t.Fatal("exit left capture active")
				}
				if phase == "judge" {
					if _, err := srv.stateStore.RetryThinkTankJudge(room, ""); err != nil {
						t.Fatalf("judge retry: %v", err)
					}
				} else {
					if _, err := srv.stateStore.ResumeThinkTank(room); err != nil {
						t.Fatalf("explicit retry: %v", err)
					}
					ended, err := srv.stateStore.EndThinkTank(room)
					if err != nil || ended.Room.Phase != state.ThinkTankPhaseEnded {
						t.Fatalf("End after exit: %+v %v", ended.Room, err)
					}
				}
			})
		}
	}
}

func TestThinkTankStaleExitCannotSettleNewGeneration(t *testing.T) {
	srv, h := roomRESTServer(t)
	rec := doJSON(t, h, http.MethodPost, "/api/think-tanks", roomBody("a_one", "a_two"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %s", rec.Body.String())
	}
	rooms, err := srv.stateStore.ListThinkTanks("", "", 10)
	if err != nil || len(rooms) != 1 {
		t.Fatalf("rooms: %+v %v", rooms, err)
	}
	room := rooms[0].RoomID
	a := beginCapturedTurn(t, srv, room, "a_one", "old-turn", t.TempDir())
	srv.interruptThinkTankOnExit(a.AgentID, a.Generation, "requested_stop")
	d, err := srv.stateStore.ResumeThinkTank(room)
	if err != nil {
		t.Fatal(err)
	}
	b, err := srv.stateStore.BeginThinkTankAttempt(state.ThinkTankBegin{RoomID: room, Revision: d.Room.Revision,
		AgentID: "a_one", Turn: state.ThinkTankTurnDiscussion, Generation: "g2", TurnID: "new-turn"})
	if err != nil {
		t.Fatal(err)
	}
	srv.beginThinkTankCapture(b.AgentID, b, "A", "alpha")
	srv.interruptThinkTankOnExit(a.AgentID, a.Generation, "process_exit")
	d, err = srv.stateStore.ReadThinkTank(room)
	if err != nil || d.Active == nil || d.Active.AttemptID != b.AttemptID {
		t.Fatalf("new attempt after stale exit: %+v %v", d.Active, err)
	}
	srv.thinkTankCaptureMu.Lock()
	defer srv.thinkTankCaptureMu.Unlock()
	if capture := srv.thinkTankCaptures[b.AgentID]; capture == nil || capture.generation != "g2" {
		t.Fatal("stale exit removed new capture")
	}
}
