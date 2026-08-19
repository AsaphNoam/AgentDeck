# TS-04 — Integration protocols

**Status:** Partial
**Code:** `internal/runtime`, `internal/hooks`, `internal/messaging`, `internal/server`, `internal/backend`
**Absorbed:** exact source mapping in the [phase archive manifest](../../archive/phases/README.md)

## 1. Scope

This spec owns protocol boundaries between AgentDeck and agent CLIs: Agent Client Protocol (ACP),
lifecycle hooks, Model Context Protocol (MCP) messaging, terminal PTY/WebSocket framing, and external
CLI compatibility policy.

## 2. Design & constraints

**R1 — Chat uses ACP over child stdio.** The adapter is launched as a process group. AgentDeck
performs ACP initialize, starts or loads a session, sends prompts, maps streamed updates to normalized
events, and terminates the whole group on stop/failure. JSON-RPC ids are correlated and malformed or
unknown notifications cannot crash the runtime.

**R2 — ACP is normalized before product code consumes it.** Provider-specific content becomes the
internal event vocabulary (`assistant_text`, tool call/update, permission request/resolution,
turn/error boundaries). Persistence, SSE, indexing, and UI consume normalized events, not raw ACP.

**R3 — Session start/load omit inherited model fields.** When federation says the provider-native
model is authoritative, AgentDeck omits the ACP model key. An explicit user/model override is sent.
Provider identifiers are adapter-owned; silently substituting a different model is a compatibility
deviation that must remain visible.

**R4 — Permissions are a single-winner protocol.** A permission request remains pending until one
approve/deny/cancel/timeout path atomically claims it. Exactly one ACP response and one normalized
resolution are emitted. Unknown/already-resolved decisions return a conflict.

**R5 — Hooks are authenticated HTTP producers.** Each launch receives a random scoped token and
loopback hook URL. Hook POSTs include token, agent identity, event type, and safe payload. The server
validates token→agent binding before updating status/tracking; hook scripts never write SQLite.

**R6 — Messaging is one in-process MCP authority.** `/mcp` exposes `list_agents`, `send_message`,
and `check_messages` from the Go process. Each chat launch receives a scoped registration/token.
No second MCP process owns state. Transport is loopback streamable HTTP.

**R7 — MCP identity is server-derived.** Tool callers cannot choose their sender id. The token maps
to the live agent; recipient resolution follows FS-06. Registration creation and teardown are
generation-scoped so an old runtime cannot delete a new runtime's identity.

**R8 — Terminal uses a driver seam.** xterm owns a server-side PTY and WebSocket bridge; tmux owns a
reattachable session; iTerm2 is an optional macOS driver. Terminal input is raw bytes, while a JSON
text frame with `cols`/`rows` requests resize. Viewer disconnect never stops the runtime.

**R9 — External CLI capabilities fail honestly.** Missing binaries, rejected flags, failed
initialize, unavailable credentials, and unsupported interface/backend combinations return bounded,
backend-specific errors. AgentDeck does not claim a capability solely because a binary exists.

**R10 — retired 2026-08-14:** Startup bounds and diagnostics shipped as R22; optional-integration
flag fallback/probing remains planned as R23.

**R11 `(planned)` — Real-provider compatibility gates are recorded.** Claude/Codex MCP registration,
Codex chat resume, Claude terminal flags/hooks, and OpenCode/OpenHands launch flows require pinned,
credentialed acceptance before a release claims those combinations.

**R13 — Claude chat uses the official adapter boundary.** The `claude-acp` backend
executes the pinned `claude-agent-acp` package entry point and speaks ACP protocol version 1. The
adapter owns its compatible native Claude executable; AgentDeck passes provider configuration only
through documented ACP session metadata and uses the adapter's `--cli` delegation for credential
checks. Interactive terminal launch and hook settings remain a direct-Claude-CLI path.

**R14 — Prompt delivery is adapter-specific and fail-closed.** Claude receives the composed prompt
through its documented ACP metadata. `codex-acp` does not consume an ACP `systemPrompt` or prompt
metadata field; AgentDeck instead merges the composed start prompt into the adapter's documented
`CODEX_CONFIG` JSON object as `developer_instructions` before spawning it. A malformed overlay or
conflicting non-string value is a launch error, not a best-effort prompt omission. The generic ACP
shape omits `systemPrompt` for Codex.

**R15 — Native-auth readiness is a fixed-command probe, not a dashboard login flow.**
One provider-metadata helper owns the CLI `agentdeck auth` login argv and each non-interactive
readiness probe. The onboarding UI supplies only static provider guidance and reuses the ordinary
backend save/check (TS-03.R15); the server never starts or proxies a login command. Each probe uses
an explicit allowlisted executable and argv through `exec.CommandContext` (never a shell), a bounded
deadline, inherited provider environment, and sanitized bounded diagnostics. Claude retains its
adapter-delegated `auth status` probe and no-color compatibility retry. Codex first asks the pinned
private Codex CLI for `login status`; a successful native result is sufficient, otherwise a
configured `OPENAI_API_KEY` is checked through the existing models endpoint. Raw status output,
account identity, and credential values never cross the process/log/API boundary.

**R17 — Pipeline tools extend the one scoped MCP authority.** The existing `/mcp`
server adds `report_pipeline_stage_result`, `propose_pipeline_template`, and
`propose_pipeline_run`; there is no `start_pipeline_run` tool. All derive caller identity from the
per-launch token and return bounded structured results. Reporting delegates to TS-09's atomic
current-attempt service. Proposal tools are limited to token-bound AgentDecker-role chat sessions,
call the canonical pipeline validator, and commit their bounded canonical proposal record before
returning data/digests: they cannot save, start, or approve. Tool registration, token generation,
teardown, transport, and redaction remain the existing R6–R7 authority rather than a second MCP
server.

**R18 — Effort delivery is adapter-declared and fail-closed.** The three shipped
providers accept effort through three structurally different mechanisms, so the adapter declares
*which* mechanism it uses and the runtime performs it; the runtime never branches on backend type
inline (the rule `internal/backend/adapter.go` already states for argv, env, and resume):

- **Model-suffix** (`codex-acp` chat) — the adapter encodes effort into the ACP model identifier as
  `model[effort]`, the shape its pinned adapter parses. Because the model id is built in two places,
  the suffix is composed by one `LaunchSpec` accessor consumed by **both** `sessionNewParams` and
  `sessionLoadParams`; INV §2 names this exact pair as having twice drifted on `model`, so effort
  must not become the third occurrence. An omitted effort yields the bare model id unchanged, so a
  catalog that declares no levels produces byte-identical parameters to today.
- **Post-session config** (`claude-acp` chat) — the adapter accepts effort only as a session
  configuration option after `session/new`/`session/load` returns, and AgentDeck will not write the
  user's native Claude settings files to seed it earlier (TS-07.R4). The runtime therefore issues the
  option call as part of its own launch sequence, **before** the runtime is registered or announced,
  and returns an error on failure so the caller's existing generation-scoped
  `teardownAgentRegistration` is the single cleanup path (INV §4). No prompt is sent in that window.
- **Argv flag** (`claude-acp` terminal) — the interactive Claude executable takes the level as a
  launch flag, composed with the existing hook-settings args.
- **None** (`opencode-acp`, `openhands-acp`) — no mechanism exists, so these adapters declare no
  effort delivery and FS-09.R39 rejects a declared level at save time rather than at launch.

Resume and switch re-apply effort through the same mechanism as launch. For the post-session
mechanism this is mandatory rather than incidental: the adapter re-reads its own settings default
when a session is loaded, so an unapplied effort silently reverts a resumed agent to a level nobody
chose — INV §1's "state derived from the old side must be explicitly republished" at a lifecycle
boundary.

**R19 — A provider-rejected effort fails the launch; it is never retried bare.**
A pinned CLI may reject a level AgentDeck's catalog declares (hand-declared Claude levels, an older
CLI, a provider that withdrew a level). INV §12's usual detect-and-retry-without-the-optional-flag
pattern is deliberately **not** applied here: retrying without effort would start an agent at a
different level than the person chose, which FS-09.R42 forbids. The rejection instead surfaces as a
bounded, backend-specific launch error naming the effort field, and the process group is terminated.
Raw provider output stays behind the bounded vocabulary per R12.

**R20 — Codex gets an isolated runtime profile.** AgentDeck launches the `codex-acp`
child with `CODEX_HOME` set to `<agentdeck-home>/codex`, a `0700` directory under the owner-only
AgentDeck home, so Codex writes its rollouts and native session index there instead of the user's
personal store (FS-09.R43). The value is composed once as a reserved, final child-environment layer:
it overrides ambient, backend, and model `CODEX_HOME` values in launch, resume, switch, and rollback.
INV §2 already names these paths as "the same thing built in parallel," and a resume that opened a
different home would abandon the session. AgentDeck's own process `CODEX_HOME` is left untouched, so
the federation resolver (`config_sources.go`) and model autosync (`codexmodels.go`) keep reading the
user's real home (FS-09.R44). Model and prompt still flow through ACP and the `CODEX_CONFIG` overlay
(R14); only the Codex child receives the isolated profile.

**R21 — The isolated profile is refreshed, never symlinked; honoring is credential-gated.**
Before each `codex-acp` child starts, one shared, serialized helper provisions its dedicated home and
one-way refreshes the provider-recognized personal setup from `${CODEX_HOME:-~/.codex}`: regular
configuration/authentication files and the skills, agents, rules, plugins, and MCP setup assets are
copied into the private profile. The helper tracks its managed destination paths so a source removal
removes only the corresponding private copy; it never deletes Codex-owned private state. It never
creates a symlink to the source, follows a source link outside the canonical personal root, or writes
into/mutates the user's real home. Session/history data, including its indexes, is excluded from the
refresh and belongs solely to the private profile; the selected top-level setup names are an explicit
allowlist, so new or unknown runtime-state files are not copied. A missing source setup asset is a
non-fatal skip; an unsafe or uncopyable selected asset fails the process start before spawn. The
profile destination must be a real directory and a canonical source/destination overlap is rejected
before mutation. Regular setup copies are owner-only while retaining a source owner-execute bit for
executable setup assets. The helper stages all selected entries and its managed-path manifest before
publication, and holds its serial guard through child process creation so every new child receives a
completed refresh. Existing children share this profile and may observe a selected setup entry being
replaced during a later refresh; publication does not promise a snapshot-atomic view to those running
children. An error at any stage retains the previous profile and manifest. The assumptions that the
packaged `codex-acp` honors `CODEX_HOME` for its rollout store, recognizes the refreshed setup, and
resumes against a non-default home are external-CLI compatibility gates (INV §12) confirmed by
credentialed acceptance before a release claims isolation, extending R11. Fake-ACP tests assert the
composed environment and profile-refresh contract only.

**R22 — ACP startup is bounded and reports safe stage-specific failures.** Each `initialize`,
`session/load`, and `session/new` call has a 30-second deadline bounded further by the caller's
context. An adapter exit or timeout before registration terminates the process group and returns the
backend plus failed stage. Claude maps recognized captured stderr to a small recovery vocabulary
(resource exhaustion, nested launch, authentication, or runtime incompatibility); unrecognized
stderr is never returned over the API and falls back to adapter/authentication verification
guidance. Launch and resume share this boundary.

**R23 `(planned)` — Optional-integration version tolerance is probed.** An adapter flag or metadata
extension known to vary by pinned CLI version will use an explicit capability probe or a documented
unsupported-option retry only when dropping it cannot change the user's requested runtime behavior.

**R24 — ACP available commands are replace-only live session state.** The ACP decode
boundary recognizes `available_commands_update` and validates its standard `availableCommands`
items (`name`, `description`, optional unstructured `input.hint`). Each valid notification atomically
replaces, rather than merges into, the owning chat runtime's command snapshot; an empty update clears
it. The snapshot retains at most 256 entries; names are capped at 256 runes and descriptions/input
hints at 2,000 runes at decode/storage so provider output cannot create an unbounded user-facing
payload (INV §8). Invalid entries are skipped without losing
the other valid entries or disrupting the ACP stream (INV §7). This state is not a normalized
transcript event: it receives no transcript sequence, durable append, index entry, or global SSE
publication. A fresh launch/resume/switch owns a fresh snapshot, and stop/crash removes it with the
live runtime state (INV §1/§4).

The runtime/registry exposes a read-only snapshot method used only by TS-03.R24. The server and UI
do not discover provider commands independently and do not infer a skill taxonomy. ACP advertises
names without the invocation slash, so UI selection inserts `/<name>` as ordinary prompt text; a
Codex skill name beginning `$` becomes `/$<skill>`. Pinned evidence: Claude adapter 0.59.0 sends the
snapshot after new/load/resume and on `commands_changed`; Codex adapter 1.1.2 sends built-ins plus
cwd/additional-directory-discovered `$` skills. The ACP v1 command contract and both pinned adapter
implementations are the compatibility authority, not provider-specific parsing in AgentDeck.

**R25 — ACP context usage follows the adapter's context channel.** The pinned Claude adapter's
`session/prompt` result `usage` object is token accounting
(`inputTokens`, `outputTokens`, `cachedReadTokens`, `cachedWriteTokens`, `totalTokens`) and does not
declare a context window, so it does not set `context_pct`. Its `session/update`
`{sessionUpdate:"usage_update",used,size}` notification is decoded at the ACP boundary and replaces
the owning live chat runtime's context percentage with `used / size`; invalid negative usage or a
non-positive size is ignored, and a value above one is capped at one. Each accepted notification also
republishes the value immediately through the existing status write+touch seam — the current status
row's `context_pct` alone, leaving its state, detail, trace, and `busy_since` untouched, because a
`usage_update` implies no status transition — so a long turn's meter does not hold a stale percentage
until the next tool/status event (FS-02.R26, INV §1). An unreadable status row is skipped rather than
replaced with an empty one; the next status write carries the value forward. The current value is
still carried into the terminal turn status and rollup, preserving the existing dashboard and resume
contracts.
The fake ACP adapter emits these same shapes so protocol tests do not assert an invented wire
contract (INV §11).

**R26 — Messaging addresses and wakes stopped wakeable agents through one helper.**
Recipient resolution and `list_agents` draw from a single shared addressable-set query: running
chat agents plus stopped chat agents passing FS-01.R33's wake gates (which exclude any agent with
a pipeline attempt association via the existing `PipelineAssociationForAgent` seam). "Single" is
literal — **one** statement over **one** SQLite snapshot, with each row carrying its own
availability. Reading the two halves as two statements let a Stop landing between them return the
agent as running and again as stopped-wakeable, duplicating it in `list_agents` and making
`role@project` resolution report a false ambiguity (INV §1/§5). The two queries also share one SQL
spelling of the wake gates so single-agent candidacy and the directory cannot drift (INV §2). Every
`list_agents` entry gains the additive `availability` field (`"running"` | `"stopped_wakeable"`,
FS-06.R22); existing fields, including `state`, are unchanged. Mail insertion stays durable-first
exactly as today (INV §15). The nudger's existing per-agent cooldown/in-flight map gains stopped
wakeable recipients with unread mail as wake candidates: the insert-time signal or sweep calls the
shared wake helper (TS-01.R16) without waiting out the resume, and once the agent is resumed and
idle, the ordinary running-recipient nudge delivers `check_messages`. A wake reuses that map's
per-agent cooldown; its re-entry guard is TS-01.R16's exclusive claim rather than the map's
in-flight marker, which stays with the `check_messages` delivery that follows the wake.

The wake bound is durable, not process-local, and is a **claim rather than a failure marker**. Wake
candidacy requires at least one unread row still marked `pending`, and an attempt takes those exact
rows out of `pending` into the `delivered_via` value `wake_attempted` — in one transaction, before
any process is spawned — then records `wake_failed` on those same ids if it fails. Both values live
in the existing column, so there is no migration. Marking only on failure was insufficient: a
successful wake left the rows `pending`, so an adapter that completed its handshake and then died
before the first `check_messages` nudge was respawned by every sweep (INV §15). Scoping the outcome
to the claimed ids is what lets mail inserted mid-wake stay `pending` and re-arm; a losing lifecycle
claim releases the rows back to `pending` because nothing was attempted. `MarkUnreadDeliveredVia`
accepts `wake_attempted` alongside `pending` so the delivering nudge restamps them honestly, and
reading the mail restamps a still-claimed row `poll`. `messages.created_at` is whole-second RFC3339,
so timestamp comparison cannot order same-second rows and is not used. New mail always inserts as
`pending`, re-arming the wake; the bound therefore holds across dashboard restarts (FS-06.R23).

## 3. Interfaces & data shapes

- ACP: JSON-RPC messages over newline-delimited child stdin/stdout; adapter determines exact
  `session/new`, `session/load`, prompt, cancel, and permission option shapes. The Codex prompt
  overlay is process configuration (`CODEX_CONFIG`), not an ACP metadata extension.
- Resume session ownership: a successful `session/load` keeps the *requested* session id
  authoritative. An adapter may echo a fresh `sessionId` (which then wins) or, like the pinned
  `codex-acp`, return an empty result — an empty result is success, not failure. Resume re-mints via
  `session/new` only when there is no prior session id or `session/load` errors; that fallback
  abandons native history, so it is logged rather than silent. Treating an empty load success as a
  failure once dropped the resumed conversation entirely (INV §11, the mock had returned a
  non-contract new-id shape).
- Hook: `POST /api/hook` with a bearer/scoped token; accepted status vocabulary is the FS-02 state
  set plus tracking events.
- MCP: streamable HTTP at `/mcp`; tools accept only their documented arguments and return
  product-safe text/structured content. Pipeline tools use the same transport/token and add
  no agent-callable start operation.
- Terminal WebSocket: binary/text terminal bytes plus JSON resize control frames.
- Effort delivery: no new ACP method. The model-suffix mechanism reuses the existing `model` key in
  `session/new`/`session/load`; the post-session mechanism uses the adapter's documented session
  configuration-option request with the option id it publishes for effort; the argv mechanism adds
  one flag to the existing terminal command. `internal/backend` stays free of process/runtime
  imports: the adapter contributes a delivery mode plus its identifier/flag, never an RPC call.
- Native-auth readiness: no new HTTP shape; `PUT /api/backends` continues to return the existing
  per-backend `{status:"ok"|"failed"|"skipped", detail?}` result after provider-specific probing.
- Available commands (R24): ACP `session/update` payload
  `{sessionUpdate:"available_commands_update",availableCommands:[...]}` becomes a bounded live
  runtime snapshot; TS-03.R24 is its only HTTP projection and invocation remains an ordinary text
  `session/prompt` block.
- Context usage (R25): Claude's `usage_update` payload
  `{sessionUpdate:"usage_update",used:<tokens>,size:<context-window>}` becomes the live
  `context_pct`; prompt-result token accounting is not interpreted as a percentage.

## 4. Invariants

- **INV §2:** the codex `CODEX_HOME` is composed once and shared by launch, resume, and switch, never
  re-derived per path, so a resumed session never opens a different store (R20).
- **INV §4:** registration and teardown are symmetric, generation-scoped, and old-before-new.
- **INV §6:** a new runtime/backend joins persistence, LaunchSpec, status, messaging, teardown, and
  capability contracts before it is advertised.
- **INV §9:** process/cancel/readiness operations have real deadlines and terminate their resources.
- **INV §12:** the packaged `codex-acp` honoring of a non-default `CODEX_HOME`, refreshed setup
  assets, and resume against that home are version-variant external-CLI behaviors gated by
  credentialed acceptance (R21).
- **R12 — Boundary redaction.** Raw provider errors, stderr, tool inputs, and hook/MCP payloads are
  sanitized before logging or returning over HTTP; diagnostic value must not expose secrets.
- **R16 — Auth probes cannot become command execution.** Provider id and argv are selected
  exclusively from the shared fixed metadata; request fields, backend names, models, environment
  values, and provider output cannot add an executable or argument. Probe processes receive no
  stdin, are cancelled on deadline/shutdown, and never become agent/session/runtime records.

## 5. Deviations & open decisions

- HTTP-only MCP registration is shipped; a stdio proxy exists only as a possible compatibility
  response if a pinned CLI rejects HTTP. It must proxy to the same in-process authority.
- Terminal agents are intentionally non-messageable until an interactive-CLI MCP path is verified.
- OpenCode/OpenHands executable overrides are honored by credential checks but not consistently by
  launch; missing/old CLI diagnostics are also incomplete. These are tracked product gaps.
- Codex session isolation (R20/R21) does not migrate `codex-acp` sessions already written into the
  user's personal home before the change ships; they stay there, may no longer native-resume through
  AgentDeck, and the user may archive them with the native `codex archive` command. Whether the
  packaged CLI honors the isolated profile is confirmed only by the credentialed A7 gate.

## 6. Traceability

- ACP/runtime: `internal/runtime/chat.go`, `transport.go`, `event.go`, `permission.go`.
- Available commands (R24): decode in `internal/runtime/acpmap.go`, replace-only snapshot on
  the live `agentState`, registry read projection, and fake-ACP new/load/replacement regressions.
- Context usage (R25): `decodeContextUsage`, `ChatRuntime.onNotification`,
  `ChatRuntime.republishContextPct`, `TestContextUsageFromRealClaudeAdapterShapes`, and
  `TestUsageUpdateRepublishesContextPctMidTurn`.
- Adapters: `internal/backend/adapter.go`; credential checks in `internal/backend/credcheck`;
  official Claude session metadata and Codex `CODEX_CONFIG` prompt delivery are pinned by runtime
  parameter/environment tests.
- Codex isolated profile (R20/R21): final `CODEX_HOME` composition in
  `internal/server/{launch,resume,switch}.go` via `composeEnv`, one-way profile refresh under
  `internal/config`, applied in `internal/runtime/chat.go` spawn; AgentDeck's own home read stays in
  `internal/server/config_sources.go` and `internal/config/codexmodels.go`.
- Hooks: `internal/hooks`, `internal/server/hook.go`, registration in `launch.go`.
- MCP: `internal/messaging/messaging.go`, `tools.go`, `internal/server/messaging_registration.go`.
- Terminal: `internal/runtime/terminal`, `internal/server/terminal.go`.
- Regression anchors: `TestLaunchPromptPermissionFlow`, `TestTakePendingSingleWinner`,
  `TestCrashTearsDownAgentRegistration`, `TestLaunchArgvHonorsComposedSpec`,
  `TestTerminalDriverUnavailableRejected`.
