# FS-14 — Configurable pipeline runs

**Status:** Current
**Code:** `internal/pipeline`, `internal/config`, `internal/state`, `internal/server`, `internal/messaging`, `internal/cli`, `ui/src/features/pipelines` · **Journeys:** J14
**Absorbed:** —

## 1. Purpose

AgentDeck already launches independently configured agents, groups them visually, and lets chat
agents coordinate through messages. It does not know that one agent's work is input to another
agent's review, whether a stage succeeded, or which stage may run next. Configurable pipeline runs
add that missing product-level coordination: a person defines reusable stages and their agent
configuration, starts a run with a concrete goal, and can leave AgentDeck to advance the run through
explicit, durable stage results while retaining supervision and recovery controls.

This feature owns pipeline templates, run and stage behavior, agent-visible stage assignments and
results, and the pipeline supervision surface. FS-01 continues to own each launched agent's
lifecycle, FS-02 owns ordinary cards and groups, FS-04 owns roles/backends/projects, and FS-06 owns
free-form agent messaging.

## 2. Behavior

Every requirement here reflects shipped behavior; the product boundaries deliberately outside it are
recorded in §6.

### 2.1 Templates and starting a run

- **R1.** A user can create, read, edit, and delete a reusable **pipeline template** with
  an immutable id, display title, ordered stage definitions, and outcome transitions. Each stage has
  a unique id, display title, role, task instruction, named input bindings, and named outputs, but no
  backend or model. Templates are separate from task groups: a group remains a visual label and
  never becomes the pipeline's authority.
- **R2.** Starting a run assigns a configured backend and model to every stage, so the
  same unchanged template may use Codex for implementation and Claude for review on one run and the
  reverse on another. Pipeline stages use the chat interface; terminal stages are rejected because
  terminal agents cannot use the structured coordination tools required to report a stage result.
- **R31.** Run setup also assigns an optional effort to each stage, alongside that
  stage's backend and model and under the same rules (FS-09.R35/R41/R42): the control appears only
  for a stage whose assigned model declares effort levels, and a level that model does not declare
  prevents the run from starting rather than substituting another. Effort stays out of the template
  for the same reason backend and model do — it is a run-time assignment, not stage semantics — and
  each stage's resolved effort is frozen into the run snapshot and shown in run supervision beside
  its effective backend and model.
- **R32.** Archiving a project stops every active pipeline run for that project before
  its agents are archived. The stopped run remains durable but cannot Continue, Retry, or launch a
  replacement stage agent while the project is archived; restoring the project does not restart a
  run automatically.
- **R3.** Before starting a template, the user may set a run display name and must set a
  project, run goal, every required named run input, and the per-stage backend/model assignments.
  Those run values and runtime assignments are the complete run-specific editing surface: stage
  names/order, roles, instructions, input/output definitions and bindings, outcome transitions,
  approval gates, and loop limits remain exactly as defined by the selected template. The resulting
  run snapshots the template, display name, goal, project, inputs, roles, and runtime assignments;
  later template edits affect future runs only.
- **R4.** A run can be started from the dashboard and through a local CLI/API surface.
  AgentDecker may start the same native run by invoking that CLI/API after the user asks it to; the
  pipeline engine, not AgentDecker's conversational context, then owns progression.

### 2.2 Assignment and explicit completion

- **R5.** AgentDeck launches only the current stage's agent. Its initial assignment names
  the run goal, stage responsibility, declared named inputs, relevant prior-stage results, scope
  boundaries, and the explicit result and named outputs it must report. Future-stage agents are not
  launched early.
- **R6.** A running stage agent can report exactly one authoritative result for its
  current attempt with outcome `success`, `failure`, or `blocked`; a bounded summary; bounded result
  details and checks; and optional named text outputs. `failure` means the agent completed the
  stage's evaluation but its success condition was not met; `blocked` means it cannot complete
  without new input or an external change. Launch errors, crashes, and other infrastructure problems
  are system-owned attempt failures, not agent-reported outcomes. The caller identity and assigned
  run/stage are server-derived, so an agent cannot report for another agent or stage.
- **R7.** Runtime status alone never completes a stage: `idle`, `done`, or process exit
  does not mean the work passed. AgentDeck may advance only after accepting the explicit result and
  observing the reporting turn reach an idle/quiescent boundary. Duplicate or stale reports do not
  trigger a second transition.
- **R8.** An accepted result is durable and visible before the next stage starts. The
  next assignment receives its declared named text inputs and structured prior results rather than
  a copied transcript; all stage-agent transcripts remain available through the ordinary AgentDeck
  archive. After a `success` or `failure` result reaches the idle boundary, AgentDeck stops the stage
  agent before
  advancing or waiting at an approval gate.

### 2.3 Routing, repair, and supervision

- **R9.** A template maps each `success` or `failure` outcome to another stage or a final
  run result and marks that transition automatic or approval-required. The first version supports
  one active stage at a time, forward routing, conditional skipping, and bounded backward repair
  loops; it does not execute parallel branches or joins. A template must bound every possible loop
  so an agent chain cannot continue indefinitely. A `blocked` result always pauses for human input
  rather than silently following an automatic route.
- **R10.** A typical quality template can express **Work → Review → Validate → Complete**,
  route confirmed validation failures to **Fix**, and route a completed fix back through Review and
  Validate. Fix is never launched when validation passes.
- **R11.** The Pipelines surface shows each run's goal, project, current stage and agent,
  effective backend/model, attempt/cycle, completed stage outcomes, and final state. It provides
  **Open agent**, **Continue**, **Retry stage**, and **Stop run** actions only when each action is
  valid; run-action failures remain visible and retryable. Ordinary agent cards show their run/stage
  association and link back to the run without making a group label authoritative.
- **R12.** A launch failure, current-agent crash before a result, or `blocked` result
  pauses the run without launching the next stage. A `failure` result follows the template's
  configured failure transition, pausing when that transition requires approval. The run explains
  every pause or failure and offers only valid recovery actions. Retrying creates a distinct attempt
  and cannot duplicate an already accepted transition. Continuing after `blocked` resumes the same
  stage agent with the user's new input under a new attempt; a crash, launch failure, explicit Retry,
  or later loop visit launches a fresh agent so independent work is not silently attributed to the
  earlier attempt.
- **R13.** Stopping a run cancels/stops its current agent through the ordinary lifecycle
  path and prevents future stages from launching. Agents and transcripts from completed attempts
  remain ordinary archived AgentDeck sessions. Completed and stopped run summaries and stage
  results have no automatic expiry and remain until the user explicitly deletes the run record;
  deleting a run or template never silently deletes its agents or transcripts.
- **R14.** Dashboard restart restores every non-finished run to its last durable stage,
  result, and recovery state. Recovery never guesses that an interrupted stage passed and never
  launches two agents for one attempt.

### 2.4 Permissions and existing product boundaries

- **R15.** A pipeline stage uses the selected role's ordinary permission policy and the
  selected backend's ordinary credential/capability checks. A pipeline never silently enables
  permission skipping, changes credentials, substitutes a model, or treats a permission prompt as a
  failed/completed stage.
- **R16.** Agents from a run remain normal AgentDeck agents: they have stable agent ids,
  cards, transcripts, archive entries, notifications, and messaging. The run adds an immutable run
  and stage association but does not replace those existing surfaces or use display names/group
  labels as identity.

- **R34** — A pipeline run that reaches a terminal state registers its outcome in the
  shared result layer so other work can depend on it: `success` when it completes with the final
  outcome `success`, `failure` for any other template-defined final outcome, and `cancelled` when it
  was stopped, retaining the raw label for display (FS-16.R13). Template authoring, routing, revisits,
  recovery, and the run's own vocabulary are unchanged, and this adds no cross-run join, parallel
  branch, or child pipeline. Verified by FS-16.A7.

## 3. States & transitions

- **Template:** absent → saved → edited or deleted. Editing/deleting affects no existing run because
  each run owns a start-time snapshot.
- **Run:** `queued → running → completed`; any active run may become `paused`, and a queued/running/
  paused run may become `stopped`. A paused run returns to running only through a valid Continue or
  Retry action.
- **Stage attempt:** `queued → launching → running → reported → completed`; an agent may report
  `blocked`, while launch/crash/infrastructure problems interrupt or fail the attempt independently
  of its result vocabulary. Continuing a blocked stage creates a new attempt on the same resumable
  agent; a retry or later route back to the stage creates a new attempt with a fresh agent. No path
  rewrites the old result.
- **Advance:** explicit accepted result → durable transition decision → reporting agent reaches idle
  boundary → current agent is stopped/archived as needed → next attempt launches. Restart resumes
  from the last durable point in that sequence.

## 4. Edge cases & errors

- **R17.** Template validation rejects missing/duplicate stage ids, unknown roles,
  empty instructions, invalid or unresolved input/output bindings, missing outcome routes, unbounded
  cycles, and routes to absent stages. Collection fields return arrays, not `null`.
- **R18.** Starting a run revalidates its effective stage selections. A template whose
  referenced role was removed remains editable but cannot start until the template is repaired; an
  unknown project/backend/model or missing required run input likewise prevents every agent launch.
- **R19.** A stage report with an unknown/stopped caller, caller not assigned to the
  current attempt, invalid outcome, oversized field, or already accepted result fails without
  changing run state or launching another agent.
- **R20.** Continue never converts a missing result into success. It resumes only a
  paused approval transition that already has enough durable information to proceed, or accepts new
  user input for a blocked stage and opens a new attempt on that stage's resumable agent. Retry
  reruns the current stage with a fresh agent, and Stop is the escape hatch when neither is
  appropriate.
- **R21.** Concurrent runs retain AgentDeck's existing shared-project-directory
  behavior and are allowed. A run serializes its own stages but does not claim filesystem isolation
  from direct agents or other runs; the start surface visibly warns when another active run or agent
  shares the project, without presenting that warning as an isolation guarantee.

### 4.1 Named arguments and artifacts

- **R22.** A template declares named required or optional run inputs, and each stage
  declares which named values it consumes and which named values it may produce. Input and output
  declarations include bounded human-readable descriptions so the run form and stage assignment can
  explain what each value represents. Every value is bounded opaque text: it may contain a
  specification, a repository-relative filename, or another reference meaningful to the agent, but
  AgentDeck has no separate path type and does not inspect, copy, or verify referenced file contents.
- **R23.** The Start Run form and API collect all required run inputs before launch.
  Stage output values join the run's durable named-value set and may satisfy later stage inputs; the
  run detail shows each value's name, producing run input or stage attempt, and current text value.
- **R24.** Before accepting a stage result whose selected transition starts another
  stage, AgentDeck verifies that every required input binding of that destination resolves to a
  non-empty named text value. Missing values are returned to the reporting agent/user by name and
  do not complete the attempt or launch the destination. `failure` and `blocked` results may still
  carry useful partial outputs.

### 4.2 Pipelines page and AgentDecker-assisted creation

- **R25.** A dedicated **Pipelines** page lists reusable templates and active/completed/
  stopped runs; provides a simple vertical stage-list template editor, Start Run form with per-stage
  runtime selectors, and run detail/history; and offers **Create manually** and **Create with
  AgentDecker**. It does not present a graph canvas. Templates are also hand-editable as versioned
  AgentDeck JSON through the same validation contract.
- **R26.** Create with AgentDecker first lets the user choose one configured active project,
  backend, and model for the template-building AgentDecker session. The project picker lists every
  configured active project and defaults to the configured default project only when that project
  still exists and is active; the builder cannot be launched until a listed project is selected, so
  a stale, removed, or archived default is visible before launch rather than only as a rejected
  launch. The picker shows configured readiness honestly; effort remains a per-run stage assignment
  rather than a template or builder setting. This creator choice is not written into the
  resulting model-neutral template.
- **R27.** The AgentDecker builder accepts a natural-language pipeline description, asks
  clarifying questions in chat, and submits a structured draft containing stages, roles,
  instructions, named inputs/outputs, outcome routes, approval gates, and loop bounds. AgentDeck
  validates and previews the draft in the ordinary editor. AgentDecker may request **Save** for the
  exact validated draft or **Start** for an exact saved-template run configuration, but neither
  action occurs until the person approves its one-time confirmation in the Pipelines UI. Editing the
  proposed payload invalidates that approval, Save and Start require separate approvals, and chat
  wording alone is not treated as approval. Launching the builder keeps the person on the Pipelines
  approval surface rather than navigating into the builder chat, because the Save/Start controls live
  there. The builder session panel reflects the session's live state: while agent state is still
  hydrating it shows a loading line; while the builder runs it shows the session id and an **Open
  AgentDecker chat** link; and once the builder has stopped it withholds that link and states that the
  session stopped while its pending proposals remain available for approval below.
- **R28.** Runs can be started and controlled from the Pipelines page and local CLI/API.
  AgentDecker may invoke that local surface after a user's request, but v1 exposes no agent-facing
  start-run MCP tool and pipelines cannot start child pipelines.
- **R29.** Pipeline-level notifications use the existing toast/desktop/mute pipeline for
  exactly two categories: **needs attention** (blocked, approval gate, launch failure, or crash) and
  **completed** (terminal success or terminal failure). Ordinary successful stage transitions do not
  notify.
- **R30.** The AgentDecker Save/Start confirmation is an interaction guard, not a new
  security boundary. The builder integration exposes no unapproved write/start operation and never
  auto-approves its own request, while AgentDeck retains TS-05's existing same-user local-API trust
  model: a shell-capable agent is not cryptographically prevented from invoking the ordinary local
  CLI/API outside this guided flow.

- **R33.** A pending proposal disappears from the Pipelines approval surface once
  the exact Save or Start it asked for has succeeded, including after a reload, so the same approval
  cannot be performed twice or replay an older run. A failed or refused approval leaves the proposal
  pending. There is no Dismiss control: a proposal the person never approves simply ages out, because
  AgentDeck retains only the newest proposals and prunes older ones. Approving an edited payload
  creates no consumption, since the edit already invalidated that approval (R27).

## 5. Acceptance criteria

- **A1** — A person creates and edits one model-neutral four-stage template, starts it
  once with Codex Work and Claude Review and again with those runtime assignments reversed, and
  observes that neither run choice nor later template edits change the other run or template.
  *Verify:* template/run API and Pipelines UI tests.
- **A2** — A fake four-stage run launches exactly one chat agent at a time with the
  configured role/backend/model and assignment, skips Fix on validation success, and routes a
  validation failure through Fix then back to Review and Validate. *Verify:* server integration test
  with fake runtimes.
- **A3** — Idle/done without a result never advances; an authenticated current-stage
  result advances once after the turn becomes idle; duplicate, stale, and spoofed reports do not.
  *Verify:* MCP identity and pipeline transition race tests.
- **A4** — A stage-reported failure follows its configured transition; launch failure,
  crash, and blocked result pause with an honest cause; blocked Continue resumes the same agent under
  a new attempt while Retry and loop revisit use a fresh agent; Stop prevents future launch.
  *Verify:* failure-injection server tests.
- **A5** — Restart at each transition boundary restores one current attempt and never
  loses a result or launches a duplicate agent. *Verify:* persistence/restart integration matrix.
- **A6** — Pipeline launch uses normal permission and backend validation, and a missing
  referenced role or unknown run-time project/backend/model prevents any process start. *Verify:*
  composition and validation tests.
- **A7** — In a browser journey, a person starts Codex Work with Claude Review/
  Validation, follows live stage status, resolves a pause, observes a repair loop, and opens every
  stage transcript from the completed run. *Verify:* usability journey J14.
- **A8** — Concurrent same-project runs are allowed only after the start surface makes
  the shared-workspace boundary visible; deleting one completed run removes its pipeline record but
  leaves every stage agent and transcript in the ordinary archive. *Verify:* run lifecycle API/UI
  tests and J14.
- **A9** — Run-start text inputs and one stage's named outputs resolve into a later
  stage's declared inputs; an unresolved required input prevents accepting the transition; text that
  names a nonexistent path remains opaque rather than receiving a false existence guarantee.
  *Verify:* template validation and run-transition tests.
- **A10** — Create with AgentDecker offers configured project/backend/model choices for
  the creator, launches the session into the selected project rather than an assumed default, holds
  the launch closed while no listed project is selected, supports a clarifying conversation, and
  places a valid model-neutral draft into the editor. Launching keeps the person on the Pipelines
  approval surface, and the builder session panel shows the loading, running (with an **Open
  AgentDecker chat** link), and stopped (link withheld, pending proposals still listed) states. Its
  Save and Start requests show the exact payload for separate one-time approval; denial
  has no effect, payload edits invalidate approval, and an approved request executes once. *Verify:*
  fake-runtime server test, `ui/src/features/pipelines/AgentDeckerBuilder.test.tsx`, and J14.

- **A11** (R31) — Run setup offers effort only for a stage whose assigned model declares
  levels, a run started with per-stage efforts launches each stage agent at its assigned level, an
  undeclared level prevents the whole run from starting with a named field error and no process, and
  run supervision plus the frozen run snapshot report each stage's effective effort. *Verify by*
  pipeline run-start validation tests, a manager test starting a run with per-stage efforts, and the
  Pipelines run-setup/supervision UI tests.

- **A12.** Archiving a project with an active pipeline stops the run before archiving its
  stage agent; no later pipeline recovery or control starts an agent while the project is archived. —
  pipeline/project-archive integration regressions; J14.

- **A13** (R33) — An approved Save and an approved Start each stop appearing under
  pending proposals after the mutation commits and after a reload, two proposals differing only in a
  caller request id resolve to one record whose exact payload is what the approval surface holds in
  either order, and one unreadable proposal record does not hide the others. *Verify:*
  `internal/pipeline/proposals_test.go`, `internal/state/pipeline_proposals_test.go`, and
  `ui/src/features/pipelines/AgentDeckerBuilder.test.tsx`.

## 6. Deviations & open decisions

The shipped first version deliberately keeps these product boundaries:

- Stages are generic named/instructed units configured with AgentDeck roles; Work,
  Review, Validate, and Fix are template examples rather than hard-coded stage types. Templates are
  model-neutral; every run supplies and snapshots its per-stage backend/model assignments.
- A run may change values and runtime assignments, not pipeline structure or stage semantics. Before
  launch, the user may edit the run name, project, goal, declared input values, and each stage's
  backend/model; changing stage roles, instructions, bindings, transitions, gates, or loop limits
  requires editing the reusable template.
- V1 runs one stage at a time with outcome-based skips/routes and bounded backward loops; it has no
  parallel fan-out/join or arbitrary condition language.
- Agents report only `success`, `failure`, or `blocked`. Templates make success/failure transitions
  automatic or approval-required; blocked always pauses for human input.
- Continuing after blocked reuses the stage agent under a new attempt; crash recovery, explicit
  Retry, and later loop visits use a fresh agent and preserve the earlier attempt.
- Run summaries/results remain until explicit deletion, while deletion leaves the underlying stage
  agents and transcripts intact.
- Concurrent runs in one project are allowed with a visible shared-workspace warning; pipelines do
  not claim filesystem isolation.
- Arguments and stage artifacts are named opaque text only. Templates declare input/output bindings;
  there is no typed path, file copying, or content verification.
- The dedicated Pipelines page supports manual editing and a provider/model-selected AgentDecker
  builder. AgentDecker submits a validated model-neutral draft and may request Save or Start, but
  each exact action requires a separate one-time human confirmation. This is a guided-flow guard,
  not a change to AgentDeck's same-user local-API trust boundary.
- Runs start through the Pipelines page or local CLI/API, including AgentDecker invoking that surface
  after a user request; there is no start-run MCP tool or child pipeline in v1.
- Only needs-attention and completed pipeline notifications join the existing notification/mute
  surface.

Effort selection is explicitly outside this feature and is recorded at the top of `docs/ideas.md` as
a separate cross-backend launch capability.

## 7. Traceability

- Existing agent identity, launch, stop, crash, and backend/model rules: FS-01.
- Existing card/group and notification presentation: FS-02.
- Role/project/backend configuration timing: FS-04 and FS-09.
- Existing agent-to-agent messaging and token-bound identity: FS-06.
- Shared project-directory boundary: FS-00 and FS-11.
- Native pipeline state machine, persistence, tools, API, and restart contracts: TS-09.
- Primary regression anchors: `internal/pipeline/{templates,manager}_test.go`,
  `internal/state/pipelines_test.go`, `internal/messaging/pipeline_tools_test.go`,
  `internal/server/pipeline_handlers_test.go`, and `ui/src/api/{pipelines,sse}.test.ts`.
- Archive containment: `internal/pipeline/manager_test.go`
  `TestStartRejectsProjectArchiveClaimBeforeDurableMutation`; `internal/server/archive_gate_test.go`.
