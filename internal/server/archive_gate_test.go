package server

import (
	"context"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/runtime"
)

func waitForArchiveGate(t *testing.T, srv *Server, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		srv.archiveMu.Lock()
		ready := predicate()
		srv.archiveMu.Unlock()
		if ready {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("archive gate did not reach the expected barrier")
}

// TS-01.R13 / INV §5: the archive claim is published before waiting for an
// older launch. A later start cannot repeatedly leapfrog it.
func TestProjectArchiveReservesClaimBeforeWaitingForStart(t *testing.T) {
	srv := testServer(t, true)
	if ae := srv.acquireProjectStart("my-app"); ae != nil {
		t.Fatal(ae)
	}
	finished := make(chan *runtime.APIError, 1)
	go func() { finished <- srv.beginProjectArchive(context.Background(), "my-app") }()
	waitForArchiveGate(t, srv, func() bool { return srv.projectArchiving["my-app"] })
	if ae := srv.acquireProjectStart("my-app"); ae == nil || ae.Code != runtime.CodeProjectArchiving {
		t.Fatalf("start during reserved archive = %+v, want project_archiving", ae)
	}
	srv.releaseProjectStart("my-app")
	if ae := <-finished; ae != nil {
		t.Fatalf("archive did not continue after earlier start released: %v", ae)
	}
	srv.endProjectArchive("my-app")
}

// TS-01.R13 / TS-02.R20: an individual agent archive claim blocks a Resume
// that arrives after the claim, while waiting for an already-starting Resume to
// leave its registration window.
func TestAgentArchiveClaimBlocksConcurrentResume(t *testing.T) {
	srv := testServer(t, true)
	if ae := srv.acquireAgentStart("my-app", "a_resume"); ae != nil {
		t.Fatal(ae)
	}
	finished := make(chan *runtime.APIError, 1)
	go func() { finished <- srv.beginAgentArchive(context.Background(), "my-app", "a_resume") }()
	waitForArchiveGate(t, srv, func() bool { return srv.agentArchiving["a_resume"] })
	if ae := srv.acquireAgentStart("my-app", "a_resume"); ae == nil || ae.Code != runtime.CodeAgentArchiving {
		t.Fatalf("resume during archive = %+v, want agent_archiving", ae)
	}
	srv.releaseAgentStart("my-app", "a_resume")
	if ae := <-finished; ae != nil {
		t.Fatalf("archive did not continue after resume lease released: %v", ae)
	}
	srv.endAgentArchive("my-app", "a_resume")
}

// TS-01.R13: project archive waits for an in-flight Restore transition, then
// excludes every later agent transition until its archive commit is complete.
func TestProjectArchiveWaitsForAgentRestoreTransition(t *testing.T) {
	srv := testServer(t, true)
	if ae := srv.beginAgentArchive(context.Background(), "my-app", "a_restore"); ae != nil {
		t.Fatal(ae)
	}
	finished := make(chan *runtime.APIError, 1)
	go func() { finished <- srv.beginProjectArchive(context.Background(), "my-app") }()
	waitForArchiveGate(t, srv, func() bool { return srv.projectArchiving["my-app"] })
	select {
	case ae := <-finished:
		t.Fatalf("project archive finished before restore transition: %v", ae)
	default:
	}
	srv.endAgentArchive("my-app", "a_restore")
	if ae := <-finished; ae != nil {
		t.Fatal(ae)
	}
	if ae := srv.beginAgentArchive(context.Background(), "my-app", "a_later_restore"); ae == nil || ae.Code != runtime.CodeProjectArchiving {
		t.Fatalf("restore during project archive = %+v, want project_archiving", ae)
	}
	srv.endProjectArchive("my-app")
}
