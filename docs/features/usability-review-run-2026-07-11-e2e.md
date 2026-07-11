# Usability review run — 2026-07-11 (full product journey sweep)

Scope was the full journey matrix in `USABILITY-REVIEW.md`, not just the recent
federation work. The tagged, embedded production binary and both Go test variants
were green; the UI suite passed 94 tests. Every live instance used an isolated
`AGENTDECK_HOME` and a deterministic local `claude-code-acp` shim.

## Observed checkpoints

| Journey | Verdict | Evidence / outcome |
|---|---|---|
| J1 Install & first paint | PARTIAL PASS | Fresh isolated dashboard served the styled shell and empty-agent state. Agent-side API fixture evidence: `.review/fresh-settings/evidence/`. |
| J2 Onboarding | BLOCKED | The seeded fixture did not expose the wizard; the separate fresh browser could not be allocated. Real credential variants remain environment-gated. |
| J3 Launch + chat | PASS | Created the first agent through the modal (it closed), sent a prompt, and observed the streamed reply plus transcript and context update. |
| J4 Permission | BLOCKED | Requires a restarted `permission` shim server; local-server control was interrupted. |
| J5 Grid/layout | PARTIAL PASS | Changed Columns from 3 to 2, waited for the write, reloaded, and observed 2 persisted. Group/reorder coverage remains open. |
| J6 Terminal | BLOCKED | Requires an interactive-CLI fixture and a separate complete browser run. |
| J7 Stop/resume/switch | BLOCKED | The UI stop confirmation stalled the browser-control channel before outcome could be observed. |
| J8 Archive/search | BLOCKED | Depends on completed lifecycle state; tagged and fallback search paths were not reached. |
| J9 Settings | PARTIAL PASS | Edited `my-app` cwd through Settings and reloaded the page; saved value persisted. Invalid-input coverage remains open. |
| J10 Messaging | BLOCKED | Not reached. |
| J11 Failure/recovery | BLOCKED | Not reached. |
| J12 Restart durability | BLOCKED | Depends on J7–J10. |
| Phase 7 source linking | PASS | Claude source discovery showed model/provenance/env-key name without the fixture secret; choosing Mirrored persisted and displayed `mirrored`; bound controls included override/reset/unlink and honest disabled detach. |

## Environment acceptance gate

No new product finding is asserted from blocked coverage. The in-app browser was
available to the primary run but not to the delegated isolated runs; it then
stalled during the Stop confirmation. The local approval service subsequently
rejected even a read-only `curl` state check because the account had reached its
usage limit. Resume J2 and J4–J12 with browser and local-loopback access restored;
do not infer their acceptance from this run.
