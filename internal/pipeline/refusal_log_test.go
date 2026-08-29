package pipeline

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// Every refused stage result is logged at Warn with the fields that separate the
// refusal conditions from one another. Before this, `internal/pipeline` logged
// only from proposals.go, so a field report of a refused report ("it tried to
// continue and got stale_assignment") could not be corroborated from
// ~/.agentdeck/dashboard.log at all — the only trace was inside the agent's own
// transcript, which the operator reporting the bug does not send.
func TestRefusedStageResultIsLogged(t *testing.T) {
	var buf bytes.Buffer
	restore := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(restore) })

	manager, _, _ := pipelineManagerFixture(t)
	detail := startPipeline(t, manager, "request-refusal-log")
	work := detail.Attempts[0]

	// A caller that is not the run's current attempt: the genuine stale case.
	if _, err := manager.Report("a_impostor", "gen_impostor", StageReport{
		Outcome: "success", Summary: "not mine to report",
	}); err == nil {
		t.Fatal("an unrelated caller's report was accepted")
	}
	// The run's own attempt reporting twice: the already_reported case, which the
	// log must distinguish from the one above.
	if _, err := manager.Report(work.AgentID, work.AgentGeneration, StageReport{
		Outcome: "blocked", Summary: "Which fallback should I use?",
	}); err != nil {
		t.Fatalf("blocked report: %v", err)
	}
	if _, err := manager.Report(work.AgentID, work.AgentGeneration, StageReport{
		Outcome: "success", Summary: "answered in chat",
	}); err == nil {
		t.Fatal("a second report from the same attempt was accepted")
	}

	logged := buf.String()
	lines := []string{}
	for _, line := range strings.Split(logged, "\n") {
		if strings.Contains(line, "stage result refused") {
			lines = append(lines, line)
		}
	}
	if len(lines) != 2 {
		t.Fatalf("refusal log lines = %d, want 2\n%s", len(lines), logged)
	}
	if !strings.Contains(lines[0], "code=assignment_unknown") && !strings.Contains(lines[0], "code=stale_assignment") {
		t.Fatalf("first refusal did not name a stale/unknown caller: %s", lines[0])
	}
	if !strings.Contains(lines[1], "code=already_reported") {
		t.Fatalf("second refusal did not name already_reported: %s", lines[1])
	}
	// The whole field set is what makes the two lines tell different stories.
	for _, field := range []string{"run=", "attempt=", "caller_agent=", "caller_generation=", "attempt_agent=", "attempt_generation=", "pending_action="} {
		if !strings.Contains(lines[1], field) {
			t.Fatalf("refusal log is missing %q: %s", field, lines[1])
		}
	}
	if !strings.Contains(lines[1], "caller_agent="+work.AgentID) {
		t.Fatalf("refusal log does not name the caller: %s", lines[1])
	}
}
