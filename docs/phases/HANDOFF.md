# AgentDeck — Implementation Handoff

**Live state. Read this first, every session. Update it after every change.**
Protocol: [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) (Claude Code or Codex, whichever the human runs).
Keep this lean — apply the condensation rules (workflow §5); old detail lives in git, not here.

---

## Current position

- **Active phase:** 6 — Flexibility: terminal runtime, switch-runtime, task groups
- **Active subphase:** 6.3 (next) — terminal runtime (xterm/PTY default + tmux)
- **Spec:** [`tech/phase-6-flexibility-techspec.md`](tech/phase-6-flexibility-techspec.md) (PRD: [`phase-6-flexibility.md`](phase-6-flexibility.md)); subphase plan at §"Subphase plan"
- **Last GREEN checkpoint:** 6.2 @ `main`: `go build ./...`, `go build -tags sqlite_fts5 ./...`, `go test ./...`, `go test -tags sqlite_fts5 ./...` (6.1/6.2 are Go-only — no `ui/` change).
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

**Subphase 6.1 ✅ — hook ingest hardened + backend adapter + Codex (chat).** `internal/backend/adapter.go`
(`BackendAdapter` for `claude-acp`/`codex-acp`: binary, env-strip keys, `ResolveResumeID`, `CanSwitchModelOnResume`,
`HookMap`/`UnsupportedHookEvents`); chat runtime resolves spawn binary/env-strip per adapter (codex now runs through
the chat runtime); `/api/hook` accepts the terminal lifecycle events + 401-on-stale-token. Details in changelog.

**Subphase 6.2 ✅ — hook scripts + registration + interface gate.** New `internal/hooks`: embedded `_post.sh`
(jq-encoded `curl POST /api/hook`, interface gate) + 5 event wrappers, `Install(home)` (rewritten on dashboard
startup), `ClaudeSettings`/`WriteAgentSettings`. Launch + resume inject `AGENTDECK_*` env and write a per-agent
settings file; `BackendAdapter.HookLaunchArgs` (claude `--settings <path>`, codex gated). The `--settings`
passthrough is gated behind `AGENTDECK_HOOK_REGISTRATION=1` (default off) so real launches aren't regressed. Details
in changelog.

**Subphase 6.3 — next to implement** (terminal runtime: xterm/PTY default + tmux; techspec §3, §8.5, task 5):
- [ ] `internal/runtime/terminal` implementing `Runtime` (`Start/SendPrompt/Cancel/Stop/Resume/CheckMessages`), registered under `interface == "terminal"` (replaces the `notImplementedRuntime` stub).
- [ ] `TerminalDriver` seam (`StartTab`, `WriteText`, `ReadTTY`, `CloseTab`, `RevealTab`); xterm.js/PTY driver (`github.com/creack/pty`) + tmux driver.
- [ ] PTY↔WebSocket bridge at `/api/sessions/{id}/terminal/ws` (keystrokes → PTY master; output → frames; `{cols,rows}` → `pty.Setsize`).
- [ ] `terminal.Capabilities()` + `GET /api/capabilities` (xterm always available, tmux if on PATH, iterm2 omitted off-darwin); `tty`/`driver`/`driver_ids` written to the running row.
- [ ] Status flows from hooks only (the 6.2 scripts already POST when `AGENTDECK_INTERFACE=terminal`); runtime sets only the initial idle (race-guarded) + a terminal done on Stop.
- **Checkpoint:** `go build ./...` + `go test ./...`; PTY-bridge unit tests (keystroke→master write, output→frame, resize→`Setsize`); `GET /api/capabilities` returns `xterm:true`, `default_driver:"xterm"`; a terminal agent launches, records `tty`, idle→busy→idle via hook POSTs.
- **Resume note:** hooks POST terminal status (6.2) but `interface=="terminal"` still returns "not implemented". Note: the running-row schema already has `tty`; `driver`/`driver_ids` columns do NOT exist yet — add a state migration or store them in an existing column. Begin with the `TerminalDriver` interface, then the PTY driver + WS bridge, then capabilities.

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

- **GATED (not blocking 6.2): live Codex (codex-acp) chat acceptance.** 6.1 wired `codex-acp` end-to-end through the
  chat runtime and proved launch→prompt→stream→stop→native-resume against **fakeacp** (the codex adapter supplies the
  binary/env/resume). What's gated: a real `codex-acp` CLI + OpenAI credentials to confirm the live handshake, model
  arg, and native resume. Same class as the Phase 1 real-CLI run. **To do (human):** install `codex-acp`, set
  `CODEX_HOME`/`OPENAI_API_KEY`, launch a Codex chat agent, run a turn, stop, resume; if the live hook event names
  differ from Claude's, note them so 6.2's registration + `codexACP.HookMap()` are corrected.

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

- **NEW (review fix, supersedes the 6.2 env-flag gate): CLI hook-registration `--settings` passthrough is now gated
  by INTERFACE, not by `AGENTDECK_HOOK_REGISTRATION`.** The launch composer always injects the `AGENTDECK_*` env and
  writes the per-agent settings file; whether it adds the CLI flag (`claude --settings <path>`) now depends on the
  agent's interface: **terminal → ON by default** (the 6.3 terminal runtime runs the *real* interactive CLI under a
  PTY — not `claude-code-acp` — where `--settings` is a known-good flag and hooks are the only status producer);
  **chat → still gated behind `AGENTDECK_HOOK_REGISTRATION=1`** (chat runs through `claude-code-acp`, whose
  `--settings` forwarding is unverified, AND doesn't need registration — the runtime owns chat status and `_post.sh`
  self-suppresses). This resolved the review's BLOCKING finding without regressing the green chat path. **Why this is
  a judgment call:** I chose interface-gating over either flipping the env-flag default (would risk the chat path) or
  building the `.claude/settings.json` project-injection fallback (writes into the user's project dir, can clobber
  user settings). **To reverse:** restore the unconditional `AGENTDECK_HOOK_REGISTRATION` gate in
  `composeHookRegistration`. Codex's `HookLaunchArgs` still returns nil (its hook surface is gated regardless).
- **NEW (6.2): hook scripts require `jq` + `curl` on PATH (POSIX `sh`).** Per techspec §2.3 these are documented
  prereqs (no python3/node at runtime). `_post.sh`'s interface gate runs before `jq`/`curl`, so a chat agent
  self-suppresses even without them; a terminal agent needs both to POST. No fallback is provided. **To reverse:**
  add a curl-less POST path (e.g. a tiny `agentdeck hook-post` subcommand) if a target host lacks them.
- **NEW (6.1): terminal-CLI `Stop` hook does NOT clear the running row.** The subphase line said "running-row
  refresh/clear on SessionStart/Stop", but Claude Code's `Stop` hook fires at the **end of each turn**, not on CLI
  exit (§4.2 footnote ties the clear to "CLI exit", a separate signal). Clearing on every `Stop` would unregister a
  live idle terminal agent. So `SessionStart` refreshes the running row's `session_id`/`tty`; `Stop` only applies
  idle/done status. The running-row clear stays with the runtime's `Stop`, the explicit internal `stopped` event, and
  the 6.6 liveness sweep. **To reverse:** if a real terminal CLI emits `Stop` only on exit, add a running-row delete
  to the `Stop` case in `manager.go::ApplyHook`.
- **NEW (6.1): `/api/hook` token errors realigned to §8.6 on the status path — 401 `bad_token`, 404 `agent_not_found`.**
  Was 403 `forbidden` / 404 `not_found`. The subphase requires "stale token → 401". The file_edit/command **tracking**
  path (Phase 4) is untouched (still 403 `forbidden`). Updated `TestHookValidationErrors` expectations accordingly.
  **To reverse:** restore the prior codes in `hook.go` (status switch) — but §8.6 mandates these.
- **NEW (6.1): Codex `HookMap` mirrors Claude's lifecycle keys — GATED, unverified against a live codex-acp.** Same
  class as the Phase 1 real-CLI / Phase 5 two-CLI gates: without codex-acp credentials I can't confirm Codex's real
  hook event names. I targeted the five Claude keys (`SessionStart`…`Stop`); any Codex rejects in 6.2 move that event
  into `UnsupportedHookEvents` and the terminal runtime backfills it from ACP. The Codex chat e2e (launch→prompt→
  stream→stop→native-resume) is proven against **fakeacp**, not a real codex-acp CLI — the credentialed live Codex run
  remains gated (see Blocked on human). **To reverse:** edit `codexACP.HookMap()` once the live surface is known.
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

- 2026-06-29 — **review fix: hook registration enabled by default for terminal + per-agent settings cleanup — green.**
  (1) BLOCKING: `composeHookRegistration` no longer disables registration in the default path — it now gates by
  *interface*: terminal agents get the `--settings` launch args by default (the terminal runtime runs the real CLI
  under a PTY where the flag is known-good and hooks are the only status producer), while chat stays gated behind
  `AGENTDECK_HOOK_REGISTRATION=1` (claude-code-acp flag-forwarding unverified; chat doesn't need hook registration).
  This unblocks 6.3 terminal status without regressing the green chat path (see Autonomous decisions for the judgment
  call). Test `TestComposeHookRegistrationTerminalDefault` (terminal → `--settings <path>` with no env flag); existing
  `TestComposeHookRegistration` keeps chat default-off + chat self-suppression covered by the hooks interface-gate
  test. (2) ADVISORY: per-agent `{home}/hooks/agents/{id}.json` is now deleted on stop, launch-rollback, and shutdown
  (new `hooks.RemoveAgentSettings`/`RemoveAllAgentSettings` + `Server.cleanupHookSettings`, mirroring
  `cleanupMessagingMCP`). Test `TestStopRemovesHookSettings` (file present at launch, gone after stop). Green both tag modes.
- 2026-06-29 — **6.2 green — hook scripts + registration + interface gate.** New `internal/hooks` package: embedded
  POSIX-`sh` script set — `_post.sh` (jq-encoded body → `curl POST /api/hook`, with the `AGENTDECK_INTERFACE=chat`
  self-suppression gate for runtime-owned events) + `session-start/user-prompt-submit/pre-tool-use/post-tool-use/stop.sh`
  wrappers; `Install(home)` atomically (re)writes them to `{home}/hooks` on dashboard startup; `ClaudeSettings` +
  `WriteAgentSettings` compose a per-agent Claude hooks settings file from the adapter `HookMap`. Launch + resume now
  inject `AGENTDECK_HOOK_URL/TOKEN/AGENT_ID/INTERFACE` env and write the settings file; new
  `BackendAdapter.HookLaunchArgs` (claude `--settings <path>`, codex nil/gated) feeds `LaunchSpec.ExtraArgs`, appended
  to the spawn argv. The `--settings` activation is gated behind `AGENTDECK_HOOK_REGISTRATION=1` (default off) so real
  launches aren't regressed by an unverified flag (see Autonomous decisions). Tests: hooks install/executability,
  `ClaudeSettings` shape, hermetic interface-gate (shimmed curl+jq: chat→no POST, terminal→POST); server hookEnv +
  composeHookRegistration; adapter `HookLaunchArgs`. Green both tag modes (Go-only).
- 2026-06-29 — **6.1 green — hook ingest hardened + backend adapter + Codex (chat).** New `internal/backend/adapter.go`:
  `BackendAdapter` for `claude-acp`/`codex-acp` carrying `Binary`/`LaunchArgs`/`StripEnvKeys`/`ResolveResumeID`/
  `CanSwitchModelOnResume`/`HookMap`/`UnsupportedHookEvents`. `ChatRuntime` now resolves the spawn binary + env-strip
  per adapter (claude strips `CLAUDECODE`; codex strips nothing) instead of hardcoding claude — **codex-acp now runs
  through the chat runtime** (gate accepts known backends, rejects unknown with `ErrNotImplemented`). `/api/hook`
  accepts the terminal lifecycle events (`SessionStart`/`UserPromptSubmit`/`PreToolUse`/`PostToolUse`/`Stop`):
  SessionStart refreshes the running row `session_id`/`tty` (new `HookPayload.TTY`); the rest are pure status
  producers; **Stop does not clear the running row** (per-turn — see Autonomous decisions). Status-path token errors
  realigned to §8.6 (`401 bad_token`, `404 agent_not_found`). Per-model env was already layered in `composeLaunch`
  (model env overrides backend env). Tests: `backend` adapter units; runtime backend-gate + Codex chat e2e
  (launch→prompt→stream→stop→native-resume vs fakeacp); server hook-lifecycle ingest (SessionStart refresh, PreToolUse
  busy+publish, Stop keeps running row, stale→401). Green both tag modes; live codex-acp run gated (Blocked on human).
- 2026-06-29 — **review fix: budget_exceeded toast names the agent + dismissed recipient-badge false positive — green.** New `bus.PublishBudgetExceeded` routes breaches through `notificationPayload` (the existing `budget_exceeded` case) using the agent's snapshot, so the toast carries `agent_name`/`address`/named title instead of the old inline generic payload; `SetBudgetExceededSink` now uses it. Tests: `TestPublishBudgetExceededNamesAgent` + `…FallsBackToAgentID`. **Dismissed** the "recipient unread badge doesn't update live" advisory as a false positive: the message-inserted sink calls `stateMgr.Touch(toAgentID)`, and `Touch`→`recomputeAndPublish` already `PublishStateUpdate`s the recipient with the recomputed `unread_messages` (the inline `SetSnapshot` was merely redundant — `PublishStateUpdate` already sets the snapshot). Dropped that redundant `SetSnapshot`; guard test `TestTouchRecipientPublishesUnread`. Green both tag modes.
- 2026-06-29 — **review fix: turn budget single-row-per-agent — green (also fixes unbounded growth).** `ResetTurnBudget` now deletes the agent's other `turn_budget` rows in-tx so at most one row survives per agent. Fixes the restart+resume blocker: `turnSeq` resets to 0 on a fresh process, so a resumed agent re-emitted low `turn_id`s while prior-session rows kept the highest rowids — `currentBudgetTx`'s `ORDER BY rowid DESC` read a stale/breached row and could block `send_message`/`check_messages`. One row per agent also caps `turn_budget`'s formerly unbounded growth (resolves that advisory too). Test `TestResetTurnBudgetReusesSingleRow` simulates the restart, asserts the freshly-reset `t_…01` is read (0 used, not the stale `t_…02`), and that exactly one row remains. Green both tag modes.
- 2026-06-29 — **5.4 green / Phase 5 COMPLETE — notifications + dashboard message indicators.** `AgentState` now includes `unread_messages` and `last_sent_at`; message sends touch recipient/sender state for unread badges and outbound pulse; bus emits edge-triggered `notification` SSE for done/waiting_input/permission_required plus the existing budget_exceeded path. `config.json` gained `notifications.desktop_enabled` + per-type mutes via existing `GET/PUT /api/config`; UI consumes notification SSE, sends hidden-tab desktop notifications when permitted, visible-tab toasts otherwise, and adds Settings notification toggles. Added read-only `GET /api/sessions/{id}/messages`. Embedded UI refreshed. Tests: Go notification/indicator/config/inbox coverage; UI mute + hidden desktop notification + settings toggle. Checkpoint green: Go standard/tagged build+tests, `cd ui && npm test`, `cd ui && npm run build`.
- 2026-06-29 — **5.3 green — registration, nudger, turn budget, janitor.** Added per-agent HTTP MCP registration files + token cleanup wired through launch/resume/stop/shutdown; chat `CheckMessages(pid)` now injects a nudge turn and runtime turns reset `turn_budget`. `send_message`/`check_messages` enforce the shared 15-action budget transactionally (`message_budget_exceeded`, persisted `breached=1`, WARN + `budget_exceeded` SSE); nudger wakes idle agents on ticker/insert signal and stamps `delivered_via='nudge'`; poll reads now stamp `delivered_via='poll'`; janitor deletes read>24h and any>7d. Tests cover registration cleanup, runtime/server nudge, budget breach/caps, retention, and poll stamping. Checkpoint green: `go build ./...`, `go build -tags sqlite_fts5 ./...`, `go test ./...`, `go test -tags sqlite_fts5 ./...`.
- 2026-06-29 — **5.2 green — message store + three MCP tools.** Migration v5 replaces the Phase-0 placeholder `messages` table with the §4.1 schema (TEXT `message_id` PK, no agent FK) + adds `turn_budget`. New `state` messaging API (`LiveAgents`, `ResolveRecipient`, `InsertMessage`, `ListMessages`, `MarkRead`, `DeleteMessages`, `UnreadCount`) + `Message`/`LiveAgent`/`AgentRef` types. `messaging` package: `list_agents`/`send_message`/`check_messages` tools replace the `ping` spike, identity from the session token (`req.Extra.Header`→`Lookup`, unknown→`session_unknown`), §9 error shapes, locked §13 constants. Budget enforcement deferred to 5.3 (static cap + TODO). New state + messaging tests; updated Phase-0 state tests (cascade now asserts mail survives a deleted sender) + `server.TestMCPRouteMounted`. Build + full tests green both tag modes.
- 2026-06-29 — **5.1 green — Go-MCP-SDK handshake spike.** Added `github.com/modelcontextprotocol/go-sdk v1.6.1` (`go` 1.22→1.25.0). New `internal/messaging` package: in-process `mcp.Server` + trivial `ping` tool over the streamable HTTP transport, mounted at `POST/GET/DELETE /mcp`; `token→agent_id` session registry; `X-AgentDeck-Token` header read per request. HTTP transport round-trip proven via `messaging.TestSpikePingRoundTrip` + `server.TestMCPRouteMounted` (go-sdk client through the real dashboard mux). Per-CLI live confirmation gated (Blocked on human). Task 1 outcome recorded in techspec §2.2. Build (both tags) + full tests green.
