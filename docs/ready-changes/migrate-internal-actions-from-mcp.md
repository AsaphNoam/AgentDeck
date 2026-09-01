# Migrate AgentDeck internal actions from MCP

**State:** Paused
**Why:** Direct operator request on 2026-08-31 to remove MCP where AgentDeck controls both sides and
it adds context/protocol cost without an interoperability benefit.
**Relevant requirements:** FS-06 §6; FS-15 §6; FS-16 §6; FS-17.R13–R20/A7–A11; TS-01.R25; TS-03.R32;
TS-04.R32–R40; TS-05.R18–R19; TS-06.R23; TS-11.R11–R12; INV §§1–6, 8–15

## Outcome

Every supported chat agent uses one packaged, generation-authenticated `agentdeck action` command
for AgentDeck coordination. AgentDeck no longer advertises its internal actions as MCP, while
provider- and user-configured MCP support continues unchanged.

## Included work

Do not begin implementation until the transport gate in FS-17.R20 passes. Until then, keep the
released internal MCP path unchanged. Once unblocked, follow the phased plan in
[`../plans/migrate-internal-actions-from-mcp.md`](../plans/migrate-internal-actions-from-mcp.md):
freeze parity, extract the typed registry, add the proven private action adapter and CLI, validate
all four providers, then remove the internal MCP path before release. Preserve all fifteen action
contracts, domain data, authorization, lifecycle ordering, and terminal capability. Do not enable
broad shell networking, add filesystem IPC, retain a provider-specific internal-MCP fallback, add a
UI change, or migrate data.

## How we will know it works

FS-17.A7–A11 pass, including every-action parity, adversarial lifecycle coverage, absence of the
internal MCP surface, unchanged external MCP federation, deterministic context measurement, and
credentialed Claude/Codex/OpenCode/OpenHands checks. Shared specification, build, test, and
distributable checks pass.

## Waiting on

Packaged Codex and its ACP adapter must expose a narrowly scoped direct transport that a managed
Codex process can reach under AgentDeck's default sandbox. AgentDeck must prove that transport with
the packaged release runtime and all four supported chat providers before this design returns to
review. Broad network access, filesystem mailbox transport, and a Codex-only MCP exception do not
satisfy the gate.
