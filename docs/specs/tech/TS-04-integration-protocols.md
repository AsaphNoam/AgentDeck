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

**R15 (planned) — Native-auth readiness is a fixed-command probe, not a dashboard login flow.**
One provider-metadata helper owns the CLI `agentdeck auth` login argv and each non-interactive
readiness probe. The onboarding UI supplies only static provider guidance and reuses the ordinary
backend save/check (TS-03.R15); the server never starts or proxies a login command. Each probe uses
an explicit allowlisted executable and argv through `exec.CommandContext` (never a shell), a bounded
deadline, inherited provider environment, and sanitized bounded diagnostics. Claude retains its
adapter-delegated `auth status` probe and no-color compatibility retry. Codex first asks the pinned
private Codex CLI for `login status`; a successful native result is sufficient, otherwise a
configured `OPENAI_API_KEY` is checked through the existing models endpoint. Raw status output,
account identity, and credential values never cross the process/log/API boundary.

## 3. Interfaces & data shapes

- ACP: JSON-RPC messages over newline-delimited child stdin/stdout; adapter determines exact
  `session/new`, `session/load`, prompt, cancel, and permission option shapes. The Codex prompt
  overlay is process configuration (`CODEX_CONFIG`), not an ACP metadata extension.
- Hook: `POST /api/hook` with a bearer/scoped token; accepted status vocabulary is the FS-02 state
  set plus tracking events.
- MCP: streamable HTTP at `/mcp`; tools accept only their documented arguments and return
  product-safe text/structured content.
- Terminal WebSocket: binary/text terminal bytes plus JSON resize control frames.
- Native-auth readiness: no new HTTP shape; `PUT /api/backends` continues to return the existing
  per-backend `{status:"ok"|"failed"|"skipped", detail?}` result after provider-specific probing.

## 4. Invariants

- **INV §4:** registration and teardown are symmetric, generation-scoped, and old-before-new.
- **INV §6:** a new runtime/backend joins persistence, LaunchSpec, status, messaging, teardown, and
  capability contracts before it is advertised.
- **INV §9:** process/cancel/readiness operations have real deadlines and terminate their resources.
- **R12 — Boundary redaction.** Raw provider errors, stderr, tool inputs, and hook/MCP payloads are
  sanitized before logging or returning over HTTP; diagnostic value must not expose secrets.
- **R16 (planned) — Auth probes cannot become command execution.** Provider id and argv are selected
  exclusively from the shared fixed metadata; request fields, backend names, models, environment
  values, and provider output cannot add an executable or argument. Probe processes receive no
  stdin, are cancelled on deadline/shutdown, and never become agent/session/runtime records.

## 5. Deviations & open decisions

- HTTP-only MCP registration is shipped; a stdio proxy exists only as a possible compatibility
  response if a pinned CLI rejects HTTP. It must proxy to the same in-process authority.
- Terminal agents are intentionally non-messageable until an interactive-CLI MCP path is verified.
- OpenCode/OpenHands executable overrides are honored by credential checks but not consistently by
  launch; missing/old CLI diagnostics are also incomplete. These are tracked product gaps.

## 6. Traceability

- ACP/runtime: `internal/runtime/chat.go`, `transport.go`, `event.go`, `permission.go`.
- Adapters: `internal/backend/adapter.go`; credential checks in `internal/backend/credcheck`;
  official Claude session metadata and Codex `CODEX_CONFIG` prompt delivery are pinned by runtime
  parameter/environment tests.
- Hooks: `internal/hooks`, `internal/server/hook.go`, registration in `launch.go`.
- MCP: `internal/messaging/messaging.go`, `tools.go`, `internal/server/messaging_registration.go`.
- Terminal: `internal/runtime/terminal`, `internal/server/terminal.go`.
- Regression anchors: `TestLaunchPromptPermissionFlow`, `TestTakePendingSingleWinner`,
  `TestCrashTearsDownAgentRegistration`, `TestLaunchArgvHonorsComposedSpec`,
  `TestTerminalDriverUnavailableRejected`.
