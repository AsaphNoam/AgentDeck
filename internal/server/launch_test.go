package server

import (
	"testing"
)

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
