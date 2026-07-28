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

**R10 `(planned)` — Readiness and version tolerance are bounded.** ACP initialization will have a
documented timeout and optional-integration flag fallback/probe so an interactive or older CLI cannot
leave launch pending forever. The current generic transport-close diagnostics are an explicit gap.

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
call the canonical pipeline validator, and return data/digests only: they cannot save, start, or
approve. Tool registration, token generation, teardown, transport, and redaction remain the existing
R6–R7 authority rather than a second MCP server.

**R18 `(planned)` — Effort delivery is adapter-declared and fail-closed.** The three shipped
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

**R19 `(planned)` — A provider-rejected effort fails the launch; it is never retried bare.**
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
refresh and belongs solely to the private profile. A missing source setup asset is a non-fatal skip;
an unsafe or uncopyable selected asset fails the process start before spawn. The assumptions that the
packaged `codex-acp` honors `CODEX_HOME` for its rollout store, recognizes the refreshed setup, and
resumes against a non-default home are external-CLI compatibility gates (INV §12) confirmed by
credentialed acceptance before a release claims isolation, extending R11. Fake-ACP tests assert the
composed environment and profile-refresh contract only.

## 3. Interfaces & data shapes

- ACP: JSON-RPC messages over newline-delimited child stdin/stdout; adapter determines exact
  `session/new`, `session/load`, prompt, cancel, and permission option shapes. The Codex prompt
  overlay is process configuration (`CODEX_CONFIG`), not an ACP metadata extension.
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
