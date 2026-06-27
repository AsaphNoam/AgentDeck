package runtime

import (
	"context"
	"errors"
	"testing"
)

// newTestRegistry builds a Registry with a nil store. Phase 1.1 dispatch logic
// touches no store, so nil is fine; later subphases that exercise Start will
// pass a real *state.Store.
func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	return NewRegistry(nil)
}

// TestRuntimeForDispatch asserts the interface→runtime dispatch: "chat" resolves
// to a real runtime, "terminal" resolves to the not-implemented stub, and an
// unknown interface yields ErrNotImplemented (techspec §3.2).
func TestRuntimeForDispatch(t *testing.T) {
	r := newTestRegistry(t)

	cases := []struct {
		iface      string
		wantErr    bool
		wantNotImp bool // when started, the resolved runtime returns ErrNotImplemented
	}{
		{iface: "chat", wantErr: false, wantNotImp: false},
		{iface: "terminal", wantErr: false, wantNotImp: true},
		{iface: "codex", wantErr: true, wantNotImp: true}, // not a registered interface
		{iface: "", wantErr: true, wantNotImp: true},      // empty
		{iface: "bogus", wantErr: true, wantNotImp: true},
	}

	for _, tc := range cases {
		t.Run(tc.iface, func(t *testing.T) {
			rt, err := r.runtimeFor(tc.iface)
			if tc.wantErr {
				if !errors.Is(err, ErrNotImplemented) {
					t.Fatalf("runtimeFor(%q) err = %v, want ErrNotImplemented", tc.iface, err)
				}
				if rt != nil {
					t.Fatalf("runtimeFor(%q) returned non-nil runtime on error", tc.iface)
				}
				return
			}
			if err != nil {
				t.Fatalf("runtimeFor(%q) unexpected err: %v", tc.iface, err)
			}
			if rt == nil {
				t.Fatalf("runtimeFor(%q) returned nil runtime", tc.iface)
			}
		})
	}
}

// TestTerminalStubReturnsNotImplemented asserts the terminal stub's methods all
// return ErrNotImplemented (techspec §3.3).
func TestTerminalStubReturnsNotImplemented(t *testing.T) {
	r := newTestRegistry(t)
	rt, err := r.runtimeFor("terminal")
	if err != nil {
		t.Fatalf("runtimeFor(terminal): %v", err)
	}
	ctx := context.Background()

	if _, err := rt.Start(ctx, LaunchSpec{}); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("terminal Start err = %v, want ErrNotImplemented", err)
	}
	if err := rt.SendPrompt(ctx, "a", "hi"); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("terminal SendPrompt err = %v, want ErrNotImplemented", err)
	}
	if _, _, err := rt.Subscribe("a"); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("terminal Subscribe err = %v, want ErrNotImplemented", err)
	}
}

// TestChatRuntimeCodexBackendNotImplemented asserts the codex backend path on
// the real chat runtime returns ErrNotImplemented at the backend gate, before
// any process spawn (techspec §3.3). The claude-acp path passes the gate and is
// exercised end-to-end (against the fake CLI) in chat_test.go.
func TestChatRuntimeCodexBackendNotImplemented(t *testing.T) {
	c := NewChatRuntime(nil)
	_, err := c.Start(context.Background(), LaunchSpec{BackendType: "codex-acp"})
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("codex Start err = %v, want ErrNotImplemented", err)
	}
}
