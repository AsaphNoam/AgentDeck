package backend

import "testing"

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
