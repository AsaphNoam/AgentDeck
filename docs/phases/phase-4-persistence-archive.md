# Phase 4 — Persistence: archive, search, resume, file/command tracking

**Status:** ready to build after Phase 2
**Features:** F9 (session history, search & resume), F10 (file & command tracking)
**Depends on:** Phases 1, 2
**Enables:** Phase 6 (switch-runtime relies on resume); parallelizable with Phases 3 and 5

---

## 1. Goal

Make work durable and recoverable. Persist every session's transcript so a stopped agent survives in a browsable archive, is full-text searchable (including transcript content), and can be resumed with full history and config re-attached to a runtime. Alongside, capture per-agent file edits and shell commands into searchable, copyable tabs.

This phase also delivers the **non-destructive resume** machinery that Phase 6's runtime/backend/model switching reuses.

---

## 2. Scope

### In scope
- Transcript persistence: raw transcript history under `sessions/{agent_id}/`; session/transcript **metadata in `state.db`** (attributed to `agent_id`).
- SQLite **FTS5 search index** (`mattn/go-sqlite3`) populated from the `sessions/` transcripts.
- Archive: list all sessions (active + inactive) with name, role, project, timestamps — a `state.db` query.
- Full-text search across name, role, project, and transcript content via FTS5.
- Resume: restore history + composed config, re-attach a chat runtime, continue the same logical session (stable `agent_id`, new ephemeral `session_id`).
- File & command tracking (F10): capture edited files and run commands (from chat-runtime tool calls and `POST /api/hook`) into `state.db`; per-agent tabs; searchable, copyable; files link to diffs where available.

### Out of scope
- Switch interface/backend/model on a live agent (Phase 6/F7 — built on this phase's resume).
- Messaging (Phase 5), activity map (Phase 7).

---

## 3. Detailed requirements

### 3.1 Transcript persistence (F9)
- Append raw transcript events to `sessions/{agent_id}/` durably as they stream (append-friendly format, e.g. NDJSON; survive crash mid-turn). Enough to reconstruct the chat panel view (assistant text, tool calls/results, diffs, permission outcomes) and the composed config used.
- On write, upsert session/transcript **metadata** (and content) into `state.db` so it is queryable and indexed for FTS5.
- Stopping an agent leaves its transcript intact (removes the running row in `state.db`, keeps `sessions/`).

### 3.2 Archive + search (F9)
- `GET /api/archive?q=...` lists sessions (active and inactive) with name, role, project, created/updated timestamps — a `state.db` query.
- Full-text search over name, role, project, and transcript content via the **SQLite FTS5** index. The index is built from the `sessions/` transcripts and is rebuildable.
- Archive UI: browsable list + search box; clicking a result opens its (read-only when inactive) transcript.

### 3.3 Resume (F9)
- `POST /api/sessions/{id}/resume` → restore full history + composed config (from `state.db` + `sessions/`), call `Runtime.Resume(agent, session_id)`, re-attach the chat runtime, register the in-process MCP server.
- Resume preserves the stable `agent_id`; a fresh ephemeral `session_id` is written to the running row in `state.db`.
- Resumed agent reappears as a live card (Phase 2) with prior transcript intact.
- Implement `Runtime.Resume` for the chat runtime (was stubbed in Phase 1).
- CLI: `agentdeck` launch of an existing identity resumes rather than duplicates.

### 3.4 File & command tracking (F10)
- Capture every file the agent edits and every shell command it runs (from chat-runtime tool calls and `POST /api/hook`) into `state.db`.
- Per-agent tabs: "Files" and "Commands"; searchable and copyable.
- Files link to diffs where a diff is available from the transcript.

---

## 4. REST surface added

```
GET  /api/archive?q=...               search historical sessions
POST /api/sessions/{id}/resume        resume from archive
GET  /api/sessions/{id}/files         tracked files
GET  /api/sessions/{id}/commands      tracked commands
```

---

## 5. Acceptance criteria

- [ ] A stopped agent appears in the archive with name/role/project/timestamps.
- [ ] That agent is findable by a distinctive phrase from its transcript via `?q=`.
- [ ] Resuming it restores the full prior transcript and config and re-attaches a runtime that continues the same logical session.
- [ ] After an agent edits 3 files and runs 2 commands, all 5 appear in the Files/Commands tabs and are copyable.
- [ ] Edited files link to their diffs where a diff exists.
- [ ] Killing the server mid-turn does not lose already-streamed transcript content.

---

## 6. Open questions
- Raw `sessions/` transcript format: single growing JSON vs. NDJSON append log (recommend NDJSON append for crash-safety and streaming reads).
- How much of the ACP raw stream to retain vs. normalized events (retain normalized; consider keeping raw for debugging behind a flag).
- FTS5 index freshness: index synchronously on transcript write vs. a background indexer — pick based on write volume.
