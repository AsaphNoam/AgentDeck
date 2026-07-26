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

- **R5 — retired 2026-07-25:** Whole-session FTS documents and their cross-turn matching semantics
  are superseded by R25; retaining them made indexing cost grow with the complete transcript.
- **R6.** On a build compiled **without** the `sqlite_fts5` tag (the shipped no-FTS5 fallback path),
  search transparently falls back to a metadata `LIKE` substring filter over name, role, project, and
  backend, with AND semantics across terms. Transcript content is not searchable in this build; the
  query still returns correctly-filtered metadata results rather than an error. The fallback also
  triggers on a tagged binary whenever the FTS5 module is reported missing at runtime.
- **R7 — retired 2026-07-25:** Whole-session `matched_in` projection is superseded by the
  document-scoped rule in R26.
- **R8.** On the FTS5 path, a transcript-matching result carries `snippet`: a short excerpt of the
  matched transcript content with an ellipsis marker. The fallback path returns no snippet.
- **R9 — retired 2026-07-25:** Phrase parsing is retained by R27 with an explicit document boundary.

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
  transcript (user prompts, assistant text, tool calls, tool results, file diffs, permission requests) via
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
- **Search-index lifecycle.** Only the current turn's searchable text is buffered. A
  completed turn or explicit non-turn flush inserts an immutable transcript document; metadata
  updates replace only the small metadata document. Restart/resume begins a new buffer and never
  reloads old transcript documents. A read-path reindex rebuilds the same document projection and
  flushes a crash-truncated final partial turn as its own document.

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
- **R25.** On the FTS5 path, a non-empty `q` searches immutable documents: one current
  metadata document per session, one transcript document per completed turn, and one document for
  transcript content explicitly flushed outside a turn. Every query term must occur in the same
  document. Matching documents collapse to one result per session; the best BM25 document supplies
  rank and snippet, with metadata columns weighted above transcript content and `updated_at` then
  `agent_id` breaking ties. Existing whole-session content migrates as one immutable legacy document,
  so an upgrade does not discard previously searchable text.
- **R26.** Each FTS5 search result carries `matched_in`: `"metadata"` appears when the
  query independently matches the session's metadata document and `"transcript"` appears when it
  independently matches at least one turn, annotation, or legacy transcript document. Both may be
  present. Terms split between documents do not produce a result. The fallback path retains R6.
- **R27.** Search accepts double-quoted phrases in `q` as single terms; unquoted
  whitespace separates terms. A phrase must occur inside one indexed document and therefore does not
  span completed turns or a separately flushed annotation boundary.
- **R28.** The Archive UI initially requests at most 50 matching sessions and offers **Load more**
  while the server's `total` exceeds the number rendered. Each activation requests the next page
  with `offset` equal to the rendered count and appends it; changing the query resets to the first
  page. A later-page failure keeps the already-rendered results visible and surfaces the error.

## 5. Acceptance criteria

- **A1** (R1–R3) — Archive lists active and inactive sessions with metadata and honors the `active`
  filter: `internal/archive/archive_test.go::TestArchiveListAndActiveFilter`,
  `internal/server/server_test.go::TestArchiveListHandler`.
- **A2** (R4, R21) — Empty archive/lists marshal as `[]`:
  `internal/archive/archive_no_fts5_test.go::TestEmptyArchiveMarshalsResultsArray`,
  `internal/server/files_commands_test.go::TestFilesEndpointEmptyList` and
  `TestCommandsEndpointEmptyList`.
- **A3** (R8, R10, R25, R26) — FTS5 search matches metadata and transcript documents, sets
  `matched_in`, and
  paginates: `internal/archive/archive_fts_test.go::TestArchiveSearchFTSMetadataTranscriptAndPagination`.
- **A4** (R25, R27) — Multi-term AND search semantics within one document, including no match when
  terms are split across turns:
  `internal/archive/archive_fts_test.go::TestArchiveSearchANDSemantics` and
  `TestArchiveSearchDoesNotCombineTurns`.
- **A5** (R6, R23) — Untagged build falls back to LIKE metadata search instead of erroring:
  `internal/archive/archive_no_fts5_test.go::TestSearchFallbackFiltersMetadata`.
- **A6** (R11–R13) — Resume restores frozen config and history under the same id, and rejects
  already-running/missing/unknown: `internal/server/resume_test.go::TestResumeHappyPath`,
  `TestResumeAlreadyRunning`, `TestResumeNoPersistedSession`, `TestResumeUnknownAgent`;
  `internal/server/switch_test.go::TestComposeResumeSpecCarriesFrozenLaunchConfig`.
- **A7** (R12, R14, R25, and §3 index lifecycle) — Existing immutable transcript documents survive
  restart and resume:
  `internal/index/indexer_test.go::TestResumeAfterRestartPreservesFTSContent`,
  `TestReindexPreservesFinalPartialTurn`; archived assistant stream
  fragments replay as one rendered message: `ui/src/features/archive/ArchiveAgentPage.test.tsx`.
- **A8** (R15–R18) — File/command rollups from both ACP and hook sources:
  `internal/server/files_commands_test.go::TestFilesEndpointRows`, `TestCommandsEndpointRows`,
  `TestHookCommandCapture`, `TestHookCommandCaptureMultiple`.
- **A9** (R19) — Unknown-agent tracking requests 404:
  `internal/server/files_commands_test.go::TestFilesEndpointUnknownAgent`, `TestCommandsEndpointUnknownAgent`.
- **A10** (R1, R11, R20, R25) — End-to-end archive + search + resume + tracking through the running UI
  on both the FTS5 and untagged builds: journey **J8** (archive & search), **J7** (stop/resume/switch)
  in `docs/features/USABILITY-REVIEW.md`.
- **A11** (R25–R27) — Indexing a later turn inserts a new transcript document without changing the
  earlier document, metadata replacement touches only the metadata document, annotations flush as
  independent documents, and archive pagination counts distinct sessions: focused regressions
  `TestTurnsAppendWithoutRewritingEarlierDocuments`, `TestReindexPreservesAnnotationFlushBoundary`,
  and `TestArchiveSearchCollapsesMatchingDocuments` under both SQLite build variants where applicable.
- **A12** (R10, R28) — An Archive result set larger than one UI page exposes **Load more**, requests
  the next `offset`, appends those sessions, and removes the control when all matching rows are
  rendered: `ui/src/features/archive/ArchivePage.test.tsx`.

## 6. Deviations & open decisions

- **Turn documents deliberately narrow search context.** Terms and phrases do not span
  turn/annotation documents. This is the simplifying trade-off that removes whole-session rewrites;
  `ideas.md` records when evidence would justify a more complex size-bounded segmented index.
- **No-FTS5 fallback is intentionally lossy for transcript search.** The untagged build (R6) can only
  match session metadata; transcript-body search requires the `sqlite_fts5` build tag. This is shipped,
  supported behavior, not a bug — the tag is present on every real build path (`make build`,
  `install.sh`), so shipped binaries retain transcript search; the fallback protects source/dev builds
  and any runtime where the FTS5 module is unavailable.
- **Cross-document matches are intentionally absent.** Per R25–R27, a query split across
  metadata and transcript, or across two turns, returns no result instead of an empty `matched_in`.

## 7. Traceability

- **Archive/search:** `internal/archive/archive.go` (`Search`, `search`, `searchFallback`,
  `matchedIn`, `isFTS5Missing`); handler `internal/server/archive.go`.
- **Index & tracking:** `internal/index/indexer.go` (`UpsertSessionMeta`, `OnEvent`, `trackEvent`,
  `upsertFile`, `CaptureHookFile`, `CaptureHookCommand`, `bumpRollups`), `internal/index/reindex.go`;
  tracking endpoints `internal/server/files_commands.go`.
- **Streaming replay:** `internal/transcript/reader.go` (`ForEach`, shared by `ReadAll`, sequence
  recovery, and reindex).
- **Resume:** `internal/server/resume.go`, `internal/server/switch.go` (`composeResumeSpec`).
- **UI:** `ui/src/features/archive/ArchivePage.tsx` (list + search), `ArchiveAgentPage.tsx` (read-only
  transcript + Resume), `ui/src/components/chat/{FilesTab,CommandsTab}.tsx`.
- **Key regression tests:** `TestSearchFallbackFiltersMetadata`, `TestArchiveSearchFTSMetadataTranscriptAndPagination`,
  `TestResumeAfterRestartPreservesFTSContent`, `TestReindexPreservesFinalPartialTurn`,
  `TestHookCommandCapture`, and `ui/src/features/archive/ArchiveAgentPage.test.tsx`.
