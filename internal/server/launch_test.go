package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/hooks"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

// TestComposeLaunchRejectsMissingCwd guards the J3/S3 blocker: launching a
// project whose cwd does not exist must fail with a clear validation error that
// names the directory, instead of the misleading fork/exec error naming the
// adapter binary that surfaced when the runtime tried to chdir into it.
func TestComposeLaunchRejectsMissingCwd(t *testing.T) {
	srv := testServer(t, true)
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if err := srv.configStore.WriteProject("ghost", config.Project{
		Title: "Ghost", Color: [3]int{1, 2, 3}, Cwd: missing,
	}); err != nil {
		t.Fatalf("WriteProject: %v", err)
	}

	_, _, ae := srv.composeLaunch(context.Background(), launchRequest{Role: "implementer", Project: "ghost"})
	if ae == nil {
		t.Fatal("composeLaunch succeeded; want validation error for missing cwd")
	}
	if ae.Code != runtime.CodeValidation {
		t.Fatalf("error code = %q, want %q", ae.Code, runtime.CodeValidation)
	}
	if !strings.Contains(ae.Message, missing) || !strings.Contains(ae.Message, "does not exist") {
		t.Fatalf("error message = %q, want it to name %q and 'does not exist'", ae.Message, missing)
	}
}

func TestNormalizeAgentName(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		optional bool
		want     string
		wantCode string
	}{
		{name: "omitted launch name", raw: "", optional: true, want: ""},
		{name: "trimmed", raw: "  Atlas  ", want: "Atlas"},
		{name: "whitespace", raw: " \t ", optional: true, wantCode: runtime.CodeEmptyName},
		{name: "nul", raw: "At\x00las", optional: true, wantCode: runtime.CodeValidation},
		{name: "unicode limit", raw: strings.Repeat("界", maxAgentNameRunes), want: strings.Repeat("界", maxAgentNameRunes)},
		{name: "too long", raw: strings.Repeat("a", maxAgentNameRunes+1), wantCode: runtime.CodeValidation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ae := normalizeAgentName(tt.raw, tt.optional)
			if tt.wantCode != "" {
				if ae == nil || ae.Code != tt.wantCode {
					t.Fatalf("error = %#v, want code %q", ae, tt.wantCode)
				}
				return
			}
			if ae != nil || got != tt.want {
				t.Fatalf("normalizeAgentName() = %q, %#v; want %q, nil", got, ae, tt.want)
			}
		})
	}
}

func TestComposeLaunchRejectsInvalidName(t *testing.T) {
	srv := testServer(t, true)
	for _, name := range []string{"   ", "At\x00las", strings.Repeat("a", maxAgentNameRunes+1)} {
		_, _, ae := srv.composeLaunch(t.Context(), launchRequest{
			Role: "implementer", Project: "my-app", Name: name,
		})
		if ae == nil {
			t.Fatalf("composeLaunch accepted invalid name %q", name)
		}
	}
}

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

	// Chat status comes from ACP. The official adapter does not accept a
	// per-launch --settings file, so the lifecycle-owned artifact is written for
	// symmetric cleanup but no argv points the adapter at it.
	args, err := srv.composeHookRegistration(agent, "claude-acp")
	if err != nil {
		t.Fatalf("composeHookRegistration: %v", err)
	}
	if len(args) != 0 {
		t.Fatalf("args = %v, want none for chat", args)
	}
	settingsPath := fmt.Sprintf("%s/agents/%s.json", hooks.Dir(srv.configStore.Home()), agent.AgentID)
	if _, err := os.Stat(settingsPath); err != nil {
		t.Fatalf("settings file not written: %v", err)
	}

	// The retired opt-in flag cannot re-enable an unsupported adapter argv.
	t.Setenv("AGENTDECK_HOOK_REGISTRATION", "1")
	args, err = srv.composeHookRegistration(agent, "claude-acp")
	if err != nil {
		t.Fatalf("composeHookRegistration (on): %v", err)
	}
	if len(args) != 0 {
		t.Fatalf("args = %v, want none for chat even with retired flag", args)
	}
}

func TestComposeHookRegistrationTerminalDefault(t *testing.T) {
	srv := testServer(t, true)
	agent := state.Agent{AgentID: "a_term", Interface: "terminal"}

	// Terminal registers hooks by DEFAULT (no AGENTDECK_HOOK_REGISTRATION): the
	// terminal runtime runs the real CLI under a PTY where --settings is
	// known-good and hooks are the only status producer.
	args, err := srv.composeHookRegistration(agent, "claude-acp")
	if err != nil {
		t.Fatalf("composeHookRegistration: %v", err)
	}
	if len(args) != 2 || args[0] != "--settings" || !strings.HasSuffix(args[1], ".json") {
		t.Fatalf("args = %v, want [--settings <path>] by default for terminal", args)
	}
	settingsPath := fmt.Sprintf("%s/agents/%s.json", hooks.Dir(srv.configStore.Home()), agent.AgentID)
	if _, err := os.Stat(settingsPath); err != nil {
		t.Fatalf("settings file not written: %v", err)
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

func TestCodexHomeEnvLayer(t *testing.T) {
	// Non-codex backends get no CODEX_HOME layer.
	if layer := codexHomeEnv("claude-acp", "/home/x/.agentdeck"); layer != nil {
		t.Errorf("claude-acp got a CODEX_HOME layer: %v", layer)
	}
	// codex-acp gets the dedicated home, and it overrides any ambient/backend
	// CODEX_HOME because it is the last composeEnv layer (FS-09.R43, TS-04.R20).
	home := "/home/x/.agentdeck"
	layer := codexHomeEnv("codex-acp", home)
	if got := layer["CODEX_HOME"]; got != config.CodexProfileDir(home) {
		t.Fatalf("CODEX_HOME = %q, want %q", got, config.CodexProfileDir(home))
	}
	base := []string{"CODEX_HOME=/personal/.codex", "PATH=/bin"}
	got := composeEnv(base, map[string]string{"CODEX_HOME": "/backend"}, layer)
	want := config.CodexProfileDir(home)
	for _, kv := range got {
		if kv == "CODEX_HOME="+want {
			return
		}
	}
	t.Fatalf("composed env did not override CODEX_HOME to %q: %v", want, got)
}

// TestComposeChildEnvCodexHomeOverride exercises the shared composer used by
// launch, resume, switch, and rollback: for codex-acp the reserved CODEX_HOME
// layer is applied last and beats ambient, backend, and per-model collisions,
// while a non-codex backend leaves the ambient value untouched (INV §2,
// FS-09.A17, TS-04.R20).
func TestComposeChildEnvCodexHomeOverride(t *testing.T) {
	t.Setenv("CODEX_HOME", "/ambient/.codex")
	home := "/home/x/.agentdeck"
	profile := config.CodexProfileDir(home)

	codexValue := func(kv []string) string {
		v := ""
		for _, e := range kv {
			if strings.HasPrefix(e, "CODEX_HOME=") {
				v = e[len("CODEX_HOME="):]
			}
		}
		return v
	}

	cases := []struct {
		name        string
		backendType string
		backendEnv  map[string]string
		modelEnv    map[string]string
		want        string
	}{
		{"codex overrides ambient", "codex-acp", nil, nil, profile},
		{"codex overrides backend", "codex-acp", map[string]string{"CODEX_HOME": "/backend"}, nil, profile},
		{"codex overrides model", "codex-acp", map[string]string{"CODEX_HOME": "/backend"}, map[string]string{"CODEX_HOME": "/model"}, profile},
		{"non-codex keeps ambient", "claude-acp", nil, nil, "/ambient/.codex"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := composeChildEnv(c.backendType, home, c.backendEnv, c.modelEnv, nil, nil)
			if v := codexValue(got); v != c.want {
				t.Errorf("CODEX_HOME = %q, want %q", v, c.want)
			}
		})
	}

	// Non-CODEX_HOME layer keys still contribute and later layers win their own keys.
	got := composeChildEnv("codex-acp", home,
		map[string]string{"B_ONLY": "1", "SHARED": "backend"},
		map[string]string{"M_ONLY": "2", "SHARED": "model"}, nil, nil)
	m := map[string]string{}
	for _, kv := range got {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			m[kv[:i]] = kv[i+1:]
		}
	}
	if m["B_ONLY"] != "1" || m["M_ONLY"] != "2" || m["SHARED"] != "model" {
		t.Errorf("layer composition lost keys: %v", m)
	}
	if m["CODEX_HOME"] != profile {
		t.Errorf("codex layer not final: CODEX_HOME = %q, want %q", m["CODEX_HOME"], profile)
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
