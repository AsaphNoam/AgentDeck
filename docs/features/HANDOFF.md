# AgentDeck — Implementation handoff

**Live agent state.** Read the **Current position** and **Active change** below, then open the
requirements they name. Settled state is archived in
[`../archive/state/HANDOFF-through-2026-09-09.md`](../archive/state/HANDOFF-through-2026-09-09.md),
[`../archive/state/HANDOFF-through-2026-09-07.md`](../archive/state/HANDOFF-through-2026-09-07.md),
[`../archive/state/HANDOFF-through-2026-09-06.md`](../archive/state/HANDOFF-through-2026-09-06.md),
[`../archive/state/HANDOFF-through-2026-09-03.md`](../archive/state/HANDOFF-through-2026-09-03.md),
and [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md). Follow
[`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md); this file holds resumable current state only.

## Current position

- **Active change:** Fixing the `queue-a-follow-up-while-busy` review; four findings are closed and
  the adapter-started steering lifecycle finding is blocked on the compatibility choice below.
- **Release:** `v0.4.2` is published and verified on tag `f56755a`; its release and CI runs passed and
  the distributable reports `0.4.2` with `sqlite_fts5`. Range details are in the state archive.
- **Review units:** `queue-a-follow-up-while-busy` (Send queues, Steer injects) was explicitly
  reviewed through its implementation commit and has one Must-fix finding open.
  `chat-session-configuration` was explicitly re-reviewed through its finding-fix commit; one
  Worth-fixing protocol replacement finding is open. The BR-3 resume-replay unit,
  `dock-the-annotation-tray-and-quiet-its-prompt`, and all earlier units through this release are
  closed. Review records, finding-fix commits, release records, and handoff/archive/queue
  bookkeeping are administrative closure.
- **Work units:** None waiting to start. `migrate-internal-actions-from-mcp.md` stays paused on its
  transport blocker.
- **Design units:** Existing entries under `Ideas being defined` may resume, and entries under
  `New ideas` are available to start. One entry from the 2026-09-07 agent-features request remains
  part-decided and resumable: streaming agent thinking (decided live-only; rendering default and
  whether `plan` ships with it still open). The permanently unaddressable pipeline
  agent remains the newest `New ideas` entry and needs `/design-feature` before code.
- **Open findings:** The queue/steer review still has the untracked adapter-started-turn finding.
  Its reload/data-loss, repeated-text, optimistic-rendering, and turn-order findings are fixed. Two usability findings from the
  2026-09-07 v0.4.2 review plus the open bug/session-configuration findings also remain: J2
  incompatible CLI status, J5 clipped lower-row card menus, live-gate finding durability,
  provider-contract oracles, explicit-empty ACP option-list replacement, and the unverified
  OpenCode/OpenHands paths. The six original `chat-session-configuration` findings are closed. The
  claimed Claude model-delivery finding was retracted after a provider-authoritative prompt probe
  disproved it.
- **Bug reports:** BR-1 through BR-3 are investigated. BR-1: Codex chat silently ignored the selected
  model from its first release and later ignored effort too; its implementation is reviewed with
  open findings. BR-2: v0.4.2 can import GPT-6-Astra from a newer personal Codex cache while its
  packaged adapter runs Codex 0.144.4, so the selectable model fails at prompt time; the immediate
  release pin is fixed and the adapter/direct-CLI dependency contradiction is closed by the adapter
  bump. The broader personal-cache versus packaged-runtime compatibility finding remains open.
  BR-3 is fixed and closed: resume held no gate over ACP `session/load`, so provider-replayed
  history was published as fresh live events and an open transcript scrolled through old work. The
  runtime now suppresses replay for the duration of that call (TS-04.R50). The report named no
  backend or version, so the field match stays probable rather than reproduced.
  Current pinned Claude model delivery through `_meta` works; its ACP model `currentValue` can be
  stale and is not an execution-model oracle.
  The postmortem corrects the earlier claim that the bug went unnoticed and records how a live
  Must-fix finding was lost between design, implementation, and review. See **Bug investigation
  reports**.
- **State:** Automated MCP contract verification is green. A historical credentialed provider run
  on 2026-07-26 detected the BR-1 model failure; the full post-fix Claude/Codex acceptance matrix
  remains open and must not be described as verified. A 2026-09-09 credentialed Claude prompt probe
  did verify current effective-model delivery for Haiku and Sonnet. The 2026-09-08 fix run drove
  both pinned adapters live for the session-configuration contract only; neither limited probe is
  the full acceptance matrix.
- **Branch:** `main`.

## Active change

**Change:** `queue-a-follow-up-while-busy` finding fix, paused on the steering compatibility choice.

**Available by role:** `/review` has no unreviewed unit; `/work` has no unit waiting to start; `/fix`
may select any one open finding unit, including `queue-a-follow-up-while-busy`; `/design-feature` may
choose an available or resumable idea. Role queues are independent.

**Changelog — 2026-09-10 (fix):** Closed four queue/steer findings (FS-03.R48/R49/A31/A32,
TS-01.R29, TS-03.R38, TS-08.R56; INV §1, §2, §4, §5, §8, §11, §15, §17). The runtime now exposes a
read-only live hold snapshot with an acceptance-sequence boundary, so browser remount restores the
pending message and only its later durable delivery clears it. Composer echoes now follow the
server outcome, and turn completion transfers the gate directly to an existing held successor.
The remaining steering lifecycle finding is blocked because Claude supports a host-owned idle
fallback while the current/latest Codex adapter does not.

**Changelog — 2026-09-10 (review):** Reviewed `queue-a-follow-up-while-busy` through `ae3740e` and
kept the unit open with three Must-fix and two Worth-fixing findings. The bundled adapters confirm
that `startedNewTurn` starts detached work whose completion is not represented by the steering RPC,
while AgentDeck records no corresponding turn gate or completion owner; the fake peer only returns
the outcome and therefore cannot prove the acceptance contract. The client-only hold mirror also
cannot survive a browser reload, transcript-wide text matching can clear a new repeated hold from an
old event, an idle-looking composer can optimistically render a message the server actually holds,
and turn completion releases its gate before claiming the existing hold. Focused runtime and UI
tests pass; focused server tests pass when loopback listeners are permitted. The invariant-index
sweep found applicable surfaces in §1–§5 and §8–§17, no new interface/runtime checklist surface in
§6, and no iterative read/repair surface in §7; no other invariant finding was found.

**Changelog — 2026-09-10 (work):** Finished `queue-a-follow-up-while-busy` (FS-03.R48–R50,
TS-01.R29, TS-02.R31, TS-03.R38/R39, TS-04.R48/R49, TS-08.R56; INV §1, §2, §4, §5, §8, §12, §16).
Send to a busy chat agent holds one message as live runtime state instead of returning `409`, and it
is delivered from the single place both the prompt turn and the activation turn already end — so
cancel sends it without a second path. Holding is a separate entry point, `SendPromptOrHold`, so the
dispatcher, pipeline, and annotation callers keep `SendPrompt`'s exact fail-closed contract. `DELETE`
on the prompt path withdraws. Steer is `POST /api/sessions/{id}/steer` over the adapters'
`_session/steering` extension, gated only on the handshake advertisement, decoded into the new
ephemeral `running.steering_available` (migration 25, defaulting closed) and projected onto the agent
payload; an empty steer promotes the held message in one server-side take-and-deliver and restores it
on refusal. The client mirror is an in-memory store rendered as a transcript-tail affordance, never
merged into the folded event list. Two notes for review rather than questions: an adapter answering
`startedNewTurn` runs a turn AgentDeck cannot see end, so status could stay `busy` (the window is the
sub-millisecond gap between our gate clearing and the adapter settling, and TS-04.R49 forbids our own
new-prompt path); and the memory-only mirror TS-08.R56 requires means a browser reload loses the
pending affordance and its withdraw control while the server still holds and will still deliver the
message. Both Go variants, a focused `-race` run on the hold/steer paths, spec checks, the UI suite,
and the distributable rebuild pass. No credentialed provider journey was authorized or run, so real
Claude and Codex steering remains an open acceptance gate and is not claimed verified.

Settled 2026-09-09 and 2026-09-10 adapter-bump entries moved to
[`../archive/state/HANDOFF-through-2026-09-09.md`](../archive/state/HANDOFF-through-2026-09-09.md)
for header budget.

Credentialed provider journeys and the real-browser checks below remain open acceptance gates, not
blockers. Never report them as verified without running them.

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
- **Worth fixing** — an explicit empty rebuilt ACP option list does not replace the previous list.
  **Where:** `internal/runtime/chat.go:1674-1676` replaces the cached advertisement only when
  `len(rebuilt) > 0`, although ACP's required `SetSessionConfigOptionResponse.configOptions` is a
  full array and `[]` is a valid full set. **Normal-use trigger:** an adapter accepts a setting and
  responds that the session now offers no configuration options. **Why it matters:** AgentDeck keeps
  advertising removed options and can send a later setting the peer no longer accepts; it also
  stores stale fast availability in the running row and header. **Requirement:** TS-04.R46,
  `INV §1`, `INV §11`. **Suggested fix/test:** decode presence separately from contents and replace
  on every present array, including `[]`; add a sequence test whose first set response empties the
  list and whose next requested option must be unavailable.
- **Worth fixing** — equivalent OpenCode/OpenHands fields remain unverified (**undetermined**).
  **Where:** neither CLI is installed. OpenHands model delivery has a separate `LLM_MODEL` env path,
  so it does not depend on the suspect ACP `model` member, but both adapters still receive an
  out-of-schema top-level `systemPrompt`; OpenCode also still depends on the top-level `model`.
  **Why it matters:** the same silent-ignore class may be live on surfaces explicitly advertised by
  AgentDeck. **Requirement:** FS-09.A6, `INV §12`. **Suggested fix/test:** keep the claims gated until
  each pinned CLI is installed and its model/prompt delivery is checked at the effective provider;
  remove any redundant unsupported top-level fields once their real mechanism is known.

## Bug investigation reports

### BR-2 — GPT-6-Astra is selectable but the packaged Codex is too old

**Report (verbatim).** “AgentDeck 0.4.2 lets me select gpt-6-astra, but sending a chat fails with
‘Model metadata not found’ followed by HTTP 400: ‘The model requires a newer version of Codex.’ Its
bundled codex-acp adapter launches Codex 0.144.4, despite 0.153.4 being installed locally. Astra
support requires 0.153.1+. The adapter already supports CODEX_PATH, so exposing that override could
provide a local workaround.” No separate log file was supplied. The reported environment is
AgentDeck 0.4.2 with a local Codex 0.153.4 installation.

**Verdict.** This is a **confirmed code defect** in the v0.4.2 release pin and a **confirmed spec
gap** in the relationship between model discovery and execution. The exact provider turn was not
re-run because that would consume a credentialed request, but the incompatible executable path is
proven from the tagged release inputs and the adapter's installed implementation. The reporter's
minimum-version statement is corroborated by the official Codex 0.153.1 release, which added
configurable GPT-6-Astra support; the official 0.153.4 release then made Astra visible in the bundled
model picker.

**Trace.** Model autosync reads the personal `${CODEX_HOME:-~/.codex}/models_cache.json` and imports
every visible slug, so a cache written by Codex 0.153.4 can add `gpt-6-astra` to AgentDeck. Launch
then resolves `codex-acp` from the private release runtime. With no `CODEX_PATH`, `codex-acp` 1.1.2
does not search for the user's newer Codex; it resolves and spawns its bundled `@openai/codex`
dependency. v0.4.2's package manifest, lockfile, assembly constant, and release manifest all pin that
dependency to 0.144.4. AgentDeck applies the selected model through ACP after session creation, and
the prompt is therefore handled by the old Codex app server. The warning and HTTP 400 are consistent
with that old process lacking Astra metadata and server support. This path also explains the
otherwise surprising split: selection is sourced from the new personal cache, while execution is
sourced from the old private runtime.

**Workaround.** `codex-acp` 1.1.2 documents and implements `CODEX_PATH`; AgentDeck does not strip it,
and backend/model environment values flow into launch, resume, and switch. In 0.4.2, set
`CODEX_PATH` under **Settings → Codex → Backend env** to the absolute path of a Codex executable
whose `--version` is at least 0.153.1, then restart the affected agent. A shell-only export may not
reach a GUI-launched dashboard, so the saved backend environment is the reliable existing path.

**Evidence.** `scripts/release/package.json`, `scripts/release/package-lock.json`,
`scripts/release/assemble.sh`, `internal/release/wrapper.go`, `internal/config/codexmodels.go`,
`internal/server/launch.go`, `internal/runtime/chat.go`, and the installed
`@agentclientprotocol/codex-acp` 1.1.2 README/implementation; official Codex releases 0.153.1 and
0.153.4. No skipped reproduction test was added because a faithful prompt-time oracle requires the
packaged runtime plus real provider credentials.

### Live adapter probe — 2026-09-08 (session-configuration contract)

Recorded during the `chat-session-configuration` fix run by driving each pinned binary directly over
stdio with a JSON-RPC script. This is a **contract probe, not the credentialed acceptance matrix**;
it exercises session setup and configuration options only, never a real prompt turn.

`claude-agent-acp` **0.59.0** (note: the binary on PATH is `@agentclientprotocol/claude-agent-acp`,
not `@zed-industries/claude-code-acp` — a source reading of the wrong package will describe a
different, older protocol with no config options at all):

- `session/new` returns `configOptions` with ids `mode`, `model`, `effort`, `fast`; each is
  `type: "select"` with a **string** `currentValue` and a `value`/`name` option list. AgentDeck's
  declared claude identifiers (`model`, `effort`, `fast`) and its `on`/`off` spelling are correct.
- A `session/new` requesting `_meta.claudeCode.options.model = "haiku"` returned
  `model.currentValue = "opus"`, the local default. This proves the adapter's initial configuration
  advertisement is stale; because this setup-only probe sent no prompt, it does **not** prove which
  model Claude executes. The original stronger conclusion was retracted on 2026-09-09.
- `session/set_config_option {configId:"model", value:"haiku"}` **succeeded** and returned the
  rebuilt option list reporting `model = "haiku"`. Post-session model delivery works for Claude.
- After that model change the rebuilt list contained only `mode` and `model`: **`effort` and `fast`
  were gone**, and calling either then failed with `Unknown config option`. This is why FS-09.R57's
  ordering is load-bearing and why the option list must be re-read after every call.

`codex-acp` **1.1.2**:

- `session/new` carrying the out-of-schema top-level `model: "gpt-5-codex[high]"` came up on the
  local default `gpt-5.6-sol`, confirming BR-1 and FS-09.R58 directly.
- `session/set_config_option {configId:"reasoning_effort", value:"high"}` returned the rebuilt full
  option list with `reasoning_effort = "high"`. Setting a model value the local install does not
  offer was refused with `Invalid params`, so model application is genuinely fail-closed.

Both adapters therefore answer `session/set_config_option` with the **rebuilt full option list**.
`currentValue` is useful adapter-configuration evidence, but is not an independent provider-execution
oracle. The `fakeacp` double now mirrors that adapter-level shape and those failure modes.

### Claude provider-authoritative model probe — 2026-09-09

The review followed the setup-only probe with one real prompt for each requested model against the
current pinned `claude-agent-acp` **0.59.0** (vendored Claude Code 2.1.207). It enabled the adapter's
raw SDK messages and inspected three provider-facing signals rather than the ACP configuration
picker:

- requested `haiku`: ACP still advertised stale `model.currentValue = "opus"`, while raw SDK
  `system/init.model`, the assistant API message's `model`, and result `modelUsage` all identified
  `claude-haiku-4-5-20251001`;
- requested `sonnet`: ACP still advertised stale `model.currentValue = "opus"`, while SDK init and
  the assistant message identified `claude-sonnet-5`; result usage included `claude-sonnet-5` plus
  an auxiliary Haiku entry.

Current Claude model delivery through `_meta.claudeCode.options.model` therefore works as designed.
The ACP `currentValue` discrepancy is adapter bookkeeping, not evidence that the prompt used Opus.
This does not retroactively prove the 2026-07-26 run (Claude Code 2.1.202) used the requested model,
and it does not close the broader provider acceptance matrix.

### BR-1 — Codex chat ignored the selected model and effort for ~10 weeks (postmortem complete)

**Verdict.** The defect did not remain unnoticed for ten weeks. It entered Codex chat when that
backend reused the generic ACP request in late June, was detected by a credentialed live-provider
run on 2026-07-26, and was explicitly written as a Must-fix. The process then lost that finding while
simultaneously designing effort on top of the broken mechanism. It survived about six more weeks
after detection until the 2026-09-07 fast-mode design rediscovered it. The initial model defect
shipped in v0.1.2 through v0.4.2; the later effort defect shipped in v0.2.0 through v0.4.2.

**Defect.** ACP's `NewSessionRequest`/`LoadSessionRequest` declare exactly `cwd`,
`additionalDirectories`, `mcpServers`, `_meta` (+`sessionId`). No `model`. AgentDeck sent
`params["model"]` anyway (`internal/runtime/chat.go`, `sessionNewParams`/`sessionLoadParams`);
`codex-acp` 1.1.2 reads no model from the request and takes model + reasoning effort from its own
`threadStart`/`threadResume` response. Every Codex chat agent ran the local Codex default while New
Agent, `PUT /api/backends` validation, and the persisted session identity all reported the operator's
selection. Claude uses a different `_meta.claudeCode.options.model` path; source inspection shows it
is spread into SDK options, and the 2026-09-09 provider-authoritative prompt probe confirms that path
currently selects the requested model despite a stale ACP `currentValue`.

**Timeline and escape chain.**

1. **Origin — 2026-06-27 to 2026-07-11.** The Phase 1 technical design invented a top-level ACP
   `model`/`systemPrompt` request and called `session/new` authoritative without a schema or adapter
   citation. `775a1e6` implemented it; Codex support reused it, `981fbaf` copied it to load, and
   `c694ed0` made source precedence depend on it. The fake accepted every JSON member, so green tests
   established only that code emitted its own assumption.
2. **Missed near-neighbour — 2026-07-16.** The Codex-history session that fixed ignored
   `systemPrompt` explicitly established that the pinned `NewSessionRequest` accepts only `cwd`,
   `additionalDirectories`, `mcpServers`, and `_meta`. It removed the prompt field for Codex but did
   not audit the adjacent `model` field. The fix and review were scoped to the reported symptom.
3. **Actual detection — 2026-07-26.** A credentialed run found Codex always reporting the local
   configured `gpt-5.6-terra[high]`, including when sent another `model[effort]`, and recorded this as
   **Must fix** in `docs/archive/reviews/live-provider-acceptance-2026-07-26.md`. The owning Claude
   conversation could not update HANDOFF because another live session was editing it, and explicitly
   said the three findings still needed transfer.
4. **Contradictory design in parallel — 2026-07-26/27.** Fourteen minutes after acceptance began,
   another Claude conversation started effort design from the still-stale HANDOFF. It found the
   internal `ModelId` parser for `model[effort]` but never traced the ACP `session/new` decoder or
   request handler to `threadStart`. It converted “the adapter can parse this string internally” into
   “the existing session request delivers it,” then wrote TS-04.R18 as a verified fact. On resuming
   the next day it knew live-provider changes were mixed into the tree but did not reread HANDOFF or
   open/reconcile the acceptance report.
5. **State burial — 2026-07-27.** Catch-all commit `7d294fb` committed both the exact Must-fix report
   and the contradictory effort design, while HANDOFF said no findings were open and that both
   adapters had been verified. Because the report lived under `docs/archive/reviews`, the normal read
   order no longer surfaced it.
6. **Review escape — 2026-07-27.** The immediate Codex `/review` was explicitly asked to inspect all
   changes from the prior 12 hours. It listed every file in `7d294fb` but never opened the newly added
   acceptance report. It reviewed product code/specs, found unrelated issues, and concluded the effort
   design was correct because the spec and code agreed. The contradictory evidence in the same commit
   was therefore never reconciled.
7. **Implementation and later reviews — 2026-07-30 onward.** `8ec8c6e` implemented exactly the
   approved false spec: one helper appended `[effort]`, and tests asserted that `fakeacp` received the
   outbound string. Reviews `aafd240`, `c507763`, and `b28a96c` checked lifecycle symmetry, teardown,
   nullability, traceability, and spec conformance, but none used the already-recorded provider result
   or an independent schema oracle. Each could honestly pass its chosen oracle while the real adapter
   discarded the field.
8. **Rediscovery — 2026-09-07.** Fast-mode design traced the pinned adapter's actual session request
   handler, then drove it over stdio: `session/new` with `model:"gpt-5.4-mini[xhigh]"` returned
   `currentModelId:"gpt-5.6-luna[high]"`; post-session `model` then `reasoning_effort` produced the
   requested pair. FS-09.R58 / TS-04.R47 now use that reachable mechanism; implementation `c640b48`
   is available for review, with the post-fix credentialed matrix still open.

**Root cause.** The primary cause was an unverified external-contract assertion entering the
normative technical spec. The enabling causes were a fake that shared that assertion, no effective
configuration readback, and narrowly scoped reviews. The six-week post-detection escape was a
separate state-management failure: a real Must-fix was archived instead of made live, then a commit
and review preserved mutually exclusive conclusions without reading them together. Open live gates
were not the cause—this particular gate ran and failed.

**Other adapters.** OpenHands model selection uses `LLM_MODEL`, a separate process-environment path,
so the Codex `model`-member failure does not govern it. OpenCode model delivery and both adapters'
top-level `systemPrompt` remain undetermined because neither pinned CLI is installed. Current pinned
Claude is closed for model delivery only: source inspection proves the adapter forwards
`_meta.claudeCode.options.model` into its SDK query, and the 2026-09-09 credentialed prompt probe
observed requested Haiku/Sonnet in provider-facing model signals. The July run had only the stale ACP
configuration field, so its historical execution model remains unknown rather than contradictory.

**Evidence.** Git commits `775a1e6`, `981fbaf`, `c694ed0`, `d0c7b4a`, `7d294fb`, `9d35042`,
`8ec8c6e`, `aafd240`, `c507763`, `b28a96c`, `02daa6e`, and `c640b48`; the archived July 26 report;
Claude histories `e43bb559-ca3f-49fd-b3e0-8f7c0ac7ad4f` (live acceptance) and
`478a9918-e1c7-413a-a833-3e3c43844fa9` (effort design); Codex histories
`019fa226-b1ef-7723-8b89-d6e490f793d5` (July 27 review),
`019fb179-fab9-7c83-a1bf-dffad222e17e` (July 30 implementation), and
`019f6a88-fce2-70a1-a269-1ef96287fb5b` (July 16 prompt fix).

## Design consistency notes

- The paused direct-action change cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 cite
  `TS-04.R32–R39` and omit R40, the direct-action redaction clause. Align them when that change
  resumes.
- FS-17 §6's opening sentence should be scoped when its planned direct-cutover work resumes; it
  currently reads as covering a section that also contains planned R13–R19 boundaries.
