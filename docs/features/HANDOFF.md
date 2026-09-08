# AgentDeck — Implementation handoff

**Live agent state.** Read the **Current position** and **Active change** below, then open the
requirements they name. Settled state is archived in
[`../archive/state/HANDOFF-through-2026-09-07.md`](../archive/state/HANDOFF-through-2026-09-07.md),
[`../archive/state/HANDOFF-through-2026-09-06.md`](../archive/state/HANDOFF-through-2026-09-06.md),
[`../archive/state/HANDOFF-through-2026-09-03.md`](../archive/state/HANDOFF-through-2026-09-03.md),
and [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md). Follow
[`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md); this file holds resumable current state only.

## Current position

- **Active change:** None.
- **Release:** `v0.4.2` is published and verified on tag `f56755a`. Release run `34085524105`
  succeeded in 4m5s, attached the macOS arm64 archive, `install.sh`, and `manifest.json`; main CI
  run `34085523868` also passed. The range adds the docked annotation tray, quieter self-target
  annotation transcripts, and Mermaid rendering fixes. It changes no agent-facing behavior, so the
  embedded `operating-agentdeck` package was not refreshed. The distributable binary reports
  `0.4.2` and carries `sqlite_fts5`.
- **Review units:** `chat-session-configuration` is reviewed, fixed, and closed.
  `dock-the-annotation-tray-and-quiet-its-prompt` is reviewed, fixed, and closed; all
  earlier units through this release are closed. Review records, finding-fix
  commits, release records, and handoff/archive/queue bookkeeping are administrative closure.
- **Work units:** None waiting. `migrate-internal-actions-from-mcp.md` stays paused on its recorded
  transport blocker.
- **Design units:** Existing entries under `Ideas being defined` may resume, and entries under
  `New ideas` are available to start. Two entries from the 2026-09-07 agent-features request are
  part-decided and resumable: streaming agent thinking (decided live-only; rendering default and
  whether `plan` ships with it still open) and steering a running turn (open on whether steering is
  Claude-native queueing or a portable hold-until-idle). The permanently unaddressable pipeline
  agent remains the newest `New ideas` entry and needs `/design-feature` before code.
- **Open findings:** Two usability findings from the 2026-09-07 v0.4.2 review plus three Must-fix
  and one Worth-fixing postmortem findings: J2 incompatible CLI status, J5 clipped lower-row card
  menus, live-gate finding durability, provider-contract oracles, the **confirmed** Claude model
  result, and the unverified OpenCode/OpenHands paths. The six
  `chat-session-configuration` findings are closed.
- **Bug reports:** BR-1 is investigated. Codex chat silently ignored the selected model from its
  first release and later ignored effort too; its implementation is reviewed with open findings.
  The postmortem corrects the earlier claim that the bug went unnoticed and records how a live
  Must-fix finding was lost between design, implementation, and review. See **Bug investigation
  reports**.
- **State:** Automated MCP contract verification is green. A historical credentialed provider run
  on 2026-07-26 detected the BR-1 model failure; the current post-fix Claude/Codex acceptance matrix
  remains open and must not be described as verified. The 2026-09-08 fix run drove both pinned
  adapters live for the session-configuration contract only; that probe is recorded under **Bug
  investigation reports** and is not the acceptance matrix.
- **Branch:** `main`.

## Active change

**Change:** None.

**Available by role:** `/review` has no unreviewed unit; `/fix` may select the BR-1 postmortem
findings; `/work` has no waiting unit; `/design-feature` may choose an available
or resumable idea, or an idea a person names from another `docs/ideas.md` section. Role queues are
independent.

**Changelog — 2026-09-08 (fix):** Closed all six `chat-session-configuration` review findings
(INV §1, §5, §8, §11, §12, §15, §17). The ACP configuration decode now keeps each option's reported
`currentValue` and is re-read from every response that carries one, so applying a model consults the
peer's rebuilt option list rather than the pre-model one, and a setting AgentDeck required is checked
against the value the peer itself reports instead of the bare RPC envelope. The live
`session-config` route takes the shared exclusive lifecycle claim, and a combined request that fails
partway now persists what actually applied before returning the error. Runtime failures are typed
(unsupported, unavailable, rejected, ignored) and mapped to distinct route envelopes instead of one
`runtime_start_failed`. The live session's fast-mode advertisement is decoded per generation, stored
on the `running` row (migration 24), projected into agent state, and rendered by the chat header, so
a launch that asked for fast mode and did not get it now says the model does not offer it rather
than showing a dead toggle. Added regression coverage with acceptance IDs for the rebuilt-option-list
and ignored-setting paths, partial-apply persistence, lifecycle serialization, unavailable-fast
header states, and task/pipeline requested-versus-applied fast mode; the option-list regression was
confirmed to fail against the pre-fix discard. TS-02.R30, TS-03.R37, and TS-04.R46 updated. Full Go
suite, SQLite-FTS variant, targeted `-race` runs, vet, all UI tests, UI build, style/presentation
checks, and spec checks pass.

**Both pinned adapters were driven live during this fix**, which resolved one open question and
confirmed another. See **Bug investigation reports** for the recorded probes: Claude model delivery
via `_meta` is broken (a session requesting `haiku` came up on the local default `opus`), and
post-session configuration works for Claude as well as Codex. That is a finding in the BR-1
postmortem unit, not this one, and it is left open with the evidence attached.

**Changelog — 2026-09-08 (review):** Reviewed `chat-session-configuration` across its design and
implementation range. The ordered provider-setting path discards the required updated option list,
the live mutation is not serialized with lifecycle changes, a combined request can partially apply,
and the UI cannot explain an unhonored fast request. Error typing and required task/pipeline and
negative UI acceptance coverage are also incomplete. The unit stays open for fix. CLI and launch
surface propagation, requested-versus-applied persistence, migrations, archive/index projections,
terminal rejection, and task/pipeline wiring had no additional finding. The invariant sweep found
no class-6 surface because the change extends existing adapters and runtimes rather than adding one;
all other triggered classes were checked. Both Go variants, Go vet, all UI tests, the UI build, style
checks, presentation contract, and spec checks pass; these findings are gaps the current suite does
not exercise.

**Changelog — 2026-09-08:** Implemented `chat-session-configuration`. Fast mode now flows through
the model catalog, launch API and CLI, task and pipeline assignments, applied agent/session state,
archive projections, and capability-gated UI controls. Chat launches and resumes apply one ordered
model → effort → fast session-configuration sequence; Codex no longer relies on the ignored ACP
session model parameter. The chat header separates staged backend/model controls from immediate
effort/fast settings, and `POST /api/sessions/{id}/session-config` persists live changes without a
process or native-session restart. Added fake-provider sequence coverage, a live-route persistence
test, catalog/migration/UI coverage, and the header state to the visual matrix. The full Go suite,
SQLite-FTS suite, UI tests/build, and rendered desktop matrix check pass. Credentialed provider
gates remain open as recorded below.

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

Nothing. Post-fix live-provider acceptance needs human authorization because it invokes real provider
sessions and disposable local configuration homes, but the operator chose not to run it and not to
let it block any role.

## Review findings

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
  request members that the pinned ACP decoder drops. **Why it matters:** design, implementation, and
  review can all agree and remain wrong about the external system. **Requirement:** `INV §11`,
  `INV §12`, `INV §17`. **Suggested fix/test:** require provider-behavior statements to cite a
  complete reachability trace or a recorded live probe, and add an independently derived contract
  oracle that rejects out-of-schema standard fields instead of mirroring `sessionNewParams`.
- **Must fix** — Claude chat model delivery through `_meta` does not work (**confirmed live
  2026-09-08**, upgraded from "likely"). **Where:** `sessionNewParams`/`sessionLoadParams` in
  `internal/runtime/chat.go` deliver the Claude model as
  `_meta.claudeCode.options.model`, which TS-04.R47 and FS-09.R58 both call unaffected and keep
  unchanged. **Reproduction (recorded, not inferred):** driving the pinned `claude-agent-acp`
  **0.59.0** directly over stdio, a `session/new` carrying
  `_meta.claudeCode.options.model = "haiku"` returned a session whose own
  `configOptions.model.currentValue` was `opus` — the local default — reproducing the July 26
  result. The same session then accepted `session/set_config_option {configId:"model",
  value:"haiku"}` and reported `model = "haiku"` back. So this is the identical defect class as
  BR-1's Codex bug, on the other chat adapter, and it has the same fix. **Why it matters:** every
  Claude chat agent runs the user's local Claude default rather than the model chosen in AgentDeck,
  while the New Agent picker, session identity, and archive all report the chosen one.
  **Requirement:** FS-09.R7/R21/A16/A27, TS-04.R47, `INV §12`. **Suggested fix:** extend R57's
  ordered post-session step to deliver `claude-acp`'s model as a configuration option, exactly as
  R58 already does for `codex-acp`, and retire the `_meta` model field. **This changes which model
  existing Claude chat agents run, from their next launch or resume** — the same consequence R58
  accepted for Codex — so it wants an explicit decision before it ships. Note the ordering
  constraint is now live-verified: setting the Claude model to one without reasoning levels removes
  the `effort` and `fast` options outright, which the 2026-09-08 fix already handles.
- **Worth fixing** — equivalent OpenCode/OpenHands fields remain unverified (**undetermined**).
  **Where:** neither CLI is installed. OpenHands model delivery has a separate `LLM_MODEL` env path,
  so it does not depend on the suspect ACP `model` member, but both adapters still receive an
  out-of-schema top-level `systemPrompt`; OpenCode also still depends on the top-level `model`.
  **Why it matters:** the same silent-ignore class may be live on surfaces explicitly advertised by
  AgentDeck. **Requirement:** FS-09.A6, `INV §12`. **Suggested fix/test:** keep the claims gated until
  each pinned CLI is installed and its model/prompt delivery is checked at the effective provider;
  remove any redundant unsupported top-level fields once their real mechanism is known.

## Bug investigation reports

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
- A `session/new` requesting `_meta.claudeCode.options.model = "haiku"` came up with
  `model.currentValue = "opus"`, the local default. **Model delivery via `_meta` does not work**;
  this reproduces the 2026-07-26 result and is now an open Must-fix above.
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

Both adapters therefore answer `session/set_config_option` with the **rebuilt full option list**,
and `currentValue` is a usable independent oracle for "was this setting honored". The `fakeacp`
double now mirrors that shape and those failure modes.

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
is spread into SDK options, but the contradictory July live result described below prevents calling
Claude unaffected without a new provider-authoritative check.

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
top-level `systemPrompt` remain undetermined because neither pinned CLI is installed. Claude is not
closed: source inspection proves the pinned adapter forwards `_meta.claudeCode.options.model` into
its SDK query, but the July live run reported the native default in `configOptions` for every
requested model and had no provider-side receipt. Treat that as contradictory evidence requiring a
new credentialed matrix, not as proof that Claude is either broken or unaffected.

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
