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
  Must-fix finding; every other unit through this release is closed. Review records, finding-fix
  commits, release records, and handoff/archive bookkeeping are administrative closure.
- **Work units:** None waiting to start. `migrate-internal-actions-from-mcp.md` stays paused on its
  transport blocker; the ACP wait-list in `docs/ideas.md` records the other capabilities held behind
  an adapter contract.
- **Design units:** Entries under `Ideas being defined` may resume, and entries under `New ideas` are
  available to start. Streaming agent thinking stays part-decided (live-only decided; rendering
  default and whether `plan` ships with it still open). The permanently unaddressable pipeline agent
  is the newest `New ideas` entry and needs `/design-feature` before code.
- **Open findings:** One Must-fix on the queue/steer unit — the adapter-started steering turn escapes
  AgentDeck's turn lifecycle — blocked on the compatibility choice below. Also open: J2 incompatible
  CLI status, J5 clipped lower-row card menus, live-gate finding durability, provider-contract
  oracles, the Codex discovery-versus-execution version authority left by BR-2, and the unverified
  OpenCode/OpenHands paths. See **Review findings**.
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

**Release state:** The annotated tag `v0.4.3` exists locally on the release commit. Whether `main`
and the tag were pushed — and therefore whether release CI ran and the archive, checksum, and
manifest are attached to the GitHub Release — is recorded by the release session's final update; do
not assume publication without checking `git log origin/main..main` and the CI run. The credentialed
Claude and Codex journeys under **Acceptance gates** are owed regardless and this release did not
run them.

**Available by role:** `/review` has no unreviewed unit; `/work` has no unit waiting to start; `/fix`
may select any one open finding unit, including `queue-a-follow-up-while-busy`; `/design-feature` may
choose an available or resumable idea. Role queues are independent.

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

- **Steering fallback compatibility:** choose whether AgentDeck should temporarily hide Steer for
  Codex until its adapter offers a host-owned idle fallback, or whether this fix should include an
  upstream/local adapter contract change. Keeping `startedNewTurn` is unsafe because it starts work
  whose completion the host cannot own. Claude already supports the required `promptRequired` mode;
  `codex-acp` 1.10.0, currently the latest published version, does not.

## Review findings

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

- **Must fix** — J2: incompatible CLI status is presented as a credential failure.
  **Where:** `internal/backend/credcheck/claude.go:25-45`, surfaced by
  `ui/src/features/onboarding/steps/BackendStep.tsx:44-46`. **Normal-use trigger:** from a fresh
  onboarding home, an installed `claude-agent-acp` that prints `error: unknown option --cli` and
  exits 2. **Why it matters:** the wizard tells the operator to repair credentials when the
  provider is actually incompatible or un-interrogable, leaving the wrong setup gate and no useful
  compatibility diagnosis. **Requirement:** `FS-04.A14`, `INV §12`. **Suggested fix/test:** classify
  unsupported CLI/status failures separately from credential failures, keep setup retryable, and
  add a J2 fixture test for an unknown option. Reproduced in
  `.review/usability-20260907/run/shots/J2-old-cli-spot-replay.png`.
- **Must fix** — J5: lower-row card context menus hide lifecycle actions below the viewport.
  **Where:** dashboard card context menu at the default 1280×720 viewport. **Normal-use trigger:**
  right-click a lower-row stopped card in a three-column grid. **Why it matters:** the fixed menu
  starts at y=640, placing Resume at y=754 and Archive at y=907 with no clipping correction or menu
  scroll; lifecycle actions become a dead-end until the operator finds a pointer-position workaround.
  **Requirement:** J5, `FS-12.A8`, `INV §8`. **Suggested fix/test:** clamp or flip the menu into the viewport
  and exercise lower-row menus across menu heights and the supported desktop floor. Reproduced in
  `.review/usability-20260907/run/shots/J5-context-menu-clipped.png`.
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
