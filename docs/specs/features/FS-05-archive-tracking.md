# FS-05 — Session archive, search, resume & tracking

**Status:** Current
**Code:** `internal/archive/`, `internal/index/`, `internal/state/` (sessions, tracked_files, tracked_commands), `internal/server/` (`archive.go`, `resume.go`, `files_commands.go`, `sessions.go`), `ui/src/features/archive/`, `ui/src/components/chat/{FilesTab,CommandsTab}.tsx` · **Journeys:** J7, J8
**Absorbed:** exact source mapping in the [phase archive manifest](../../archive/phases/README.md)

## 1. Purpose

Every session AgentDeck has ever run — active or stopped — is durably recorded, browsable, and
searchable, and any inactive session can be resumed with its history and composed config restored.
Alongside each session, AgentDeck rolls up the files the agent edited and the shell commands it ran.
This spec governs the archive list/search surface (`GET /api/archive`), resume-from-archive, the
read-only transcript view of an archived session, and the per-session file/command tracking tabs. It
does **not** cover live launch/stop/switch (FS-01), the live chat panel (FS-03), or config federation
resume semantics (FS-08).

## 2. Behavior

Requirements are user- and API-observable. R-item numbering is continuous through §4.

### 2.1 Archive listing

- **R1.** `GET /api/archive` returns every session AgentDeck has recorded — both currently-running
  and stopped — as a `results` array with `total`, `limit`, and `offset`. Each result carries
  `agent_id`, `name`, `role`, `project`, `backend`, `model`, `interface`, optional `group`,
  `created_at`, `updated_at`, `turn_count`, `files_touched`, `commands_run`, and `active` (true iff
  the agent currently has a running row).
- **R2.** With no `q`, results are ordered by `updated_at` descending, then `agent_id` — most
  recently active first. `total` is the full count of matching sessions, independent of the returned
  page.
- **R3.** The `active` query parameter filters the listing: `active=true` returns only running
  sessions, `active=false` only stopped ones, absent returns both. Any other value is rejected
  `422 validation`.
- **R4.** `results` is always a JSON array, never `null`, including for an empty archive (an empty
  archive returns `{"total":0,...,"results":[]}`).

### 2.2 Search

- **R5.** A non-empty `q` searches across session metadata (name, role, project, backend, model,
  group) **and** transcript content. On a build compiled with the `sqlite_fts5` tag, search uses the
  `sessions_fts` full-text index: query terms are matched as quoted FTS tokens combined with AND
  semantics (every term must be present), results are ranked by BM25 with metadata columns weighted
  above transcript content, then by `updated_at` descending.
- **R6.** On a build compiled **without** the `sqlite_fts5` tag (the shipped no-FTS5 fallback path),
  search transparently falls back to a metadata `LIKE` substring filter over name, role, project, and
  backend, with AND semantics across terms. Transcript content is not searchable in this build; the
  query still returns correctly-filtered metadata results rather than an error. The fallback also
  triggers on a tagged binary whenever the FTS5 module is reported missing at runtime.
- **R7.** Each search result carries `matched_in`: the array contains `"metadata"` when every query
  term appears within the combined metadata fields, and `"transcript"` when every query term appears
  within transcript content. Both may be present; the array may be empty when the terms are satisfied
  only by being split across the two field groups.
- **R8.** On the FTS5 path, a transcript-matching result carries `snippet`: a short excerpt of the
  matched transcript content with an ellipsis marker. The fallback path returns no snippet.
- **R9.** Search accepts double-quoted phrases in `q` as single terms; unquoted whitespace separates
  terms.

### 2.3 Result limits & pagination

- **R10.** `GET /api/archive` accepts `limit` (default 50, valid range 1–200; out-of-range →
  `422 validation`) and `offset` (default 0, must be ≥ 0; negative → `422 validation`). The archive
  layer additionally clamps any limit above 200 down to 200. `total` always reports the full match
  count so a client can page through with `offset`.

### 2.4 Resume from archive

- **R11.** `POST /api/sessions/{id}/resume` on an inactive session re-attaches a runtime under the
  same stable `agent_id`, restoring the session from its frozen snapshot: `cwd`, `system_prompt`,
  `skip_permissions`, `add_dirs`, the last upstream session id (so the agent CLI continues the same
  logical conversation), and the frozen launch/federation config. Backend, model, and interface are
  taken from the live identity row (which switch-runtime keeps current), with optional request-body
  overrides.
- **R12.** After resume, the prior transcript is intact and the session's tracked files, commands,
  messages, and search index continue to accumulate under the same `agent_id` rather than starting a
  new session.
- **R13.** Resuming an already-running session returns `409 conflict`; resuming a session id with no
  persisted snapshot returns `422 validation`; an unknown `agent_id` returns `404 not_found`.

### 2.5 Archived transcript viewing

- **R14.** An inactive session opens in a **read-only** archived view that renders its recorded
  transcript (assistant text, tool calls, tool results, file diffs, permission requests) via
  `GET /api/sessions/{id}/transcript`, with no composer and no ability to send a prompt. The view
  exposes a Resume control (R11). An `active` session instead opens the live chat panel (FS-03).

### 2.6 File & command tracking rollups

- **R15.** For each session, `GET /api/sessions/{id}/files` returns every distinct file path the
  agent touched, each with `edit_count`, first/last transcript seq, first/last timestamp, a
  `has_diff` flag, and `diff_refs` (references to the diff events for that file). Files are ordered
  most-recently-touched first. The list is always an array, never `null`.
- **R16.** `GET /api/sessions/{id}/commands` returns every shell command the agent ran, each with
  `command`, transcript `seq`, timestamp, `tool_call_id`, `exit_status`, and `exit_error`, ordered
  newest-first. The list is always an array, never `null`.
- **R17.** Tracking has two sources. Chat (ACP) sessions capture file edits from file-editing tool
  calls and diff events, and commands from command-running tool calls, with tool results updating a
  command's exit status. Terminal sessions capture the same via `POST /api/hook`
  (`CaptureHookFile`/`CaptureHookCommand`), which allocates a synthetic seq when the hook omits one.
- **R18.** The session's `files_touched` and `commands_run` rollup counts reported in the archive
  listing (R1) reflect the distinct tracked-file count and tracked-command count. The chat path
  refreshes them at turn boundaries; the hook path refreshes them directly on capture.
- **R19.** Both requests return `404 not_found` for an unknown `agent_id`.
- **R20.** Both Files and Commands lists are exposed in the chat panel as tabs; their contents are
  copyable by the user.

## 3. States & transitions

- **Session persistence.** A session row is written at launch and updated as the agent runs (identity,
  frozen config, rollup counts, `last_seq`, `updated_at`). It survives agent stop and server restart;
  it is the durable record the archive lists.
- **active ↔ inactive.** A session is `active` while a running row exists for its `agent_id`, else
  `inactive`. Stopping an agent flips it to inactive (still archived); resume (R11) flips it back to
  active under the same id. The archive listing derives `active` per-row from the running registry at
  query time.
- **Search-index lifecycle.** The full-text content buffer for a session is (re)built in memory and
  rewritten to `sessions_fts` at each turn boundary; on server restart or resume it is re-seeded from
  the durable `sessions_fts` row before the next flush so previously-indexed transcript text is not
  lost. A crash-truncated final partial turn is preserved by the read-path reindex.

## 4. Edge cases & errors

- **R21.** An empty archive and empty transcript/file/command lists serialize as empty arrays, never
  `null` (guards the null-hostility class that once dead-ended the UI).
- **R22.** A malformed `limit`/`offset`/`active` query value is rejected with `422 validation` and a
  message naming the constraint, not a silent default.
- **R23.** On the untagged fallback build, an archive search never surfaces a raw `no such module:
  fts5` error to the client; the fallback (R6) is engaged instead and stale pre-search rows are
  replaced by the filtered result set.
- **R24.** A file path recorded as an absolute path under the session cwd is displayed relative to the
  cwd; paths outside the cwd are displayed as their cleaned absolute form.

## 5. Acceptance criteria

- **A1** (R1–R3) — Archive lists active and inactive sessions with metadata and honors the `active`
  filter: `internal/archive/archive_test.go::TestArchiveListAndActiveFilter`,
  `internal/server/server_test.go::TestArchiveListHandler`.
- **A2** (R4, R21) — Empty archive/lists marshal as `[]`:
  `internal/archive/archive_no_fts5_test.go::TestEmptyArchiveMarshalsResultsArray`,
  `internal/server/files_commands_test.go::TestFilesEndpointEmptyList` and
  `TestCommandsEndpointEmptyList`.
- **A3** (R5, R7, R8, R10) — FTS5 search matches metadata and transcript, sets `matched_in`, and
  paginates: `internal/archive/archive_fts_test.go::TestArchiveSearchFTSMetadataTranscriptAndPagination`.
- **A4** (R5, R9) — Multi-term AND search semantics:
  `internal/archive/archive_fts_test.go::TestArchiveSearchANDSemantics`.
- **A5** (R6, R23) — Untagged build falls back to LIKE metadata search instead of erroring:
  `internal/archive/archive_no_fts5_test.go::TestSearchFallbackFiltersMetadata`.
- **A6** (R11–R13) — Resume restores frozen config and history under the same id, and rejects
  already-running/missing/unknown: `internal/server/resume_test.go::TestResumeHappyPath`,
  `TestResumeAlreadyRunning`, `TestResumeNoPersistedSession`, `TestResumeUnknownAgent`;
  `internal/server/switch_test.go::TestComposeResumeSpecCarriesFrozenLaunchConfig`.
- **A7** (R12, R14, and §3 index lifecycle) — Transcript/FTS content survives restart and resume:
  `internal/index/indexer_fts_test.go::TestResumeAfterRestartPreservesFTSContent`,
  `internal/index/reindex_test.go::TestReindexPreservesFinalPartialTurn`.
- **A8** (R15–R18) — File/command rollups from both ACP and hook sources:
  `internal/server/files_commands_test.go::TestFilesEndpointRows`, `TestCommandsEndpointRows`,
  `TestHookCommandCapture`, `TestHookCommandCaptureMultiple`.
- **A9** (R19) — Unknown-agent tracking requests 404:
  `internal/server/files_commands_test.go::TestFilesEndpointUnknownAgent`, `TestCommandsEndpointUnknownAgent`.
- **A10** (R1, R5, R11, R20) — End-to-end archive + search + resume + tracking through the running UI
  on both the FTS5 and untagged builds: journey **J8** (archive & search), **J7** (stop/resume/switch)
  in `docs/features/USABILITY-REVIEW.md`.

## 6. Deviations & open decisions

- **Unbounded transcript indexing.** Full-text indexing keeps
  the entire transcript for a session in memory and rewrites the whole `sessions_fts` row at every turn
  boundary, so all previously-streamed content stays searchable (`internal/index/indexer.go`
  `addContent`). Very long sessions make this progressively more expensive in memory and write cost; a
  chunked/segmented index would bound the cost at the price of the "every phrase ever streamed is
  searchable" guarantee. This is deliberate current behavior; a chunked replacement starts with a
  TS-02/FS-05 delta.
- **No-FTS5 fallback is intentionally lossy for transcript search.** The untagged build (R6) can only
  match session metadata; transcript-body search requires the `sqlite_fts5` build tag. This is shipped,
  supported behavior, not a bug — the tag is present on every real build path (`make build`,
  `install.sh`), so shipped binaries retain transcript search; the fallback protects source/dev builds
  and any runtime where the FTS5 module is unavailable.
- **`matched_in` is empty for terms split across field groups.** Per R7, a field group is reported only
  when it contains *every* query term; a query whose terms are satisfied jointly by metadata and
  transcript returns a valid hit with an empty `matched_in`. Current shipped behavior; a tracked
  advisory covers refining per-field coverage.

## 7. Traceability

- **Archive/search:** `internal/archive/archive.go` (`Search`, `search`, `searchFallback`,
  `matchedIn`, `isFTS5Missing`); handler `internal/server/archive.go`.
- **Index & tracking:** `internal/index/indexer.go` (`UpsertSessionMeta`, `OnEvent`, `trackEvent`,
  `upsertFile`, `CaptureHookFile`, `CaptureHookCommand`, `bumpRollups`), `internal/index/reindex.go`;
  tracking endpoints `internal/server/files_commands.go`.
- **Resume:** `internal/server/resume.go`, `internal/server/switch.go` (`composeResumeSpec`).
- **UI:** `ui/src/features/archive/ArchivePage.tsx` (list + search), `ArchiveAgentPage.tsx` (read-only
  transcript + Resume), `ui/src/components/chat/{FilesTab,CommandsTab}.tsx`.
- **Key regression tests:** `TestSearchFallbackFiltersMetadata`, `TestArchiveSearchFTSMetadataTranscriptAndPagination`,
  `TestResumeAfterRestartPreservesFTSContent`, `TestReindexPreservesFinalPartialTurn`,
  `TestHookCommandCapture`.
