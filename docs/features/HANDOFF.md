# AgentDeck — Implementation handoff

**Live agent state.** Read the **Current position** and **Active change** below, then open the
requirements they name. Settled state is archived in
[`../archive/state/HANDOFF-through-2026-09-10.md`](../archive/state/HANDOFF-through-2026-09-10.md),
[`../archive/state/HANDOFF-through-2026-09-09.md`](../archive/state/HANDOFF-through-2026-09-09.md),
[`../archive/state/HANDOFF-through-2026-09-07.md`](../archive/state/HANDOFF-through-2026-09-07.md),
[`../archive/state/HANDOFF-through-2026-09-06.md`](../archive/state/HANDOFF-through-2026-09-06.md),
[`../archive/state/HANDOFF-through-2026-09-03.md`](../archive/state/HANDOFF-through-2026-09-03.md),
and [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md). Follow
[`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md); this file holds resumable current state only.

## Current position

- **Active change:** None; the next role picks from the queues below.
- **Release:** `v0.4.3` is tagged and published; **Release state** and the release record carry its
  contents. `v0.4.2` and earlier are in the state archive.
- **Review units:** `fix-model-recommendations` awaits independent review once committed, and
  `open-a-file-from-chat` is newly available for review. Every other unit through this release is
  closed, including `queue-a-follow-up-while-busy`, its `steering-host-owned-fallback` continuation,
  and `usability-20260907`. `stop-telling-agents-to-poll` shipped without entering
  this queue on the operator's explicit 2026-09-10 instruction; it can be added later.
- **Work units:** None waiting to start. `migrate-internal-actions-from-mcp.md` stays paused on its transport
  blocker; the ACP wait-list in `docs/ideas.md` holds the rest behind an adapter contract.
  Queue hygiene: `bump-pinned-acp-adapters.md` reads `State: Finished` but is still in
  `docs/ready-changes/` and absent from that directory's index; per its README a finished change's
  file is removed. Left in place rather than deleted unasked.
- **Design units:** `Ideas being defined` entries may resume; `New ideas` entries are available.
  Streaming agent thinking stays part-decided (live-only decided; rendering default and whether
  `plan` ships still open). The permanently unaddressable pipeline agent is the newest `New ideas`
  entry and needs `/design-feature` before code.
- **Open findings:** The separate injected-steer lifetime edge case remains outside the closed
  host-owned fallback unit. Also open: live-gate finding durability, provider-contract oracles, the Codex
  discovery-versus-execution version authority left by BR-2, and the unverified OpenCode/OpenHands
  paths. See **Review findings**.
- **Bug reports:** BR-1, BR-2, and BR-3 are investigated and archived with this release; BR-3 is
  fixed and closed. BR-1's Codex model/effort defect is fixed and reviewed and BR-2's release pin is
  fixed by the adapter bump; their still-open findings are listed above. Pinned Claude model delivery
  through `_meta` works; an ACP model `currentValue` can be stale and is no execution-model oracle.
- **State:** The file viewer's credentialed rendered forms are owed: journey J3 now carries the
  file-link steps (docked and transcript-width forms, the refusal branch, the dashboard-pane
  navigation), and none of them has been exercised against a real browser.
  Automated MCP contract verification is green. The full post-fix Claude/Codex acceptance
  matrix remains open and must not be called verified; the 2026-09-08/09 probes were limited contract
  and model-delivery checks, not that matrix. Real Claude and Codex steering is unexercised.
- **Branch:** `main`.

## Active change

**Change:** None.

**Changelog — 2026-09-11 (fix):** Closed **keep steering inside AgentDeck's turn lifecycle**
(FS-03.R56/A38, TS-01.R30, TS-03.R41, TS-04.R51, TS-06.R14; `INV §5`, `INV §10`, `INV §12`,
`INV §15`, `INV §17`). Steer now reserves its possible host-owned successor before the adapter call;
turn settlement transfers the gate to that reservation without firing it, and the adapter result
atomically commits `promptRequired` or unwinds injected, legacy, and refusal outcomes. The wire-level
regression holds the steering reply after the old prompt settles, accepts a concurrent Send, and
proves provider order remains old prompt, correction, then held Send. Release assembly now checks
exact pre- and post-patch SHA-256 fingerprints and disables patch fuzz, and shipped traceability no
longer says planned. The originating `queue-a-follow-up-while-busy` unit and its fallback continuation
are closed. The full Go matrix, tagged build, focused race tests, spec lint, shell syntax, patch
applicability, and diff check pass.

**Changelog — 2026-09-11 (review):** Reviewed **keep steering inside AgentDeck's turn lifecycle**.
The host-owned fallback works in the covered lifecycle, but the reservation is created only after
the adapter returns; a Send accepted during that wait can therefore become the next turn first.
Release assembly also allows patch fuzz, and three technical-spec traceability entries still call
the shipped lifecycle planned. The unit remains open with one Must-fix and two Worth-fixing findings.
**Fix model:** medium — Codex Terra or Claude Opus. The full Go matrix, tagged build, focused race
tests, spec lint, patch applicability, shell syntax, and diff check pass. Invariant classes 1, 2, 4,
5, 8, 10, 11, 12, 15, 16, and 17 apply; classes 3, 6, 7, 9, 13, and 14 have no applicable surface.

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

**Release state:** `v0.4.3` is published and verified on tag `8ad5261`. Release and CI runs passed,
the local distributable reports `0.4.3` with `sqlite_fts5`, and the GitHub Release carries the
darwin/arm64 archive, `install.sh`, and a manifest declaring `0.4.3` with its SHA-256.
The release shipped with five open Must-fix findings on the operator's explicit decision; all five
are now closed. The credentialed Claude and Codex journeys under
**Acceptance gates** are owed; real steering has never been exercised against a provider.

**Available by role:** `/review` may take `fix-model-recommendations` or `open-a-file-from-chat`;
`/work` has no waiting unit; `/fix` may take BR-1 (difficult, Sol) or BR-2 (medium, Terra/Opus);
`/design-feature` may choose an available or
resumable idea. Queues are independent.

## Decisions needing your input

- **API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
  provider model-ID ownership. Standardizing either is a compatibility change.
- **Failed pipeline-stage chat:** Confirm whether a pause after a failed launch or resume should
  keep withholding **Open agent**, matching restart recovery (FS-14.R48), or whether chat should
  remain reachable with a wider continuation contract.

## Acceptance gates

**Not blocking as of 2026-09-05.** These gates have not been run, and no agent may describe them as
verified, passed, or closed. The operator chose to let roles proceed with them open.

- [ ] Pinned real-provider stage-result/file-edit approval journey (FS-03.A26/J14).
- [ ] Post-fix credentialed Claude and Codex chat, MCP, resume, task, effective-model/effort, and
      reported-result checks. The historical 2026-07-26 run failed model precedence and is evidence,
      not closure for the current implementation.
- [ ] Pinned Claude terminal flags/hooks and live xterm journeys.
- [ ] Pinned OpenCode/OpenHands launch and credential checks.
- [ ] Real macOS native folder-panel checks (FS-04.A22/J2/J9/J16).
- [ ] Real-browser permission-pane and drag-refusal journeys (FS-02.A35/A43).
- [ ] Phase 7 federation matrix against real Claude and Codex installations.
- [ ] Real-browser worktree creation/launch and archive-with-uncommitted-work journeys (FS-19).
- [ ] Six-tab same-origin dashboard check against a `make dist` build (FS-02.A27).

## Blocked on human

- None.

## Review findings

### BR-2 — **Fix model:** medium — Codex Terra or Claude Opus.

- **Worth fixing** — Codex model discovery and execution use different version authorities
  (**confirmed spec gap**). **Where:** `internal/config/codexmodels.go:31-86` imports every visible
  model from `${CODEX_HOME:-~/.codex}/models_cache.json`, while `internal/release/wrapper.go:10-25`
  and the pinned adapter execute the release-private Codex. `FS-09.R47` explicitly says import does
  not claim future availability but defines no compatibility check or actionable degraded state.
  **Normal-use trigger:** the personal Codex CLI/cache advances beyond AgentDeck's pinned runtime
  and advertises a newly introduced model. **Why it matters:** this can recur after any model/runtime
  rollout: AgentDeck presents the model as selectable and discovers incompatibility only after an
  attempted session or prompt. A local workaround exists but is obscure: `codex-acp` 1.1.2 honors
  `CODEX_PATH`, AgentDeck preserves that variable, and Settings' generic **Backend env** editor can
  set it to an absolute compatible Codex executable. **Requirement:** coverage gap between
  `FS-09.R28/R47` and `TS-06.R14/R15/R22`; `INV §8`, `INV §10`, `INV §12`. **Suggested fix/test:**
  specify one compatibility policy (discover from the execution runtime, filter/mark models by the
  packaged version, or expose a first-class validated Codex executable override), show the effective
  runtime/version before launch, and test a personal cache that is newer than the packaged CLI.

### BR-1 — **Fix model:** difficult — Codex Sol.

- **Must fix** — BR-1 finding state was not durable across concurrent roles (**confirmed**).
  **Where:** `docs/archive/reviews/live-provider-acceptance-2026-07-26.md` recorded the exact Codex
  model failure as a Must-fix on July 26, but the live acceptance session could not edit HANDOFF
  while another session owned the shared docs. The report and a contradictory effort design were
  then swept into `7d294fb` without the finding entering HANDOFF. **Why it matters:** the mandatory
  read order made the archived report invisible to every later role, so an already-detected critical
  defect was treated as an open gate for another six weeks. **Requirement:** workflow §§1.1/12,
  `INV §1`, `INV §10`. **Suggested fix/test:** a failed live gate must be recorded in HANDOFF before
  its role can close; if state-file ownership blocks that write, leave the role explicitly blocked
  rather than archiving the only finding. A commit/review that contains an acceptance report must
  reconcile every Must-fix in it with live state.
- **Must fix** — provider-contract claims can still be proved by a self-authored oracle
  (**confirmed**). **Where:** TS-04.R18 asserted a `model[effort]` request shape after inspecting
  `codex-acp`'s internal `ModelId` parser without tracing `session/new` to `threadStart`; FS-09.A15
  and `fakeacp` then checked only that AgentDeck emitted the asserted field. The fake accepts unknown
  request members that the pinned ACP decoder drops. The same oracle error recurred in the 2026-09-08
  finding-fix: it called ACP `configOptions.model.currentValue` an independent effective-model oracle
  and declared Claude `_meta` delivery broken without sending a prompt. A real prompt then showed
  stale `currentValue = opus` alongside requested Haiku/Sonnet in SDK init, assistant, and usage
  signals. **Why it matters:** design, implementation, review, and an adapter-local readback can all
  agree and remain wrong about the external provider; following the false Claude finding would add a
  redundant delivery path and change launch failure/order behavior. **Requirement:** `INV §11`,
  `INV §12`, `INV §17`. **Suggested fix/test:** require provider-behavior statements to cite a
  complete reachability trace or a credentialed prompt receipt; label ACP `currentValue` as adapter
  configuration evidence, not provider execution evidence; and add an independently derived contract
  oracle that rejects out-of-schema standard fields instead of mirroring `sessionNewParams`.
- **Worth fixing** — equivalent OpenCode/OpenHands fields remain unverified (**undetermined**).
  **Where:** neither CLI is installed. OpenHands model delivery has a separate `LLM_MODEL` env path,
  so it does not depend on the suspect ACP `model` member, but both adapters still receive an
  out-of-schema top-level `systemPrompt`; OpenCode also still depends on the top-level `model`.
  **Why it matters:** the same silent-ignore class may be live on surfaces explicitly advertised by
  AgentDeck. **Requirement:** FS-09.A6, `INV §12`. **Suggested fix/test:** keep the claims gated until
  each pinned CLI is installed and its model/prompt delivery is checked at the effective provider;
  remove any redundant unsupported top-level fields once their real mechanism is known.

## Design consistency notes

- The paused direct-action change cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 cite
  `TS-04.R32–R39` and omit R40, the direct-action redaction clause. Align them when that change
  resumes.
- FS-17 §6's opening sentence should be scoped when its planned direct-cutover work resumes; it
  currently reads as covering a section that also contains planned R13–R19 boundaries.
