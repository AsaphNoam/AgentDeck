package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/AsaphNoam/Chuck/internal/runtime"
	"github.com/AsaphNoam/Chuck/internal/state"
)

// Room-source annotations (FS-13.R26–R27, FS-21.R26, R30). A room entry, an
// attempt's retained activity, or a file from a retained participant source is
// the anchor; attribution is resolved here from room-owned rows, never taken
// from the client. Room delivery is shared input held for the turn boundary.
// Selected-agent delivery (including an agent New task just launched) commits
// room history and ordinary annotation mail together, so a failed send leaves
// the browser tray for retry and a lost response cannot duplicate mail (INV §15).

type thinkTankAnnotationIn struct {
	// Anchor is entry, activity or file.
	Anchor      string `json:"anchor"`
	Seq         int64  `json:"seq,omitempty"`
	SourceID    string `json:"source_id,omitempty"`
	Path        string `json:"path,omitempty"`
	Side        string `json:"side,omitempty"`
	StartLine   int    `json:"start_line,omitempty"`
	EndLine     int    `json:"end_line,omitempty"`
	Excerpt     string `json:"excerpt"`
	Instruction string `json:"instruction"`
}

type thinkTankAnnotationRequest struct {
	CommandID          string                  `json:"command_id"`
	Annotations        []thinkTankAnnotationIn `json:"annotations"`
	OverallInstruction string                  `json:"overall_instruction,omitempty"`
	Target             struct {
		Kind    string `json:"kind"`
		AgentID string `json:"agent_id,omitempty"`
	} `json:"target"`
}

// thinkTankAnchor is the resolved, attributed anchor kept as entry context.
type thinkTankAnchor struct {
	Anchor      string `json:"anchor"`
	Seq         int64  `json:"seq,omitempty"`
	SourceID    string `json:"source_id,omitempty"`
	Path        string `json:"path,omitempty"`
	Side        string `json:"side,omitempty"`
	StartLine   int    `json:"start_line,omitempty"`
	EndLine     int    `json:"end_line,omitempty"`
	Attribution string `json:"attribution"`
}

func lineRange(side string, start, end int) string {
	if start == 0 {
		return ""
	}
	out := fmt.Sprintf("lines %d", start)
	if end > start {
		out = fmt.Sprintf("lines %d–%d", start, end)
	}
	if side != "" {
		out = side + " " + out
	}
	return " (" + out + ")"
}

// resolveThinkTankAnchors attributes every anchor from room-owned rows and
// refuses one that names nothing in this room.
func (s *Server) resolveThinkTankAnchors(roomID string, in []thinkTankAnnotationIn) ([]thinkTankAnchor, *runtime.APIError) {
	if len(in) == 0 || len(in) > maxAnnotationCount {
		return nil, apiError(runtime.CodeValidation, fmt.Sprintf("annotations must contain 1..%d entries", maxAnnotationCount))
	}
	sources, err := s.stateStore.ThinkTankSources(roomID)
	if err != nil {
		return nil, apiError(runtime.CodeInternal, err.Error())
	}
	out := make([]thinkTankAnchor, 0, len(in))
	for i, a := range in {
		if strings.TrimSpace(a.Excerpt) == "" || strings.TrimSpace(a.Instruction) == "" ||
			utf8.RuneCountInString(a.Instruction) > maxAnnotationChars {
			return nil, apiError(runtime.CodeValidation, fmt.Sprintf("annotation %d needs an excerpt and an instruction of at most %d characters", i+1, maxAnnotationChars))
		}
		anchor := thinkTankAnchor{Anchor: a.Anchor, Seq: a.Seq, SourceID: a.SourceID, Path: a.Path, Side: a.Side,
			StartLine: a.StartLine, EndLine: a.EndLine}
		switch a.Anchor {
		case "entry":
			entries, err := s.stateStore.ListThinkTankEntries(roomID, a.Seq-1, 1)
			if err != nil || len(entries) == 0 || entries[0].Seq != a.Seq {
				return nil, apiError(runtime.CodeValidation, fmt.Sprintf("annotation %d names no room entry", i+1))
			}
			author := "the user"
			if entries[0].AgentName != "" {
				author = entries[0].AgentName + " (" + entries[0].Project + ")"
			}
			anchor.Attribution = fmt.Sprintf("Room entry %d by %s", a.Seq, author)
		case "activity":
			rows, err := s.stateStore.ListThinkTankActivity(roomID, a.Seq-1, 1)
			if err != nil || len(rows) == 0 || rows[0].Seq != a.Seq {
				return nil, apiError(runtime.CodeValidation, fmt.Sprintf("annotation %d names no room activity", i+1))
			}
			anchor.Attribution = fmt.Sprintf("Room activity %d by %s (%s)", a.Seq, rows[0].AgentName, rows[0].Project)
			if a.Path != "" {
				anchor.Attribution += " — " + a.Path + lineRange(a.Side, a.StartLine, a.EndLine)
			}
		case "file":
			found := false
			for _, src := range sources {
				if src.SourceID == a.SourceID {
					found = true
					anchor.Attribution = fmt.Sprintf("File %s%s in %s's workspace (%s)", a.Path,
						lineRange("", a.StartLine, a.EndLine), src.AgentName, src.Project)
				}
			}
			if !found || strings.TrimSpace(a.Path) == "" {
				return nil, apiError(runtime.CodeValidation, fmt.Sprintf("annotation %d names no room file source", i+1))
			}
		default:
			return nil, apiError(runtime.CodeValidation, "anchor must be entry, activity or file")
		}
		out = append(out, anchor)
	}
	return out, nil
}

// formatThinkTankAnnotations writes the readable batch both the room and a
// recipient agent receive, clipping excerpts to fit the shared text bound.
func formatThinkTankAnnotations(goal string, anchors []thinkTankAnchor, in []thinkTankAnnotationIn, overall string) (string, *runtime.APIError) {
	limit := maxAnnotationChars
	for {
		var b strings.Builder
		title := []rune(goal)
		if len(title) > 80 {
			title = append(title[:79], '…')
		}
		fmt.Fprintf(&b, "[Think Tank annotations] %s\n", string(title))
		for i, a := range anchors {
			fmt.Fprintf(&b, "\n%d. %s\nExcerpt:\n---\n%s\n---\nInstruction: %s\n", i+1, a.Attribution,
				clipAnnotationExcerpt(in[i].Excerpt, limit), in[i].Instruction)
		}
		if strings.TrimSpace(overall) != "" {
			fmt.Fprintf(&b, "\nOverall instruction: %s\n", overall)
		}
		if b.Len() <= state.ThinkTankMaxTextBytes-8<<10 {
			return b.String(), nil
		}
		if limit <= 64 {
			return "", apiError(runtime.CodeValidation, "the annotations are too long to deliver together")
		}
		limit /= 2
	}
}

func (s *Server) handleThinkTankAnnotation(w http.ResponseWriter, r *http.Request) {
	var req thinkTankAnnotationRequest
	if !decodeThinkTankBody(w, r, &req) {
		return
	}
	roomID := r.PathValue("id")
	d, err := s.stateStore.ReadThinkTank(roomID)
	if err != nil {
		s.writeThinkTankError(w, err)
		return
	}
	if utf8.RuneCountInString(req.OverallInstruction) > maxAnnotationChars {
		writeAPIError(w, apiError(runtime.CodeValidation, fmt.Sprintf("overall instruction must be at most %d characters", maxAnnotationChars)))
		return
	}
	anchors, ae := s.resolveThinkTankAnchors(roomID, req.Annotations)
	if ae != nil {
		writeAPIError(w, ae)
		return
	}
	body, ae := formatThinkTankAnnotations(d.Room.Goal, anchors, req.Annotations, req.OverallInstruction)
	if ae != nil {
		writeAPIError(w, ae)
		return
	}
	context := map[string]any{"target": req.Target, "anchors": anchors}
	switch req.Target.Kind {
	case "room":
		raw, _ := json.Marshal(context)
		input, updated, err := s.stateStore.AddThinkTankInput(roomID, req.CommandID, state.ThinkTankEntryAnnotation, body, string(raw))
		if err != nil {
			s.writeThinkTankError(w, err)
			return
		}
		s.publishThinkTankUpdate(updated)
		s.kickThinkTanks()
		writeJSON(w, http.StatusOK, map[string]any{"version": thinkTankWireVersion, "input_id": input.InputID, "published_seq": input.EntrySeq})
	case "agent":
		target, err := s.stateStore.ReadAgent(req.Target.AgentID)
		if err != nil || target.Interface != "chat" {
			writeAPIError(w, apiError(runtime.CodeValidation, "target must be a running chat agent"))
			return
		}
		if _, err := s.stateStore.ReadRunning(target.AgentID); err != nil {
			writeAPIError(w, apiError(runtime.CodeConflict, "target agent is not running"))
			return
		}
		context["recipient"] = target.Name
		raw, _ := json.Marshal(context)
		input, updated, err := s.stateStore.AddThinkTankAnnotationMail(roomID, req.CommandID, body, string(raw), state.Message{
			FromAgent: "user", FromAddress: "user@dashboard", FromName: "Dashboard user",
			ToAgent: target.AgentID, Subject: "Think Tank annotations",
		})
		if err != nil {
			s.writeThinkTankError(w, err)
			return
		}
		s.publishThinkTankUpdate(updated)
		select {
		case s.activationCh <- target.AgentID:
		default:
		}
		if _, err := s.stateMgr.Touch(target.AgentID); err != nil {
			s.log.Debug("touch annotation recipient failed", "agent", target.AgentID, "err", err)
		}
		writeJSON(w, http.StatusOK, map[string]any{"version": thinkTankWireVersion, "input_id": input.InputID, "published_seq": input.EntrySeq})
	default:
		writeAPIError(w, apiError(runtime.CodeValidation, "target kind must be room or agent"))
	}
}
