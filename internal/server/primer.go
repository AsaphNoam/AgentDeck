package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/transcript"
)

const (
	defaultPrimerTokenBudget = 8000
	primerTailTurns          = 6
)

type primerSummarizer func(context.Context, primerSummaryRequest) (string, error)

type primerSummaryRequest struct {
	AgentID    string
	Target     string
	Backend    string
	Model      string
	Transcript string
}

func defaultPrimerSummarizer(context.Context, primerSummaryRequest) (string, error) {
	return "", fmt.Errorf("target-backend one-shot summary is not configured")
}

func (s *Server) buildHistoryPrimer(ctx context.Context, target runtime.LaunchSpec, budget int) (string, error) {
	if budget <= 0 {
		budget = defaultPrimerTokenBudget
	}
	events, err := transcript.ReadFile(s.configStore.Home(), target.Agent.AgentID, transcript.ReadOptions{})
	if err != nil {
		return "", err
	}
	return synthesizeHistoryPrimer(ctx, events, target, budget, s.primerSummarizer)
}

func synthesizeHistoryPrimer(ctx context.Context, events []runtime.Event, target runtime.LaunchSpec, budget int, summarize primerSummarizer) (string, error) {
	if budget <= 0 {
		budget = defaultPrimerTokenBudget
	}
	turns := transcriptTurns(events)
	tailStart := len(turns) - primerTailTurns
	if tailStart < 0 {
		tailStart = 0
	}
	older := strings.TrimSpace(strings.Join(turns[:tailStart], "\n\n"))
	tail := strings.TrimSpace(strings.Join(turns[tailStart:], "\n\n"))

	var summary string
	if older != "" && summarize != nil {
		got, err := summarize(ctx, primerSummaryRequest{
			AgentID:    target.Agent.AgentID,
			Target:     target.Agent.Backend + "/" + target.Agent.Model,
			Backend:    target.BackendType,
			Model:      target.ModelID,
			Transcript: older,
		})
		if err == nil {
			summary = strings.TrimSpace(got)
		}
	}
	if summary == "" && older != "" {
		summary = truncateToBudget(older, budget/2)
	}

	parts := []string{
		"AgentDeck backend-switch history primer.",
		"The native CLI session could not be resumed across backends. Continue from this AgentDeck transcript summary.",
	}
	if summary != "" {
		parts = append(parts, "Older context summary:\n"+summary)
	}
	if tail != "" {
		parts = append(parts, fmt.Sprintf("Last %d transcript turns verbatim:\n%s", primerTailTurns, tail))
	}
	return truncateToBudget(strings.Join(parts, "\n\n"), budget), nil
}

func appendBackendSwitchMarker(home string, agentID, from, to string, at time.Time) error {
	w, err := transcript.Open(home, agentID, nil)
	if err != nil {
		return err
	}
	defer w.Close()
	when := at.UTC().Format(time.RFC3339)
	data, err := json.Marshal(runtime.BackendSwitchData{From: from, To: to, At: when})
	if err != nil {
		return err
	}
	return w.Append(runtime.Event{
		AgentID: agentID,
		Type:    runtime.EvBackendSwitch,
		Data:    data,
		Ts:      when,
	})
}

func transcriptTurns(events []runtime.Event) []string {
	var turns []string
	var cur []string
	flush := func() {
		text := strings.TrimSpace(strings.Join(cur, "\n"))
		if text != "" {
			turns = append(turns, text)
		}
		cur = nil
	}
	for _, ev := range events {
		switch ev.Type {
		case runtime.EvAssistantText:
			var d runtime.AssistantTextData
			if json.Unmarshal(ev.Data, &d) == nil && strings.TrimSpace(d.Delta) != "" {
				cur = append(cur, "assistant: "+strings.TrimSpace(d.Delta))
			}
		case runtime.EvToolCall:
			var d runtime.ToolCallData
			if json.Unmarshal(ev.Data, &d) == nil {
				cur = append(cur, "tool_call: "+strings.TrimSpace(d.Title))
			}
		case runtime.EvToolResult:
			var d runtime.ToolResultData
			if json.Unmarshal(ev.Data, &d) == nil {
				label := "tool_result: " + d.Status
				if d.Error != "" {
					label += " error=" + d.Error
				}
				cur = append(cur, label)
			}
		case runtime.EvDiff:
			var d runtime.DiffData
			if json.Unmarshal(ev.Data, &d) == nil {
				cur = append(cur, "diff: "+d.Path)
			}
		case runtime.EvTurnEnd:
			flush()
		}
	}
	flush()
	return turns
}

func truncateToBudget(s string, budget int) string {
	if budget <= 0 {
		return ""
	}
	maxChars := budget*4 - 3
	if len(s) <= maxChars {
		return s
	}
	if maxChars <= len("...[truncated]") {
		return s[:maxChars]
	}
	return strings.TrimSpace(s[:maxChars-len("...[truncated]")]) + "\n...[truncated]"
}
