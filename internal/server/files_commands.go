package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/runtime"
)

// trackedFile is one row from tracked_files, as returned by GET /api/sessions/{id}/files.
type trackedFile struct {
	Path      string `json:"path"`
	EditCount int    `json:"edit_count"`
	FirstSeq  int64  `json:"first_seq"`
	LastSeq   int64  `json:"last_seq"`
	FirstTs   string `json:"first_ts"`
	LastTs    string `json:"last_ts"`
	HasDiff   bool   `json:"has_diff"`
	DiffRefs  any    `json:"diff_refs"` // []{"seq":N,"tool_call_id":"tc_3"}
}

// trackedCommand is one row from tracked_commands, as returned by GET /api/sessions/{id}/commands.
type trackedCommand struct {
	Command    string `json:"command"`
	Seq        int64  `json:"seq"`
	Ts         string `json:"ts"`
	ToolCallID string `json:"tool_call_id"`
	ExitStatus string `json:"exit_status"`
	ExitError  string `json:"exit_error"`
}

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.stateStore.ReadAgent(id); err != nil {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
		return
	}
	files, err := queryTrackedFiles(s.stateStore.DB(), id)
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agent_id": id, "files": files})
}

func (s *Server) handleCommands(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.stateStore.ReadAgent(id); err != nil {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
		return
	}
	cmds, err := queryTrackedCommands(s.stateStore.DB(), id)
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agent_id": id, "commands": cmds})
}

// handleFileSearch serves GET /api/sessions/{id}/file-search?q=<text> for the
// composer `@` picker (TS-03.R24, FS-03.R30/R32/R34). It resolves the known chat
// session's working directory on the server and returns bounded, ranked relative
// paths; the query is decoded as text, never joined as a caller-chosen path. An
// unknown agent is 404 and a non-chat agent is a typed conflict; a missing or
// unreadable working directory returns an empty list so the composer can still send.
func (s *Server) handleFileSearch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	agent, err := s.stateStore.ReadAgent(id)
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
		return
	}
	if agent.Interface != "chat" {
		writeAPIError(w, apiError(runtime.CodeConflict, "file search is only available for chat agents"))
		return
	}
	// A stopped chat agent keeps its durable session row, but the composer that
	// consumes this read exists only for a running agent. Gate on the running
	// record so a stopped agent yields the shared typed runtime error instead of
	// listing its former working directory (TS-03.R24, INV §1).
	if _, err := s.stateStore.ReadRunning(id); err != nil {
		writeAPIError(w, apiError(runtime.CodeAgentNotRunning, "agent is not running"))
		return
	}
	files := []string{}
	if snap, serr := s.stateStore.ReadSession(id); serr == nil {
		cwd := snap.Cwd
		if expanded, eerr := config.ExpandTilde(cwd); eerr == nil {
			cwd = expanded
		}
		files = searchWorkspaceFiles(cwd, r.URL.Query().Get("q"), fileSearchResultCap)
	}
	writeJSON(w, http.StatusOK, map[string]any{"agent_id": id, "files": files})
}

// handleAvailableCommands serves GET /api/sessions/{id}/available-commands for the
// composer `#` picker (TS-03.R24, FS-03.R33). It returns the owning chat runtime's
// latest ACP command snapshot without asking the adapter to refresh and without
// creating any persisted row or transcript event. An unknown agent is 404; a
// stopped or non-chat agent yields the shared typed runtime/conflict error so the
// composer treats it as "no commands" and stays usable (FS-03.R32).
func (s *Server) handleAvailableCommands(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.stateStore.ReadAgent(id); err != nil {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
		return
	}
	cmds, err := s.registry.Commands(id)
	if err != nil {
		writeAPIError(w, commandsSnapshotError(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agent_id": id, "commands": cmds})
}

// commandsSnapshotError maps a command-snapshot read error to the typed API error
// (TS-03.R24): an agent with no live handle (never started, stopped, crashed) is
// agent_not_running, and a non-chat owner is a conflict.
func commandsSnapshotError(err error) *runtime.APIError {
	switch {
	case errors.Is(err, runtime.ErrNoHandle):
		return apiError(runtime.CodeAgentNotRunning, "agent is not running")
	case errors.Is(err, runtime.ErrNotImplemented):
		return apiError(runtime.CodeConflict, "available commands are only provided by chat agents")
	default:
		return apiError(runtime.CodeInternal, err.Error())
	}
}

func queryTrackedFiles(db *sql.DB, agentID string) ([]trackedFile, error) {
	rows, err := db.Query(`
SELECT path, edit_count, first_seq, last_seq, first_ts, last_ts, has_diff, diff_refs
FROM tracked_files
WHERE agent_id = ?
ORDER BY last_ts DESC`, agentID)
	if err != nil {
		return nil, fmt.Errorf("files: query: %w", err)
	}
	defer rows.Close()
	var out []trackedFile
	for rows.Next() {
		var f trackedFile
		var hasDiff int
		var diffRefsRaw string
		if err := rows.Scan(&f.Path, &f.EditCount, &f.FirstSeq, &f.LastSeq, &f.FirstTs, &f.LastTs, &hasDiff, &diffRefsRaw); err != nil {
			return nil, fmt.Errorf("files: scan: %w", err)
		}
		f.HasDiff = hasDiff == 1
		var diffRefs []map[string]any
		if err := json.Unmarshal([]byte(diffRefsRaw), &diffRefs); err != nil || diffRefs == nil {
			diffRefs = []map[string]any{}
		}
		f.DiffRefs = diffRefs
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("files: rows: %w", err)
	}
	if out == nil {
		out = []trackedFile{}
	}
	return out, nil
}

func queryTrackedCommands(db *sql.DB, agentID string) ([]trackedCommand, error) {
	rows, err := db.Query(`
SELECT command, seq, ts, tool_call_id, exit_status, exit_error
FROM tracked_commands
WHERE agent_id = ?
ORDER BY seq DESC`, agentID)
	if err != nil {
		return nil, fmt.Errorf("commands: query: %w", err)
	}
	defer rows.Close()
	var out []trackedCommand
	for rows.Next() {
		var c trackedCommand
		if err := rows.Scan(&c.Command, &c.Seq, &c.Ts, &c.ToolCallID, &c.ExitStatus, &c.ExitError); err != nil {
			return nil, fmt.Errorf("commands: scan: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("commands: rows: %w", err)
	}
	if out == nil {
		out = []trackedCommand{}
	}
	return out, nil
}
