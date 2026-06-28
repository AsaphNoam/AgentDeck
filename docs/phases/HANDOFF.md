# AgentDeck — Implementation Handoff

**Live state. Read this first, every session. Update it after every change.**
Protocol: [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) (Claude Code or Codex, whichever the human runs).
Keep this lean — apply the condensation rules (workflow §5); old detail lives in git, not here.

---

## Current position

- **Active phase:** 4 — Persistence: archive, search, resume, file/command tracking
- **Active subphase:** 4.6 — File/command endpoints, hook capture, archive/read-only UI
- **Spec:** [`phase-4-persistence-archive.md`](phase-4-persistence-archive.md), [`tech/phase-4-persistence-archive-techspec.md`](tech/phase-4-persistence-archive-techspec.md)
- **Last GREEN checkpoint:** 4.5 @ `impl/phase-3`: `go build -tags sqlite_fts5 ./...`, `go build ./...`, `go test ./...`
- **Branch:** `impl/phase-3` (do not commit to `main`; do not push unless asked).

---

## Phase status

- [x] Phase 0 — Foundation (data model, file store, server & CLI skeleton) ✅
- [x] Phase 1 — Core loop (ACP chat runtime, launch, streaming chat) ✅ — verified against real `claude-code-acp` v0.16.2
- [x] Phase 2 — State manager, SSE bus, dashboard card grid ✅
- [x] Phase 3 — Config CRUD & onboarding ✅
- [ ] Phase 4 — Persistence: archive, search, resume, file/command tracking
- [ ] Phase 5 — Coordination: MCP messaging, nudger, budgets, notifications
- [ ] Phase 6 — Flexibility: terminal runtime, switch-runtime, task groups
- [ ] Phase 7 — Polish: activity map

Build order: `0 → 1 → 2 → {3, 4, 5} → 6 → 7` (3/4/5 are independent after 2).

---

## Active subphase detail

> The ONLY place granular steps live.

**Phase 3 complete ✅** (3.1–3.6 all green; details in git history).

**Subphase 4.1 ✅** — `internal/transcript` package added: append-only `transcript.ndjson` writer (`Open`/`Append`/`Sync`/`Close`, `O_APPEND`, one JSON record write per event, fsync on `turn_end`/`error`), `session_meta` first record for new logs, max-seq recovery on reopen, `NextSeq()`, and replay reader with `since_seq`, `include_meta`, default meta skip, malformed-line tolerance, and 8 MiB scanner cap. Added additive runtime event types/payloads: `session_meta`, `permission_resolved`. Tests cover append→read, reopen seq continuation, bad middle line, partial trailing line, and >64 KiB line replay. No runtime hot-path wiring yet.

**Subphase 4.2 ✅** — Phase-4 state migration added: `sessions`, `sessions_fts`, `tracked_files`, `tracked_commands`; `state.Open` now also sets `PRAGMA synchronous=NORMAL`. With `-tags sqlite_fts5`, `sessions_fts` is a real FTS5 virtual table; without the tag it degrades to a compatible plain table so the standard checkpoint remains green. Added `internal/index.Indexer` (`UpsertSessionMeta`, `OnEvent`, `OnTurnEnd`) with FTS content accumulation, session rollups, file diff rollups, command tracking/result correlation. Added `index.Reindex(home, db)` and CLI `agentdeck reindex`, which resets derived Phase-4 tables and replays `sessions/{agent_id}/transcript.ndjson`.

**Subphase 4.3 ✅** — Server runtime path now enables persistence via `Registry.SetPersistence(home, transcript.Open, indexer)`. `ChatRuntime.Start` opens `transcript.ndjson`, writes `session_meta`, sets seq from `Writer.NextSeq`, and upserts `sessions`; `emit` appends to raw log before hub/SSE publish and feeds `Indexer.OnEvent`; `turn_end` syncs and calls `OnTurnEnd`; `error` syncs without double-counting turns. `Stop`/crash close the writer. Permission decisions now emit/persist `permission_resolved` (`approve`/`deny`/`timeout`/`auto_approve`). `GET /api/sessions/{id}/transcript` now reads persisted NDJSON with `since_seq` and `include_meta`. Crash-mid-turn server integration asserts delivered text exists in the API response and raw log.

**Subphase 4.4 ✅** — Added `internal/archive.Archive` with listing over `sessions` joined to `running`, `active` filtering, pagination, FTS5 search over `sessions_fts`, snippets, bm25 ordering, and `matched_in` labels. Added `GET /api/archive?q=&limit&offset&active` with validation. Tests cover active/inactive listing, transcript-only hit+snippet, metadata hit, pagination, negative query, and handler envelope.

**Subphase 4.5 ✅** — `ChatRuntime.Resume` (spawn/handshake, best-effort `session/load→session/new`, append-mode transcript reopen, resumed `session_meta` with `resumed_at`, fresh running row + restored `context_pct`). `POST /api/sessions/{id}/resume` (404/409/422 guards; optional backend/model/interface override seam for Phase 6). `Registry.Resume` with nil-sentinel double-resume guard. `state.ReadSession`/`ListInactiveSessions`. `UpsertSessionMeta` `updated_at` max guard. CLI: `agentdeck resume <id>`, `--resume <id>`, `--new`, bare-form single-inactive auto-resume. fakeacp `session/load` handler. Integration tests: happy path (agent_id unchanged, new session_id, prior transcript + resumed_at, monotonic seq after post-resume prompt), 409 already-running, 422 no persisted session, 404 unknown agent. CLI unit tests: `--new` / `--resume` flag parsing.

**Subphase 4.6 — File/command endpoints + hook capture + archive UI (next)**
- `GET /api/sessions/{id}/files` — list tracked files from `tracked_files` table (top-N by edit count, with `path`, `edits`, `last_edited_at`).
- `GET /api/sessions/{id}/commands` — list tracked commands from `tracked_commands` table.
- Hook capture for file/command events: if the hook payload carries `event:"file_edit"` or `event:"command"`, write rows into `tracked_files`/`tracked_commands` via the indexer/state store.
- Read-only archive UI: route `/archive` in the React app; lists inactive sessions with search, links to `/sessions/{id}` read-only transcript view.
- Tests: files/commands endpoints return correct rows; archive UI renders and filters (Vitest + MSW). Checkpoint: `go build -tags sqlite_fts5 ./...` and full test suite.

---

## Decisions & notes (durable contracts from Phase 1)

- **Normalized `Event` is the cross-phase contract.** `internal/runtime`: `event.go` (envelope +
  `*Data` payloads), `acpmap.go` (the ONLY place ACP wire shapes are decoded — §12.1 isolation rule).
  Phase 2 streams these `Event`s as `new_message` payloads; the interim SSE `data:` object is already
  byte-identical to what Phase 2 wraps. Permanent fields: `agent_id,seq,type,ts,data` (append-only).
- **`Registry` is the server's entry to runtimes** (`Launch`/`SendPrompt`/`Cancel`/`Stop`/`Permission`/
  `Subscribe`/`Shutdown`; dispatch by `agent.interface`; `Chat()` + `ChatRuntime.SetCommand` inject the
  adapter binary). `chat.go` owns `agentState` per agent (process group, transport, hub, status writes);
  `permission.go` is the withhold-the-response gate; `reconcile.go::ReconcileStale` cleans stale rows on start.
- **Status vocabulary (§4.4)** is the dashboard contract Phase 2 reads: `state ∈
  {busy,idle,waiting_input,done,error}`, `last_trace ∈ {SessionStart,UserPromptSubmit,PreToolUse:*,
  PostToolUse:*,PermissionRequest:*,PermissionResolved,Stop,Cancelled,Error}`.
- **REST surface (server pkg):** `POST /api/sessions` (launch), `GET /api/sessions/{id}`,
  `POST .../{prompt,cancel,stop,permission}`, `GET .../events` (interim SSE). Session routes use the §7.7
  nested error envelope via `writeAPIError`. `server.New` takes a `*runtime.Registry`. CLI launch
  (`internal/cli/launch.go`) just POSTs to `/api/sessions` (CLI≡modal parity).
- **fakeacp** (`internal/runtime/testdata/fakeacp`) is the deterministic test adapter — under `testdata/`
  so `go build ./...` skips it; build explicitly with `go build -o /dev/null ./internal/runtime/testdata/fakeacp`.
- The **real-CLI acceptance** is gated behind `//go:build acceptance` (5 tests: stream, permission
  deny/approve, cancel, stop); run with `go test -tags acceptance ./internal/runtime -run TestRealCLI -v`
  (needs `claude-code-acp` + a logged-in Claude account). Recipe + Appendix A: [`phase-1-acceptance.md`](phase-1-acceptance.md).

## Blocked on human

_(empty — the 1.6 credentialed acceptance ran GREEN against `claude-code-acp` v0.16.2. Nothing blocking.)_

## Review findings (BLOCKING items from the last review)

> Written by the review agent (workflow §8). Remove an entry once fixed and verified green.

- **[A1] TOCTOU double-start in `Registry.Launch`** `internal/runtime/registry.go:88–101`. Check and insert are two separate lock acquisitions — two concurrent launches for the same agent both pass the existence check and both spawn processes. Fix: use a nil sentinel in `rtByAgent` before releasing the lock on first check; guard `ownerFor` against nil entry; replace sentinel with real runtime on success.

- **[A2] `emit()` split critical section** `internal/runtime/chat.go:430–460`. `as.seq++` and `as.transcript = append(…)` are two separate `as.mu` acquisitions — concurrent emitters can interleave, producing out-of-order seq in the in-memory transcript and disk log. Fix: hold a single `as.mu` across both seq increment and transcript append.

- **[A3] `initialize` response never validated** `internal/runtime/chat.go:181–186`. Protocol version from the adapter is discarded. Fix: parse the response, log a warning if `protocolVersion` ≠ 1.

- **[A4] `state.TranscriptEvent` dead code** `internal/state/types.go:75–81`. Defined but never referenced — all live code uses `runtime.Event`. Fix: delete the type.

- **[A5] `SessionMetaData.SystemPromptSHA` always empty** `internal/runtime/event.go:115` / `chat.go:530–545`. Field is declared and serialized as `""` in every transcript. Fix: compute SHA256 of `SystemPrompt` in `runtimeMeta()`; add `omitempty` so it serializes to empty string only when absent.

- **[A6] CORS missing PUT/DELETE** `internal/server/middleware.go:38`. `Access-Control-Allow-Methods` only lists `GET, POST, OPTIONS`. PUT/DELETE routes are blocked by cross-origin preflight. Fix: add `PUT, DELETE`.

- ✅ **RESOLVED — A7: `notification` SSE event type not registered.** `ui/src/api/sse.ts`. Phase 5 will emit `notification` events; client registers no listener. Fix: add a no-op `notification` listener now per spec §4.3.

- ✅ **RESOLVED — A8: SSE double-open wipes hydration burst.** Added `hydrating` flag; `onopen` skips `hydrateBegin`/`hydrationIds=[]` if already hydrating. `hydrating` cleared when `hydrateComplete` fires. Vitest green.

- ✅ **RESOLVED — A9: Seq gap detection not implemented.** Added `lastAgentSeq: Record<string, number>`. `onNewMessage` reads `envelope.data.seq`; on gap triggers full `getTranscript` refetch. `lastAgentSeq` reset on fresh connect. Vitest green.

- **[A10] `client.dropped` never observed** `internal/bus/bus.go`. Counter is incremented on slow-consumer drop but never logged, exposed, or acted on. Fix: log a warning on each drop.

- ✅ **RESOLVED — A11: `ConnectionDot` inline in Header.** Extracted to `ui/src/components/shell/ConnectionDot.tsx`; Header now imports it. Vitest green.

- ✅ **RESOLVED — A12: `cachedCredCheck` held mutex during 6 s probe.** Refactored to: lock→read cache→unlock; run probe unlocked; lock→write result→unlock. Concurrent `GET /api/config` calls no longer block on the probe. `go build ./...` green.

- ✅ **RESOLVED — A13: `ProjectsEditor` dialog stayed open on `cwd_not_found`.** Both create and update `onSuccess` now always call `setOpen(false)`; warnings are non-blocking. Test updated to assert dialog closes. Vitest green.

- ✅ **RESOLVED — A14: `add_dirs` absent from `ProjectForm`.** Added `addDirs` state (initialised from `initial.add_dirs`); rendered as a removable list with an inline input+Add button (Enter also commits). Submit passes `addDirs` instead of the old `initial?.add_dirs ?? []` passthrough. Vitest green.

- ✅ **RESOLVED — A15: `OnboardingGate` mounts dashboard behind wizard.** When `!satisfied`, renders only `<OnboardingWizard>`; children are not mounted at all. Keyboard focus can no longer reach dashboard controls through the overlay. Vitest green (all 6 OnboardingGate tests pass).

- **[A16] `Stop()` closes persistence before transport goroutine drains** `internal/runtime/chat.go:308–309`. Events arriving between SIGTERM and EOF may be written to a closed writer (logged error, not crash). Fix: remove `closePersistence()` from `Stop()`; call it in `onTransportClosed`'s early-return path so it runs after the goroutine exits.

- **[A17] `addContent` map accumulates unboundedly** `internal/index/indexer.go:141–152`. Content is appended for the entire session lifetime without a cap. Fix: cap accumulated content at 1 MiB per agent, dropping oldest bytes.

- ✅ **RESOLVED — A18: `Reindex` unsafe under concurrent server use.** Added doc comment to `internal/index/reindex.go::Reindex` explaining the danger. CLI `reindex` command now checks pidfile + `processAlive`; prints a clear warning to stderr if the server appears live. `go build ./...` green.

- **[A19] `matchedIn` fallback always returns `"transcript"`** `internal/archive/archive.go:218–222`. When neither metadata nor content matches substring terms, the fallback appends `"transcript"` even though the match location is unknown. Fix: return `nil` when no classification succeeds (field has `omitempty`).

- **[A20] `Registry.Stop()` window wider than Launch TOCTOU** `internal/runtime/registry.go:134–145`. Entry is not removed from `rtByAgent` until after `rt.Stop()` returns (up to 5 s). Concurrent `SendPrompt`/`Permission` calls race Stop for the full grace period. Fix: delete from `rtByAgent` before calling `rt.Stop()`.

- **[A21] `spaHandler` path not guarded against `..` components** `internal/server/spa.go:18`. `path.Clean` + `TrimPrefix("/")` can yield a `../…` path passed to `fs.Stat`. Go's `fs.ValidPath` blocks this on current versions, but the protection is implicit. Fix: add explicit `strings.HasPrefix(reqPath, "..")` guard.

- ✅ **RESOLVED — A22: `firstNonEmpty` duplicated.** Extracted to `internal/strutil/strutil.go` as `FirstNonEmpty`. All three call sites in `acpmap.go`, one in `chat.go`, two in `indexer.go` updated; local definitions deleted. `go build ./...` green.

- **[A23] Migration system has no max-version guard** `internal/state/migrate.go:42–56`. No check for migrations applied by a newer binary — downgrade silently runs against an unknown schema. Fix: after migrations, check `MAX(version)` in `schema_migrations`; fail with a clear error if it exceeds the latest known migration version.

- ✅ **RESOLVED — B1: data race on `client.dropped` in bus.go.** Changed `dropped uint64` → `dropped atomic.Uint64`; `c.dropped++` → `c.dropped.Add(1)`. `go test -race ./internal/bus` green.

- ✅ **RESOLVED — B2: PermissionPrompt optimistic collapse.** Now awaits `decidePermission` POST before calling `resolve`; try/catch shows inline error message on failure so the prompt stays interactive. All Vitest tests green.

- ✅ **RESOLVED — B3: `UpsertSessionMeta` dropped `system_prompt` on conflict.** Added `system_prompt=excluded.system_prompt` to the `ON CONFLICT DO UPDATE SET` clause in `internal/index/indexer.go`. Both tagged and untagged index tests green.

- ✅ **RESOLVED — B4: unified HTTP error envelope for 500s.** Migrated all `writeError(w, http.StatusInternalServerError, ...)` calls in `handlers.go`, `config_handlers.go`, and `static.go` to `writeAPIError(w, apiError("internal", "internal error"))`. The only remaining `writeError` is the 404 catch-all (intentional). Spec-mandated Phase-3 shapes (`writeValidationError`, `inUseBody`) are unchanged. Full `go build ./...`, `go test ./...`, and tagged FTS5 builds green.

- ✅ **RESOLVED — path-param slug validation added to all PUT/DELETE handlers.** `handlePutRole`, `handleDeleteRole`, `handlePutProject`, `handleDeleteProject` now call `config.ValidSlug(id)` before any store call; non-slug ids return `validation_failed` 400. `TestPathTraversalRejected` + `TestPathTraversalEncodedDots` added and green.

- ✅ **RESOLVED — Phase 2.6 committed + review advisories fixed.** 2.6 chat panel is now a committed
  green checkpoint. Fixed alongside it: PermissionPrompt collapses to an Approved/Denied chip after a
  decision (resolved state stored per `tool_call_id`); TranscriptView autoscrolls with a jump-to-latest
  affordance; CardContextMenu closes on click-outside/Escape; `resolvePermission` now uses its
  `toolCallId`; CardGrid skips the layout PUT on initial load; and `ToolCall`/`ToolResult`/`TurnError`
  renderers added (collapsible args, truncated results). Embedded `internal/server/ui/dist` re-synced.

- ✅ **RESOLVED — advisory: onboarding cred-check cache invalidated on PUT /api/backends.** `handlePutBackends` now clears `onboardingCache` after every successful write, ensuring a changed API key is always re-probed on the next `GET /api/config` rather than serving a stale ok/failed result for up to 60s.

- ✅ **RESOLVED — advisory: force-delete UI flow added to RolesEditor and ProjectsEditor.** DELETE 409 responses are now caught; the UI parses `body.agents`, shows a confirm listing affected agents, and retries with `?force=true` if the user confirms. Running agents are unaffected per the spec.

- ✅ **RESOLVED — full real-adapter Appendix A coverage added & PASSED.** The gated acceptance suite
  (`internal/runtime/acceptance_test.go`, `//go:build acceptance`) now has five real-CLI tests, all green
  against `claude-code-acp` v0.16.2 (`go test -tags acceptance ./internal/runtime -run TestRealCLI -v`):
  `TestRealCLIAcceptance` (incremental stream + turn_end + idle), `TestRealCLIPermissionDeny` (real gate;
  denied tool's side effect never happens), `TestRealCLIPermissionApprove` (approved tool runs),
  `TestRealCLICancel` (cancel interrupts an in-flight turn → idle), `TestRealCLIStop` (terminates the
  process group + removes the running row + status `done`). Confirmed real option kinds are
  `allow_once`/`reject_once`/`allow_always` — `selectOption` (§5.3) maps approve/deny correctly.

## Autonomous decisions (please review)

> Resolved without stopping; the human should still see them. Remove once acknowledged (workflow §3, §5).

- **`internal/store` (spec) → `internal/state` (Phase 0 reality).** The runtime imports `internal/state`
  throughout; the spec's `store` is the older name for the same package. No behavior change.
- **`Stop` implemented in 1.3** (spec slots it in 1.4) for test teardown — matches §8.5 exactly; no reversal needed.
- **Tool `Name` ← ACP `kind`** (fallback `title`, then `"tool"`); §4.3 didn't pin the field. Isolated in
  `acpmap.go::toolName`. Verified against the real adapter (turn streamed cleanly).
- **RESOLVED in 2.2: hook token persisted in `running.hook_token`.** `Server.hookTokens` still exists as
  Phase 1 launch scaffolding but hook validation now reads the live `running` row, not the map.
- **Two error-envelope shapes coexist** — new session routes use the §7.7 nested shape; Phase-0 GET routes
  keep flat `{"error":"msg"}` (not migrated, to avoid breaking Phase-0 tests). Migrate later if §7.7 is meant
  to be truly project-wide.
- **`messagingServer.Command = os.Executable()`** with `["mcp-stdio","--agent",ID,"--token",T]` —
  registration-only; the `mcp-stdio` subcommand lands in Phase 5.
- **NEW: runtime strips `CLAUDECODE` from the spawned adapter's env** (`chat.go::stripEnv`). The real
  `claude-code-acp` refuses a "nested" session when `CLAUDECODE` is set (true when AgentDeck is launched
  from a Claude Code terminal). AgentDeck spawns independent agents, so the nested guard must never apply.
  Discovered during the 1.6 run. **To reverse:** drop the strip if it ever causes surprise; production
  (standalone server) is unaffected since `CLAUDECODE` isn't set there.
- **RESOLVED: `CLAUDE_ACP_VERSION` pinned to `0.16.2`** (was an unverified `0.4.1` placeholder; corrected
  via `npm view` to the real latest-stable, against which acceptance passed).
- **Wire-shape note (no fix needed):** the real adapter's `session/new` ignores our `model` param and
  exposes its own modelIds (`default`/`sonnet`/`haiku`/`opus`) + permission `modes`
  (incl. `bypassPermissions`/`acceptEdits`). Phase 1 doesn't assert the model, so this is fine; a future
  phase wanting real model/mode selection should map our model→adapter modelId in `acpmap.go`/`sessionNewParams`.
- **Phase 2.1 manager contract:** `state.Manager` wraps the existing Phase 0 `Store`; it does not replace
  typed CRUD. It emits `AgentStateUpdate` through `StatePublisher`, now implemented by `internal/bus`.
  `status.updated_at` is migration v2, `running.hook_token` is migration v3, and `Store.WriteStatus` stamps
  `updated_at` when callers omit it.
- **Phase 2.1 transcript mirror kept generic.** The spec asked for transcript types in `internal/state/types.go`
  but Phase 1's concrete normalized event shapes already live in `internal/runtime/event.go`. I added only
  `state.TranscriptEvent {Kind, Data}` as a storage/UI-facing mirror to avoid duplicating runtime structs.
  To reverse: replace it with concrete state-owned transcript structs when 2.4/2.6 needs them.
- **Phase 2.3 kept runtime Hub internally.** The HTTP route `GET /api/sessions/{id}/events` is deleted and
  transcript deltas now publish as bus `new_message`, but `Runtime.Subscribe`/per-agent `Hub` still exist for
  runtime tests and local internal compatibility. To reverse: remove the hub API once no tests/internal callers need it.
- **Phase 2.4 replaced the walkthrough UI source.** The repo had a product-demo React app, not the dashboard shell
  scaffold described by the spec. I replaced `ui/src` with the Phase 2 shell/stores/SSE foundation and refreshed
  `internal/server/ui/dist`. To reverse: recover the demo from git history, but it is no longer the Phase 2 target UI.
- **Phase 4.1 writer API takes optional metadata.** The tech spec pseudo-signature said `Open(home, agentID)` but also
  requires the writer to create the first `session_meta` record. I implemented `transcript.Open(home, agentID, meta)`
  so runtime wiring can pass the frozen launch snapshot at creation; `nil` skips meta for tests/recovery cases. To
  reverse: split this into `Open` + explicit `AppendSessionMeta` before 4.3 runtime wiring.
- **Phase 4.2 no-tag FTS fallback.** The Phase 4 spec requires SQLite FTS5 with `-tags sqlite_fts5`, but the canonical
  workflow still requires ordinary `go build ./...` and `go test ./...` to pass. Migration v4 creates a real FTS5
  virtual table when the tag is enabled and a schema-compatible plain `sessions_fts` table otherwise. Tagged builds/tests
  cover real `MATCH`; no-tag builds keep state.Open usable. To reverse: make all checkpoints/builds always pass
  `-tags sqlite_fts5` and remove the fallback branch in `ensureSessionsFTS`.
- **Phase 4.3 adds full `system_prompt` to new `session_meta` records.** The DB schema requires exact `system_prompt`,
  but 4.1 initially only modeled `system_prompt_sha`. Runtime wiring now writes `system_prompt` into `session_meta`
  and `sessions.system_prompt`; reindex of any older raw log without that field leaves the DB column empty. To reverse:
  remove `SessionMetaData.SystemPrompt` and require runtime Start to upsert the DB snapshot out-of-band.

## Changelog

_(most recent first; keep ~10, older history is in git)_

- 2026-06-28 — **4.5 green.** Full `ChatRuntime.Resume` (spawn+handshake, best-effort `session/load→session/new`, append-mode transcript reopen, resumed `session_meta` with `resumed_at`, restored `context_pct`). `POST /api/sessions/{id}/resume` endpoint + `Registry.Resume` nil-sentinel guard. `state.ReadSession`/`ListInactiveSessions`. `UpsertSessionMeta` max(`updated_at`) guard. CLI: `agentdeck resume`, `--resume`, `--new`, bare-form auto-resume. fakeacp `session/load`. Integration+CLI tests green. Checkpoint: `go build -tags sqlite_fts5 ./...`, `go build ./...`, `go test ./...`.
- 2026-06-28 — **Review fixes: B1–B4 resolved.** Bus `dropped` race → `atomic.Uint64`; `PermissionPrompt` now awaits POST before collapsing; `UpsertSessionMeta` ON CONFLICT now updates `system_prompt`; all server 500s unified to `writeAPIError`. Full build + tests + FTS5 green.
- 2026-06-28 — **4.4 green.** Added `internal/archive` list/search queries and `GET /api/archive`
  handler; FTS5 search covers transcript/content hits, metadata hits, snippets, active filters, pagination,
  and negative queries. Checkpoint: tagged archive/index tests, tagged build, standard build, full Go tests.
- 2026-06-28 — **4.3 green.** Wired server runtime persistence: `transcript.ndjson` writer + indexer in
  `ChatRuntime.Start`/`emit`/`Stop`; persisted `permission_resolved`; transcript endpoint reads raw NDJSON with
  `since_seq`/`include_meta`; crash-mid-turn integration verifies delivered text survives in the API response and raw log.
- 2026-06-28 — **4.2 green.** Added Phase-4 state migration (`sessions`, `sessions_fts`,
  `tracked_files`, `tracked_commands`) plus `synchronous=NORMAL`; added `internal/index.Indexer` for
  session rollups, FTS content, file rollups, command tracking/result correlation; added `index.Reindex`
  and CLI `agentdeck reindex`. Tagged FTS tests and standard checkpoint green.
- 2026-06-28 — **4.1 green.** Added `internal/transcript` raw NDJSON writer/reader with `session_meta`
  first record, max-seq recovery/`NextSeq`, `since_seq`/`include_meta` replay, malformed-line tolerance,
  and 8 MiB scanner cap. Added additive `runtime` event types/payloads for `session_meta` and
  `permission_resolved`. Checkpoint: `go test ./internal/transcript/...`, `go build ./...`, `go test ./...`.
- 2026-06-28 — **Phase 3 COMPLETE / 3.6 green.** `OnboardingGate` + `OnboardingWizard` (3 steps: BackendStep/ProjectStep/LaunchStep); resume-from-step logic; non-dismissible (Esc/overlay blocked); sets `onboarding_complete` on first launch; 26 Vitest+MSW tests; embedded dist refreshed. Checkpoint: all Vitest tests + `go build ./...` + `go test ./...`.
- 2026-06-28 — **3.5 green.** `BackendsEditor`+`ModelRow` (default radios, masked env editor, cred chip); `useSuggestedName`; `NewAgentModal` (role/project/backend/model, terminal disabled); "New agent" CTA in CardGrid/EmptyState; 20 Vitest+MSW tests; embedded dist refreshed. Checkpoint: all Vitest tests + `go build ./...` + `go test ./...`.
- 2026-06-28 — **3.4 green.** Zod schemas; TanStack Query hooks; SettingsPage tabs; RolesEditor/RoleForm + ProjectsEditor/ProjectForm (RGB swatch, cwd_not_found); Settings route; 11 Vitest+MSW tests green; embedded dist refreshed. Checkpoint: all Vitest tests + `go build ./...` + `go test ./...`.
- 2026-06-28 — **3.3 green.** `GET /api/config` with computed onboarding block (min-viable check + ~60s cred-check cache); `PUT /api/config` partial merge; `Config.OnboardingComplete` field; disk-on-demand audit (reads clean, only cred-check cached). Checkpoint: `go build ./...` + `go test ./...`.
- 2026-06-28 — **3.2 green.** `internal/backend/credcheck/` (claude auth-status + codex /v1/models probers, 6s timeout, CredResult, env merge); `ValidateBackendsConfig` (invariants + auto-promote); `PUT /api/backends` with injected credCheck for tests; all invariant + cred-check tests. Checkpoint: `go build ./...` + `go test ./...`.
- 2026-06-28 — **3.1 green.** `internal/config/validate.go` (`ValidSlug`, `FieldError`, role/project validators); `POST/PUT/DELETE /api/roles/{role}` + `POST/PUT/DELETE /api/projects/{project}` in `internal/server/config_handlers.go`; in-use guard; `cwd_not_found` warning; disk-on-demand; tests. Checkpoint: `go build ./...` + `go test ./...`.
- 2026-06-28 — **Phase 2 COMPLETE / 2.6 green.** Added full chat route/panel with live header,
  transcript renderers (markdown + code highlight, tool/diff/error/permission), prompt send/cancel, Approve/Deny,
  reconnect transcript refetch, and refreshed embedded UI assets. Checkpoint: `go build ./...` + `go test ./...` +
  `cd ui && npm test` + `cd ui && npm run build`.
- 2026-06-28 — **2.5 green.** Added live card grid route with layout load/save, dnd-kit reorder,
  density control, cards/badges/context meter, empty-state launch, context menu with Open/Rename/Stop and
  disabled future actions, plus `POST /api/sessions/{id}/rename`. Checkpoint: `go build ./...`,
  `go test ./...`, `cd ui && npm test`, `cd ui && npm run build`.
- 2026-06-28 — **2.4 green.** Added `GET/PUT /api/layout` Phase 2 API shape, `GET /api/sessions/{id}/transcript`,
  retained in-memory runtime transcript events, React Router shell, Zustand stores, SSE singleton, REST/types modules,
  Vitest store tests, and refreshed embedded UI assets. Checkpoint: `go build ./...`, `go test ./...`, `cd ui && npm test`,
  `cd ui && npm run build`.
- 2026-06-28 — **2.3 green.** Added `internal/bus` with global-seq envelopes, snapshot hydration, drop-oldest
  clients, and state/runtime publishers; replaced per-agent HTTP SSE with `GET /api/events`; runtime now mirrors
  transcript events as bus `new_message` and touches state manager after status writes. Checkpoint: `go build ./...`,
  `go test ./...`, `go test -race ./internal/bus`.
- 2026-06-28 — **2.2 green.** Added `POST /api/hook` with header/body token support and fixed
  `{error,message}` envelope; persisted hook tokens in `running.hook_token`; added `Manager.ApplyHook`
  for `running`/`status`/`stopped`; added fsnotify + periodic sessions reconciliation that only corrects
  stale status rows. Checkpoint: `go build ./...` + `go test ./...`.
- 2026-06-28 — **2.1 green.** Added `state.Manager`, `AgentState`/`AgentStateUpdate`, migration v2
  (`status.updated_at`), `busy_timeout=5000`, effective identity+running+status recompute, startup scan,
  tombstone removal semantics, and focused manager tests. Checkpoint: `go build ./...` + `go test ./...`.
- 2026-06-28 — **Review fix: full Appendix A real-adapter coverage.** Added 4 gated tests
  (permission deny/approve, cancel, stop) alongside the stream test — all 5 PASS against
  `claude-code-acp` v0.16.2. Real option kinds confirmed (`allow_once`/`reject_once`/`allow_always`).
  Resolves the BLOCKING review finding. Default suite untouched (tests tagged off).
- 2026-06-28 — **Phase 1 COMPLETE.** Real-CLI acceptance PASSED against `claude-code-acp` v0.16.2:
  handshake + incremental stream + turn_end + idle. Fixed: runtime strips `CLAUDECODE` from the spawned
  adapter env (adapter refuses nested sessions); `install.sh` pin corrected `0.4.1`→`0.16.2`.
- 2026-06-27 — **1.6 code/docs.** Gated `acceptance` build-tag test + `install.sh` adapter pin +
  `phase-1-acceptance.md` curl/SSE recipe.
- 2026-06-27 — **1.5 green** (incl. `-race`). Launch composition + REST (`POST /api/sessions`, detail,
  prompt/cancel/stop/permission) + interim SSE + CLI launch. Tests: composeEnv/joinSystemPrompt/resolveSkip
  units, CLI parseLaunch + parity, full HTTP integration (launch→SSE→prompt→permission_request→approve→
  sentinel→turn_end), §7.7 validation/404 envelopes. Replaced the Phase-0 CLI launch stub.
- 2026-06-27 — **1.4 green** (incl. `-race`). Permission gating (withhold/approve/deny/timeout/skip),
  Cancel, crash handling, stale-row reconcile. Tests: approve→sentinel, deny→no sentinel, timeout auto-deny,
  skip auto-approve, unknown-tool 409, cancel-during-pending, crash (fatal err + running row deleted), reconcile.
- 2026-06-27 — **1.3 green.** Real `ChatRuntime`: process-group spawn + ACP handshake, isolated
  `acpmap.go`, per-agent `Hub` (drop-oldest), async `SendPrompt` streaming a turn end-to-end, §4.4 status
  writes, working `Stop`. Tests (incl. `-race`): `stream_text` (multi-delta + monotonic seq + context_pct),
  `tool_flow` (correlated call/result/diff), idle→busy→idle. fakeacp gained `tool_flow` + usage in result.
- 2026-06-27 — **1.2 green.** Added JSON-RPC stdio transport (8 MiB scanner, serialized writer, Call/Notify,
  correlation map, IncomingRequest withhold/Respond) + standalone fakeacp CLI (stream_text/big_frame/
  malformed_then_valid). Tests: >64 KiB frame, malformed-then-valid resync, Call/response, incoming-request reply.
- 2026-06-27 — **1.1 green.** Created `internal/runtime`: sentinel + APIError/code vocab, Event envelope +
  payload structs, Runtime interface, Registry dispatch + terminal/ChatRuntime stubs. Tests: payload JSON
  round-trips, code→status map, dispatch table. `go build ./...` + `go test ./...` green.
- 2026-06-27 — Handoff + workflow created. Phase 0 confirmed complete (build + tests green). Phase 1 ready to start at 1.1.
