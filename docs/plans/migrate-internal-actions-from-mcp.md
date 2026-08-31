# Migrate AgentDeck internal actions from MCP

This is sequencing guidance for the ready change. The linked specifications remain authoritative.

## 1. Freeze the behavioral baseline

- Capture the fifteen-action manifest, resolved input schemas, serialized MCP catalog size, result
  fixtures, retry classifications, state effects, and lifecycle/authorization matrix.
- Add representative success and refusal goldens before changing delivery.

## 2. Extract the transport-neutral core

- Turn the current MCP registrations into one typed action registry and dispatcher.
- Keep existing domain services, action names, handlers, result helpers, and retry classifier.
- Keep the MCP adapter temporarily in tests so identical calls can be compared; do not expose a
  product selector or commit a released compatibility promise.

## 3. Add the direct transport and client

- Generalize the launch-token registry to resolve `{agent_id,generation}` for hooks and actions.
- Add the bounded authenticated private GET/POST action routes and packaged CLI.
- Add the runtime-only chat overlay and exact activation/assignment pointers; exclude terminals.
- Prove teardown, failed composition, restart, switch, stale generation, redaction, oversize input,
  malformed JSON, and unknown-field behavior before switching providers.

## 4. Prove parity and provider portability

- Run every action through the golden domain/result matrix and compare it with the frozen baseline.
- Exercise fresh launch, resume, switch, wake, task, and pipeline flows with fake ACP.
- Run release-blocking credentialed checks for Claude, Codex, OpenCode, and OpenHands. Record context
  bytes removed, representative command latency, structured success, and structured refusal.

## 5. Cut over atomically

- Change operating knowledge and activation text to direct actions.
- Remove the internal `/mcp` route, registration injection, generated config, reserved-name check,
  runtime `MCPServers` field, and MCP SDK dependency; retain `jsonschema-go` directly.
- Prove provider/user-configured MCP discovery and launch behavior is unchanged.

## 6. Close the migration

- Search code, tests, docs, fixtures, release assets, and generated paths for obsolete internal-MCP
  references and classify every remaining MCP reference as external/provider-owned or historical.
- Run the full shared checks and distributable build, then repeat the four live-provider gate.
- Ship only after the direct path is the sole internal action surface; rollback is the prior release,
  not a hidden dual-transport mode.
