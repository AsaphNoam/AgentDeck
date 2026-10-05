package messaging

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/AsaphNoam/Chuck/internal/state"
)

func callRoomTool(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) (map[string]any, bool) {
	t.Helper()
	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	assertWireChannelsEqual(t, name, res)
	return res.StructuredContent.(map[string]any), res.IsError
}

func wantRoomRefusal(t *testing.T, obj map[string]any, isErr bool, code, class string) {
	t.Helper()
	if !isErr || obj["error"] != code {
		t.Fatalf("refusal = %v (isError %v), want %s", obj, isErr, code)
	}
	if got := obj["retry"].(map[string]any)["class"]; got != class {
		t.Fatalf("%s retry class = %v, want %s", code, got, class)
	}
}

// FS-17.A12, FS-21.A28: real registered room tools with token-bound identity;
// every refusal is classified and changes nothing.
func TestThinkTankToolsOverMCP(t *testing.T) {
	store := newStore(t)
	srv := New(store, nil)
	srv.Register("tok-a", "a")
	srv.Register("tok-b", "b")
	srv.Register("tok-z", "z")
	a, b, z := connect(t, srv, "tok-a"), connect(t, srv, "tok-b"), connect(t, srv, "tok-z")
	noLeave := state.ThinkTankMember{AgentID: "b", AgentName: "Bea", Project: "p2", Cap: 3}
	d, err := store.CreateThinkTank(state.ThinkTankCreate{CommandID: "c", Goal: "Pick storage", OriginProject: "p",
		Members: []state.ThinkTankMember{{AgentID: "a", AgentName: "Ari", Project: "p", Cap: 3, MayLeave: true}, noLeave}})
	if err != nil {
		t.Fatal(err)
	}
	room := d.Room.RoomID

	obj, isErr := callRoomTool(t, a, "read_think_tank", map[string]any{})
	wantRoomRefusal(t, obj, isErr, "room_not_found", "never")
	obj, isErr = callRoomTool(t, z, "read_think_tank", map[string]any{"room_id": room})
	wantRoomRefusal(t, obj, isErr, "room_forbidden", "never")

	att, err := store.BeginThinkTankAttempt(state.ThinkTankBegin{RoomID: room, Revision: d.Room.Revision,
		AgentID: "a", Turn: state.ThinkTankTurnDiscussion, Generation: "g", TurnID: "t1"})
	if err != nil {
		t.Fatal(err)
	}
	obj, isErr = callRoomTool(t, a, "submit_think_tank_turn", map[string]any{
		"turn_token": att.Token, "disposition": "reply", "message": "x", "read_receipt": "guess"})
	wantRoomRefusal(t, obj, isErr, "room_read_incomplete", "after_change")

	obj, isErr = callRoomTool(t, a, "read_think_tank", map[string]any{})
	if isErr || obj["goal"] != "Pick storage" || obj["turn_token"] != att.Token || obj["complete"] != true ||
		obj["read_receipt"] == nil || obj["turns_remaining"] != float64(3) {
		t.Fatalf("read = %v", obj)
	}
	guidance, _ := obj["guidance"].(string)
	if !strings.Contains(guidance, "maximum, not a quota") || !strings.Contains(guidance, "may leave") {
		t.Fatalf("guidance = %q", guidance)
	}
	receipt := obj["read_receipt"].(string)

	obj, isErr = callRoomTool(t, b, "submit_think_tank_turn", map[string]any{
		"turn_token": att.Token, "disposition": "reply", "message": "x", "read_receipt": receipt})
	wantRoomRefusal(t, obj, isErr, "room_forbidden", "never")
	obj, isErr = callRoomTool(t, a, "submit_think_tank_turn", map[string]any{
		"turn_token": att.Token, "disposition": "decline_closing", "read_receipt": receipt})
	wantRoomRefusal(t, obj, isErr, "closing_only", "never")
	obj, isErr = callRoomTool(t, a, "submit_think_tank_turn", map[string]any{
		"turn_token": att.Token, "disposition": "reply", "message": "   ", "read_receipt": receipt})
	wantRoomRefusal(t, obj, isErr, "validation", "never")

	args := map[string]any{"turn_token": att.Token, "disposition": "reply", "message": "Use SQLite", "read_receipt": receipt}
	obj, isErr = callRoomTool(t, a, "submit_think_tank_turn", args)
	if isErr || obj["staged"] != true {
		t.Fatalf("stage = %v", obj)
	}
	again, isErr := callRoomTool(t, a, "submit_think_tank_turn", args)
	if isErr || again["receipt"] != obj["receipt"] {
		t.Fatalf("exact replay = %v", again)
	}
	args["message"] = "Use Postgres"
	obj, isErr = callRoomTool(t, a, "submit_think_tank_turn", args)
	wantRoomRefusal(t, obj, isErr, "room_reply_conflict", "never")
	if _, err := store.FinalizeThinkTankAttempt("a", "g", "t1"); err != nil {
		t.Fatal(err)
	}
	obj, isErr = callRoomTool(t, a, "submit_think_tank_turn", args)
	wantRoomRefusal(t, obj, isErr, "stale_room_turn", "never")

	d, _ = store.ReadThinkTank(room)
	bt, err := store.BeginThinkTankAttempt(state.ThinkTankBegin{RoomID: room, Revision: d.Room.Revision,
		AgentID: "b", Turn: state.ThinkTankTurnDiscussion, Generation: "g", TurnID: "t2"})
	if err != nil {
		t.Fatal(err)
	}
	obj, _ = callRoomTool(t, b, "read_think_tank", map[string]any{})
	entries := obj["entries"].([]any)
	first := entries[0].(map[string]any)
	if first["author"] != "Ari" || first["project"] != "p" || first["text"] != "Use SQLite" {
		t.Fatalf("b's read = %v", obj)
	}
	if !strings.Contains(obj["guidance"].(string), "not permitted to leave") {
		t.Fatalf("guidance = %v", obj["guidance"])
	}
	obj, isErr = callRoomTool(t, b, "submit_think_tank_turn", map[string]any{
		"turn_token": bt.Token, "disposition": "leave", "read_receipt": obj["read_receipt"]})
	wantRoomRefusal(t, obj, isErr, "leave_forbidden", "never")
	obj, isErr = callRoomTool(t, b, "read_think_tank", map[string]any{"cursor": "bogus"})
	wantRoomRefusal(t, obj, isErr, "invalid_cursor", "never")
	if entries, _ := store.ListThinkTankEntries(room, 0, 10); len(entries) != 1 {
		t.Fatalf("refusals mutated the room: %d entries", len(entries))
	}
	obj, isErr = callRoomTool(t, z, "read_think_tank", map[string]any{"room_id": room, "view": "activity"})
	wantRoomRefusal(t, obj, isErr, "room_forbidden", "never")
	obj, isErr = callRoomTool(t, a, "read_think_tank", map[string]any{"room_id": room, "view": "activity"})
	if isErr || obj["complete"] != true || len(obj["activity"].([]any)) != 0 {
		t.Fatalf("member activity read = %v", obj)
	}
}
