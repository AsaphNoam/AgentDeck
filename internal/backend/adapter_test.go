package backend

import (
	"reflect"
	"testing"
)

func TestForKnownAndUnknown(t *testing.T) {
	for _, typ := range []string{"claude-acp", "codex-acp"} {
		ad, ok := For(typ)
		if !ok {
			t.Fatalf("For(%q) not found", typ)
		}
		if ad.Type() != typ {
			t.Fatalf("For(%q).Type() = %q", typ, ad.Type())
		}
		if ad.Binary() == "" {
			t.Fatalf("For(%q).Binary() empty", typ)
		}
		// Every lifecycle event maps for both backends today; none unsupported.
		if len(ad.UnsupportedHookEvents()) != 0 {
			t.Fatalf("For(%q).UnsupportedHookEvents() = %v, want none", typ, ad.UnsupportedHookEvents())
		}
		for _, ev := range agentDeckHookEvents {
			if _, ok := ad.HookMap()[ev]; !ok {
				t.Fatalf("For(%q).HookMap() missing %q", typ, ev)
			}
		}
	}

	if _, ok := For("openai-direct"); ok {
		t.Fatalf("For(unknown) returned ok")
	}
}

// TestNewBackendAdapters covers the Phase 7 opencode/openhands adapters: they
// resolve, launch via `<bin> acp`, run hookless (chat status from the ACP
// stream), and strip the env keys they own so a shell-level value never leaks.
func TestNewBackendAdapters(t *testing.T) {
	cases := []struct {
		typ, bin string
		strip    []string
	}{
		{"opencode-acp", "opencode", []string{"CLAUDECODE", "OPENCODE_CONFIG", "OPENCODE_CONFIG_CONTENT"}},
		{"openhands-acp", "openhands", []string{"CLAUDECODE", "LLM_MODEL"}},
	}
	for _, tc := range cases {
		ad, ok := For(tc.typ)
		if !ok {
			t.Fatalf("For(%q) not found", tc.typ)
		}
		if ad.Type() != tc.typ {
			t.Fatalf("For(%q).Type() = %q", tc.typ, ad.Type())
		}
		if ad.Binary() != tc.bin {
			t.Fatalf("%s Binary() = %q, want %q", tc.typ, ad.Binary(), tc.bin)
		}
		if args := ad.LaunchArgs(); len(args) != 1 || args[0] != "acp" {
			t.Fatalf("%s LaunchArgs() = %v, want [acp]", tc.typ, args)
		}
		if got := ad.StripEnvKeys(); !reflect.DeepEqual(got, tc.strip) {
			t.Fatalf("%s StripEnvKeys() = %v, want %v", tc.typ, got, tc.strip)
		}
		// Hookless: no map, every lifecycle event unsupported, no settings flag.
		if ad.HookMap() != nil {
			t.Fatalf("%s HookMap() = %v, want nil", tc.typ, ad.HookMap())
		}
		if got := ad.UnsupportedHookEvents(); !reflect.DeepEqual(got, sortedHookEvents()) {
			t.Fatalf("%s UnsupportedHookEvents() = %v, want all five", tc.typ, got)
		}
		if got := ad.HookLaunchArgs("/h/a.json"); got != nil {
			t.Fatalf("%s HookLaunchArgs() = %v, want nil", tc.typ, got)
		}
		// Resume: native session/load attempt same-backend, primer floor cross-backend.
		if got := ad.ResolveResumeID("sess-1", true); got != "sess-1" {
			t.Fatalf("%s same-backend resolve = %q, want sess-1", tc.typ, got)
		}
		if got := ad.ResolveResumeID("sess-1", false); got != "" {
			t.Fatalf("%s cross-backend resolve = %q, want empty", tc.typ, got)
		}
		if !ad.CanSwitchModelOnResume() {
			t.Fatalf("%s CanSwitchModelOnResume() = false, want true", tc.typ)
		}
	}
}

// TestOpenHandsExtraEnvCarriesModel proves OpenHands selects its model via the
// LLM_MODEL env var (not the ACP session param) and — with LLM_MODEL in
// StripEnvKeys — never inherits the shell's model.
func TestOpenHandsExtraEnvCarriesModel(t *testing.T) {
	ad, _ := For("openhands-acp")
	ep, ok := ad.(ExtraEnvProvider)
	if !ok {
		t.Fatal("openhands adapter must implement ExtraEnvProvider")
	}
	if got := ep.ExtraEnv("anthropic/claude-sonnet-4-5", false); len(got) != 1 || got[0] != "LLM_MODEL=anthropic/claude-sonnet-4-5" {
		t.Fatalf("openhands ExtraEnv = %v, want [LLM_MODEL=anthropic/claude-sonnet-4-5]", got)
	}
	if got := ep.ExtraEnv("", false); got != nil {
		t.Fatalf("openhands ExtraEnv(empty model) = %v, want nil", got)
	}
	// LLM_MODEL must be stripped so the ExtraEnv value is authoritative.
	var strips bool
	for _, k := range ad.StripEnvKeys() {
		if k == "LLM_MODEL" {
			strips = true
		}
	}
	if !strips {
		t.Fatal("openhands StripEnvKeys() must include LLM_MODEL")
	}
}

// sortedHookEvents returns the canonical lifecycle events in sorted order, the
// shape UnsupportedHookEvents() returns for a hookless backend.
func sortedHookEvents() []string {
	return unsupported(nil)
}

func TestHookLaunchArgs(t *testing.T) {
	claude, _ := For("claude-acp")
	if got := claude.HookLaunchArgs("/h/agents/a.json"); len(got) != 2 || got[0] != "--settings" || got[1] != "/h/agents/a.json" {
		t.Fatalf("claude HookLaunchArgs = %v, want [--settings /h/agents/a.json]", got)
	}
	if got := claude.HookLaunchArgs(""); got != nil {
		t.Fatalf("claude HookLaunchArgs(\"\") = %v, want nil", got)
	}
	codex, _ := For("codex-acp")
	if got := codex.HookLaunchArgs("/h/agents/a.json"); got != nil {
		t.Fatalf("codex HookLaunchArgs = %v, want nil (gated)", got)
	}
}

func TestStripEnvKeys(t *testing.T) {
	claude, _ := For("claude-acp")
	if got := claude.StripEnvKeys(); len(got) != 1 || got[0] != "CLAUDECODE" {
		t.Fatalf("claude StripEnvKeys = %v, want [CLAUDECODE]", got)
	}
	codex, _ := For("codex-acp")
	if got := codex.StripEnvKeys(); len(got) != 0 {
		t.Fatalf("codex StripEnvKeys = %v, want none", got)
	}
}

func TestResolveResumeID(t *testing.T) {
	for _, typ := range []string{"claude-acp", "codex-acp"} {
		ad, _ := For(typ)
		// Same backend → keep the native session id.
		if got := ad.ResolveResumeID("sess-1", true); got != "sess-1" {
			t.Fatalf("%s same-backend resolve = %q, want sess-1", typ, got)
		}
		// Cross-backend → no compatible native session (primer path, 6.5).
		if got := ad.ResolveResumeID("sess-1", false); got != "" {
			t.Fatalf("%s cross-backend resolve = %q, want empty", typ, got)
		}
		if !ad.CanSwitchModelOnResume() {
			t.Fatalf("%s CanSwitchModelOnResume = false, want true", typ)
		}
	}
}
