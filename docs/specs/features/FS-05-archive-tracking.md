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

- **R1 — retired 2026-07-29:** The flat all-session response is superseded by R36's project-grouped
  response and its per-project agent rows.
- **R2 — retired 2026-07-29:** Flat-row ordering is superseded by R36's group and agent-row ordering.
- **R3 — retired 2026-07-29:** Flat-list active filtering is superseded by R36's all-session corpus
  filtering before project grouping.
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

- **R10 — retired 2026-07-29:** Flat-list paging is superseded by R36's independent group and
  per-project agent paging; its limit validation remains part of the replacement contract.

### 2.4 Resume from archive

- **R11.** `POST /api/sessions/{id}/resume` on an inactive, non-archived session re-attaches a runtime under the
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

- **R14.** An archived session opens in a **read-only** archived view that renders its recorded
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
  agent rows (the current R1 listing and the planned R36 project-detail/search results) reflect the
  distinct tracked-file count and tracked-command count. The chat path refreshes them at turn
  boundaries; the hook path refreshes them directly on capture.
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
- **R28 — retired 2026-07-29:** The flat UI pager is superseded by R36's independent group and
  per-project agent pagers, including their reordered-boundary recovery.
- **R29.** The Archive search affordance states that all query terms must match within one session
  metadata record, transcript turn, or annotation and are not combined across turns. The search
  input is programmatically associated with that scope note, so an empty result does not imply that
  the terms are absent everywhere in the session.
- **R30.** Every Archive response reports `search_mode` as `"full_text"` when the current SQLite
  connection can use the FTS5 virtual table and `"metadata"` otherwise. The UI advertises
  transcript search and the one-turn scope only in `full_text` mode; in `metadata` mode its
  placeholder and scope note state that only agent/session metadata is searchable and transcript
  snippets are unavailable. Before capability hydration, the placeholder makes no transcript claim.
- **R31.** An opened archived transcript identifies the session before **Resume**: its header shows
  the recorded agent name, project, backend/model, and creation date alongside its archived,
  read-only state. The view obtains this identity from the transcript's existing
  `include_meta=true` session metadata rather than relying on live-agent state.

- **R32.** Agent archival is an explicit, reversible lifecycle action distinct from Stop.
  It requires no confirmation: a running agent is stopped first, then archived; a stopped agent is
  archived immediately. It removes only that agent from its active project dashboard, without
  changing its immutable identity, frozen session configuration, normalized transcript, search index,
  tracked files, commands, or messages. Archiving a project is a separate, warning-confirmed action:
  it stops every running agent in the project, archives every agent, and moves the project out of the
  active dashboard collection with the same preservation guarantees.

- **R33.** The Archive workspace is grouped by project, not a global flat agent list. It
  includes a project whenever the project is archived or it has at least one archived agent; beneath
  each project are its archived agents. Each project row reports its archived-agent count and whether
  that project remains active on the main dashboard. An ordinary unfiltered Archive lists only
  archived projects/agents, while R36 preserves all-session search and explicit active filtering.

- **R34.** Restore is independent per agent and per project. Restoring an archived agent
  returns only that agent to its active project's dashboard in stopped state, after which it can
  Resume under FS-01.R10. An agent under an archived project cannot be restored or resumed; the
  project must be reactivated first. Restoring an archived project returns only the project to the
  active dashboard; its agents remain archived until individually restored. An archived project
  cannot host a newly launched or running agent. A project that remains active appears in both
  collections when it contains archived agents. An archived agent's read-only transcript exposes
  **Restore**, not **Resume**; direct Resume returns `409 agent_archived`. If its project is also
  archived, project restoration is required first and the request returns `409 project_archived`.
  After agent restoration, the ordinary stopped-agent dashboard/transcript exposes Resume under R14.
  When a project definition cannot be read at all — corrupt or otherwise unreadable, as opposed to
  simply absent — Resume, Switch runtime, agent Restore, and pipeline start/control fail closed with
  an internal error rather than proceeding, because the unreadable definition may itself record the
  project archived. A missing definition remains treated as an unavailable-but-active project so its
  agents can still be worked, and agent Archive stays available for cleanup regardless.

- **R35.** The Archive workspace's empty state explains that it contains archived agents
  under their projects, while stopped agents remain on an active project's dashboard until archived.
  Project and agent search results are visibly grouped by their project, and an archived project with
  no individually restored agents remains distinguishable from an active project with archived agents.

- **R36.** Grouping replaces, rather than silently reinterprets, the flat collection contract. R1–R3,
R10, and R28 are retired and superseded by this requirement; R4's non-null `results` guarantee and
R22's `active` validation remain binding.

  `GET /api/archive` returns `{results,total,limit,offset,search_mode}`, where `results` contains
  project groups, `total` counts matching groups, and `limit`/`offset` page groups with the same
  defaults, bounds, and validation currently stated by R10/R22. Groups order by current display
  title (case-insensitive), then durable project id. With neither `q` nor `active`, it returns every
  archived project plus every project with an archived agent. With a non-empty `q` or an explicit
  `active`, it filters the complete recorded-session corpus under R6 and R25–R27 before grouping;
  this keeps active and ordinary stopped transcripts searchable even when their agents are not
  archived. A group may therefore exist only because a non-archived session matched.

  `GET /api/archive/projects/{project}` uses the same envelope, but `results` contains agent rows and
  `total`/`limit`/`offset` count and page those rows. With neither `q` nor `active`, it returns only
  archived agents; otherwise it filters all recorded sessions in that durable project. Non-search
  rows order by `updated_at` descending then `agent_id`; search rows retain R25's ranking. Every row
  reports `active` and `archived`, every group reports `project_status` and the unfiltered
  `archived_agent_count`, and both endpoints always report `search_mode`. The UI pages project groups
  and each expanded project's agents independently, resets both levels when the query/filter changes,
  labels non-archived hits by running/stopped state, and preserves retry-visible error behavior.

## 5. Acceptance criteria

- **A1 — retired 2026-07-29:** Flat archive listing acceptance is superseded by A19.
- **A2** (R4, R21) — Empty archive/lists marshal as `[]`:
  `internal/archive/archive_no_fts5_test.go::TestEmptyArchiveMarshalsResultsArray`,
  `internal/server/files_commands_test.go::TestFilesEndpointEmptyList` and
  `TestCommandsEndpointEmptyList`.
- **A3 — retired 2026-07-29:** Flat archive pagination acceptance is superseded by A19; FTS5
  document matching remains covered by A11 and A19.
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
- **A10 — retired 2026-07-29:** The flat Archive journey acceptance is superseded by A19 and J8.
- **A11** (R25–R27) — Indexing a later turn inserts a new transcript document without changing the
  earlier document, metadata replacement touches only the metadata document, annotations flush as
  independent documents, and archive pagination counts distinct sessions: focused regressions
  `TestTurnsAppendWithoutRewritingEarlierDocuments`, `TestReindexPreservesAnnotationFlushBoundary`,
  and `TestArchiveSearchCollapsesMatchingDocuments` under both SQLite build variants where applicable.
- **A12 — retired 2026-07-29:** Flat Archive paging acceptance is superseded by A19.
- **A13** (R25–R27, R29) — The Archive search input exposes the one-document/one-turn scope before
  and after an empty cross-turn query: `ui/src/features/archive/ArchivePage.test.tsx`.
- **A14** (R6, R23, R30) — Tagged and untagged Archive responses report their effective search
  mode, and the UI changes its placeholder and scope note without advertising unavailable
  transcript search: archive build-variant tests and `ui/src/features/archive/ArchivePage.test.tsx`.
- **A15** (R14, R31) — The archived transcript requests session metadata and renders name, project,
  backend/model, creation date, and read-only state before Resume:
  `ui/src/features/archive/ArchiveAgentPage.test.tsx`.

- **A16.** Warning-confirmed project archival stops every running agent, removes the
  project and every agent from the active dashboard, and preserves their configuration, transcript,
  search results, and tracking data across restart. A barrier regression pauses archival after its
  stop phase and proves a concurrent launch cannot register a new running row before commit. —
  `TestArchiveAgentReapsLiveOrphanBeforeCommit`, `TestArchiveProjectConfigFailureRestoresMixedAgentFlags`,
  `TestProjectArchiveReservesClaimBeforeWaitingForStart`; J5, J7, J8, J12.

- **A17.** Restoring an archived project returns only that project to the active dashboard;
  restoring an archived agent returns only that agent as stopped, with the same identity and history.
  Before restore, its transcript shows Restore instead of Resume and a direct Resume returns
  `agent_archived`; after restore it can Resume. An agent under an archived project cannot restore or
  resume until the project is reactivated and receives `project_archived`. —
  `TestAgentArchiveClaimBlocksConcurrentResume`, `TestProjectArchiveWaitsForAgentRestoreTransition`,
  lifecycle/archive route regressions; J7, J8.

- **A18.** The Archive workspace distinguishes an empty project archive from an active
  dashboard containing stopped agents, lists archived agents beneath their project rather than in a
  separate global collection, counts them, and marks a project that is still active. —
  `ArchivePage.test.tsx`, `ProjectDashboard.test.tsx`; J8.

- **A19.** The grouped API pages project groups and per-project agent rows independently,
  keeps every list non-null, and reports group-versus-agent totals unambiguously. A full-text query
  finds a transcript-only hit from a stopped, non-archived agent and renders it under a project group
  that exists only because of that match; `active` filters the same all-session corpus and both
  endpoints report `search_mode`. It supersedes A1, A3, A10, and A12; A2 remains the non-null
  acceptance criterion. — `TestGroupedArchivePagesPastTwoHundredSessions`,
  `TestArchiveProjectRespondsWithActionLists`,
  `ArchivePage.test.tsx` "reaches every agent when a per-project page is reordered" and
  "loads the next result page using the rendered count as offset"; J8.

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
- **Confirmed project-archive boundary.** R32–R36 preserve the complete search corpus while making
  the ordinary Archive project-grouped and archive-only. Project archival warns, stops running agents,
  and archives every agent; project and agent restoration are independent; archived agents restore
  before Resume. TS-02.R20 and TS-03.R20 own the confirmed persistence and API compatibility design.

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
