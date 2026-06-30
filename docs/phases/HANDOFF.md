# AgentDeck — Implementation Handoff

**Live state. Read this first, every session. Update it after every change.**
Protocol: [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) (Claude Code or Codex, whichever the human runs).
Keep this lean — apply the condensation rules (workflow §5); old detail lives in git, not here.

---

## Current position

- **Active phase:** 6 — Flexibility: terminal runtime, switch-runtime, task groups
- **Active subphase:** 6.1 (next) — hook ingest + backend adapter + Codex (chat)
- **Spec:** [`tech/phase-6-flexibility-techspec.md`](tech/phase-6-flexibility-techspec.md) (PRD: [`phase-6-flexibility.md`](phase-6-flexibility.md)); subphase plan at §"Subphase plan"
- **Last GREEN checkpoint:** 5.4 / Phase 5 complete @ `main`: `go build ./...`, `go build -tags sqlite_fts5 ./...`, `go test ./...`, `go test -tags sqlite_fts5 ./...`, `cd ui && npm test`, `cd ui && npm run build`.
- **Branch:** `main` — **trunk-based: all work commits directly to `main`, no per-phase branches, no PRs** (workflow §6). Don't push to origin unless asked.

---

## Phase status

- [x] Phase 0 — Foundation (data model, file store, server & CLI skeleton) ✅
- [x] Phase 1 — Core loop (ACP chat runtime, launch, streaming chat) ✅ — verified against real `claude-code-acp` v0.16.2
- [x] Phase 2 — State manager, SSE bus, dashboard card grid ✅
- [x] Phase 3 — Config CRUD & onboarding ✅
- [x] Phase 4 — Persistence: archive, search, resume, file/command tracking ✅
- [x] Phase 5 — Coordination: MCP messaging, nudger, budgets, notifications ✅
- [ ] Phase 6 — Flexibility: terminal runtime, switch-runtime, task groups
- [ ] Phase 7 — Polish: activity map

Build order: `0 → 1 → 2 → {3, 4, 5} → 6 → 7` (3/4/5 are independent after 2).

---

## Active subphase detail

> The ONLY place granular steps live.

**Phases 0–4 complete ✅** (all subphases green; details in git history & Phase status above).

**Phase 5 complete ✅.** MCP messaging server, message store/tools, per-agent registration, nudger, per-turn budgets, janitor, notification SSE, config-backed notification mutes, Web Notification/in-app toast client, message badges/outbound pulse, and read-only inbox endpoint are all green. Details live in git history (`5.1`–`5.4`) and changelog.

**Subphase 6.1 — next to implement** (hook ingest + backend adapter + Codex chat; techspec §4.4, §6, §8.6):
- [ ] Harden `POST /api/hook`: per-launch-token validation, running-row refresh/clear on `SessionStart`/`Stop`, valid token applies status + emits `state_update`, stale token → `401`.
- [ ] Extract `internal/backend/adapter.go` with `BackendAdapter` interface and `claude-acp` implementation from current inline launch/runtime logic; carry capability flags, `hookMap`, and `resolveResumeId`.
- [ ] Add `codex-acp` adapter and per-model env in the launch composer; make Codex launch → prompt → stream → stop → native resume work through existing chat runtime.
- **Checkpoint:** `go build ./...` + `go test ./...`; include hook-ingest tests and the Codex chat acceptance gate if credentials/tools are available, otherwise leave the credentialed live run gated like prior real-CLI checks.

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

- **GATED (not blocking 6.1): live two-CLI MCP registration confirmation.** Subphase 5.1 proved the
  in-process HTTP streamable MCP transport works (round-trips a `ping` via the go-sdk client, both
  directly and through the real dashboard mux). What can't be done without credentials: confirming that
  the **real Claude Code and Codex CLIs** each accept the transport-(A) HTTP MCP entry (vs. needing the
  transport-(B) stdio `agentdeck mcp` subcommand). This is a credentialed acceptance, same class as the
  Phase 1 real-CLI run. **To do (human, ~30min):** launch the dashboard, register an HTTP MCP server
  entry (`type:"http"`, `url:http://127.0.0.1:{port}/mcp`, header `X-AgentDeck-Token`) with each CLI and
  confirm a `ping` tool call round-trips; if a CLI rejects HTTP, note it so 5.3's `RegisterMessagingMCP`
  emits the stdio entry for that backend. This does **not** block 5.2/5.3 — they proceed targeting HTTP
  with the stdio fallback ready. Subphase 5.3 currently emits HTTP MCP entries for both backends pending this verdict.

## Review findings (from the last review — BLOCKING and ADVISORY)

> Written by the review agent (workflow §8), one bullet per finding tagged with its severity
> (`BLOCKING` / `ADVISORY`). Consumed by the fix agent (`/fix-review`, workflow §9), which validates
> each is actually true, then **deletes the bullet** once it's fixed-and-green or dismissed as a
> validated false positive — recording the outcome in the changelog + its end-of-turn summary (§5).
> **This section holds only OPEN findings** — no resolved/dismissed graveyard.
> Blocking items must be fixed before the next phase starts; advisory items when convenient.

_(no open findings)_

## Autonomous decisions (please review)

> Resolved without stopping; the human should still see them. Remove once acknowledged (workflow §3, §5).

- **NEW (5.4): notification edge detection lives in `internal/bus`, not `state.Manager`.** The tech spec phrases this as a state-manager extension, but the bus already owns the prior `AgentStateUpdate` snapshot needed to edge-detect `done`/`waiting_input` without adding another state cache. `state.Manager` still recomputes `unread_messages`; `bus.PublishStateUpdate` emits `notification` on transitions, and `bus.PublishRuntimeEvent` emits `permission_required`. **To reverse:** move the previous-state cache and notification publishing into `state.Manager` and have the bus only transport events.
- **NEW (5.3): HTTP MCP entries emitted for both `claude-acp` and `codex-acp` while live CLI verdict remains gated.** The spec's Task 1 wants a per-CLI HTTP-vs-stdio decision, but the credentialed live confirmation is still blocked on the human. I chose the already-proven in-process HTTP transport for both backends and left the stdio fallback branch in `registerMessagingMCP` for a future verdict. **To reverse:** change `usesHTTPMessagingMCP(backendType)` for any backend that rejects HTTP and implement/enable the `agentdeck mcp` proxy path.
- **NEW (5.3): direct MCP calls without a runtime turn use implicit turn `t_000000000000`.** Runtime-owned turns still reset real `t_` counters at user/nudge turn boundaries. The implicit row exists so direct MCP tests/manual calls have deterministic budget accounting instead of bypassing the loop cap or failing before a runtime turn. **To reverse:** make `CurrentTurnBudget`/`ConsumeTurnBudget` return an error when no runtime-created row exists and require tests/manual callers to reset one first.
- **NEW (5.1): `go` directive bumped `1.22 → 1.25.0`.** `go get github.com/modelcontextprotocol/go-sdk`
  auto-raised the directive to the SDK's minimum (1.25.0); local toolchain is go1.25.5, all builds/tests
  green. Forced, not chosen — the v1.x SDK the spec mandates requires it. **To reverse:** only by dropping
  the SDK, which the phase can't do. No action expected; flagging because a toolchain-floor bump is a
  durable repo change.
- **NEW (5.1): `/mcp` registered for explicit `POST`/`GET`/`DELETE`, not method-agnostic.** A bare
  method-agnostic `mux.Handle("/mcp", …)` panics — Go 1.22 mux rejects it as conflicting with the
  existing `OPTIONS /` CORS route ("matches more methods but more specific path"). I registered the three
  methods the streamable transport actually uses. **To reverse/extend:** if a future transport needs more
  verbs on `/mcp`, add them explicitly (don't go method-agnostic while `OPTIONS /` exists).
- **NEW (5.2): Phase-0 placeholder `messages` table + its CRUD were REPLACED, not extended.** Migration v5
  drops+recreates `messages` with the §4.1 shape (TEXT `message_id` PK vs the old INTEGER autoincrement) and
  **removes the agent FK / `ON DELETE CASCADE`** (mail must outlive a stopped/deleted agent until the janitor —
  §4.3). The old `state.Message` type and `WriteMessage`/`ReadMessage`/`DeleteMessage`/`ListMessages(to)` are
  gone, replaced by the §3.2 API. The spec contradicted shipped Phase-0 code here; I treated the Phase-0 table
  as the placeholder it was. **Test impact (flagged):** `TestDeleteAgentCascades` now asserts a message
  *survives* its deleted sender (was: cascaded away); migration-count asserts 5 not 4. **To reverse:** none
  sensible — Phase 5 needs this schema. Existing local DBs auto-migrate (the placeholder table held no real data).
- **NEW (5.2): `InsertMessage` returns `(string, error)`, not the spec's `error`.** §3.2 lists
  `InsertMessage(m Message) error`, but §4.1 also requires the server to mint `message_id` with collision-retry.
  I put that minting in `InsertMessage` and return the id (the `send_message` handler needs it for its response).
  **To reverse:** move id-minting into the handler and restore the `error`-only signature.
- **NEW (5.2): tool results are JSON-in-TextContent with `IsError`, `Out`=`any` (no output schema).** The spec's
  success and error payloads have different shapes; rather than fight the typed-output inference I marshal each
  payload to a single text content and set `IsError` on errors (matching §3.3–§3.5 "content[0].text = JSON"). The
  go-sdk still validates *input* schemas strictly (extra args are rejected before the handler — relevant when
  testing). **To reverse:** define typed `Out` structs per tool and use structured content.
- **NEW (5.1): spike kept, not throwaway; `messaging.New` already takes `*state.Store`.** The spec allows
  throwaway-or-keep; I built `internal/messaging` as the keep-able foundation 5.2 extends (the `ping` tool
  is the only throwaway part — 5.2 replaces it with the three real tools). `New(store, log)` takes the
  store now (the ping tool ignores it) to avoid a constructor-signature churn next subphase. The existing
  `launch.go::messagingServer` stdio stub is left untouched and will be **superseded** by 5.3's
  `RegisterMessagingMCP`. **To reverse:** none needed; it's additive.

- **NEW (review fix): seeded-`my-app`-cwd advisory addressed only by surfacing the failure, not by
  pre-launch validation.** The advisory offered two arms: (a) steer users to set a real project before
  launch, or (b) surface the launch failure more directly. I did (b) — `NewAgentModal` now shows the
  server's `error.message` (e.g. "project cwd does not exist") instead of "HTTP 502" — because it's
  bounded and clearly correct. I did **not** do (a): adding pre-launch cwd validation or changing the
  `cwd_not_found` onboarding gate is a design decision the spec explicitly permits as-is, so it's left
  for the human. The seed still points `my-app` → `~/Projects/my-app`. **To take arm (a):** add a
  pre-launch existence check (server 422 or modal-side warning) and/or promote `cwd_not_found` to a hard
  gate. Deleted the finding bullet since the actionable part is fixed.
- **NEW (review fix): archive FTS now indexes the COMPLETE transcript — unbounded buffer chosen over a
  segment model.** The 1 MiB cap was data-loss (older phrases unsearchable), so I removed it. The
  reviewer offered two fixes: (a) index complete content, or (b) a bounded-but-specified segment model.
  I took (a) because it's minimal and zero-risk to the existing single-row `sessions_fts` schema and the
  archive search/COUNT/snippet query — a segment model would need a schema migration (FTS5 can't
  `ALTER ADD COLUMN`, so a drop+recreate) and dedupe/aggregation across multiple rows per agent.
  **Tradeoff:** the per-agent in-memory `content` buffer now grows with the session, and each `turn_end`
  flush rewrites the full FTS row (DELETE+INSERT) → O(n) per turn, ~O(n²) cumulative over one very long
  session. Fine for normal personal use (transcripts of a few MiB); a multi-tens-of-MiB single session
  would get costly. **To reverse / harden later:** implement the segment model (bounded chunk rows per
  agent, append-only, rewrite only the active chunk; archive query groups by `agent_id`, best snippet
  per agent). Guard test: `TestIndexerFTSLongTranscript`.

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
- **NEW (4.6): `Server` stores a shared `*index.Indexer` field.** The registry's persistence path and the hook capture both use the same indexer instance so the in-memory FTS content accumulator is shared. To reverse: create a second indexer for hook capture only (no harm beyond a second seed per agent per process).
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

- 2026-06-29 — **review fix: budget_exceeded toast names the agent + dismissed recipient-badge false positive — green.** New `bus.PublishBudgetExceeded` routes breaches through `notificationPayload` (the existing `budget_exceeded` case) using the agent's snapshot, so the toast carries `agent_name`/`address`/named title instead of the old inline generic payload; `SetBudgetExceededSink` now uses it. Tests: `TestPublishBudgetExceededNamesAgent` + `…FallsBackToAgentID`. **Dismissed** the "recipient unread badge doesn't update live" advisory as a false positive: the message-inserted sink calls `stateMgr.Touch(toAgentID)`, and `Touch`→`recomputeAndPublish` already `PublishStateUpdate`s the recipient with the recomputed `unread_messages` (the inline `SetSnapshot` was merely redundant — `PublishStateUpdate` already sets the snapshot). Dropped that redundant `SetSnapshot`; guard test `TestTouchRecipientPublishesUnread`. Green both tag modes.
- 2026-06-29 — **review fix: turn budget single-row-per-agent — green (also fixes unbounded growth).** `ResetTurnBudget` now deletes the agent's other `turn_budget` rows in-tx so at most one row survives per agent. Fixes the restart+resume blocker: `turnSeq` resets to 0 on a fresh process, so a resumed agent re-emitted low `turn_id`s while prior-session rows kept the highest rowids — `currentBudgetTx`'s `ORDER BY rowid DESC` read a stale/breached row and could block `send_message`/`check_messages`. One row per agent also caps `turn_budget`'s formerly unbounded growth (resolves that advisory too). Test `TestResetTurnBudgetReusesSingleRow` simulates the restart, asserts the freshly-reset `t_…01` is read (0 used, not the stale `t_…02`), and that exactly one row remains. Green both tag modes.
- 2026-06-29 — **5.4 green / Phase 5 COMPLETE — notifications + dashboard message indicators.** `AgentState` now includes `unread_messages` and `last_sent_at`; message sends touch recipient/sender state for unread badges and outbound pulse; bus emits edge-triggered `notification` SSE for done/waiting_input/permission_required plus the existing budget_exceeded path. `config.json` gained `notifications.desktop_enabled` + per-type mutes via existing `GET/PUT /api/config`; UI consumes notification SSE, sends hidden-tab desktop notifications when permitted, visible-tab toasts otherwise, and adds Settings notification toggles. Added read-only `GET /api/sessions/{id}/messages`. Embedded UI refreshed. Tests: Go notification/indicator/config/inbox coverage; UI mute + hidden desktop notification + settings toggle. Checkpoint green: Go standard/tagged build+tests, `cd ui && npm test`, `cd ui && npm run build`.
- 2026-06-29 — **5.3 green — registration, nudger, turn budget, janitor.** Added per-agent HTTP MCP registration files + token cleanup wired through launch/resume/stop/shutdown; chat `CheckMessages(pid)` now injects a nudge turn and runtime turns reset `turn_budget`. `send_message`/`check_messages` enforce the shared 15-action budget transactionally (`message_budget_exceeded`, persisted `breached=1`, WARN + `budget_exceeded` SSE); nudger wakes idle agents on ticker/insert signal and stamps `delivered_via='nudge'`; poll reads now stamp `delivered_via='poll'`; janitor deletes read>24h and any>7d. Tests cover registration cleanup, runtime/server nudge, budget breach/caps, retention, and poll stamping. Checkpoint green: `go build ./...`, `go build -tags sqlite_fts5 ./...`, `go test ./...`, `go test -tags sqlite_fts5 ./...`.
- 2026-06-29 — **5.2 green — message store + three MCP tools.** Migration v5 replaces the Phase-0 placeholder `messages` table with the §4.1 schema (TEXT `message_id` PK, no agent FK) + adds `turn_budget`. New `state` messaging API (`LiveAgents`, `ResolveRecipient`, `InsertMessage`, `ListMessages`, `MarkRead`, `DeleteMessages`, `UnreadCount`) + `Message`/`LiveAgent`/`AgentRef` types. `messaging` package: `list_agents`/`send_message`/`check_messages` tools replace the `ping` spike, identity from the session token (`req.Extra.Header`→`Lookup`, unknown→`session_unknown`), §9 error shapes, locked §13 constants. Budget enforcement deferred to 5.3 (static cap + TODO). New state + messaging tests; updated Phase-0 state tests (cascade now asserts mail survives a deleted sender) + `server.TestMCPRouteMounted`. Build + full tests green both tag modes.
- 2026-06-29 — **5.1 green — Go-MCP-SDK handshake spike.** Added `github.com/modelcontextprotocol/go-sdk v1.6.1` (`go` 1.22→1.25.0). New `internal/messaging` package: in-process `mcp.Server` + trivial `ping` tool over the streamable HTTP transport, mounted at `POST/GET/DELETE /mcp`; `token→agent_id` session registry; `X-AgentDeck-Token` header read per request. HTTP transport round-trip proven via `messaging.TestSpikePingRoundTrip` + `server.TestMCPRouteMounted` (go-sdk client through the real dashboard mux). Per-CLI live confirmation gated (Blocked on human). Task 1 outcome recorded in techspec §2.2. Build (both tags) + full tests green.
- 2026-06-29 — **review fix: bare-form CLI resume respects `--name` — green.** `listInactiveSessions` now takes a `name` arg and filters by it when non-empty, so `agentdeck role@project --name X` only auto-resumes the inactive session actually named X (not any inactive role@project session). New `resume_test.go::TestListInactiveSessionsNameFilter` (no-name → all; named → exact). Build (both tags) + full tests green.
- 2026-06-29 — **review fix: 4 Phase 2/3 UI advisories — green (53/53 UI, embedded dist refreshed).** (1) `transcriptStore` gained `foldTranscript`, used by `setTranscript`, so a REST refetch/archive replay folds `permission_resolved` into its `permission_request` (was left visually unresolved); regression test added. (2) `useDeleteRole`/`useDeleteProject` now throw the structured `{status, body}` error (shared `httpError` helper) so the editors' 409 `?force=true` retry actually fires; `useDeleteRole` rejection test. (3) `NewAgentModal` now reads `/api/config` and preselects `default_role`/`default_project` (falls back to first entry only when absent); preselect test. (4) Launch failures now surface the server's `error.message` (e.g. nonexistent project cwd) instead of opaque "HTTP 502"; test added — partially addresses the seeded-`my-app`-cwd advisory (see Autonomous decisions).
- 2026-06-29 — **review fix: ACP protocol version mismatch now fails the handshake — green.** `ChatRuntime.Start`/`Resume` previously only `slog.Warn`ed on an out-of-range `protocolVersion`; per techspec §12.1 they now fail via new `checkACPVersion` + `ErrProtocolVersion` (pinned `[minACPVersion,maxACPVersion]` = `[1,1]`; missing/0 tolerated). fakeacp honors `FAKEACP_PROTO_VERSION`; new `TestStartProtocolVersionMismatch` asserts Start errors with `ErrProtocolVersion`. Build (both tags) + full tests green.
- 2026-06-29 — **review fix: stop is idempotent for known agents — green.** `handleStop` now returns 200 `{stopped:true}` when `Registry.Stop` reports `ErrNoHandle` but the identity row still exists (double-click / lost-response retry); 404 reserved for ids with no identity. New `TestStopIdempotent` (first stop 200, repeat 200, unknown id 404).
- 2026-06-29 — **review fix: archive FTS no longer drops content past 1 MiB — green.** Removed the `maxContentBytes` keep-newest cap in `index.Indexer.addContent`; the FTS content buffer now accumulates the COMPLETE transcript so every phrase ever streamed stays searchable (and `reindex` rebuilds it complete). New tagged `TestIndexerFTSLongTranscript` indexes an early phrase + >1 MiB of later content and asserts the early phrase still `MATCH`es. Build (both tag modes) + full + tagged index/archive/state tests green. See Autonomous decisions for the unbounded-growth tradeoff.
- 2026-06-29 — **review fix: session/load resume now applies fresh MCP registration — green.** `ChatRuntime.Resume` called `session/load` with only `{sessionId}`, so adapters where load succeeds never received the freshly-minted messaging MCP server (Phase 5 blocker). Added `sessionLoadParams(spec, sessionID)` (sessionId + cwd + mcpServers, mirroring ACP loadSession) and use it on the load path. fakeacp now dumps received `session/load` params via `FAKEACP_LOAD_DUMP`; new `TestResumeSessionLoadAppliesMCP` asserts the load path carries sessionId + the messaging server. Go build (both tag modes) + full tests green.
- 2026-06-29 — **review fix: SSE watchdog permanent reconnect loop — green.** `ui/src/api/sse.ts` now resets `lastPing` in `connect()` so each fresh/reconnected stream gets the full 25s liveness window instead of inheriting a stale timestamp that reaped it before its first ping. New `src/api/sse.test.ts` drives a mock `EventSource` + fake timers: reaps the first (ping-less) stream at 30s, then asserts the reconnected stream survives the 5s watchdog tick before its ~10s first ping. UI 49/49, build green, embedded dist refreshed.
- 2026-06-29 — **Workflow: trunk-based + `/fix-review` added; `impl/phase-4` merged to `main`.** Switched the build/review/fix workflows to commit **directly on `main`** (no per-phase branches, no PRs — workflow §6, work-phase/fix-review skills, AGENTS.md). Added the **`/fix-review`** skill + workflow §9: validate each review finding is actually true, then fix the real ones to green; review-phase (§8) now writes **both** BLOCKING and ADVISORY findings to `## Review findings`, and resolved/dismissed findings are **deleted** (changelog is the record), not kept (§5). Fast-forwarded `impl/phase-4` (Phase 4.6) into `main` and re-verified green: tagged + standard `go build`, full `go test`, `cd ui && npm test` (48/48), `cd ui && npm run build`. Not pushed.
- 2026-06-29 — **Phase 4 COMPLETE / 4.6 green.** `GET /api/sessions/{id}/files` + `GET /api/sessions/{id}/commands` over `tracked_files`/`tracked_commands`; `POST /api/hook` extended for `file_edit`/`command` events via `Indexer.CaptureHookFile`/`CaptureHookCommand`; `Store.ValidateHookToken` token guard. Frontend: `/archive` route (search + result list + snippet + state chip), `/archive/:id` read-only transcript view with Resume button, ChatPanel Files/Commands tabs with filter/copy/diff-link, Archive nav link. 18 new Vitest tests. All 48 UI tests green; `go build ./...`; full Go tests; tagged FTS build; UI build.
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
- 2026-06-28 — **Codex review pass (branch `claude/codex-issue-review-jhrf6m`).** Fixed two implemented-code
  issues with tests (build + `go test ./...` + `-race` green): crash-teardown registry-ownership leak
  (BLOCKING — `chat.go`/`registry.go`, new `registry_crash_test.go`) and idle-cancel no-op reporting
  (`Runtime.Cancel`→`(bool,error)`, `sessions.go`). Recorded the remaining future-phase findings into
  `## 0` sections of the Phase 2/3/4 techspecs (see Review findings above).
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
