package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// codex112Init is the reviewed codex-acp 1.12.0 initialize advertisement.
const codex112Init = `{"protocolVersion":1,"agentCapabilities":{"loadSession":true,"sessionCapabilities":{"resume":{},"list":{},"close":{},"delete":{},"fork":{},"additionalDirectories":{},"subagents":{}}},"_meta":{"steering":{"supported":true},"jetbrains":{"air":{"version":1,"capabilities":["sessionFailure","agentFileChangeReport","nativeSubagentSessions","asyncTasks","recommendedValue"]}}}}`

func TestNegotiateCapabilitiesIsBilateral(t *testing.T) {
	all := clientOffer{Subagents: true, AsyncTasks: true, FileChangeReports: true}
	full := SessionCapabilities{Fork: true, Subagents: true, BackgroundTasks: true, BackgroundTaskStop: true, FileChangeReports: true}
	cases := []struct {
		name  string
		init  string
		offer clientOffer
		want  SessionCapabilities
	}{
		{"full advertisement and offer", codex112Init, all, full},
		// Fork needs no client offer; everything else is off until AgentDeck offers it.
		{"no client offer", codex112Init, clientOffer{}, SessionCapabilities{Fork: true}},
		{"absent advertisement", `{"protocolVersion":1,"agentCapabilities":{}}`, all, SessionCapabilities{}},
		{"malformed response", `not json`, all, SessionCapabilities{}},
		{"wrong-typed fork", `{"agentCapabilities":{"sessionCapabilities":{"fork":true}}}`, all, SessionCapabilities{}},
		{"old AIR version", `{"_meta":{"jetbrains":{"air":{"version":0,"capabilities":["asyncTasks"]}}}}`, all, SessionCapabilities{}},
		{"malformed AIR list", `{"_meta":{"jetbrains":{"air":{"version":1,"capabilities":"asyncTasks"}}}}`, all, SessionCapabilities{}},
		{"async only", `{"_meta":{"jetbrains":{"air":{"version":1,"capabilities":["asyncTasks"]}}}}`, all,
			SessionCapabilities{BackgroundTasks: true, BackgroundTaskStop: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := negotiateCapabilities(json.RawMessage(tc.init), tc.offer); got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestClientCapabilitiesOfferOnlyImplementedSurfaces(t *testing.T) {
	if got, _ := json.Marshal(clientCapabilitiesFor(clientOffer{})); string(got) != `{}` {
		t.Fatalf("empty offer = %s", got)
	}
	got, _ := json.Marshal(clientCapabilitiesFor(clientOffer{Subagents: true, AsyncTasks: true, FileChangeReports: true}))
	want := `{"_meta":{"jetbrains":{"air":{"capabilities":["asyncTasks","agentFileChangeReport"],"version":1}}},"subagents":{}}`
	if string(got) != want {
		t.Fatalf("full offer = %s, want %s", got, want)
	}
}

// The live handshake sends this build's offer and keeps the negotiated value on
// the session, from the peer's advertisement alone.
func TestStartNegotiatesCapabilitiesFromInitialize(t *testing.T) {
	c, spec := newChatTest(t, "")
	dump := filepath.Join(t.TempDir(), "init.json")
	spec.Env = append(spec.Env, "FAKEACP_CAPS=1", "FAKEACP_INIT_DUMP="+dump)
	ctx := context.Background()
	h, err := c.Start(ctx, spec)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = c.Stop(ctx, h.AgentID) })

	raw, err := os.ReadFile(dump)
	if err != nil {
		t.Fatalf("init dump: %v", err)
	}
	var params struct {
		ClientCapabilities json.RawMessage `json:"clientCapabilities"`
	}
	_ = json.Unmarshal(raw, &params)
	wantOffer, _ := json.Marshal(clientCapabilitiesFor(offered))
	if string(params.ClientCapabilities) != string(wantOffer) {
		t.Fatalf("clientCapabilities = %s, want %s", params.ClientCapabilities, wantOffer)
	}
	as, err := c.lookup(h.AgentID)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got, want := as.capabilities(), negotiateCapabilities(json.RawMessage(codex112Init), offered); got != want || !got.Fork {
		t.Fatalf("session capabilities = %+v, want %+v", got, want)
	}
}
