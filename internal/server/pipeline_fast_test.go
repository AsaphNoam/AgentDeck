package server

import (
	"context"
	"testing"

	"github.com/agentdeck/agentdeck/internal/pipeline"
)

// FS-14.A34 / FS-09.R54 / INV §3 — a pipeline stage assignment carries the fast
// mode it *requested*, while the stage agent it launches records the fast mode
// that *actually applied*. A pipeline run is unattended by design, so a stage
// whose speed tier the live session refused must still leave an honest record
// behind rather than reporting the request back as the outcome.
func TestAPipelineStageRecordsTheFastModeThatApplied(t *testing.T) {
	for _, tt := range []struct {
		name       string
		fastOption string
		want       bool
	}{
		{name: "session advertises fast mode", fastOption: "fast", want: true},
		{name: "session does not advertise it", fastOption: "", want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("FAKEACP_FAST_OPTION", tt.fastOption)
			srv, _ := wakeTestServer(t)
			writeFastCapableBackend(t, srv)

			execution := pipeline.StageExecution{
				RunID: "r_1", RunName: "run", AttemptID: "at_1", StageID: "s_1", StageTitle: "Stage",
				Role: "impl", Project: "tmpproj", Backend: "claude", Model: "sonnet", Fast: true,
				AgentID: "a_stage01", Generation: "g1", AgentName: "Stage", Assignment: "do the work",
			}
			if err := srv.LaunchStage(context.Background(), execution); err != nil {
				t.Fatalf("LaunchStage: %v", err)
			}
			t.Cleanup(func() { _ = srv.StopStage(context.Background(), execution.AgentID) })

			if !execution.Fast {
				t.Fatal("the stage assignment no longer carries the fast mode it requested")
			}
			agent, err := srv.stateStore.ReadAgent(execution.AgentID)
			if err != nil {
				t.Fatalf("ReadAgent: %v", err)
			}
			if agent.Fast != tt.want {
				t.Fatalf("stage agent fast = %v, want %v (the value the live session applied)", agent.Fast, tt.want)
			}
			session, err := srv.stateStore.ReadSession(execution.AgentID)
			if err != nil {
				t.Fatalf("ReadSession: %v", err)
			}
			if session.Fast != tt.want {
				t.Fatalf("stage session snapshot fast = %v, want %v", session.Fast, tt.want)
			}
		})
	}
}
