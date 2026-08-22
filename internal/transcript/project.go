package transcript

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agentdeck/agentdeck/internal/runtime"
)

// Disposition is a projection's explicit classification of a normalized event.
type Disposition string

const (
	// DispositionText — the event carries readable content in Parts.
	DispositionText Disposition = "text"
	// DispositionMetadata — the event is deliberately content-free. Session
	// snapshots and backend-switch markers describe the conversation rather than
	// belonging to it, so no text consumer renders or indexes them.
	DispositionMetadata Disposition = "metadata"
	// DispositionUnknown — the event type is outside runtime.AllEventTypes. A
	// consumer must surface it as an explicit marker rather than dropping it.
	DispositionUnknown Disposition = "unknown"
)

// PartKind names one typed, presentation-neutral fragment of an event. Framing
// (labels, quoting, ordering) belongs to the consumer, not to this projection.
type PartKind string

const (
	PartPrompt                PartKind = "prompt"
	PartAssistantDelta        PartKind = "assistant_delta"
	PartToolName              PartKind = "tool_name"
	PartToolArgs              PartKind = "tool_args"
	PartToolResultStatus      PartKind = "tool_result_status"
	PartToolResultContent     PartKind = "tool_result_content"
	PartToolError             PartKind = "tool_error"
	PartDiffPath              PartKind = "diff_path"
	PartDiffText              PartKind = "diff_text"
	PartDiffPatch             PartKind = "diff_patch"
	PartPermissionTool        PartKind = "permission_tool"
	PartPermissionReason      PartKind = "permission_reason"
	PartPermissionDecision    PartKind = "permission_decision"
	PartErrorScope            PartKind = "error_scope"
	PartErrorMessage          PartKind = "error_message"
	PartTurnStopReason        PartKind = "turn_stop_reason"
	PartAnnotationExcerpt     PartKind = "annotation_excerpt"
	PartAnnotationInstruction PartKind = "annotation_instruction"
	PartAnnotationOverall     PartKind = "annotation_overall"
)

// Part is one text fragment of a projected event. Indexed marks the fragments
// that make up the full-text search bag; the remainder is readable framing that
// search deliberately ignores.
type Part struct {
	Kind    PartKind
	Text    string
	Indexed bool
}

// Projection is the decoded, presentation-neutral view of one normalized event.
type Projection struct {
	Seq         int64
	Type        string
	Ts          string
	Role        string // "user" | "assistant" | "system" | ""
	Disposition Disposition
	Parts       []Part
}

// SearchText is the full-text search bag for this event: the indexed parts
// joined by a single space and trimmed.
func (p Projection) SearchText() string {
	var b strings.Builder
	for _, part := range p.Parts {
		if !part.Indexed {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(part.Text)
	}
	return strings.TrimSpace(b.String())
}

// ProjectEvent decodes one normalized transcript event into typed text parts and
// an explicit disposition. It is the single Go seam every transcript text
// consumer builds on — the search indexer takes its bag from SearchText, and the
// context renderer frames these same parts (INV §2, TS-01.R22). Adding a
// runtime.Ev* constant without a case here fails TestProjectEventCoversEveryType
// rather than silently dropping the event from search and pulled context.
func ProjectEvent(ev runtime.Event) (Projection, error) {
	p := Projection{Seq: ev.Seq, Type: ev.Type, Ts: ev.Ts, Disposition: DispositionText}
	switch ev.Type {
	case runtime.EvUserPrompt:
		var d runtime.UserPromptData
		if err := decode(ev, &d); err != nil {
			return Projection{}, err
		}
		p.Role = "user"
		p.Parts = []Part{{Kind: PartPrompt, Text: d.Text, Indexed: true}}
	case runtime.EvAssistantText:
		var d runtime.AssistantTextData
		if err := decode(ev, &d); err != nil {
			return Projection{}, err
		}
		p.Role = "assistant"
		p.Parts = []Part{{Kind: PartAssistantDelta, Text: d.Delta, Indexed: true}}
	case runtime.EvToolCall:
		var d runtime.ToolCallData
		if err := decode(ev, &d); err != nil {
			return Projection{}, err
		}
		p.Role = "assistant"
		p.Parts = []Part{
			{Kind: PartToolName, Text: d.Name, Indexed: true},
			{Kind: PartToolArgs, Text: string(d.Args), Indexed: true},
		}
	case runtime.EvToolResult:
		var d runtime.ToolResultData
		if err := decode(ev, &d); err != nil {
			return Projection{}, err
		}
		p.Role = "assistant"
		p.Parts = []Part{
			{Kind: PartToolResultContent, Text: string(d.Content), Indexed: true},
			{Kind: PartToolResultStatus, Text: d.Status},
			{Kind: PartToolError, Text: d.Error},
		}
	case runtime.EvDiff:
		var d runtime.DiffData
		if err := decode(ev, &d); err != nil {
			return Projection{}, err
		}
		p.Role = "assistant"
		p.Parts = []Part{
			{Kind: PartDiffPath, Text: d.Path, Indexed: true},
			{Kind: PartDiffText, Text: d.NewText, Indexed: true},
			{Kind: PartDiffPatch, Text: d.Patch},
		}
	case runtime.EvPermissionRequest:
		var d runtime.PermissionRequestData
		if err := decode(ev, &d); err != nil {
			return Projection{}, err
		}
		p.Role = "system"
		p.Parts = []Part{
			{Kind: PartPermissionReason, Text: d.Reason, Indexed: true},
			{Kind: PartPermissionTool, Text: d.Name},
		}
	case runtime.EvPermissionResolved:
		var d runtime.PermissionResolvedData
		if err := decode(ev, &d); err != nil {
			return Projection{}, err
		}
		p.Role = "system"
		p.Parts = []Part{{Kind: PartPermissionDecision, Text: d.Decision}}
	case runtime.EvError:
		var d runtime.ErrorData
		if err := decode(ev, &d); err != nil {
			return Projection{}, err
		}
		p.Role = "system"
		p.Parts = []Part{
			{Kind: PartErrorScope, Text: d.Scope},
			{Kind: PartErrorMessage, Text: d.Message},
		}
	case runtime.EvTurnEnd:
		var d runtime.TurnEndData
		if err := decode(ev, &d); err != nil {
			return Projection{}, err
		}
		p.Role = "system"
		p.Parts = []Part{{Kind: PartTurnStopReason, Text: d.StopReason}}
	case runtime.EvAnnotation:
		var d runtime.AnnotationData
		if err := decode(ev, &d); err != nil {
			return Projection{}, err
		}
		p.Role = "user"
		p.Parts = make([]Part, 0, len(d.Annotations)*2+1)
		for _, a := range d.Annotations {
			p.Parts = append(p.Parts,
				Part{Kind: PartAnnotationExcerpt, Text: a.Excerpt, Indexed: true},
				Part{Kind: PartAnnotationInstruction, Text: a.Instruction, Indexed: true},
			)
		}
		p.Parts = append(p.Parts, Part{Kind: PartAnnotationOverall, Text: d.OverallInstruction, Indexed: true})
	case runtime.EvSessionMeta, runtime.EvBackendSwitch:
		// Deliberately content-free: a launch/resume snapshot and a cross-backend
		// hand-off marker describe the conversation rather than belonging to it.
		p.Disposition = DispositionMetadata
	default:
		p.Disposition = DispositionUnknown
	}
	return p, nil
}

func decode(ev runtime.Event, into any) error {
	if err := json.Unmarshal(ev.Data, into); err != nil {
		return fmt.Errorf("transcript: project %s seq %d: %w", ev.Type, ev.Seq, err)
	}
	return nil
}
