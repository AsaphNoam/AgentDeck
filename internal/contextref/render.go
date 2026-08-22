package contextref

import (
	"fmt"
	"strings"

	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
	"github.com/agentdeck/agentdeck/internal/transcript"
)

// transcriptRenderer turns a resolved span into deterministic plain text. It
// consumes transcript.ProjectEvent rather than switching on runtime.Event
// itself, and folds adjacent assistant deltas the way every other transcript
// consumer does (TS-01.R22, INV §2).
type transcriptRenderer struct {
	span    state.ContextSource
	out     *pageWriter
	pending strings.Builder // folded assistant deltas awaiting a boundary
	folding bool
	empty   bool
}

func newTranscriptRenderer(span state.ContextSource, out *pageWriter) *transcriptRenderer {
	return &transcriptRenderer{span: span, out: out, empty: true}
}

func (r *transcriptRenderer) event(ev runtime.Event) error {
	if ev.Seq < r.span.FirstSeq || ev.Seq > r.span.LastSeq {
		return nil
	}
	p, err := transcript.ProjectEvent(ev)
	if err != nil {
		// A single undecodable record must not abort the span (INV §7).
		r.flush()
		r.emit(fmt.Sprintf("[AgentDeck could not decode the %s record at sequence %d]\n\n", ev.Type, ev.Seq))
		return nil
	}
	switch p.Disposition {
	case transcript.DispositionMetadata:
		return nil
	case transcript.DispositionUnknown:
		r.flush()
		r.emit(fmt.Sprintf("[AgentDeck could not render an unknown event type %q at sequence %d]\n\n", ev.Type, ev.Seq))
		return nil
	}
	if p.Type == runtime.EvAssistantText {
		r.pending.WriteString(partText(p, transcript.PartAssistantDelta))
		r.folding = true
		return nil
	}
	r.flush()
	r.emit(renderProjection(p))
	return nil
}

// oversized records the reader's skipped-record diagnostic at its stream
// position, but only when that position falls inside the selected span.
func (r *transcriptRenderer) oversized(afterSeq int64) error {
	if afterSeq < r.span.FirstSeq || afterSeq >= r.span.LastSeq {
		return nil
	}
	r.flush()
	r.emit(OversizedRecordMarker + "\n\n")
	return nil
}

func (r *transcriptRenderer) flush() {
	if !r.folding {
		return
	}
	text := r.pending.String()
	r.pending.Reset()
	r.folding = false
	if strings.TrimSpace(text) == "" {
		return
	}
	r.emit("assistant:\n" + text + "\n\n")
}

func (r *transcriptRenderer) emit(s string) {
	r.empty = false
	r.out.write(s)
}

func (r *transcriptRenderer) done() {
	r.flush()
}

func renderProjection(p transcript.Projection) string {
	switch p.Type {
	case runtime.EvUserPrompt:
		return section("user", partText(p, transcript.PartPrompt))
	case runtime.EvToolCall:
		header := "tool call " + partText(p, transcript.PartToolName)
		return section(header, partText(p, transcript.PartToolArgs))
	case runtime.EvToolResult:
		header := "tool result"
		if status := partText(p, transcript.PartToolResultStatus); status != "" {
			header += " (" + status + ")"
		}
		body := partText(p, transcript.PartToolResultContent)
		if e := partText(p, transcript.PartToolError); e != "" {
			body = strings.TrimRight(body, "\n") + "\nerror: " + e
		}
		return section(header, body)
	case runtime.EvDiff:
		body := partText(p, transcript.PartDiffPatch)
		if body == "" {
			body = partText(p, transcript.PartDiffText)
		}
		return section("diff "+partText(p, transcript.PartDiffPath), body)
	case runtime.EvPermissionRequest:
		return section("permission requested for "+partText(p, transcript.PartPermissionTool),
			partText(p, transcript.PartPermissionReason))
	case runtime.EvPermissionResolved:
		return line("permission " + partText(p, transcript.PartPermissionDecision))
	case runtime.EvError:
		return section("error ("+partText(p, transcript.PartErrorScope)+")", partText(p, transcript.PartErrorMessage))
	case runtime.EvTurnEnd:
		return line("--- turn end (" + partText(p, transcript.PartTurnStopReason) + ") ---")
	case runtime.EvAnnotation:
		var b strings.Builder
		for _, part := range p.Parts {
			if strings.TrimSpace(part.Text) == "" {
				continue
			}
			switch part.Kind {
			case transcript.PartAnnotationExcerpt:
				b.WriteString("excerpt: " + part.Text + "\n")
			case transcript.PartAnnotationInstruction:
				b.WriteString("instruction: " + part.Text + "\n")
			case transcript.PartAnnotationOverall:
				b.WriteString("overall: " + part.Text + "\n")
			}
		}
		return section("annotation", strings.TrimRight(b.String(), "\n"))
	default:
		// Unreachable while ProjectEvent classifies every registered type, but a
		// silent empty string would be exactly the drop the registry prevents.
		return line(fmt.Sprintf("[AgentDeck has no renderer for event type %q]", p.Type))
	}
}

func section(header, body string) string {
	body = strings.TrimRight(body, "\n")
	if body == "" {
		return line(header)
	}
	return header + ":\n" + body + "\n\n"
}

func line(s string) string { return s + "\n\n" }

func partText(p transcript.Projection, kind transcript.PartKind) string {
	for _, part := range p.Parts {
		if part.Kind == kind {
			return part.Text
		}
	}
	return ""
}

// renderPipelineReport writes the accepted immutable report. Only the reported
// outcome, summary, details, checks, and declared outputs are included: run
// state, assignments, and mutable named values are not part of this source
// (TS-04.R28, FS-15.R16).
func renderPipelineReport(attempt state.PipelineAttemptRecord, out *pageWriter) {
	out.write(section("outcome", attempt.ReportOutcome))
	out.write(section("summary", attempt.ReportSummary))
	if attempt.ReportDetails != "" {
		out.write(section("details", attempt.ReportDetails))
	}
	if attempt.ReportChecks != "" {
		out.write(section("checks", attempt.ReportChecks))
	}
	if outputs := decodeOutputs(attempt.ReportOutputs); len(outputs) > 0 {
		var b strings.Builder
		for _, name := range sortedKeys(outputs) {
			b.WriteString(name + ": " + outputs[name] + "\n")
		}
		out.write(section("outputs", strings.TrimRight(b.String(), "\n")))
	}
}
