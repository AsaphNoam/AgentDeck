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

- **Active change:** None. `v0.4.3` closed the release epoch; the next role picks from the queues
  below.
- **Release:** `v0.4.3` is tagged on this commit and ships chat session configuration, fast mode
  across launch/tasks/pipeline stages, the held follow-up and Steer, the BR-3 resume-replay fix, and
  the Claude 0.75.1 / codex-acp 1.10.0 / Codex 0.153.4 adapter bump. Publication state is recorded in
  **Active change**. `v0.4.2` and earlier ranges are in the state archive.
- **Review units:** `queue-a-follow-up-while-busy` (Send queues, Steer injects) stays open with one
  Must-fix finding. `fix-model-recommendations` awaits independent review once committed. The
  `usability-20260907` unit is closed — its J2 and J5 Must-fixes were its only open findings. Every
  other unit through this release is closed. The remaining open findings belong to the BR-1 and BR-2
  investigations, not to a review unit awaiting closure. Review records, finding-fix commits,
  release records, and handoff/archive bookkeeping are administrative closure.
- **Work units:** `open-a-file-from-chat.md` is waiting to start with nothing unresolved.
  `migrate-internal-actions-from-mcp.md` stays paused on its transport blocker; the ACP wait-list in
  `docs/ideas.md` records the other capabilities held behind an adapter contract.
- **Design units:** Entries under `Ideas being defined` may resume, and entries under `New ideas` are
  available to start. Streaming agent thinking stays part-decided (live-only decided; rendering
  default and whether `plan` ships with it still open). The permanently unaddressable pipeline agent
  is the newest `New ideas` entry and needs `/design-feature` before code.
- **Open findings:** The duplicate-Steer investigation has one Must-fix. One
  Must-fix remains on the queue/steer unit — the adapter-started steering turn escapes AgentDeck's
  turn lifecycle. No longer blocked: the operator decided on 2026-09-10 not to hide Steer, which
  selects host ownership of the adapter-started turn; see the finding for the resulting approach and
  structural work. Also open: live-gate finding durability, provider-contract oracles, the Codex
  discovery-versus-execution version authority left by BR-2, and the unverified OpenCode/OpenHands
  paths. The `usability-20260907` J2 and J5 Must-fixes are closed and removed. See **Review findings**.
- **Bug reports:** BR-1, BR-2, and BR-3 are investigated and their reports are archived with this
  release. BR-3 is fixed and closed. BR-1's Codex model/effort defect is fixed and reviewed, with the
  durability and oracle findings still open. BR-2's release pin is fixed by the adapter bump; its
  personal-cache-versus-packaged-runtime compatibility finding remains open. Pinned Claude model
  delivery through `_meta` works; an ACP model `currentValue` can be stale and is not an
  execution-model oracle.
- **State:** Automated MCP contract verification is green. The full post-fix Claude/Codex acceptance
  matrix remains open and must not be described as verified; the 2026-09-08 and 2026-09-09 probes
  were limited contract and model-delivery checks, not that matrix. Real Claude and Codex steering is
  unexercised.
- **Branch:** `main`.

## Active change

**Change:** None. `v0.4.3` is cut.

**Changelog — 2026-09-10 (fix):** Closed the `prompt-echo-race` Must-fix
(FS-03.R6/R7/R48/A31, TS-08.R41/R56; `INV §2`, `INV §5`, `INV §17`). User-message
reconciliation now handles both arrival orders: a durable sequenced event replaces an existing
optimistic echo, while an optimistic echo is suppressed when the durable event already arrived.
The regression test was confirmed failing before the fix and now asserts the single retained event
has its durable sequence. The originating investigation unit is closed.

**Changelog — 2026-09-10 (design):** Designed **open a file an agent mentioned** and made it ready
to start: [`open-a-file-from-chat.md`](../ready-changes/open-a-file-from-chat.md). A filepath link an
agent already wrote opens a read-only viewer beside the transcript instead of navigating to a
non-route, which the SPA fallback and the router catch-all currently turn into a full reload onto the
dashboard. Planned requirements: FS-03.R51–R55/A34–A37 (link classification, the one-file viewer, the
left-docked and transcript-width forms, `?file=`/`?fileLine=` as the open-file state, and confined
reads with stated refusals), FS-05.R37/A20 (tracked-file rows and diff headings open the same
viewer), TS-03.R40 (`GET /api/sessions/{id}/file`, JSON, typed boundary refusals, not gated on a
running record), TS-05.R21 (the new file-content boundary and its policy, sharing `filesearch.go`'s
`withinRoot` containment), and TS-08.R57 (a leading track on `.transcript-wrap`'s shipped container
query, with the width cap relaxed by a `data-file-open` state attribute rather than measurement).
FS-03, FS-05, and TS-08 moved Current → Partial for the planned items; journey J3 carries the
rendered steps. Decisions recorded: only agent-authored links are upgraded (no prose path
detection), the readable root is the session working directory alone, Git-ignored files inside it are
readable, a dashboard-pane link opens the agent screen, and archived sessions read from their
recorded directory. No product code changed.

**Changelog — 2026-09-10 (workflow):** Code review and bug investigation now attach one exact fix
model recommendation to each grouped fix unit: trivial/easy uses Claude Sonnet or Codex Luna, medium
uses Codex Terra or Claude Opus, and difficult uses Codex Sol. The unit takes the level of its most
difficult open fix because one agent handles the group; the band remains independent from finding
severity and is repeated in the human update. The specification checker rejects per-item, missing,
duplicate, or mismatched recommendations; launcher contract and mutation checks protect the mirrored
role instructions. Existing findings were grouped and classified under the corrected rule.

**Changelog — 2026-09-10 (fix):** Closed the `usability-20260907` unit's two Must-fix findings
(FS-04.R34/A14, TS-04.R15, FS-12.R41/A17; `INV §8`, `INV §12`, `INV §2`, `INV §10`, `INV §17`).
J2: an installed Claude adapter that rejects the readiness argv now reports
`skipped`/`cli_incompatible` with compatibility guidance instead of a credential failure, so
onboarding no longer sends the operator to repair working credentials. J5: pointer-anchored menus
measure themselves and clamp into the viewport through one shared `useMenuPlacement` helper, with a
scroll floor for menus taller than the viewport. The helper also replaces the same unclamped
positioning in the project-card, project-background, and annotation menus — same defect class, one
helper rather than four copies (`INV §2`, `INV §10`) — and that widening is recorded here rather
than hidden in the closure. The originating unit is closed.

**Usability review — 2026-09-10:** The current-tree browser follow-up found no new usability
finding. J2 compatibility guidance and J5 lower-row menu placement passed; the full matrix and its
unrun real-provider/native-OS gates are recorded in
[`usability-review-run-2026-09-10.md`](../archive/reviews/usability-review-run-2026-09-10.md).

**Release state:** `v0.4.3` is published and verified on tag `8ad5261`. Both the release and CI runs
passed, the local distributable reports `0.4.3` with `sqlite_fts5`, and the GitHub Release carries
the darwin/arm64 archive, `install.sh`, and a manifest declaring version `0.4.3` with its SHA-256.
The release shipped with five open Must-fix findings on the operator's explicit decision; two
remain, listed under **Review findings**, and none was closed by the release itself. The credentialed Claude and
Codex journeys under **Acceptance gates** are owed and this release did not run them — real steering
in particular has never been exercised against a provider.

**Available by role:** `/review` may take `fix-model-recommendations`; `/work` may start
`open-a-file-from-chat.md`; `/fix` may select any one open finding unit — `duplicate-steer-submission`
(trivial/easy, Sonnet/Luna), `queue-a-follow-up-while-busy` (difficult,
Sol), BR-1 (difficult, Sol), or BR-2 (medium, Terra/Opus); `/design-feature` may choose an available
or resumable idea. Role queues are independent.

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

### duplicate-steer-submission — **Fix model:** trivial/easy — Claude Sonnet or Codex Luna.

- **Must fix** — Steer accepts duplicate submission while its first request is in flight
  (**confirmed by reproduction test**). **Field report (verbatim):** “The steer message feature
  makes messages appear in double while the agent is working.” AgentDeck version/commit and
  environment were not supplied; no logs were supplied. **Where:**
  `ui/src/components/chat/Composer.tsx:223-245,336-338` starts an unrestricted request on every
  click and leaves the Steer control enabled; `ui/src/api/client.ts:131-139` sends each as an
  independent `POST`; `internal/runtime/chat.go:481-492` delivers and records each accepted call.
  The skipped reproduction in `ui/src/components/chat/Composer.test.tsx` holds the first response,
  clicks Steer twice, and receives two requests where one is required. **Normal-use trigger:** a
  person double-clicks Steer, or clicks again before a slow adapter response returns. **Why it
  matters:** the adapter receives the correction twice and AgentDeck emits two distinct durable
  `user_text` events, so the duplicate is both visible and actionable rather than a display-only
  artifact. **Requirement:** `FS-03.R50/A33`, `TS-03.R39`; `INV §5`, `INV §17`. **Suggested fix/test:**
  give Steer one client-side in-flight claim, disable the control until the request settles, unskip
  the reproduction, and assert one HTTP request and one delivered transcript event. If one physical
  click still reproduces after that guard, capture Network requests and SSE sequence numbers to
  distinguish duplicate DOM submission from a provider/runtime event defect.

### queue-a-follow-up-while-busy — **Fix model:** difficult — Codex Sol.

- **Must fix** — An adapter-started steering turn escapes AgentDeck's turn lifecycle
  (**confirmed implementation and test-contract gap**). **Where:** `internal/runtime/chat.go:437-484`
  accepts `startedNewTurn`, emits only the user event, and leaves `turnActive` and status ownership
  unchanged; `internal/runtime/queue_steer_test.go:243-255` checks only the returned enum, while
  `internal/runtime/testdata/fakeacp/main.go:170-189` starts no turn. The bundled Claude 0.75.1 and
  Codex 1.10.0 sources both return `startedNewTurn` as soon as detached work starts, before it
  completes. **Normal-use trigger:** the active turn settles between a Steer click and adapter
  delivery, which is the fallback `FS-03.R50` explicitly promises. **Why it matters:** AgentDeck has
  no owner for that new turn's completion: it can show idle while the agent works, omit the waiting
  indicator, and let the next Send call ordinary `session/prompt` concurrently instead of holding
  it; Claude may native-queue it and Codex may supersede the detached turn. The current specs also
  forbid AgentDeck's own retry but do not define how the adapter-started turn rejoins the host gate.
  **Requirement:** `FS-03.R48/R50/A33`, `TS-01.R29`, `TS-04.R49`; `INV §5`, `INV §11`, `INV §12`,
  `INV §17`. **Suggested fix/test:** first specify the ownership/completion contract for
  `startedNewTurn` (or use the adapters' host-owned `promptRequired` mode and an ordinary host turn),
  then make the fake peer keep that turn active through output and completion and assert status,
  Send-holding, Cancel, and exactly one terminal event.
  **Operator decision — 2026-09-10: do not hide Steer.** Steer stays available on every adapter that
  advertises it, including Codex. That rules out the hide-for-Codex option and, with it, the
  `promptRequired`-only fix: the pinned adapters are npm-pinned published packages with no vendored
  source (`scripts/release/package.json`), so a local adapter contract change would mean forking or
  publishing `codex-acp`, and `codex-acp` 1.10.0's `startNewTurnFromSteering` returns
  `startedNewTurn` unconditionally with no `promptRequired` mode. Claude 0.75.1 gates
  `promptRequired` behind opt-in request `_meta.steering.idleBehavior`, so opting in there would
  still leave Codex on the detached path and would add a second contract rather than removing one.
  **Therefore the remaining fix is host ownership of the adapter-started turn:** claim the existing
  turn gate on `startedNewTurn` and complete it from the session's turn-end notification instead of
  an RPC result. Note the structural work this implies — `runPromptTurn` currently owns completion by
  blocking on its own `session/prompt` Call (`internal/runtime/chat.go:599-625`), and a detached turn
  has no such outstanding request, so the gate needs a notification-driven release path that still
  yields exactly one terminal event and an answerable Cancel (`INV §2`, `INV §5`, `INV §17`; needs
  focused `-race` coverage).

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
