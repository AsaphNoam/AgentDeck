# Live-provider acceptance run — 2026-07-26

Human-authorized run of the credentialed acceptance gates recorded in `HANDOFF.md`
("Blocked on human"). Nothing in product code or the specifications was changed by this run.

**Pinned versions:** Claude Code CLI 2.1.202 · `@agentclientprotocol/claude-agent-acp` 0.59.0
(installed to a scratch prefix, addressed through `ACP_CMD`) · Codex CLI 0.142.5 ·
`@agentclientprotocol/codex-acp` 1.1.2 · Node 22.23.1.

**Isolation:** a disposable `AGENTDECK_HOME` and a disposable project working directory. Both
provider homes (`~/.claude`, `~/.codex`) were read with real credentials and were not modified
(`settings.json` mtime unchanged at 2026-07-24; `config.toml` unchanged by this run).

## Gate results

| Gate | Result |
|---|---|
| Claude chat: handshake, streamed turn, permission approve/deny, stop | **Pass** |
| Claude chat: cancel | **Fail** — see Finding 1 |
| Codex chat: launch, streamed turn, `CODEX_CONFIG` prompt delivery, stop | **Pass** |
| Codex chat: resume | **Fail** — see Finding 2 |
| Claude resume (native `session/load`) | **Pass** — session id and model-side history retained |
| Messaging MCP over HTTP, both backends | **Pass** |
| Claude terminal: flags, hooks, xterm journey J6 | **Pass**, with one harness caveat below |
| Federation discovery / freeze / redaction / refresh / collision | **Pass** |
| Federation model precedence observed at the provider | **Fail** — see Finding 3 |
| OpenCode / OpenHands | **Not run** — neither CLI is installed |

## What was exercised

**Claude chat (gated suite).** `go test -tags acceptance ./internal/runtime` against the pinned
adapter: `TestRealCLIAcceptance`, `TestRealCLIPermissionDeny`, `TestRealCLIPermissionApprove` and
`TestRealCLIStop` pass against the live provider. Permission options arrive in the expected
allow_always/allow_once/reject_once shape, denial prevents the tool's side effect, approval performs
it, and Stop kills the process group and removes the running row. `TestRealCLICancel` fails.

**Codex chat.** Live launch, one streamed turn, and Stop all behave. The composed system prompt
reaches the model through the `CODEX_CONFIG` `developer_instructions` overlay: an agent seeded with a
pass phrase returned it verbatim, confirming FS-09.R32/A11 against the real adapter. Resume fails.

**Messaging MCP.** With both a live Claude and a live Codex agent registered against the in-process
HTTP MCP server: the Claude agent listed agents through `mcp__agentdeck-messaging__list_agents`
(permission round-trip included), the Codex agent sent a message through `send_message`, AgentDeck
delivered it as a nudge, and the Claude agent woke and read it through `check_messages`. Cross-backend
messaging works end to end against real providers.

**Claude terminal.** The interactive CLI accepted every composed flag —
`--settings <per-agent hooks file> --model sonnet --add-dir <project resources> --append-system-prompt
<role prompt>` — and stayed alive. Hooks fired for real: three `POST /api/hook` calls, the running row
picked up the CLI's own session id, and status settled to `idle` / "turn complete" with
`last_trace: Stop`. In the browser the xterm view rendered the live CLI, a typed prompt produced a real
answer, resizing reflowed without losing output, and navigating away and back replayed the scrollback
intact.

*Harness caveat:* synthetic clicks from the automation browser never focused the xterm helper
textarea, and synthetic key events produced no input even after focusing it programmatically. Input
was therefore driven over the same WebSocket contract the UI uses (binary frames for keystrokes,
`{cols,rows}` text frames for resize), which exercised the server side of the bridge but not physical
typing. Whether a real keyboard on a real browser is affected is **unverified** — it needs one human
minute at a keyboard, and should not be assumed broken or working.

**Federation.** Real discovery of both provider roots reports `health: ok`. A mirrored Claude binding
resolved the real `settings.json` (`model: haiku`) with correct provenance; the launch froze a
redacted federation object (`requested_model: sonnet`, `resolved.model: haiku`,
`native_inherited: true`, source digest and file fingerprints, no secrets). The mirrored cache is
`0600` and contains only paths, key names, and hashes. Against a disposable source root: a configured
env secret surfaced as the key name `MY_TOKEN` only, never its value; instructions, skills and MCP
declarations were inventoried as `reference_only` / `native_passthrough`; an external edit adding the
reserved `agentdeck-messaging` id was picked up by the watcher without a manual refresh and made the
next launch fail `409 source_conflict`; removing it restored launch; and an explicit refresh reported
the new model and effort.

## Findings

**Must fix — Cancel kills a live Claude agent instead of interrupting its turn.**
The pinned `claude-agent-acp` 0.59.0 ignores ACP `session/cancel`: with escalation disabled, a
cancelled turn ran *longer* than an uncancelled control (57.4s vs 41.1s) and streamed 131 assistant
deltas *after* the cancel. The shipped 3s grace therefore always escalates, and the fallback SIGINT
ends the adapter process every time. The user sees a fatal error
`Cancelled — agent process exited after ignoring cancellation.`, status `error` /
"cancelled — process exited", the running row removed, and must relaunch. FS-01.R7 and FS-03.R9/A5
describe this as the degraded branch for a peer that ignores cancellation; against the pinned adapter
it is the only branch. Deciding what Cancel should do here (longer grace, a softer signal aimed at the
CLI child, or accepting termination and saying so up front) is a product decision.
*Reproduce:* live Claude chat agent, long prompt, Cancel mid-turn.

**Must fix — Codex resume silently starts a new session and loses the agent's memory.**
`codex-acp` 1.1.2 advertises `loadSession: true` and its `session/load` **succeeds** and replays prior
history — but its result object carries no `sessionId` field (keys: `models`, `modes`,
`configOptions`). `ChatRuntime.Resume` treats a missing id as failure and falls back to `session/new`,
discarding the loaded session. Observed through the real API: after Resume the running row shows a new
session id and the agent answers `NONE` when asked what it sent earlier in the same conversation,
while AgentDeck's own transcript still displays the history. FS-01.R10 promises Resume restores an
inactive agent's full history. The same probe against `claude-agent-acp` returns the session id, so
Claude resume is unaffected. *Fix direction:* when `session/load` returns no error, treat the
requested session id as the loaded id rather than falling through to `session/new`.
*Reproduce:* Codex chat agent, one turn, Stop, Resume, ask about the earlier turn.

**Must fix (needs a decision) — neither pinned adapter applies the model AgentDeck requests.**
`claude-agent-acp` reports the same session model whatever AgentDeck asks for: with the real home its
`configOptions` model `currentValue` stayed `haiku` (the value in `settings.json`) when `sonnet` was
requested, and with a disposable home containing no settings it stayed `default` when `haiku` or
`opus` was requested — through both `_meta.claudeCode.options.model` and a top-level `model`, and the
adapter exposes no `session/set_model`-style method. `codex-acp` behaves the same way: its
`models.currentModelId` stayed `gpt-5.6-terra[high]` (the value in `config.toml`) for every requested
id, including the `slug[effort]` form. AgentDeck's per-agent model picker and FS-08.R17's
explicit-model-wins rule therefore have no observable effect on either real backend; the effective
model is whatever the user's own CLI config says. AgentDeck's own bookkeeping is correct — the frozen
launch object records requested vs resolved and `native_inherited` accurately.
*Residual uncertainty:* the evidence is each adapter's own reported session config, not a
provider-side receipt; model self-reports were unreliable (a session requesting `opus` under a
`haiku` config answered "Sonnet"). Worth one more confirmation against a provider-side signal before
any upstream report.

## Not run

- **OpenCode / OpenHands (FS-09.A6).** Neither CLI is installed, and both would need credentials
  entered to authenticate. Still gated.
- **Terminal `--resume` and tmux/iTerm2 drivers.** `tmux` is not installed on this machine; the xterm
  driver was the only one exercised.
