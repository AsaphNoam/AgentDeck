# AgentDeck — handoff state settled through 2026-09-11

Archived under AGENT-WORKFLOW §16.7 to keep the live handoff header inside its session-start budget.
Everything below is settled history: changelog entries whose units are closed. Findings those
entries left open stay live in [`../../features/HANDOFF.md`](../../features/HANDOFF.md).
Earlier epochs are in
[`HANDOFF-through-2026-09-10.md`](HANDOFF-through-2026-09-10.md),
[`HANDOFF-through-2026-09-09.md`](HANDOFF-through-2026-09-09.md),
[`HANDOFF-through-2026-09-07.md`](HANDOFF-through-2026-09-07.md),
[`HANDOFF-through-2026-09-06.md`](HANDOFF-through-2026-09-06.md),
[`HANDOFF-through-2026-09-03.md`](HANDOFF-through-2026-09-03.md), and
[`HANDOFF-pre-sdd.md`](HANDOFF-pre-sdd.md).

## Changelog entries

**Changelog — 2026-09-11 (fix):** Closed **open a file an agent mentioned** (FS-03.R55/A37,
TS-05.R21; `INV §14`, `INV §17`). File reads now use `os.OpenInRoot`, then inspect and read the
returned descriptor, so a concurrent file or symlink replacement cannot redirect an accepted path
outside the recorded working directory. A deterministic regression replaces the checked file with
an outside symlink immediately before open and proves outside bytes are refused. The full Go matrix,
tagged build, focused file-read tests, spec checks, and diff check pass. The originating unit is
closed; BR-1 and BR-2 findings remain open.

**Changelog — 2026-09-11 (review):** Reviewed **open a file an agent mentioned**. The route refuses
lexical and resolved symlink escapes, but it checks the pathname and then resolves that pathname
again for `Stat` and `Open`; a concurrent workspace change can swap an accepted file or symlink for
an outside target between those operations. The unit remains open with one Must-fix finding.
**Fix model:** medium — Codex Terra or Claude Opus. The full Go matrix and tagged build, all 449 UI
tests, style and presentation-contract checks, focused file-read tests, and diff check pass.
Invariant classes 1, 2, 8, 10, 11, 13, 14, 16, and 17 apply; classes 3, 4, 5, 6, 7, 9, 12, and 15
have no applicable surface.

**Changelog — 2026-09-11 (work):** Finished **keep steering inside AgentDeck's turn lifecycle**
(FS-03.R56/A38, TS-01.R30, TS-03.R41, TS-04.R51, TS-06.R14; `INV §2`, `INV §5`, `INV §11`,
`INV §12`, `INV §17`). AgentDeck now opts into the adapters' no-consumption `promptRequired`
steering result and routes that unchanged text through the ordinary prompt gate. The replacement
turn therefore owns busy state, cancellation, transcript events, Send holding, and one terminal
outcome; `startedNewTurn` remains a legacy result that is reported but never retried. A dedicated
fallback reservation takes priority over — and preserves — a Send accepted while the adapter is
still deciding the race.

Claude 0.75.1 already supplies the request-level opt-in. Release assembly applies a fail-closed,
version-locked patch to Codex ACP 1.10.0 and records the component as
`1.10.0+agentdeck.1`; source drift or a missing result contract stops packaging. The fake peer now
settles the original prompt before returning `promptRequired`, and focused runtime/route coverage
proves the fallback lifecycle and no-consumption request metadata. The full automated Go matrix,
build, spec lint, patch applicability, and shell syntax pass. Real provider steering remains an
acceptance gate.

**Changelog — 2026-09-11 (work):** Shipped **open a file an agent mentioned**
(FS-03.R51–R55/A34–A37, FS-05.R37/A20, TS-03.R40, TS-05.R21, TS-08.R57; `INV §1`, `INV §2`,
`INV §13`, `INV §14`, `INV §16`, `INV §17`). A filepath link an agent wrote now opens a read-only
viewer beside the transcript instead of navigating to a non-route that the SPA fallback and router
catch-all turned into a full reload onto the dashboard.

`GET /api/sessions/{id}/file` (`internal/server/fileread.go`) reads one bounded UTF-8 text file
confined to the working directory recorded on that agent's own session snapshot, sharing
`filesearch.go`'s `withinRoot` resolve-and-recheck rather than copying it. The requested path is
decided on its form before that path is ever touched on disk, so traversal, absolute-path escape,
and `.git` are refused identically whether or not the target exists — `fileread_test.go` asserts an
existing and an absent outside path produce byte-identical responses. New typed codes
`path_refused`, `not_a_file`, `not_text`, and `workspace_unavailable` (all 422) join the shared
vocabulary in `internal/runtime/errors.go`. The read is deliberately **not** gated on a running
record, so archived sessions work; Git-ignored files inside the root read successfully by decision.

On the UI, `renderers/filePath.ts` is the one place a link target is classified and its
`:line`/`:line:col` suffix parsed, and `renderers/SanitizedMarkdown.tsx` is now the single
sanitized Markdown renderer shared by assistant messages and the viewer's rendered form. Two
separate gates were dropping `file://` links before classification — the sanitizer's allowed link
protocols and react-markdown's own `urlTransform`; both were widened for `file:` alone, and a local
path still never reaches an `href` because the override renders a control instead. `.transcript-wrap`
gained a leading grid track for the viewer opposite the tray's trailing one; with three tracks the
three in-flow children are now placed explicitly, because auto-placement would drop the transcript
into the viewer's content-sized column. `?file=`/`?fileLine=` are the open file's only state — no
store, context, or persisted key. `react-syntax-highlighter` overwrites a line's `className` with
its own token classes, so the cited-line mark is a `data-file-marked` attribute, not a class.

Not done and owed: every rendered form is unverified in a real browser. J3 carries the steps.

**Changelog — 2026-09-10 (fix):** Closed the `duplicate-steer-submission` Must-fix
(FS-03.R50/A33, TS-03.R39; `INV §5`, `INV §17`). Steer now takes a synchronous per-agent in-flight
claim and disables its control until the request settles, so a double-click produces one delivery
and the control becomes available again afterward. The previously skipped reproduction failed with
two requests before the fix and now passes with one. The originating investigation unit is closed.

**Changelog — 2026-09-10 (fix):** Closed the `prompt-echo-race` Must-fix
(FS-03.R6/R7/R48/A31, TS-08.R41/R56; `INV §2`, `INV §5`, `INV §17`). User-message
reconciliation now handles both arrival orders: a durable sequenced event replaces an existing
optimistic echo, while an optimistic echo is suppressed when the durable event already arrived.
The regression test was confirmed failing before the fix and now asserts the single retained event
has its durable sequence. The originating investigation unit is closed.

**Changelog — 2026-09-10 (design + work):** Shipped **stop telling agents to poll for work**
(FS-18.R12/R13/A9, FS-04.R47/A27, TS-11.R13; `INV §2`, `INV §7`, `INV §8`, `INV §10`, `INV §17`).
The request named a 60-second update requirement that does not exist; the real remnant was four
seeded prompts telling agents to find work themselves. `teammate` no longer opens its loop with a
per-turn coordination check, and `implementer`/`reviewer`/`researcher` dropped their "woken with no
new instruction" mail check — a case `internal/runtime/activation_kinds.go:27` makes impossible.
`MigrateLegacyAgentDecker` became `MigrateSupersededRolePrompts`: one sorted pass over
`supersededRolePromptDigests` with replacement text read from `seedRoles()`, where a per-role read,
decode, or write failure is joined and skipped rather than aborting the pass. Existing installs are
corrected; a prompt edited by one byte stays user-owned. `testdata/superseded_*_prompt.txt` holds the
pre-change bytes as the digest oracle, and those same bytes fail the new banned-phrase guard.
FS-18.R7, FS-04.R44, and TS-11.R6 are superseded, not weakened; FS-04 and FS-18 stay Current. Three
settled entries were archived for budget.
