package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

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
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("files: close: %w", err)
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
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("commands: close: %w", err)
	}
	if out == nil {
		out = []trackedCommand{}
	}
	return out, nil
}
