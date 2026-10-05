package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/AsaphNoam/Chuck/internal/runtime"
	"github.com/AsaphNoam/Chuck/internal/state"
)

// Retained room activity over REST (TS-14 §3, R10, R12, R17). Activity windows
// are bounded by record count and bytes; Files and Commands are projections of
// that same retained activity, never of a participant's private history.

const (
	thinkTankActivityPage  = 500
	thinkTankActivityBytes = 1 << 20
	// thinkTankScanLimit bounds the records one Files/Commands projection reads.
	thinkTankScanLimit = 20 * thinkTankActivityPage
)

func (s *Server) handleThinkTankActivity(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("id")
	if _, err := s.stateStore.ReadThinkTank(roomID); err != nil {
		s.writeThinkTankError(w, err)
		return
	}
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	rows, err := s.stateStore.ListThinkTankActivity(roomID, max(after, 0), thinkTankActivityPage+1)
	if err != nil {
		s.writeThinkTankError(w, err)
		return
	}
	out := []thinkTankActivityWire{}
	bytes := 0
	complete := true
	for i, a := range rows {
		if i == thinkTankActivityPage || (len(out) > 0 && bytes+len(a.Payload) > thinkTankActivityBytes) {
			complete = false
			break
		}
		bytes += len(a.Payload)
		out = append(out, thinkTankActivityFor(a))
	}
	writeJSON(w, http.StatusOK, map[string]any{"version": thinkTankWireVersion, "activity": out, "complete": complete})
}

type thinkTankSourceWire struct {
	SourceID  string `json:"source_id"`
	AgentID   string `json:"agent_id"`
	AgentName string `json:"agent_name"`
	Project   string `json:"project"`
}

type thinkTankFileWire struct {
	SourceID  string `json:"source_id"`
	AgentName string `json:"agent_name"`
	Project   string `json:"project"`
	Path      string `json:"path"`
	LastSeq   int64  `json:"last_seq"`
}

type thinkTankCommandWire struct {
	SourceID   string `json:"source_id"`
	AgentName  string `json:"agent_name"`
	Project    string `json:"project"`
	ToolCallID string `json:"tool_call_id"`
	Command    string `json:"command"`
	Status     string `json:"status,omitempty"`
	Seq        int64  `json:"seq"`
}

// scanThinkTankActivity reads visible retained activity in bounded batches and
// resolves each record's opaque source id.
func (s *Server) scanThinkTankActivity(roomID string, visit func(state.ThinkTankActivity, string, map[string]any)) ([]thinkTankSourceWire, error) {
	sources, err := s.stateStore.ThinkTankSources(roomID)
	if err != nil {
		return nil, err
	}
	sourceOf := map[string]string{}
	out := []thinkTankSourceWire{}
	for _, src := range sources {
		sourceOf[src.AgentID+"\x00"+src.Cwd] = src.SourceID
		out = append(out, thinkTankSourceWire{SourceID: src.SourceID, AgentID: src.AgentID, AgentName: src.AgentName, Project: src.Project})
	}
	var after int64
	for read := 0; read < thinkTankScanLimit; {
		rows, err := s.stateStore.ListThinkTankActivity(roomID, after, thinkTankActivityPage)
		if err != nil {
			return nil, err
		}
		for _, a := range rows {
			var ev map[string]any
			if json.Unmarshal([]byte(a.Payload), &ev) == nil {
				visit(a, sourceOf[a.AgentID+"\x00"+a.Cwd], ev)
			}
			after = a.Seq
		}
		read += len(rows)
		if len(rows) < thinkTankActivityPage {
			break
		}
	}
	return out, nil
}

func eventData(ev map[string]any) map[string]any {
	d, _ := ev["data"].(map[string]any)
	return d
}

func (s *Server) handleThinkTankFiles(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("id")
	if _, err := s.stateStore.ReadThinkTank(roomID); err != nil {
		s.writeThinkTankError(w, err)
		return
	}
	files := []thinkTankFileWire{}
	index := map[string]int{}
	sources, err := s.scanThinkTankActivity(roomID, func(a state.ThinkTankActivity, source string, ev map[string]any) {
		if ev["type"] != runtime.EvDiff {
			return
		}
		path, _ := eventData(ev)["path"].(string)
		if path == "" {
			return
		}
		// Equal relative paths in different workspaces stay distinct (FS-21.R26).
		key := source + "\x00" + path
		if i, ok := index[key]; ok {
			files[i].LastSeq = a.Seq
			return
		}
		index[key] = len(files)
		files = append(files, thinkTankFileWire{SourceID: source, AgentName: a.AgentName, Project: a.Project, Path: path, LastSeq: a.Seq})
	})
	if err != nil {
		s.writeThinkTankError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"version": thinkTankWireVersion, "sources": sources, "files": files})
}

func (s *Server) handleThinkTankCommands(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("id")
	if _, err := s.stateStore.ReadThinkTank(roomID); err != nil {
		s.writeThinkTankError(w, err)
		return
	}
	commands := []thinkTankCommandWire{}
	index := map[string]int{}
	sources, err := s.scanThinkTankActivity(roomID, func(a state.ThinkTankActivity, source string, ev map[string]any) {
		d := eventData(ev)
		id, _ := d["tool_call_id"].(string)
		switch ev["type"] {
		case runtime.EvToolCall:
			args, _ := d["args"].(map[string]any)
			command, _ := args["command"].(string)
			if command == "" {
				return
			}
			index[a.AttemptID+"\x00"+id] = len(commands)
			commands = append(commands, thinkTankCommandWire{SourceID: source, AgentName: a.AgentName,
				Project: a.Project, ToolCallID: id, Command: command, Seq: a.Seq})
		case runtime.EvToolResult:
			if i, ok := index[a.AttemptID+"\x00"+id]; ok {
				commands[i].Status, _ = d["status"].(string)
			}
		}
	})
	if err != nil {
		s.writeThinkTankError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"version": thinkTankWireVersion, "sources": sources, "commands": commands})
}

// handleThinkTankSourceFile opens a file from a retained participant source
// context with the ordinary bounded reader and its absolute/relative rules.
// Viewing never archives file contents into the room (TS-14.R12).
func (s *Server) handleThinkTankSourceFile(w http.ResponseWriter, r *http.Request) {
	roomID, sourceID := r.PathValue("id"), r.PathValue("source_id")
	sources, err := s.stateStore.ThinkTankSources(roomID)
	if err != nil {
		s.writeThinkTankError(w, err)
		return
	}
	for _, src := range sources {
		if src.SourceID != sourceID {
			continue
		}
		path, display, ae := resolveFileReadPath(src.Cwd, r.URL.Query().Get("path"))
		if ae != nil {
			writeAPIError(w, ae)
			return
		}
		out, ae := readLocalFileWithOptions(path, display, false)
		if ae != nil {
			writeAPIError(w, ae)
			return
		}
		out.AgentID = src.AgentID
		writeJSON(w, http.StatusOK, out)
		return
	}
	writeAPIError(w, apiError(runtime.CodeNotFound, "no such room source"))
}
