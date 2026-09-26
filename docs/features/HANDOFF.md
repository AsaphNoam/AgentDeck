# AgentDeck — Implementation handoff

**Live agent state.** Read the **Current position** and **Active change** below, then open the
requirements they name. Settled state is archived in `../archive/state/`: the dated
[`HANDOFF-through-2026-09-25`](../archive/state/HANDOFF-through-2026-09-25.md),
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

- **Active change:** None. `tighten-chat-actions-and-triage-notes` finished 2026-09-26 and awaits
  `/review`; `simplify-agent-and-automation-setup` retains its open `/fix` findings.
- **Release:** `v0.5.0` is tagged and published; **Release state** carries its contents. `v0.4.3` and
  earlier are in the state archive.
- **Review units:** `tighten-chat-actions-and-triage-notes` (finished 2026-09-26; review
  `a716aea..0c24d3a`, excluding state-only `7738065`),
  `add-studio-skin` (finished 2026-09-23), and `complete-studio-composition`
  (finished 2026-09-25) await `/review`. Review `add-studio-skin` against `e474d8a..1d78e1d` and
  `complete-studio-composition` against `9ae10ee..4bb5b2e`; `71c2810` is the latter's
  evidence-only handoff follow-up, not another unit. `simplify-agent-and-automation-setup`
  (`7d6db5e..d359640`, reviewed 2026-09-26) stays open on its **Review findings**; `d804d90` is
  administrative closure, not another unit.
  FS-12.A19–A23 and TS-08.R68 remain planned despite the shipped requirements.
  `simplify-pipeline-run-detail` (reviewed 2026-09-25, no findings),
  `adopt-modern-codex-acp-capabilities` (reviewed and fixed 2026-09-23), and
  `persistent-pipeline-orchestration` (2026-09-13) are closed; fix commits are not new units.
  `stop-telling-agents-to-poll` shipped outside this queue on the operator's explicit
  2026-09-10 instruction; it can be added later.
- **Work units:** `rename-product-to-deckhand.md` and `add-mobile-remote-control.md` are Waiting to
  start. `migrate-internal-actions-from-mcp.md` stays
  paused on its transport blocker. Queue hygiene: `bump-pinned-acp-adapters.md` reads
  `State: Finished` but is still in `docs/ready-changes/`; left in place rather than deleted unasked.
- **Design units:** `Ideas being defined` entries may resume (the operator deleted the
  uncommitted Cursor backend draft on 2026-09-23); `New ideas`
  entries are available; the permanently unaddressable pipeline agent needs `/design-feature`.
- **Open findings:** `simplify-agent-and-automation-setup` has two **Must fix** and one
  **Worth fixing** under **Review findings**. `adopt-modern-codex-acp-capabilities`, `persistent-pipeline-orchestration`, BR-1, and BR-4 are all closed.
  The injected-steer lifetime edge case is still named in prose but was never recorded
  as a finding; it needs `/investigate-bug` before `/fix` can take it.
- **Bug reports:** BR-1, BR-2, BR-3, and BR-4 are investigated, fixed and closed. Pinned Claude model
  delivery through `_meta` works; an ACP model `currentValue` is adapter configuration evidence and
  no execution-model oracle (TS-04.R54). BR-4 (2026-09-22, "the main project page looks off, the
  cards are stretched and stuck to the bottom") is fixed the same day; see **Review findings**.
- **State:** Automated MCP contract verification is green.
- **Branch:** `main`.

## Active change

**Change:** None. `tighten-chat-actions-and-triage-notes` finished 2026-09-26 and awaits `/review`.

**Changelog — 2026-09-26 (implementation: chat actions and stale-note triage):** Clone now opens
the forked conversation; selected transcript text has Copy beside Annotate; the agent header copies
the stable thread id and uses a compact left identity/runtime band. Six unresolved product/security
questions were recorded under **Ideas being defined**; stale global-template and pipeline-run notes
were not duplicated. The follow-up workflow audit regenerated the embedded UI, added the staged
Switch and visible-error states to the deterministic matrix, and exercised the actual built product
against fakeACP: Clone navigated to the new agent while retaining history, selected transcript text
and the header copied the expected text/id, and the full live header plus rollback error stayed
inside 1024px and 1440px with no horizontal overflow. The Core, Sky & Grove and Studio matrix also
kept its full staged/error controls inside both widths (197.1px high at 1024px and 121.5px at
1440px). Focused UI tests and presentation checks pass. `make test`, `make build`, the UI suite, the
UI production/embed build, specification checks, and diff checks pass.

**Changelog — 2026-09-26 (review: compact agent and automation setup, `7d6db5e..d359640`):**
Two **Must fix** and one **Worth fixing** recorded under **Review findings**. The implementation
(FS-01.R37, FS-04.R49, FS-08.R35–R36, FS-12.R50, FS-14.R80, FS-16.R39–R40, TS-08.R69–R72) passed
`make test`, `make build`, the UI suite (58 files / 473 tests) and a 1024px/1440px built-app pass
across Core, Sky & Grove and Studio. **Fix model:** trivial/easy — Claude Sonnet or Codex Luna.

**Changelog — 2026-09-25 (design: mobile remote control):** Waiting-to-start
`add-mobile-remote-control.md`: paired phones supervise and direct work over the person's tailnet
(planned FS-20, FS-00.R18, TS-13, TS-02.R37, TS-03.R46, TS-05.R23, TS-06.R27, TS-08.R73, INV §14
note). User chose embedded Tailscale, QR pairing, a web app over Expo, an `/api` allowlist and a
node-bound cookie. No product code changed; spec, twin-skill and diff checks pass.

**Owed from archived entries** ([`HANDOFF-through-2026-09-25`](../archive/state/HANDOFF-through-2026-09-25.md)):
A46's real-browser J14 pass; the credentialed Codex 1.12.0 receipt (TS-06.R26) gating
FS-03.A41/A42 and FS-01.A20; Sky & Grove unviewed for Codex capabilities. Pre-existing, not Studio:
Sky & Grove tints the whole user event row; `--ad-shadow-project-edge` resolves at `:root`.

**Changelog — 2026-09-25 (acceptance evidence: Studio composition, FS-12.A21–A23 /
TS-08.R68):** Exercised an isolated built application backed by fakeACP and real API state: all four
Studio onboarding steps; populated Tasks with an attention failure, armed wait, and terminal source;
the active pipeline ledger, timeline, genuine paused/retryable stage, and three-stage template editor;
Archive search plus its archived read-only long transcript; an active permission-required chat; and all
six Settings tabs. Core, Sky & Grove, and Studio were compared on populated Tasks at the available
wide desktop geometry. The Studio full-chat transcript had no horizontal overflow (`clientWidth` and
`scrollWidth` both 1215px), and the active permission surface retained its joined header, transcript,
and composer.

**Acceptance remains planned, not shipped:** FS-12.A19–A23 and TS-08.R68 still need the complete
route/state comparison. In this pass, the available Chrome browser adapter reported 1280×1125 after
its 1024×900 viewport request, so the populated built-app review does not establish true-1024
behavior; the in-app browser was unavailable to that run. A faithful desaturated cross-skin
comparison beyond the dashboard was also unavailable. fakeACP produced active and paused pipeline
states but not a genuine finished-run timeline. No product code or specifications changed in this
evidence pass.

**Release state:** `v0.5.0` is published and verified on tag `8ab84d3`. `make test` (both tag
variants, including `make check-specs`), the UI suite (54 files, 437 tests), and
`make dist VERSION=0.5.0` pass; the local distributable reports `0.5.0` with `sqlite_fts5`. The CI
and Release macOS installer runs both succeeded, and the GitHub Release carries the darwin/arm64
archive, `install.sh`, and a manifest declaring `0.5.0` whose SHA-256 and size match the uploaded
archive. No credentialed or real-browser journey was run for this release, and none may be described
as verified. The standing acceptance-gate checklist
was retired from this file on the operator's explicit decision during this release; the underlying
verification debt is unchanged and is recorded in
[`HANDOFF-through-2026-09-13`](../archive/state/HANDOFF-through-2026-09-13.md).

**Available by role:** `/review` may take `add-studio-skin` or `complete-studio-composition`.
`/work` may take `rename-product-to-deckhand` or `add-mobile-remote-control`;
`/fix` may take `simplify-agent-and-automation-setup`'s findings;
`/design-feature` may choose an available or resumable idea. Queues are independent.

## Decisions needing your input

- **API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
  provider model-ID ownership. Standardizing either is a compatibility change.
- **Failed pipeline-stage chat:** Confirm whether a pause after a failed launch or resume should
  keep withholding **Open agent**, matching restart recovery (FS-14.R48), or whether chat should
  remain reachable with a wider continuation contract.

## Blocked on human

- None.

## Review findings

### simplify-agent-and-automation-setup — **Fix model:** trivial/easy — Claude Sonnet or Codex Luna.

Range `7d6db5e..d359640`, reviewed 2026-09-26.

- **Must fix** (INV §1) — `ui/src/features/onboarding/OnboardingWizard.tsx`: a returning wizard with
  the backend step done but the project step not done starts at Project with `backend` seeded once
  at mount. `OnboardingGate` waits only for config, so the backend catalog is usually not loaded yet
  and the seed falls back to the hard-coded `claude`/`claude-acp`; the resume effect only runs when
  both steps are done. After Project, a person whose configured backend is Codex is offered Claude
  linking (binding the wrong backend id), and an OpenCode/OpenHands person gets a Config step they
  should skip. Violates FS-04.R49's last sentence and TS-08.R70 ("including resumed entry").
  *Fix:* when Backend was not chosen in this wizard session, derive backend id/type from the loaded
  catalog when leaving Project (or extend the resume effect to the backend-done case). *Test:*
  resume with `backend.done`, `project.done=false`, a Codex or OpenCode default backend served
  after first render; complete Project and assert Codex linking or direct advance to Launch.
- **Must fix** (INV §10) — `ui/src/features/launch/NewAgentModal.tsx`: with `fixedProject` the
  Project control is hidden and nothing shows which project the agent will launch into. FS-01.R37
  requires a project-scoped launch to display its fixed project without another chooser; the test
  "locks a scoped launch to its fixed project" asserts the absence instead. *Fix:* render the
  fixed project's title as read-only text in the Project position. *Test:* assert the fixed
  project's title is visible and no project combobox exists.
- **Worth fixing** (INV §8) — `ui/src/features/pipelines/RunStartForm.tsx`: when an assignment is
  missing (e.g. a catalog with no usable default model), Review is disabled and the blocker says
  "Customize runtimes…", but the disclosure stays collapsed. FS-14.R80 requires missing settings to
  expose the relevant controls. *Fix:* open **Customize runtimes** once catalogs have loaded and
  `assignmentsMissing` is true. *Test:* a backend with no models keeps Review disabled and shows
  the runtime controls without a toggle.

Invariant sweep: §§1, 8, 10 found above; §13 checked (every new class has a selector); §§2, 3, 11,
16, 17 checked with no finding (shared dependency helpers, draft ownership, existing APIs, paged
run loading, focused tests); §§4–7, 9, 12, 14, 15 have no surface in this UI-only diff.

## Design consistency notes

- The paused direct-action change cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 cite
  `TS-04.R32–R39` and omit R40, the direct-action redaction clause. Align them when that change
  resumes.
- FS-17 §6's opening sentence should be scoped when its planned direct-cutover work resumes; it
  currently reads as covering a section that also contains planned R13–R19 boundaries.
