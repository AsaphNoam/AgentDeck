# Composer file and command autocomplete

**State:** Waiting to start
**Why:** The 2026-08-10 play session requested Codex/Claude/Cursor-style file and skill autocomplete;
the human confirmed plain-text file completion and one picker containing every ACP-advertised command.
**Relevant requirements:** FS-03.R30, FS-03.R31, FS-03.R32, FS-03.R33, FS-03.R34, TS-03.R24,
TS-04.R24, INV §1, INV §2, INV §7, INV §8

## Outcome

In a live chat composer, `@` finds project files and inserts their relative paths, while `#` finds
the current ACP session's advertised commands and skills and inserts an executable slash command.

## Included work

Add bounded session-scoped file search, a replace-only live ACP command snapshot and read route, and
one reusable keyboard-operated composer picker. File mentions and commands remain ordinary durable
prompt text; there are no structured file attachments, embedded contents, command persistence, or
provider-specific command discovery.

Verified design evidence: ACP v1 specifies `available_commands_update` as a replaceable session
snapshot and invokes advertised names as `/<name>`. The locally pinned Claude 0.59.0 and Codex 1.1.2
packages both publish that shape; Codex includes `$` skills and Claude republishes command changes.
The current 2026-08-15 releases—Claude 0.63.0 and Codex 1.1.7—retain the same command surface. Sources:
[ACP slash commands](https://agentclientprotocol.com/protocol/v1/slash-commands),
[`@agentclientprotocol/claude-agent-acp`](https://www.npmjs.com/package/%40agentclientprotocol/claude-agent-acp),
[`@agentclientprotocol/codex-acp`](https://www.npmjs.com/package/%40agentclientprotocol/codex-acp),
`scripts/release/package.json`, and the pinned adapters under `scripts/release/node_modules/`.

## How we will know it works

FS-03.A15, FS-03.A16, and FS-03.A17: focused server/runtime/fake-ACP/component tests cover file
containment and ignores, picker interaction, command snapshot replacement, exact slash insertion,
unavailable sources, and unchanged prompt submission.

## Waiting on

Nothing.
