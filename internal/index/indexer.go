package index

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/strutil"
)

// execer is satisfied by both *sql.DB and *sql.Tx so rollup updates can run
// either standalone or inside an existing transaction.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

type Indexer struct {
	db      *sql.DB
	mu      sync.Mutex
	content map[string][]string
}

func New(db *sql.DB) *Indexer {
	return &Indexer{db: db, content: map[string][]string{}}
}

func (ix *Indexer) UpsertSessionMeta(agentID string, meta runtime.SessionMetaData) error {
	if agentID == "" {
		return fmt.Errorf("index: agent id is required")
	}
	// created_at/updated_at must be a real timestamp; fall back to now (never the
	// session id, which would sort lexically after real RFC3339 values and corrupt
	// the MAX(updated_at) guard below).
	now := meta.CreatedAt
	if now == "" {
		now = time.Now().UTC().Format(time.RFC3339)
	}
	envKeys, err := json.Marshal(meta.EnvKeys)
	if err != nil {
		return fmt.Errorf("index: marshal env keys: %w", err)
	}
	addDirs, err := json.Marshal(meta.AddDirs)
	if err != nil {
		return fmt.Errorf("index: marshal add dirs: %w", err)
	}
	launchConfig := "{}"
	if len(meta.LaunchConfig) > 0 {
		launchConfig = string(meta.LaunchConfig)
	}
	// Put the session upsert and metadata document replacement in one
	// transaction so both succeed or both roll back (TS-02.R16, INV §15).
	tx, err := ix.db.Begin()
	if err != nil {
		return fmt.Errorf("index: begin metadata transaction: %w", err)
	}
	defer tx.Rollback()
	_, err = tx.Exec(`
INSERT INTO sessions(agent_id, name, role, project, backend, model, interface, grp, cwd, system_prompt, env_keys, skip_permissions, add_dirs, launch_config_json, last_session_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(agent_id) DO UPDATE SET
  name=excluded.name,
  role=excluded.role,
  project=excluded.project,
  backend=excluded.backend,
  model=excluded.model,
  interface=excluded.interface,
  grp=excluded.grp,
  cwd=excluded.cwd,
  system_prompt=excluded.system_prompt,
  env_keys=excluded.env_keys,
  skip_permissions=excluded.skip_permissions,
  add_dirs=excluded.add_dirs,
  launch_config_json=excluded.launch_config_json,
  last_session_id=excluded.last_session_id,
  updated_at=MAX(excluded.updated_at, sessions.updated_at)`,
		agentID, meta.Name, meta.Role, meta.Project, meta.Backend, meta.Model, meta.Interface,
		meta.Group, meta.Cwd, meta.SystemPrompt, string(envKeys), meta.SkipPermissions, string(addDirs), launchConfig, meta.SessionID, now, now)
	if err != nil {
		return fmt.Errorf("index: upsert session meta: %w", err)
	}
	if err := replaceMetadataDocument(tx, agentID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("index: commit metadata transaction: %w", err)
	}
	return nil
}

func (ix *Indexer) OnEvent(agentID string, ev runtime.Event) error {
	if agentID == "" {
		agentID = ev.AgentID
	}
	if agentID == "" {
		return fmt.Errorf("index: agent id is required")
	}
	if ev.Type == runtime.EvSessionMeta {
		var meta runtime.SessionMetaData
		if err := json.Unmarshal(ev.Data, &meta); err != nil {
			return fmt.Errorf("index: decode session_meta: %w", err)
		}
		return ix.UpsertSessionMeta(agentID, meta)
	}
	text, err := searchableText(ev)
	if err != nil {
		return err
	}
	if text != "" {
		ix.addContent(agentID, text)
	}
	if err := ix.trackEvent(agentID, ev); err != nil {
		return err
	}
	if ev.Seq > 0 {
		_, err := ix.db.Exec(`
UPDATE sessions
SET event_count = event_count + 1,
    last_seq = CASE WHEN last_seq < ? THEN ? ELSE last_seq END,
    updated_at = CASE WHEN updated_at < ? THEN ? ELSE updated_at END
WHERE agent_id = ?`, ev.Seq, ev.Seq, ev.Ts, ev.Ts, agentID)
		if err != nil {
			return fmt.Errorf("index: update event counters: %w", err)
		}
	}
	return nil
}

func (ix *Indexer) OnTurnEnd(agentID string, rollup runtime.TurnRollup) error {
	return ix.flush(agentID, rollup, true, "turn")
}

// FlushContent persists indexed content that arrives outside an ACP turn, such
// as a durable dashboard annotation. It intentionally does not increase the
// turn count.
func (ix *Indexer) FlushContent(agentID string, lastSeq int64, updatedAt string) error {
	return ix.flush(agentID, runtime.TurnRollup{LastSeq: lastSeq, UpdatedAt: updatedAt}, false, "flush")
}

func (ix *Indexer) flush(agentID string, rollup runtime.TurnRollup, countTurn bool, documentKind string) error {
	if agentID == "" {
		return fmt.Errorf("index: agent id is required")
	}
	ix.mu.Lock()
	defer ix.mu.Unlock()
	content := strings.Join(ix.content[agentID], "\n")

	tx, err := ix.db.Begin()
	if err != nil {
		return fmt.Errorf("index: begin turn flush: %w", err)
	}
	defer tx.Rollback()

	if content != "" {
		documentID := fmt.Sprintf("%s:%d", documentKind, rollup.LastSeq)
		if err := insertTranscriptDocument(tx, agentID, documentID, content); err != nil {
			return err
		}
	}
	turnInc := 0
	if countTurn {
		turnInc = 1
	}
	if _, err := tx.Exec(`
UPDATE sessions
SET turn_count = turn_count + ?,
    last_seq = CASE WHEN last_seq < ? THEN ? ELSE last_seq END,
    last_context_pct = ?,
    updated_at = CASE WHEN ? <> '' THEN ? ELSE updated_at END,
    files_touched = (SELECT COUNT(*) FROM tracked_files WHERE agent_id = ?),
    commands_run = (SELECT COUNT(*) FROM tracked_commands WHERE agent_id = ?)
WHERE agent_id = ?`,
		turnInc, rollup.LastSeq, rollup.LastSeq, rollup.LastContextPct, rollup.UpdatedAt, rollup.UpdatedAt, agentID, agentID, agentID); err != nil {
		return fmt.Errorf("index: update turn rollup: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("index: commit turn flush: %w", err)
	}
	delete(ix.content, agentID)
	return nil
}

func (ix *Indexer) addContent(agentID, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	ix.mu.Lock()
	defer ix.mu.Unlock()
	ix.content[agentID] = append(ix.content[agentID], text)
}

func replaceMetadataDocument(tx *sql.Tx, agentID string) error {
	var name, role, project, grp, model, backend string
	if err := tx.QueryRow(`SELECT name, role, project, grp, model, backend FROM sessions WHERE agent_id = ?`, agentID).
		Scan(&name, &role, &project, &grp, &model, &backend); err != nil {
		return fmt.Errorf("index: read session for fts: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM sessions_fts WHERE agent_id = ? AND document_id = 'metadata'`, agentID); err != nil {
		return fmt.Errorf("index: delete metadata document: %w", err)
	}
	if _, err := tx.Exec(`
INSERT INTO sessions_fts(agent_id, document_id, name, role, project, grp, model, backend, content)
VALUES (?, 'metadata', ?, ?, ?, ?, ?, ?, '')`, agentID, name, role, project, grp, model, backend); err != nil {
		return fmt.Errorf("index: insert metadata document: %w", err)
	}
	return nil
}

func insertTranscriptDocument(tx *sql.Tx, agentID, documentID, content string) error {
	if _, err := tx.Exec(`
INSERT INTO sessions_fts(agent_id, document_id, name, role, project, grp, model, backend, content)
VALUES (?, ?, '', '', '', '', '', '', ?)`, agentID, documentID, content); err != nil {
		return fmt.Errorf("index: insert transcript document: %w", err)
	}
	return nil
}

func searchableText(ev runtime.Event) (string, error) {
	switch ev.Type {
	case runtime.EvUserPrompt:
		var d runtime.UserPromptData
		if err := json.Unmarshal(ev.Data, &d); err != nil {
			return "", fmt.Errorf("index: user_text: %w", err)
		}
		return d.Text, nil
	case runtime.EvAssistantText:
		var d runtime.AssistantTextData
		if err := json.Unmarshal(ev.Data, &d); err != nil {
			return "", fmt.Errorf("index: assistant_text: %w", err)
		}
		return d.Delta, nil
	case runtime.EvToolCall:
		var d runtime.ToolCallData
		if err := json.Unmarshal(ev.Data, &d); err != nil {
			return "", fmt.Errorf("index: tool_call: %w", err)
		}
		return strings.TrimSpace(d.Name + " " + string(d.Args)), nil
	case runtime.EvToolResult:
		var d runtime.ToolResultData
		if err := json.Unmarshal(ev.Data, &d); err != nil {
			return "", fmt.Errorf("index: tool_result: %w", err)
		}
		return string(d.Content), nil
	case runtime.EvDiff:
		var d runtime.DiffData
		if err := json.Unmarshal(ev.Data, &d); err != nil {
			return "", fmt.Errorf("index: diff: %w", err)
		}
		return strings.TrimSpace(d.Path + " " + d.NewText), nil
	case runtime.EvPermissionRequest:
		var d runtime.PermissionRequestData
		if err := json.Unmarshal(ev.Data, &d); err != nil {
			return "", fmt.Errorf("index: permission_request: %w", err)
		}
		return d.Reason, nil
	case runtime.EvAnnotation:
		var d runtime.AnnotationData
		if err := json.Unmarshal(ev.Data, &d); err != nil {
			return "", fmt.Errorf("index: annotation: %w", err)
		}
		parts := make([]string, 0, len(d.Annotations)*2+1)
		for _, a := range d.Annotations {
			parts = append(parts, a.Excerpt, a.Instruction)
		}
		parts = append(parts, d.OverallInstruction)
		return strings.TrimSpace(strings.Join(parts, " ")), nil
	default:
		return "", nil
	}
}

func (ix *Indexer) trackEvent(agentID string, ev runtime.Event) error {
	switch ev.Type {
	case runtime.EvToolCall:
		var d runtime.ToolCallData
		if err := json.Unmarshal(ev.Data, &d); err != nil {
			return fmt.Errorf("index: track tool_call: %w", err)
		}
		if p := pathFromArgs(d.Args); p != "" && isFileTool(d.Name) {
			if err := ix.upsertFile(agentID, p, ev, d.ToolCallID, false); err != nil {
				return err
			}
		}
		if cmd := commandFromArgs(d.Args); cmd != "" && isCommandTool(d.Name) {
			if _, err := ix.db.Exec(`
INSERT OR REPLACE INTO tracked_commands(agent_id, seq, ts, tool_call_id, command, exit_status, exit_error)
VALUES (?, ?, ?, ?, ?, 'in_progress', '')`, agentID, ev.Seq, ev.Ts, d.ToolCallID, cmd); err != nil {
				return fmt.Errorf("index: insert command: %w", err)
			}
		}
	case runtime.EvToolResult:
		var d runtime.ToolResultData
		if err := json.Unmarshal(ev.Data, &d); err != nil {
			return fmt.Errorf("index: track tool_result: %w", err)
		}
		if _, err := ix.db.Exec(`
UPDATE tracked_commands
SET exit_status = ?, exit_error = ?
WHERE agent_id = ? AND tool_call_id = ?`, strutil.FirstNonEmpty(d.Status, "completed"), d.Error, agentID, d.ToolCallID); err != nil {
			return fmt.Errorf("index: update command result: %w", err)
		}
	case runtime.EvDiff:
		var d runtime.DiffData
		if err := json.Unmarshal(ev.Data, &d); err != nil {
			return fmt.Errorf("index: track diff: %w", err)
		}
		if d.Path != "" {
			if err := ix.upsertFile(agentID, d.Path, ev, d.ToolCallID, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func (ix *Indexer) upsertFile(agentID, p string, ev runtime.Event, toolCallID string, hasDiff bool) error {
	display, abs := ix.normalizePath(agentID, p)
	diffRefs := "[]"
	if hasDiff {
		ref, _ := json.Marshal([]map[string]any{{"seq": ev.Seq, "tool_call_id": toolCallID}})
		diffRefs = string(ref)
	}
	_, err := ix.db.Exec(`
INSERT INTO tracked_files(agent_id, path, abs_path, edit_count, first_seq, last_seq, first_ts, last_ts, has_diff, diff_refs)
VALUES (?, ?, ?, 1, ?, ?, ?, ?, ?, ?)
ON CONFLICT(agent_id, path) DO UPDATE SET
  edit_count = tracked_files.edit_count + 1,
  last_seq = excluded.last_seq,
  last_ts = excluded.last_ts,
  has_diff = CASE WHEN excluded.has_diff = 1 THEN 1 ELSE tracked_files.has_diff END,
  diff_refs = CASE
    WHEN excluded.has_diff = 1 AND tracked_files.diff_refs = '[]' THEN excluded.diff_refs
    WHEN excluded.has_diff = 1 THEN substr(tracked_files.diff_refs, 1, length(tracked_files.diff_refs)-1) || ',' || substr(excluded.diff_refs, 2)
    ELSE tracked_files.diff_refs
  END`,
		agentID, display, abs, ev.Seq, ev.Seq, ev.Ts, ev.Ts, boolInt(hasDiff), diffRefs)
	if err != nil {
		return fmt.Errorf("index: upsert file: %w", err)
	}
	return nil
}

func (ix *Indexer) normalizePath(agentID, p string) (display, abs string) {
	cwd := ""
	_ = ix.db.QueryRow(`SELECT cwd FROM sessions WHERE agent_id = ?`, agentID).Scan(&cwd)
	if filepath.IsAbs(p) {
		abs = filepath.Clean(p)
		if cwd != "" {
			if rel, err := filepath.Rel(cwd, abs); err == nil && !strings.HasPrefix(rel, "..") {
				return filepath.ToSlash(rel), abs
			}
		}
		return abs, abs
	}
	display = filepath.ToSlash(filepath.Clean(p))
	if cwd != "" {
		abs = filepath.Join(cwd, p)
	} else {
		abs = p
	}
	return display, abs
}

func pathFromArgs(raw json.RawMessage) string {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return ""
	}
	for _, key := range []string{"path", "file_path", "filepath"} {
		if v, ok := args[key].(string); ok {
			return v
		}
	}
	return ""
}

func commandFromArgs(raw json.RawMessage) string {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return ""
	}
	for _, key := range []string{"command", "cmd", "script"} {
		if v, ok := args[key].(string); ok {
			return v
		}
	}
	return ""
}

func isFileTool(name string) bool {
	switch strings.ToLower(name) {
	case "edit", "write", "multiedit", "notebookedit", "create":
		return true
	default:
		return false
	}
}

func isCommandTool(name string) bool {
	switch strings.ToLower(name) {
	case "bash", "shell", "run", "terminal":
		return true
	default:
		return false
	}
}

// CaptureHookFile records a file-edit event from POST /api/hook (terminal runtime producer).
// path is the edited file path; ts is RFC3339; seq is an optional transcript seq (0 if unknown).
func (ix *Indexer) CaptureHookFile(agentID, path, ts string, seq int64) error {
	if agentID == "" || path == "" {
		return fmt.Errorf("index: agent_id and path are required")
	}
	ev := runtime.Event{AgentID: agentID, Seq: seq, Ts: ts}
	if err := ix.upsertFile(agentID, path, ev, "", false); err != nil {
		return err
	}
	return bumpRollups(ix.db, agentID)
}

// CaptureHookCommand records a command event from POST /api/hook (terminal runtime producer).
// command is the shell command; ts is RFC3339; seq is an optional transcript seq (0 if unknown).
func (ix *Indexer) CaptureHookCommand(agentID, command, ts, toolCallID string, seq int64) error {
	if agentID == "" || command == "" {
		return fmt.Errorf("index: agent_id and command are required")
	}
	tx, err := ix.db.Begin()
	if err != nil {
		return fmt.Errorf("index: begin capture command: %w", err)
	}
	defer tx.Rollback()
	// Hook callers may omit seq (terminal producer with no transcript seq). seq<=0
	// would collide on the (agent_id, seq) PK and overwrite prior commands, so
	// allocate the next free seq for this agent instead.
	if seq <= 0 {
		if err := tx.QueryRow(`SELECT COALESCE(MAX(seq), 0) + 1 FROM tracked_commands WHERE agent_id = ?`, agentID).Scan(&seq); err != nil {
			return fmt.Errorf("index: next command seq: %w", err)
		}
	}
	if toolCallID == "" {
		toolCallID = fmt.Sprintf("hook_%d", seq)
	}
	if _, err := tx.Exec(`
INSERT OR REPLACE INTO tracked_commands(agent_id, seq, ts, tool_call_id, command, exit_status, exit_error)
VALUES (?, ?, ?, ?, ?, 'completed', '')`, agentID, seq, ts, toolCallID, command); err != nil {
		return fmt.Errorf("index: capture hook command: %w", err)
	}
	if err := bumpRollups(tx, agentID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("index: commit capture command: %w", err)
	}
	return nil
}

// bumpRollups refreshes the sessions row's files_touched/commands_run counts from
// the tracked_* tables. The runtime hot path maintains these on turn_end, but the
// hook-capture path has no turn boundary, so it must refresh them directly. A
// no-op (0 rows) when the agent has no sessions row yet (e.g. terminal-only).
func bumpRollups(db execer, agentID string) error {
	if _, err := db.Exec(`
UPDATE sessions
SET files_touched = (SELECT COUNT(*) FROM tracked_files WHERE agent_id = ?),
    commands_run  = (SELECT COUNT(*) FROM tracked_commands WHERE agent_id = ?)
WHERE agent_id = ?`, agentID, agentID, agentID); err != nil {
		return fmt.Errorf("index: bump rollups: %w", err)
	}
	return nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
