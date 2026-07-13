# Usability review run — 2026-07-11 (full product journey sweep)

Scope was the full journey matrix in `USABILITY-REVIEW.md`, not just the recent
federation work. The tagged, embedded production binary and both Go test variants
were green; the UI suite passed 94 tests. Every live instance used an isolated
`AGENTDECK_HOME` and a deterministic local `claude-code-acp` shim.

## Observed checkpoints

| Journey | Verdict | Evidence / outcome |
|---|---|---|
| J1 Install & first paint | PARTIAL PASS | Fresh isolated dashboard served the styled shell and empty-agent state. Agent-side API fixture evidence: `.review/fresh-settings/evidence/`. |
| J2 Onboarding | PARTIAL PASS | Fresh state opened the styled four-step wizard. The installed-but-logged-out branch displayed its failed credential result; real logged-in provider acceptance remains environment-gated. |
| J3 Launch + chat | PASS | Created the first agent through the modal (it closed), sent a prompt, and observed the streamed reply plus transcript and context update. |
| J4 Permission | PASS | Fake ACP displayed one inline request with Approve/Deny; both actions resolved the request and restored the composer. |
| J5 Grid/layout | PARTIAL PASS | Changed Columns from 3 to 2, waited for the write, reloaded, and observed 2 persisted. Group/reorder coverage remains open. |
| J6 Terminal | BLOCKED | Requires an interactive-CLI fixture and a separate complete browser run. |
| J7 Stop/resume/switch | PARTIAL PASS | Stop persisted the session in Archive and Resume restored its transcript/config to chat. The browser did not surface the native switch-runtime prompt. |
| J8 Archive/search | BLOCKING | Tagged search filtered transcript/metadata correctly. The untagged build emits raw `no such module: fts5` and retains stale results. |
| J9 Settings | PARTIAL PASS | Edited `my-app` cwd through Settings and reloaded the page; saved value persisted. Invalid-input coverage remains open. |
| J10 Messaging | PASS | A local MCP `send_message` created the recipient's Mail badge; `check_messages` returned the message/`remaining:0`, and the badge cleared after reload. |
| J11 Failure/recovery | PARTIAL PASS | Dashboard restart was exercised as J12; crash and garbage-input variants remain open. |
| J12 Restart durability | PASS | Restart preserved the card, stopped status, transcript preview, and persisted density. |
| Phase 7 source linking | PASS | Claude source discovery showed model/provenance/env-key name without the fixture secret; choosing Mirrored persisted and displayed `mirrored`; bound controls included override/reset/unlink and honest disabled detach. |

## Finding

SEVERITY: MAJOR (maps to BLOCKING)
WHERE: J8 untagged fallback build, Archive search (`Permission`)
REPRO: build `go build -o agentdeck-notags ./cmd/agentdeck`; open an archive with sessions; enter a query
EXPECTED: the documented non-FTS5 fallback filters or clearly handles search without stale results
OBSERVED: the page displays `archive: count search: no such module: fts5` while retaining all prior rows
EVIDENCE: browser DOM snapshot from the isolated `:4616/archive` run

## Remaining acceptance scope

Terminal PTY, native switch-runtime prompt automation, true agent crash, invalid-form
coverage, group/reorder, and real-provider login remain incomplete or credential-gated.
