# AgentDeck architecture flow

Descriptive orientation only. Binding boundaries, protocols, data rules, and security constraints
live in [TS-01](docs/specs/tech/TS-01-architecture.md),
[TS-02](docs/specs/tech/TS-02-data-persistence.md),
[TS-03](docs/specs/tech/TS-03-http-api.md),
[TS-04](docs/specs/tech/TS-04-integration-protocols.md), and
[TS-05](docs/specs/tech/TS-05-security.md).

## Runtime topology

```mermaid
flowchart LR
  UI["React dashboard"] <-->|"REST + SSE + terminal WebSocket"| Server["Go server on 127.0.0.1"]
  Server <-->|"ACP JSON-RPC over stdio"| Chat["Chat agent CLI"]
  Server <-->|"PTY / tmux / iTerm2 driver"| Terminal["Terminal agent CLI"]
  Chat <-->|"scoped streamable HTTP"| MCP["In-process messaging MCP at /mcp"]
  MCP --- Server
  Hooks["Lifecycle hooks"] -->|"token-scoped POST /api/hook"| Server
```

Launch composition may create registration artifacts for terminal agents, but the interactive
runtime does not consume them and recipient resolution/nudging excludes terminal agents. Host/Origin validation
wraps every browser-accessible route; hook and MCP producers use per-launch tokens. The remaining
same-machine API is intentionally unauthenticated (TS-05.R3).

## Launch, state, and UI flow

```mermaid
sequenceDiagram
  participant User
  participant UI
  participant Server
  participant Store as Config + state.db
  participant CLI as Agent CLI

  User->>UI: Launch role@project
  UI->>Server: POST /api/sessions
  Server->>Store: Read config / resolve optional federation
  Server->>Server: Compose and freeze LaunchSpec
  Server->>CLI: Start ACP or terminal runtime
  Server->>Store: Commit identity, running row, snapshot
  Server-->>UI: SSE state_update
  CLI-->>Server: ACP events or token-scoped hooks
  Server->>Store: Persist transcript/status/tracking
  Server-->>UI: SSE new_message/state_update
```

Launch, resume, and switch share the same composition and registration rules. Authoritative status
and identity mutations commit before their SSE publication; transcript events follow append-then-
publish on the successful path. SSE reconnect starts with a snapshot/hydration boundary.

## Data authority

```text
AgentDeck JSON config     human/server edited, atomic owner-only files
Native Claude/Codex      provider input when linked/mirrored; composed by TS-07 precedence
state.db                 server-sole-writer identity, runtime state, messages, index
transcript.ndjson        AgentDeck normalized append-only chat history
mirror/effective views   redacted, regenerable federation projections
```

See [the spec index](docs/specs/README.md) for feature ownership and acceptance criteria. The
pre-SDD long-form diagram is preserved in `docs/archive/snapshots/architecture-flow-pre-sdd.md`.
