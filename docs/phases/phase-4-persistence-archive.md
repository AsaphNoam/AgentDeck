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
- Transcript persistence to `sessions/{agent_id}/` as turns/events stream (Phase 1/2 transcript events).
- Archive: list all sessions (active + inactive) with name, role, project, timestamps.
- Full-text search across name, role, project, and transcript content.
- Resume: restore history + composed config, re-attach a chat runtime, continue the same logical session (stable `agent_id`, new ephemeral `session_id`).
- File & command tracking (F10): capture edited files and run commands from tool calls/hooks into per-agent tabs; searchable, copyable; files link to diffs where available.

### Out of scope
- Switch interface/backend/model on a live agent (Phase 6/F7 — built on this phase's resume).
- Messaging (Phase 5), activity map (Phase 7).

---

## 3. Detailed requirements

### 3.1 Transcript persistence (F9)
- Append transcript events to `sessions/{agent_id}/` durably as they stream (one file per session or per-turn append log — choose append-friendly format; survive crash mid-turn).
- Persist enough to reconstruct the chat panel view (assistant text, tool calls/results, diffs, permission outcomes) and the composed config used.
- Stopping an agent leaves its transcript intact (removes `running/`, keeps `sessions/`).

### 3.2 Archive + search (F9)
- `GET /api/archive?q=...` lists sessions (active and inactive) with name, role, project, created/updated timestamps.
- Full-text search over name, role, project, and transcript content. Define an index strategy (on-the-fly scan acceptable at small scale; note a path to an index if it grows).
- Archive UI: browsable list + search box; clicking a result opens its (read-only when inactive) transcript.

### 3.3 Resume (F9)
- `POST /api/sessions/{id}/resume` → restore full history + composed config, call `Runtime.Resume(agent, session_id)`, re-attach the chat runtime, register MCP messaging (Phase 5) if present.
- Resume preserves the stable `agent_id`; a fresh ephemeral `session_id` is written to `running/`.
- Resumed agent reappears as a live card (Phase 2) with prior transcript intact.
- Implement `Runtime.Resume` for the chat runtime (was stubbed in Phase 1).
- CLI: `agentdeck` launch of an existing identity resumes rather than duplicates.

### 3.4 File & command tracking (F10)
- Capture every file the agent edits and every shell command it runs from tool calls (chat runtime) and/or hooks.
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
- Search index: scan-on-query vs. maintained index — depends on expected session volume.
- Transcript file format: single growing JSON vs. NDJSON append log (recommend NDJSON append for crash-safety and streaming reads).
- How much of the ACP raw stream to retain vs. normalized events (retain normalized; consider keeping raw for debugging behind a flag).
