package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AsaphNoam/Chuck/internal/config"
	"github.com/AsaphNoam/Chuck/internal/state"
)

func roomRESTServer(t *testing.T) (*Server, http.Handler) {
	t.Helper()
	srv := testServer(t, true)
	for _, p := range []string{"alpha", "beta", "old"} {
		if err := srv.configStore.WriteProject(p, config.Project{Title: p, Cwd: t.TempDir(), Archived: p == "old"}); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	agents := []state.Agent{
		{AgentID: "a_one", Name: "Ari", Role: "impl", Project: "alpha", Backend: "claude", Interface: "chat", CreatedAt: now},
		{AgentID: "a_two", Name: "Bea", Role: "impl", Project: "beta", Backend: "claude", Interface: "chat", CreatedAt: now},
		{AgentID: "a_term", Name: "Term", Role: "impl", Project: "alpha", Backend: "claude", Interface: "terminal", CreatedAt: now},
		{AgentID: "a_arch", Name: "Arc", Role: "impl", Project: "alpha", Backend: "claude", Interface: "chat", CreatedAt: now, Archived: true},
		{AgentID: "a_old", Name: "Olde", Role: "impl", Project: "old", Backend: "claude", Interface: "chat", CreatedAt: now},
	}
	for _, a := range agents {
		if err := srv.stateStore.WriteAgent(a); err != nil {
			t.Fatal(err)
		}
	}
	return srv, srv.routes()
}

func roomBody(participants ...string) string {
	parts := []string{}
	for _, p := range participants {
		parts = append(parts, `{"agent_id":"`+p+`","limit":2}`)
	}
	return `{"command_id":"cmd-1","goal":"Pick a cache","origin_project":"alpha","participants":[` + strings.Join(parts, ",") + `]}`
}

// FS-21.A14, A24: setup validation over REST, mixed projects, defaults, and
// an exact replay returning the original room.
func TestThinkTankCreateOverREST(t *testing.T) {
	_, h := roomRESTServer(t)
	for name, body := range map[string]string{
		"duplicate":    roomBody("a_one", "a_one"),
		"terminal":     roomBody("a_one", "a_term"),
		"archived":     roomBody("a_one", "a_arch"),
		"old project":  roomBody("a_one", "a_old"),
		"insufficient": roomBody("a_one"),
		"missing":      roomBody("a_one", "a_nope"),
	} {
		if rec := doJSON(t, h, http.MethodPost, "/api/think-tanks", body); rec.Code == http.StatusCreated {
			t.Fatalf("%s: created %s", name, rec.Body.String())
		}
	}
	if rec := doJSON(t, h, http.MethodPost, "/api/think-tanks", strings.Replace(roomBody("a_one", "a_two"), `"alpha"`, `"old"`, 1)); rec.Code == http.StatusCreated {
		t.Fatal("archived origin project accepted")
	}
	rec := doJSON(t, h, http.MethodPost, "/api/think-tanks", roomBody("a_one", "a_two"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", rec.Code, rec.Body.String())
	}
	var room thinkTankDetailWire
	if err := json.Unmarshal(rec.Body.Bytes(), &room); err != nil {
		t.Fatal(err)
	}
	if room.Phase != state.ThinkTankPhaseDiscussion || room.Openings || room.Judge.Enabled || len(room.Members) != 2 ||
		!room.Members[0].MayLeave || room.Members[1].Project != "beta" {
		t.Fatalf("defaults = %+v", room)
	}
	again := doJSON(t, h, http.MethodPost, "/api/think-tanks", roomBody("a_one", "a_two"))
	var replay thinkTankDetailWire
	_ = json.Unmarshal(again.Body.Bytes(), &replay)
	if again.Code != http.StatusCreated || replay.RoomID != room.RoomID {
		t.Fatalf("replay = %d %s", again.Code, replay.RoomID)
	}
	list := doJSON(t, h, http.MethodGet, "/api/think-tanks?project=alpha", "")
	if !strings.Contains(list.Body.String(), room.RoomID) {
		t.Fatalf("project list = %s", list.Body.String())
	}
	if other := doJSON(t, h, http.MethodGet, "/api/think-tanks?project=beta", ""); strings.Contains(other.Body.String(), room.RoomID) {
		t.Fatal("room listed under a participant's project rather than its origin")
	}
}

func TestThinkTankCreateReplaysReservedParticipants(t *testing.T) {
	for _, participants := range []string{
		`{"agent_id":"a_one","limit":2},{"new":{"name":"New","project":"alpha","backend":"claude"},"limit":3}`,
		`{"new":{"name":"First","project":"alpha","backend":"claude"},"limit":2},{"new":{"name":"Second","project":"beta","backend":"claude"},"limit":3}`,
	} {
		t.Run(participants, func(t *testing.T) {
			_, h := roomRESTServer(t)
			body := `{"command_id":"new-command","goal":"Choose","origin_project":"alpha","participants":[` + participants + `]}`
			first := doJSON(t, h, http.MethodPost, "/api/think-tanks", body)
			if first.Code != http.StatusCreated {
				t.Fatalf("create: %d %s", first.Code, first.Body.String())
			}
			var room thinkTankDetailWire
			_ = json.Unmarshal(first.Body.Bytes(), &room)
			second := doJSON(t, h, http.MethodPost, "/api/think-tanks", body)
			var replay thinkTankDetailWire
			_ = json.Unmarshal(second.Body.Bytes(), &replay)
			if second.Code != http.StatusCreated || replay.RoomID != room.RoomID || len(replay.Members) != 2 {
				t.Fatalf("replay: %d %s", second.Code, second.Body.String())
			}
			for i := range room.Members {
				if replay.Members[i].AgentID != room.Members[i].AgentID {
					t.Fatal("replay replaced reserved identity")
				}
			}
			changed := strings.Replace(body, `"goal":"Choose"`, `"goal":"Different"`, 1)
			if got := doJSON(t, h, http.MethodPost, "/api/think-tanks", changed); got.Code != http.StatusConflict {
				t.Fatalf("changed intent: %d %s", got.Code, got.Body.String())
			}
		})
	}
}

// FS-21.A9, A30: shared input, pause/resume/end and guarded deletion over
// REST; deletion leaves participant agents intact.
func TestThinkTankControlsOverREST(t *testing.T) {
	srv, h := roomRESTServer(t)
	rec := doJSON(t, h, http.MethodPost, "/api/think-tanks", roomBody("a_one", "a_two"))
	var room thinkTankDetailWire
	_ = json.Unmarshal(rec.Body.Bytes(), &room)
	base := "/api/think-tanks/" + room.RoomID

	if rec := doJSON(t, h, http.MethodPost, base+"/messages", `{"command_id":"m1","body":"Consider eviction"}`); rec.Code != http.StatusOK {
		t.Fatalf("message = %d %s", rec.Code, rec.Body.String())
	}
	entries := doJSON(t, h, http.MethodGet, base+"/entries", "")
	if !strings.Contains(entries.Body.String(), "Consider eviction") || !strings.Contains(entries.Body.String(), `"complete":true`) {
		t.Fatalf("entries = %s", entries.Body.String())
	}
	if rec := doJSON(t, h, http.MethodDelete, base, ""); rec.Code != http.StatusConflict {
		t.Fatalf("delete while running = %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, base+"/pause", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"control":"paused"`) {
		t.Fatalf("pause = %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, base+"/resume", ""); rec.Code != http.StatusOK {
		t.Fatalf("resume = %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, base+"/end", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"end_reason":"operator"`) {
		t.Fatalf("end = %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, base+"/messages", `{"command_id":"m2","body":"late"}`); rec.Code != http.StatusConflict {
		t.Fatalf("room message after end = %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, base+"/retry", `{"target":"bogus"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad retry target = %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodDelete, base, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodGet, base, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("deleted room = %d", rec.Code)
	}
	for _, id := range []string{"a_one", "a_two"} {
		if _, err := srv.stateStore.ReadAgent(id); err != nil {
			t.Fatalf("participant %s deleted with the room: %v", id, err)
		}
	}
}

// thinkTankWireFixturePath is the room payload the UI tests read. Go's encoder
// defines its shape (INV §11, §17); regenerate with
// CHUCK_UPDATE_THINK_TANK_FIXTURE=1 after a deliberate change.
const thinkTankWireFixturePath = "../../ui/src/features/thinktank/fixtures/room.json"

func TestThinkTankWireFixtureMatchesServerEncoding(t *testing.T) {
	srv, _ := roomRESTServer(t)
	at := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	d := state.ThinkTankDetail{
		Room: state.ThinkTank{RoomID: "tt_fixture", Title: "Cache choice", Goal: "Pick a cache", OriginProject: "alpha",
			Phase: state.ThinkTankPhaseDiscussion, Control: state.ThinkTankRunning, Revision: 7,
			JudgeConfig: `{"role":"impl","project":"alpha","interface":"chat"}`, JudgeStatus: state.ThinkTankJudgeWaiting,
			CreatedAt: at, UpdatedAt: at},
		Members: []state.ThinkTankMember{
			{AgentID: "a_one", AgentName: "Ari", Project: "alpha", Role: state.ThinkTankRoleParticipant, Order: 0, Cap: 3, Completed: 1, MayLeave: true, State: state.ThinkTankMemberActive, SetupState: state.ThinkTankSetupReady},
			{AgentID: "a_gone", AgentName: "Gone", Project: "beta", Role: state.ThinkTankRoleParticipant, Order: 1, Cap: 2, Completed: 2, State: state.ThinkTankMemberExhausted, SetupState: state.ThinkTankSetupReady},
		},
		Pending:  []state.ThinkTankInput{},
		Attempts: []state.ThinkTankAttempt{},
	}
	detail := srv.thinkTankDetailWire(d)
	entries := []thinkTankEntryWire{
		thinkTankEntryFor(state.ThinkTankEntry{Seq: 1, Kind: state.ThinkTankEntryReply, AgentID: "a_one", AgentName: "Ari", Project: "alpha", Body: "Use **LRU**.", AttemptID: "tta_1", CreatedAt: at}),
		thinkTankEntryFor(state.ThinkTankEntry{Seq: 2, Kind: state.ThinkTankEntryUser, Body: "Consider eviction", InputID: "tti_1", CreatedAt: at}),
	}
	fixture := map[string]any{"room": detail, "entries": entries}
	got, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if !bytes.Contains(got, []byte(`"pending": []`)) || !bytes.Contains(got, []byte(`"failed": []`)) {
		t.Fatalf("collections must encode as arrays: %s", got)
	}
	if os.Getenv("CHUCK_UPDATE_THINK_TANK_FIXTURE") == "1" {
		if err := os.MkdirAll("../../ui/src/features/thinktank/fixtures", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(thinkTankWireFixturePath, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(thinkTankWireFixturePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s is stale; regenerate with CHUCK_UPDATE_THINK_TANK_FIXTURE=1", thinkTankWireFixturePath)
	}
}
