# AgentDeck — Implementation handoff

**Live agent state.** Read the **Current position** and **Active change** below, then open the
requirements they name. Settled state is archived in `../archive/state/`: the dated
[`HANDOFF-through-2026-09-27`](../archive/state/HANDOFF-through-2026-09-27.md),
[`-25`](../archive/state/HANDOFF-through-2026-09-25.md),
[`-14`](../archive/state/HANDOFF-through-2026-09-14.md),
[`-13`](../archive/state/HANDOFF-through-2026-09-13.md),
[`-12`](../archive/state/HANDOFF-through-2026-09-12.md),
[`-11`](../archive/state/HANDOFF-through-2026-09-11.md),
[`-10`](../archive/state/HANDOFF-through-2026-09-10.md),
[`-09`](../archive/state/HANDOFF-through-2026-09-09.md),
[`-07`](../archive/state/HANDOFF-through-2026-09-07.md),
[`-06`](../archive/state/HANDOFF-through-2026-09-06.md) and
[`-03`](../archive/state/HANDOFF-through-2026-09-03.md) files, plus
[`HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md). Follow
[`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md); this file holds resumable current state only.

## Current position

- **Active change:** None.
- **Release:** `v0.6.0` is tagged and published; **Release state** carries its contents. `v0.5.0` and earlier
  are in the state archive, as are the units, findings and bug reports it closed.
- **Review units:** `add-studio-skin` (finished 2026-09-23) and `complete-studio-composition`
  (finished 2026-09-25) await `/review`; the operator shipped them unreviewed in `v0.6.0`. Review
  `add-studio-skin` against `e474d8a..1d78e1d` and `complete-studio-composition` against
  `9ae10ee..4bb5b2e`; `71c2810` is the latter's evidence-only handoff follow-up, not another unit.
  FS-12.A19–A23 and TS-08.R68 remain planned despite the shipped requirements.
  `stop-telling-agents-to-poll` shipped outside this queue on the operator's explicit
  2026-09-10 instruction; it can be added later.
- **Work units:** `rename-product-to-deckhand.md` and `add-mobile-remote-control.md` are Waiting to
  start. `migrate-internal-actions-from-mcp.md` stays
  paused on its transport blocker. Queue hygiene: `bump-pinned-acp-adapters.md` reads
  `State: Finished` but is still in `docs/ready-changes/`; left in place rather than deleted unasked.
- **Design units:** `Ideas being defined` entries may resume (the operator deleted the
  uncommitted Cursor backend draft on 2026-09-23); `New ideas`
  entries are available; the permanently unaddressable pipeline agent needs `/design-feature`.
- **Open findings:** BR-6's investigation unit (four findings) and BR-7's (one observability
  finding), both in **Review findings**. The injected-steer lifetime edge case is still named in prose
  but was never recorded as a finding; it needs `/investigate-bug` before `/fix` can take it.
  FilesTab and CommandsTab still copy silently via bare `writeText`.
- **Bug reports:** BR-6 investigated; findings await `/fix`. BR-7 (whole UI dead, `startTime`
  error) not reproduced on repeat investigation, root cause undetermined; needs the affected
  browser/URL and full error stack (see BR-7).
- **State:** Automated MCP contract verification is green.
- **Branch:** `main`.

## Active change

**Change:** None.

**Owed from archived entries** ([`HANDOFF-through-2026-09-25`](../archive/state/HANDOFF-through-2026-09-25.md)):
A46's real-browser J14 pass; the credentialed Codex 1.12.0 receipt (TS-06.R26) gating
FS-03.A41/A42 and FS-01.A20; Sky & Grove unviewed for Codex capabilities. Pre-existing, not Studio:
Sky & Grove tints the whole user event row; `--ad-shadow-project-edge` resolves at `:root`.

**Acceptance remains planned, not shipped:** FS-12.A19–A23 and TS-08.R68 still need the complete
route/state comparison. In this pass, the available Chrome browser adapter reported 1280×1125 after
its 1024×900 viewport request, so the populated built-app review does not establish true-1024
behavior; the in-app browser was unavailable to that run. A faithful desaturated cross-skin
comparison beyond the dashboard was also unavailable. fakeACP produced active and paused pipeline
states but not a genuine finished-run timeline. No product code or specifications changed in this
evidence pass.

**Release state:** `v0.6.0` is published on tag `24ab07b` (range `v0.5.0..main`, 49 commits). The
CI and Release macOS installer runs both succeeded; the GitHub Release carries the darwin/arm64
archive, `install.sh`, and a manifest declaring `0.6.0` whose SHA-256 and size match the uploaded
archive.
`make test` (both tag variants, including `make check-specs`), the UI suite (58 files, 481 tests),
and `make dist VERSION=0.6.0` pass; the local distributable reports `0.6.0` with `sqlite_fts5`. The
range changed nothing an operating agent must know, so `operating-agentdeck` is unchanged; README,
`install.sh` and `scripts/release/assemble.sh` pins still match. Two Studio review units shipped
unreviewed on the operator's explicit decision. Owed: the credentialed Claude and Codex
login/chat gates (TS-06.R21) and every real-browser journey; none may be described as verified.

**Available by role:** `/review` may take `add-studio-skin` or `complete-studio-composition`.
`/work` may take `rename-product-to-deckhand` or `add-mobile-remote-control`;
`/design-feature` may choose an available or resumable idea. Queues are independent.

## Decisions needing your input

- **API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
  provider model-ID ownership. Standardizing either is a compatibility change.
- **Failed pipeline-stage chat:** Confirm whether a pause after a failed launch or resume should
  keep withholding **Open agent**, matching restart recovery (FS-14.R48), or whether chat should
  remain reachable with a wider continuation contract.

## Blocked on human

- BR-7: identify the affected browser and URL, where the `startTime` message appears, and copy
  its full error stack. The connected in-app browser had no existing tabs; its fresh session does
  not reproduce the reported failure, so it cannot establish the failing browser's state.

## Review findings

### BR-6 investigation unit (2026-09-27) — **Fix model:** medium — Codex Terra or Claude Opus.

Report, verbatim: "links clicked on in the agents aren't opened (refused), even if it's in the
working directory". Version: `v0.6.0` era (the live dev instance runs `main`); macOS. No link, agent,
or log was supplied. Nothing was logged: the local instances' request logs contain no `/file`
request, and the dev instance logs to its terminal. Governing items: FS-03.R51–R55/A34–A37,
TS-03.R40, TS-05.R21. The server route was probed live against real agents: relative, `./`, and
absolute in-directory paths read correctly, so the failures are in which path reaches it.

- **Must fix** — confirmed (reproduced), spec gap plus defect; INV §11. Links inside a file shown in the
  viewer's **Rendered** form are sent verbatim and resolved against the agent's working directory,
  not the viewed file's directory. From `docs/features/HANDOFF.md`, `../archive/state/x.md` is
  refused as outside the working directory though the target is inside it, and a sibling
  `AGENT-WORKFLOW.md` reads as missing; this repository's own docs link this way throughout.
  `FileViewer.tsx:95` passes `onOpenFile` straight to `SanitizedMarkdown`. FS-03.R52 does not say
  what a rendered file's relative link is relative to; specify the file's own directory (ordinary
  Markdown semantics), resolve it client-side before `onOpenFile`, and let a result that climbs
  above the working directory still reach the server's refusal. The existing A35 test expects the
  current behavior (`other.go` from `docs/notes.md`) and must change. Regression: the skipped
  BR-6 test in `ui/src/components/chat/FileViewer.test.tsx`.
- **Worth fixing** — confirmed (reproduced); INV §11. A bare file name with a line suffix
  (a link whose target is `README.md:12`) matches `filePath.ts`'s scheme regex (`README.md:` reads as a
  scheme), so it is not a file link, the default URL transform strips it, and the click does
  nothing. FS-03.R51/A34. Fix: treat a target whose apparent scheme is followed only by a line
  suffix as a path. Regression: skipped test in `renderers/filePath.test.ts`.
- **Worth fixing** — confirmed (reproduced); INV §11. A `:start-end` line range, which Codex writes (18
  occurrences in this machine's recent Codex sessions, e.g. `dashboard.css:49-125`), stays part of
  the file name, so the viewer reports the file missing. FS-03.R51 names only `:line` and
  `:line:col`; extend it to a range cited by its start line. Regression: skipped test in
  `renderers/filePath.test.ts`.
- **Worth fixing** — probable (refusal reproduced live, trigger not observed); INV §14. On macOS's
  case-insensitive disk, an absolute link whose spelling differs from the recorded working
  directory only by case (`/users/…/projects/agentdeck/README.md` against
  `/Users/…/Projects/AgentDeck`) is refused as outside the directory by `rebaseAbsolute`'s lexical
  `filepath.Rel` (`internal/server/fileread.go:117`). An agent that takes its root from `git` or
  `realpath` while the directory was typed in another case produces this. Fix at that seam, e.g.
  by also accepting a base whose canonical on-disk spelling matches; keep the refusal on form
  alone (TS-05.R21). Regression: a `fileread_test.go` case on a case-insensitive temp volume.
- **Worth fixing** — observability, confirmed from the code path; INV §8. A refused read leaves no trace: `requestLog` records the path
  without the query and the status without the refusal code, so this report could not be tied to a
  link form. Log the refusal code and the requested `path` at `handleFileRead` (local-only log), so
  the next report answers "which link, which boundary".

### BR-7 investigation unit (2026-09-27) — **Fix model:** medium — Codex Terra or Claude Opus.

Report, verbatim: "nothing loads, opening any page does nothing and I get "cannot read properties of
undefined (reading 'startTime')". Version, browser, route, and where the message appeared were not
given; no stack trace or log was supplied. The dashboard log has no matching entry, and could not:
browser errors never reach the server. Governing items: none — FS-12 and TS-03 do not specify client
failure handling or reporting (spec gap, below).

Not reproduced (root cause **undetermined**). A scripted clean-profile Chrome loaded every top-level
route, a project page, two live agent pages, and their tabs on all four running instances (the
`main` dev server on 4317 and three `v0.6.0`/`88e7e06` binaries on 4405/4416/4417) with no page
error or console error. Every `startTime` read in the main bundle is guarded: React's scheduler
reads it only off a non-null heap peek, and Monaco's smooth-scroll reads it only inside
`if (this._smoothScrolling)`. The other reads live in lazily loaded Mermaid gantt/cynefin and
Cytoscape chunks, which load only when a chat renders such a diagram and cannot stop page
navigation on their own. Correction from repeat investigation: the dashboard error boundary only
wraps `Outlet` (`ui/src/App.tsx:11`); Header and NotificationCenter are outside it, as are the root
providers (`ui/src/main.tsx`). A failure outside that boundary can reach React Router's default
error UI or escape the router, so the reported wording does not establish a scheduler or injected
script failure. No evidence identifies either as the cause. Next evidence: the full error stack,
affected browser/URL, and whether a private window without extensions reproduces it.

Repeat report, verbatim (2026-09-27): "/investigate-bug nothing loads, opening any page does
nothing and I get \"cannot read properties of undefined (reading 'startTime'). Opus tried andfailed".
At clean commit `2b52de9`, the connected Codex in-app browser had no existing tabs. A new tab at
`http://localhost:4317` rendered the populated Dashboard, Settings roles, Tasks, Pipelines runs,
and Archive via visible navigation, with no captured warning/error logs. This is fresh-session
evidence only, not a reproduction of the reporter's browser. Expected route navigation follows
FS-12.R6/R16/R17; client error reporting remains the separate gap below. Source tracing again
found no application-source `startTime` read. Scheduler reads are protected by its queue contract;
the Cytoscape animation candidate reads `ani_p.easing` before `ani_p.startTime`, so an absent
`_private` would fail on `easing` first and does not explain this exact error. No new confirmed
cause or skipped reproduction test; no product/spec changes. Awaiting the requested browser/URL,
message location, and full stack rather than treating another clean session as a resolution.

Follow-up (2026-09-28): the reporter says it began after installing the latest release. Release
check: `git log -S startTime -- ui/src` is empty, so no AgentDeck UI source has ever read
`startTime`; `ui/package.json` and `ui/package-lock.json` are identical across `v0.5.0..v0.6.0`,
so every bundled library that does read it is unchanged from `v0.5.0`. The release's 61 changed UI
files cannot produce this message directly. A new-code bug that hands a library bad data (for
example an unguarded diagram or runtime-activity payload), a skipped-version upgrade, or a browser
tab left open from the old version remain possible and untested. This Mac's installed release is still
`0.1.2`, so the reporter's install is elsewhere and its prior version is unknown.

- **Worth fixing** — observability plus spec gap, confirmed from the code path; INV §8, §14, §16.
  A browser-side failure leaves no trace the operator can send: `ErrorBoundary.componentDidCatch`
  (`ui/src/components/ErrorBoundary.tsx:22`) only `console.error`s, its fallback shows no error
  text, and nothing handles `window` `error`/`unhandledrejection`. This report therefore carries a
  bare message with no stack, file, route, or build. Specify (FS-12 plus a TS-03 route) that
  uncaught errors and boundary catches are posted with message, stack, component stack, route, and
  build version to a local-only, size-bounded, rate-limited endpoint that writes them to the
  dashboard log, and that the boundary fallback shows the message. Test: a UI test that a thrown
  render error and a window error each post one bounded report, and a Go test that the endpoint
  truncates and logs it.

## Design consistency notes

- The paused direct-action change cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 cite
  `TS-04.R32–R39` and omit R40, the direct-action redaction clause. Align them when that change
  resumes.
- FS-17 §6's opening sentence should be scoped when its planned direct-cutover work resumes; it
  currently reads as covering a section that also contains planned R13–R19 boundaries.
