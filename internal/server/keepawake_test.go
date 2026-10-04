package server

import (
	"context"
	goruntime "runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AsaphNoam/Chuck/internal/remote"
	"github.com/AsaphNoam/Chuck/internal/state"
)

// FS-20.A6 — with keep-awake on, the assertion is held exactly while work is
// active and released when idle; with it off, nothing is held.
func TestKeepAwakeFollowsWorkAndSetting(t *testing.T) {
	if goruntime.GOOS != "darwin" {
		t.Skip("keep-awake is macOS-only")
	}
	s := testServer(t, true)
	var held atomic.Bool
	s.keepAwake = remote.NewKeepAwake(func() (func(), error) {
		held.Store(true)
		return func() { held.Store(false) }, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.runKeepAwake(ctx); close(done) }()

	waitHeld := func(want bool) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for held.Load() != want {
			if time.Now().After(deadline) {
				t.Fatalf("held = %v, want %v", held.Load(), want)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	busy := func(st string) {
		update := state.AgentStateUpdate{AgentState: state.AgentState{AgentID: "a1", State: st}}
		s.eventBus.PublishStateUpdate(update)
		s.eventBus.SetSnapshot(update)
	}

	busy("busy")
	time.Sleep(700 * time.Millisecond)
	if held.Load() {
		t.Fatal("assertion held with keep-awake off")
	}
	h := s.routes()
	putRemote(t, h, `{"keep_awake":true}`)
	waitHeld(true)
	busy("idle")
	waitHeld(false)

	// A pending permission request counts as active work.
	s.permissionMu.Lock()
	s.permissionTools[permissionToolKey("a1", "g", "c")] = "Bash"
	s.permissionMu.Unlock()
	s.nudgeKeepAwake()
	waitHeld(true)
	putRemote(t, h, `{"keep_awake":false}`)
	waitHeld(false)

	putRemote(t, h, `{"keep_awake":true}`)
	waitHeld(true)
	cancel()
	<-done
	if held.Load() {
		t.Fatal("shutdown left the assertion held")
	}
}
