package server

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/agentdeck/agentdeck/internal/hooks"
	"github.com/agentdeck/agentdeck/internal/state"
)

func TestHookEnvInjected(t *testing.T) {
	srv := testServer(t, true)
	agent := state.Agent{AgentID: "a_h1", Interface: "terminal"}
	env := srv.hookEnv(agent, "tok-xyz")
	wantURL := fmt.Sprintf("http://127.0.0.1:%d/api/hook", srv.cfg.Port)
	if env["AGENTDECK_HOOK_URL"] != wantURL {
		t.Fatalf("AGENTDECK_HOOK_URL = %q, want %q", env["AGENTDECK_HOOK_URL"], wantURL)
	}
	if env["AGENTDECK_HOOK_TOKEN"] != "tok-xyz" || env["AGENTDECK_AGENT_ID"] != "a_h1" || env["AGENTDECK_INTERFACE"] != "terminal" {
		t.Fatalf("hook env = %v", env)
	}
}

func TestComposeHookRegistration(t *testing.T) {
	srv := testServer(t, true)
	agent := state.Agent{AgentID: "a_h2", Interface: "chat"}

	// Default (flag off): writes the per-agent settings file, returns no launch args.
	args, err := srv.composeHookRegistration(agent, "claude-acp")
	if err != nil {
		t.Fatalf("composeHookRegistration: %v", err)
	}
	if len(args) != 0 {
		t.Fatalf("args = %v, want none while registration flag is off", args)
	}
	settingsPath := fmt.Sprintf("%s/agents/%s.json", hooks.Dir(srv.configStore.Home()), agent.AgentID)
	if _, err := os.Stat(settingsPath); err != nil {
		t.Fatalf("settings file not written: %v", err)
	}

	// Flag on: claude points the CLI at the settings file.
	t.Setenv("AGENTDECK_HOOK_REGISTRATION", "1")
	args, err = srv.composeHookRegistration(agent, "claude-acp")
	if err != nil {
		t.Fatalf("composeHookRegistration (on): %v", err)
	}
	if len(args) != 2 || args[0] != "--settings" || !strings.HasSuffix(args[1], ".json") {
		t.Fatalf("args = %v, want [--settings <path>]", args)
	}
}

func TestComposeEnvLayering(t *testing.T) {
	base := []string{"PATH=/bin", "HOME=/home/x", "SHARED=base"}
	backend := map[string]string{"SHARED": "backend", "B_ONLY": "1"}
	model := map[string]string{"SHARED": "model", "M_ONLY": "2"}

	got := composeEnv(base, backend, model)
	m := map[string]string{}
	for _, kv := range got {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				m[kv[:i]] = kv[i+1:]
				break
			}
		}
	}
	// Per-model wins on collision; both layers contribute their unique keys.
	if m["SHARED"] != "model" {
		t.Errorf("SHARED = %q, want model (per-model override wins)", m["SHARED"])
	}
	if m["B_ONLY"] != "1" || m["M_ONLY"] != "2" {
		t.Errorf("layer keys lost: %v", m)
	}
	if m["PATH"] != "/bin" || m["HOME"] != "/home/x" {
		t.Errorf("base env lost: %v", m)
	}
	// No duplicate keys in the output.
	seen := map[string]bool{}
	for k := range m {
		if seen[k] {
			t.Errorf("duplicate key %q", k)
		}
		seen[k] = true
	}
}

func TestJoinSystemPrompt(t *testing.T) {
	cases := []struct {
		ctx, sys, want string
	}{
		{"project ctx", "role persona", "project ctx\n\nrole persona"},
		{"", "role persona", "role persona"},
		{"project ctx", "", "project ctx"},
		{"", "", ""},
		{"  ", "role", "role"}, // whitespace-only context skipped
	}
	for _, c := range cases {
		if got := joinSystemPrompt(c.ctx, c.sys); got != c.want {
			t.Errorf("joinSystemPrompt(%q,%q) = %q, want %q", c.ctx, c.sys, got, c.want)
		}
	}
}

func TestResolveSkip(t *testing.T) {
	tru, fls := true, false
	if resolveSkip(false, &tru) != true {
		t.Error("role override true should win over global false")
	}
	if resolveSkip(true, &fls) != false {
		t.Error("role override false should win over global true")
	}
	if resolveSkip(true, nil) != true {
		t.Error("nil role should inherit global true")
	}
	if resolveSkip(false, nil) != false {
		t.Error("nil role should inherit global false")
	}
}
