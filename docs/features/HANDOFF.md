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

- **Active change:** None. `tighten-chat-actions-and-triage-notes` was reviewed 2026-09-26 with two
  **Worth fixing** findings open for `/fix`. `simplify-agent-and-automation-setup` was fixed and
  closed 2026-09-26.
- **Release:** `v0.5.0` is tagged and published; **Release state** carries its contents. `v0.4.3` and
  earlier are in the state archive.
- **Review units:** `tighten-chat-actions-and-triage-notes` (`a716aea..0c24d3a`, excluding
  state-only `7738065`) was reviewed 2026-09-26 and stays open on its findings.
  `add-studio-skin` (finished 2026-09-23) and `complete-studio-composition`
  (finished 2026-09-25) await `/review`. Review `add-studio-skin` against `e474d8a..1d78e1d` and
  `complete-studio-composition` against `9ae10ee..4bb5b2e`; `71c2810` is the latter's
  evidence-only handoff follow-up, not another unit.
  FS-12.A19–A23 and TS-08.R68 remain planned despite the shipped requirements.
  `simplify-agent-and-automation-setup` (reviewed and fixed 2026-09-26),
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
- **Open findings:** Two **Worth fixing** on `tighten-chat-actions-and-triage-notes` (see
  **Review findings**). The 2026-09-26 usability review's J14 stale run-status
  **Must fix** was fixed and closed 2026-09-26. `simplify-agent-and-automation-setup`,
  `adopt-modern-codex-acp-capabilities`, `persistent-pipeline-orchestration`, BR-1, and BR-4 are all closed.
  The injected-steer lifetime edge case is still named in prose but was never recorded
  as a finding; it needs `/investigate-bug` before `/fix` can take it.
- **Bug reports:** BR-1, BR-2, BR-3, and BR-4 are investigated, fixed and closed. Pinned Claude model
  delivery through `_meta` works; an ACP model `currentValue` is adapter configuration evidence and
  no execution-model oracle (TS-04.R54). BR-4 (2026-09-22, "the main project page looks off, the
  cards are stretched and stuck to the bottom") is fixed the same day; see **Review findings**.
- **State:** Automated MCP contract verification is green.
- **Branch:** `main`.

## Active change

**Change:** None. `tighten-chat-actions-and-triage-notes` has two open review findings.

**Changelog — 2026-09-26 (review: chat actions and stale-note triage):** Two **Worth fixing**
findings (INV §17 annotation-excerpt trim regression; INV §2 duplicated copy handler). Specs cover
the shipped Clone, header copy, selection copy, and compact header (FS-01.R38, FS-03.R62–R63,
FS-12.R51); no local-choice notes were recorded. Swept classes 1, 3–9, 11–16: no applicable
surface or no defect (§8 copy/clone errors surface as toasts; §10 Clone has one UI entry point;
§13 new selectors resolve).

**Changelog — 2026-09-26 (fix: usability review J14 run status, closed):** INV §1 — the
dispatcher-confirmed stage start now moves a dispatch-pending run `queued → running` and republishes
it (FS-14 §3, R37); a test helper that forced `running` was removed (INV §17). Regression tests fail
pre-fix; no spec change. `make test`, `make build`, focused `-race` pass; no browser re-run.

**Changelog — 2026-09-26 (implementation: chat actions and stale-note triage):** Clone now opens
the forked conversation; selected transcript text has Copy beside Annotate; the agent header copies
the stable thread id and uses a compact left identity/runtime band. Six unresolved product/security
questions were recorded under **Ideas being defined**. The built product was exercised against
fakeACP: Clone kept history, both copy paths copied the expected text/id, and the header, rollback
error and Core/Sky & Grove/Studio staged/error matrix stayed inside 1024px and 1440px without
overflow. `make test`, `make build`, the UI suite, UI embed build, spec and diff checks pass.

**Changelog — 2026-09-26 (review + fix: compact agent and automation setup, closed):** INV §1
wizard backend from the loaded catalog (FS-04.R49); INV §10 scoped launch shows its fixed project
(FS-01.R37); INV §8 **Customize runtimes** opens for missing assignments (FS-14.R80). All checks pass.

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
`/fix` may take the `tighten-chat-actions-and-triage-notes` findings;
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

### `tighten-chat-actions-and-triage-notes` (reviewed 2026-09-26, `a716aea..0c24d3a`)

**Fix model:** trivial/easy — Claude Sonnet or Codex Luna.

- **Worth fixing** (INV §17, FS-03.R63/A44) — `ui/src/components/chat/TranscriptView.tsx`
  `selectionWithin` stopped trimming so Copy writes the exact selection, but the same untrimmed
  string now also feeds `clipAnnotationExcerpt` for **Annotate selection**. Trigger: triple-click a
  paragraph (browsers include the trailing newline) or drag across surrounding whitespace, then
  annotate; the stored and delivered excerpt now carries leading/trailing whitespace that shipped
  behavior removed. R63 changes Copy only, and no test covers annotate with padded selection. Fix:
  keep the raw text for Copy and trim for the annotation draft; add a test that annotating
  `"  Second line  "` stores `"Second line"`.
- **Worth fixing** (INV §2) — `ChatPanel.tsx` header **Copy thread identity** and
  `TranscriptView.tsx` **Copy selection** each inline the same clipboard-availability check,
  `writeText`, and `Copy failed` toast. Trigger: the next copy action or error-wording change edits
  one path and not the other. Fix: one small `copyText(text, pushError)` helper in `ui/src/lib`
  used by both; existing tests keep covering both actions.

## Design consistency notes

- The paused direct-action change cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 cite
  `TS-04.R32–R39` and omit R40, the direct-action redaction clause. Align them when that change
  resumes.
- FS-17 §6's opening sentence should be scoped when its planned direct-cutover work resumes; it
  currently reads as covering a section that also contains planned R13–R19 boundaries.
