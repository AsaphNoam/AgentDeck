# Usability review — release follow-up — 2026-09-10

## Scope and outcome

- **Commit:** `f43db8f` (`fix: close the J2 and J5 usability findings`).
- **Scope:** current-tree browser verification of the J2 onboarding compatibility guidance and
  J5 viewport-clamped menus, plus a release-follow-up pass over the core dashboard, archive,
  pipeline, task, settings, terminal-route, and draft journeys.
- **Outcome:** no new usability findings. The full post-fix acceptance matrix remains open because
  real providers, native macOS dialogs, real terminal hooks/xterm, worktrees, and several failure
  and collaboration paths were not available in this isolated run.

## Setup

- **Browser rung:** 1 — the in-app Browser's cached Chromium, driven through Playwright at the
  desktop review floor (`1280 × 720`). Driven pages had zero console/page diagnostics reported by
  the browser harness.
- **Builds:** tagged `make build` passed; current `make dist` passed, including the UI typecheck,
  style/presentation contract checks, embed, and `sqlite_fts5` build. A separate untagged binary
  exercised the no-FTS5 fallback.
- **Fixtures:** isolated homes under `/private/tmp/agentdeck-usability-20260910.JBhk7v/homes/`
  (`fresh`, `seeded`, `lived-in`, `pipeline`, `tasks`, `permission`, and `nofts`), one loopback
  server per journey, and the deterministic fake ACP peer. The incompatible-adapter branch used
  a review-only PATH shim; no product files were changed by the fixture.
- **Evidence:** [review screenshots](usability-review-2026-09-10-evidence/).

## Journey matrix

| Journey | Result | Current evidence / limitation |
|---|---|---|
| J1 Install & first paint | PASS | Tagged build starts and the styled shell renders with zero browser diagnostics. [J1](usability-review-2026-09-10-evidence/J1-first-paint.png) |
| J2 Onboarding wizard | PARTIAL | Missing adapter gives install guidance; an installed adapter rejecting the readiness argv gives compatibility guidance and a retry path, not credential-failure guidance. Real logged-out/logged-in provider branches and native folder panels remain open. [J2 missing](usability-review-2026-09-10-evidence/J2-missing-adapter.png), [J2 incompatible](usability-review-2026-09-10-evidence/J2-incompatible-adapter-current.png) |
| J3 First launch + chat | PARTIAL | Fake ACP launch, prompt, response, idle state, and transcript reload passed. Mermaid streaming/injection variants were not run. [round trip](usability-review-2026-09-10-evidence/J3-chat-roundtrip.png), [reload](usability-review-2026-09-10-evidence/J3-transcript-after-reload.png) |
| J4 Permission flow | PARTIAL | Prompt, approve, and deny passed with no stuck prompt. Timeout was not exercised. [prompt](usability-review-2026-09-10-evidence/J4-permission-prompt.png), [approve](usability-review-2026-09-10-evidence/J4-permission-approved.png), [deny](usability-review-2026-09-10-evidence/J4-permission-denied.png) |
| J5 Grid & layout | PARTIAL | The current lower-row context-menu fix keeps the full menu inside the viewport and exposes Resume/Archive; stopped-card Resume and running-card menu state also passed. The broader grid/pane matrix is covered by the 2026-08-30 report, not rerun in full here. [J5](usability-review-2026-09-10-evidence/J5-lower-row-menu-current.png) |
| J6 Terminal runtime | BLOCKED | The terminal route and tab mount, but the fake peer has no native PTY and reports `[connection error]`/`[closed]`; real terminal hooks/xterm are an open acceptance gate. |
| J7 Stop / resume / switch | PARTIAL | Stopped-card Resume passed; after resuming, the running-card menu offered Stop and did not offer Resume. Runtime switching and process-level identity preservation were not run. |
| J8 Archive & search | PARTIAL | FTS build archived/search-filtered/restored sessions; the untagged build's metadata-only fallback search also returned a clean no-results state. Empty/one/many-session permutations on both builds were not exhaustive. [FTS list](usability-review-2026-09-10-evidence/J8-archive-list.png), [FTS search](usability-review-2026-09-10-evidence/J8-archive-search.png), [no-FTS](usability-review-2026-09-10-evidence/J8-nofts-search.png) |
| J9 Settings & config | PARTIAL | Settings surfaces mounted; task-budget `0` was refused without enabling Save, and `3` round-tripped after reload. Every settings collection/form was not edited. |
| J10 Multi-agent + messaging | SKIPPED | No two-agent send-message, nudge, or unread-badge pass in this run. |
| J11 Failure & recovery | SKIPPED | No server-kill, agent-crash, reconnect, or garbage-form sweep in this run. |
| J12 Restart durability | SKIPPED | Individual page reloads passed for chat, archive, settings, and drafts; the complete post-J3–J10 restart matrix was not run. |
| J13 Annotate & assign | SKIPPED | Live/archived annotation and assignment paths were not run. |
| J14 Configurable pipelines | PARTIAL | Runs/Templates split, restart-recovery pause, retry-stage launch, execution timeline, and stage-agent link passed. Full four-stage mixed-runtime, builder-confirmation, repair-loop, and retained-history sweep was not rerun; the 2026-08-30 report covers the changed pipeline surface. [retry](usability-review-2026-09-10-evidence/J14-retry-stage.png) |
| J15 Dependent work | PARTIAL | Empty state, signal-gated task, `armed → ready → running → finished`, and recorded success passed. Cancel/failure/retry/re-arm/restart permutations were not run. [J15](usability-review-2026-09-10-evidence/J15-dependent-task.png) |
| J16 Task attention & budget | PARTIAL | Zero attention count and budget validation/round-trip passed. Queue pressure, worktree creation/archive, and native project-canvas picker flows remain open. |
| J17 Browser-local drafts | PARTIAL | An unsent draft survived a full dashboard reload and was cleared from the fixture afterward. Two-chat isolation, navigation, send-clears, and stopped-agent leakage permutations were not run. [J17](usability-review-2026-09-10-evidence/J17-draft-after-reload.png) |

## Finding status

No new finding met the review protocol's threshold. The two current-tree checks that motivated this
run passed:

- **J2:** an adapter that rejects `--cli auth status` is rendered as “installed Claude adapter is
  too old” with an actionable update and retry path.
- **J5:** the lower-row context menu measured `210 × 309` at `left=220, top=359, right=430,
  bottom=668` inside the `1280 × 720` viewport, with Resume and Archive visible.

The real-provider credential/model/MCP checks, Claude terminal hooks/xterm, OpenCode/OpenHands,
native macOS folder picker, real-browser drag refusal, federation, worktrees, and six-tab
same-origin check remain acceptance gates rather than verified behavior.
