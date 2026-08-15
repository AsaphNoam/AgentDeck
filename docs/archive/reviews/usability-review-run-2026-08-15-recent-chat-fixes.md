# Usability review — recent chat features and fixes — 2026-08-15

## Scope and setup

- Browser rung: in-app Chromium through the Browser skill.
- Build: `make build` (`sqlite_fts5`).
- Fixture: fresh isolated `AGENTDECK_HOME`, seeded by the built binary; `my-app` was pointed at the
  AgentDeck worktree through Settings.
- Runtime: deterministic `fakeacp` exposed as `claude-agent-acp`; separate server restarts exercised
  streamed turns, an ignored cancellation, and an initialize-time resource failure.
- Scope was limited to the newest unreviewed user-facing work: composer file/command autocomplete and
  its send/stopped-session fixes, the Working indicator, transcript selection styling, Claude startup
  guidance, and persistent dashboard logging. The previously reviewed backend-creation journey and
  the legacy journey matrix were not repeated.

## Results

| Step | Result | Observation and evidence |
|---|---|---|
| Build and isolated start | PASS | The tagged binary built and served the seeded dashboard on an isolated localhost port with zero captured browser console errors. |
| `@` file picker | PASS | `@` at a boundary opened a 50-entry bounded file list; `user@example.com` opened no picker; `@READ` filtered to repository matches; Enter inserted `@README.md ` including the trailing space. |
| `#` command picker | PASS | The fake session advertised `review` and `$plan`; `#pl` showed `/$plan`, including its description, and Tab inserted `/$plan `. |
| Verbatim submission | PASS | The live transcript rendered `/$plan ` and `@README.md ` with their trailing spaces intact, followed by the streamed fake response. |
| Stopped-session autocomplete | PASS | After stopping the agent, file search returned the typed `409` path, the picker rendered `No matching files`, and the composer remained editable. |
| Working indicator | PASS | A deliberately non-finishing turn rendered the inline `Working…` indicator and changed Send to Cancel; after cancellation escalated and the process exited, the indicator cleared and the actionable cancellation error remained. Evidence: [working-indicator.jpg](usability-review-2026-08-15-recent-chat-evidence/working-indicator.jpg). |
| Transcript selection | BLOCKED(browser automation could not establish a text selection) | The live browser computed `user-select: text`; the user bubble's computed `::selection` background was `rgb(36, 87, 245)`, distinct from its `rgb(223, 255, 79)` bubble background. The actual pointer-selection highlight was not captured and is not claimed as passed. |
| Claude startup guidance | PASS | An initialize exit containing `EMFILE` plus a secret path produced `Claude initialize failed: the adapter could not open required files; close unused agent processes and retry`; neither the raw path, secret, nor generic transport-close text appeared. The failed launch created no additional card. Evidence: [startup-guidance.jpg](usability-review-2026-08-15-recent-chat-evidence/startup-guidance.jpg). |
| Persistent dashboard log | PASS | Across foreground restarts, `dashboard.log` remained `0600` and appended the exercised requests, including the controlled `502` launch failure. |

## Findings

None. Severity counts: 0 BLOCKER, 0 MAJOR, 0 MINOR, 0 POLISH.

The blocked pointer-selection gesture is an unable-to-run visual step, not a product finding. Live
computed styles and the presentation checks support the intended behavior, but a human drag-selection
smoke remains the only missing evidence in this narrow scope.

## Supporting verification

- Focused runtime/server regressions passed for safe Claude initialize guidance, stopped-agent file
  search, Git-aware file search, and the available-command snapshot endpoint.
- The composer and transcript-view component suites passed (15 tests), including style and
  presentation-contract checks.
