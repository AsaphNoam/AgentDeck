# FS-14 — Configurable pipeline runs

**Status:** Partial
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

Every requirement here reflects shipped behavior unless it is marked `(planned)`, which the feature's
Partial status records; a `(planned)` requirement or acceptance item states behavior that is designed
but not yet available. The product boundaries deliberately outside this feature are recorded in §6.

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
- **R52** — R15's guarantee is scoped, not withdrawn. A pipeline still never
  enables permission skipping for the work a stage agent does in a person's workspace: file reads and
  edits, shell commands, network fetches, and every provider- or user-configured MCP server keep the
  selected role's ordinary permission policy, and a pipeline still never changes credentials,
  substitutes a model, or treats a permission prompt as a failed or completed stage. What R15 no
  longer implies is that a stage agent must ask a person for permission to use AgentDeck's own
  actions — reporting its stage result, reading its assignment, messaging, delegating, or sharing
  context. Those are exempt for every agent under FS-03.R40–R42, pipeline or not, so a stage that
  reports its result advances without a human click and can no longer be failed by an unanswered
  approval. This adds no pipeline-specific permission setting: a template, a run, and a stage carry
  no autonomy field, and nothing about a run changes which tools a person is asked about. *Verified
  through* FS-03.A24 and FS-03.A26.

- **R16.** Agents from a run remain normal AgentDeck agents: they have stable agent ids,
  cards, transcripts, archive entries, notifications, and messaging. The run adds an immutable run
  and stage association but does not replace those existing surfaces or use display names/group
  labels as identity.

- **R58** `(planned)` — **A run's stage agents arrive already grouped.** Every agent a
  run launches is created carrying its stage's group label — the stage title and the run's display
  name, the same pair the stage agent is already named for — as its ordinary task group. A stage's
  agents therefore land in one collapsible dashboard section with the member count, per-state
  summary, persisted collapse, and **Release group** any group has (FS-02.R18–R20) instead of
  scattering through Ungrouped, and a retried or loop-revisited stage collects its later agents in
  the same section. Nothing about the group is special: it is the same visual label a person sets by
  hand, and the run neither owns it nor defends it. A person may rename it, clear it, or move a
  member out with **Move to group**, and nothing in the run notices. R16 is unchanged rather than
  withdrawn: the immutable run/stage association a card shows and an API client reads stays the only
  authority for what belongs to a run, and the group label is never identity — clearing it removes
  an agent from a dashboard section, never from its stage. Releasing such a group stops its members
  through the ordinary lifecycle path, which is not **Stop run** (R13): the run observes its current
  stage agent stop without a result and pauses with its ordinary recovery actions (R12), and a later
  stage may still be launched from that pause.

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

- **R25 — superseded 2026-08-27 by R35–R45:** A single dedicated **Pipelines** page carried the
  template list, template editor, Start Run form, and run detail/history together on one scrolling
  screen. Combining authoring with supervision made both jobs hard to read and left a live run's
  position visible only as a stage id. R35–R45 split the surface and replace it; the no-graph-canvas
  boundary is carried forward by R35 and hand-editable versioned template JSON by R41.
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
  AgentDeck retains only the newest proposals and prunes older ones — that clause alone is
  superseded by R49 (planned), which adds an explicit decline; every other obligation in R33 is
  unchanged. Approving an edited payload creates no consumption, since the edit already invalidated
  that approval (R27).

### 4.3 Split pipelines surface

- **R35** — Pipelines is one shell destination with two addressable
  sub-destinations, **Runs** and **Templates**, and Runs is where the destination opens. Supervising
  a run and authoring a template never share a screen. Every run and every template has its own
  addressable page that can be linked to, bookmarked, and reloaded directly. The surface still does
  not present a graph canvas (carried forward from R25).
- **R36** — The Runs sub-destination lists pipeline runs with each run's
  display name, project, state, final outcome when it has one, and the current stage's human title
  from that run's frozen template snapshot rather than only its stage id. Rows are newest first and
  load in bounded pages through an explicit **More runs** action until the retained history is
  exhausted; already loaded rows stay visible while another page or a live refresh is loading, and
  the end of the history is explicit rather than a silent first-page cutoff. It offers a
  **Start run** action that collects the whole run-start surface R3 and R31 already define —
  template, display name, project, goal, required named inputs, per-stage backend/model/effort, and
  the shared-workspace warning of R21 — in a dialog rather than a permanently mounted form. A
  successful start opens that run's own page; a rejected start keeps the entered values visible with
  its named field errors.
- **R37** — A run's page shows its goal, project, state and final outcome, the
  attention reason when one is set, its frozen setup including each stage's effective
  backend/model/effort, and its named values with the run input or stage attempt that produced each.
  It carries the **Open agent**, **Continue**, **Retry stage**, **Stop run**, and **Delete run
  record** actions under exactly the validity gating R11, R12, R13, and R20 already define, and a
  refused action stays visible and retryable.
- **R38** — A run's page presents its attempts as an **execution timeline** in
  the order they actually ran, so a bounded repair loop appears as repeated entries for the same
  stage rather than being collapsed into one. Each entry names its stage, visit number, effective
  runtime, reported outcome or attempt state, result summary, and its result details and checks. An
  attempt that produced no result says so rather than appearing complete.
- **R39** — Each timeline entry also shows compact agent cards: the stage agent
  for that attempt, plus one card for every agent assigned to a task that this stage agent created
  directly during that attempt. Delegation is followed exactly one hop; an agent a delegated agent
  creates in turn is ordinary dependent work visible on the Tasks surface and does not appear here.
  A card shows the
  agent's name, current state, and a bounded short preview. An available card opens that agent's live
  conversation when it is running or its archived transcript when it is not. A retained task whose
  agent identity or current status is unavailable still produces one honest, non-linking fallback
  card named from its task and agent id rather than disappearing or failing the run page. Cards
  perform no lifecycle action: stopping, renaming, cloning, and every other agent mutation stay on
  the surfaces FS-01 and FS-02 own.
- **R40** — When a run has advanced past, or finished after, a stage whose
  delegated agents are still running, the run's page states how many remain running. This is
  disclosure only: it does not delay a transition, change R7's advance rule, add an attention
  reason, or stop any agent.
- **R41** — The Templates sub-destination lists saved templates with each
  template's title, id, stage count, and validation state, and offers **Create manually** and
  **Create with AgentDecker** under the rules R26, R27, R30, and R33 already define. A template
  opens in its own full-width editor page carrying the stage-list editing surface R1 and R17
  define. Templates remain hand-editable as versioned AgentDeck JSON through the same validation
  contract (carried forward from R25).
- **R42** — A pending AgentDecker proposal appears on the sub-destination that
  can act on it: a `save_template` proposal on Templates and a `start_run` proposal on Runs. The
  other sub-destination shows a count of the proposals waiting on it so neither is hidden. Proposal
  approval, one-time consumption, and edit invalidation are unchanged from R27 and R33.
- **R43** — A previously shared link to the combined page's selected run opens
  that run's page instead of failing. A link naming a run or template that no longer exists explains
  that it is gone and returns the person to the corresponding list rather than showing an empty page.
- **R44** — The split is a purpose-built supervision and authoring experience,
  not the existing panels rearranged. The Pipelines heading and Runs/Templates switcher remain
  spatially stable across its routes. Runs is a compact operational ledger rather than a card grid.
  A run page gives its live state, current stage, attention, and valid actions the primary position;
  the execution timeline owns the main reading column, while frozen setup and named values are
  secondary disclosures. Timeline entries expose their stage/outcome/runtime summary at a glance,
  keep the current or attention-bearing entry open, and let completed result details, checks, and
  agent rows expand without making every historical attempt full height.

  Template editing uses the full page as a focused workspace: a compact numbered stage navigator
  remains visible beside one selected stage form, run inputs have their own disclosure, switching
  stages preserves the unsaved draft, and validation identifies the affected stage/field. Starting
  a run uses one three-step dialog — **Setup**, **Runtimes**, and **Review** — with a stable footer;
  Back/Next preserve every value, the runtime step uses compact per-stage rows, and a rejected start
  returns to and focuses the owning step/field without losing the rest of the setup. These layouts
  use the available width without horizontal page overflow at the existing desktop floor and stack
  their secondary rail before reducing the primary timeline/editor to an unreadable measure.
- **R45** — Pipelines motion communicates continuity and state, never decorates
  waiting. Sub-destination/page changes, dialog open/close, disclosures, and a newly appended
  timeline entry use brief, restrained transitions consistent with the existing core controls; the
  persistent shell, Pipelines heading, and already rendered content do not jump or remount. First
  load reserves the destination's final geometry, while background REST refetches retain the last
  successful content with a quiet updating treatment instead of replacing the page with a loading
  blank. Status/SSE refetches do not replay entrance motion. Under `prefers-reduced-motion: reduce`,
  non-essential transform/entrance motion is removed and every state change remains immediate and
  equally legible.
- **R46** — The run-start path never presents an unexplained dead end. When no
  saved template is valid to run, **Start run** is refused with a visible reason that points to
  Templates instead of opening a dialog that cannot proceed. While a start-dialog step control is
  disabled, the dialog names the value it is still waiting for beside that control and marks the
  empty required named input, so the client-side gate explains itself exactly as the server-side
  rejection of R36 does.

- **R47** — A stage agent is told where its participation ends. The assignment
  states that the one `report_pipeline_stage_result` call ends the agent's part in that assignment —
  that clause alone is superseded by R53, which restates the boundary in terms of the
  result AgentDeck accepted rather than the call the agent made, leaving every other obligation in
  R47 unchanged — that a `blocked` result pauses the run for a person, that anything said in the
  agent's chat during that pause is out of band and cannot be recorded against the run, and that the
  person's answer
  arrives as a new assignment in the same shape. The accepted result repeats the boundary at the
  moment it starts to apply, with the pause wording for a `blocked` outcome, and a refusal of a
  further report from the same attempt says that its participation has ended and that work done
  since cannot be recorded. This states R20's existing recovery boundary rather than moving it:
  Continue remains the route by which a person's answer reaches the stage. It is stated because a
  `blocked` pause leaves the stage agent live and idle beside the run's **Open agent** action, so an
  operator who answers there instead of in Continue otherwise gets work the run can never accept,
  with nothing in the agent's own instructions having warned either of them.

- **R48** — A pause that no chat can resolve does not offer a chat. A restart that
  pauses a run awaiting a result or quiescence stops its stage agent; a failed stage launch and a
  failed stage resume likewise leave no running stage agent. In all three, Continue rejects that
  state and an ordinary chat resume of that agent mints an unrelated launch generation whose report
  is refused for the life of the run — so **Retry**, with a fresh agent, is the only action that
  moves it. The run page therefore withholds **Open agent** on such a pause and says that the stage
  agent can no longer report against the run and that Retry runs the stage again, naming whether the
  agent was stopped by the restart or left unstarted by the failure. Every other pause, including a
  `blocked` pause whose stage agent stays live, keeps **Open agent** unchanged.

### 4.4 Reviewing and declining pending proposals

- **R49 (planned)** — Every pending proposal on the sub-destination that can act on it
  (R42) carries an explicit **Reject** action beside its review action. Rejecting removes the offer
  from the pending list and moves it, unchanged, into a **Declined** list on that same
  sub-destination, which appears only when it holds an entry and states when each offer was
  declined. A declined entry carries **Delete**, which removes that record permanently; a pending
  entry offers no Delete, because rejecting first is what makes the removal deliberate. Neither
  action opens a confirmation, because a proposal is only an offer: declining or deleting one
  writes no template, starts no run, changes no existing run, and stops no agent. Both actions are
  silent to the agent — they send no message, change no already-returned tool result, and add no
  agent-facing surface; the person redirects the builder in its chat as they do today. Rejecting a
  proposal that a concurrent approval already consumed does not produce a declined entry: the
  refusal is explained and the entry disappears as consumed, and deleting a record another tab
  already deleted reports it as gone rather than failing silently. R57 governs both orderings of
  that race. A refused Reject or Delete leaves
  the entry visible and its action retryable, as R37 already requires of run actions. The pending
  count each sub-destination shows for the other (R42) counts pending offers only; a declined entry
  never contributes to it.

- **R50 (planned)** — A later re-proposal outranks an earlier decline. Because a
  proposal id is derived from its content, an agent proposing content byte-identical to a declined
  or deleted record returns exactly one pending offer to the approval surface, timed by that newest
  proposal, in the same way R33's re-arm already returns one after an approved Save. Declining
  refuses one offer; it is not a standing block on that content, and AgentDeck therefore holds no
  per-content refusal list. Suppressing the re-proposal instead would let an agent receive tool
  success with nothing on any human surface, which is the discoverability defect the durable
  proposal record exists to remove.

- **R51 (planned)** — Every pending and declined proposal is collapsed by default.
  Collapsed, it states whether it asks to save a template or start a run, the template's title and
  stage count for a save proposal or the template title, run display name, and run goal for a start
  proposal, and how long it has been pending — or, for a declined record, when it was declined.
  A start proposal's durable payload names its template by id alone, so its summary resolves the
  title from that template as it stands now: a renamed template summarizes under its current name,
  and a proposal whose template has since been deleted summarizes with the template id and says the
  template is gone rather than dropping the line or hiding the offer. The payload a person reviews on
  expansion is unchanged either way. Those values are bounded so that no single draft can
  push the template library or run ledger off the surface. Expanding one proposal reveals its exact
  canonical payload unchanged, which remains the payload a person reviews before approving; the
  summary is a way to find the right offer, never a substitute for the exact payload an approval
  acts on. Expansion is per proposal and browser-local, and it resets on reload rather than
  persisting. A proposal whose payload cannot be summarized still lists honestly with its kind and
  proposal id instead of disappearing or failing the surface.

- **R57 (planned)** — An approval that commits always beats a Reject; Reject
  withdraws the offer, never the mutation. Declining is a claim on the offer, not on its content
  (R50), so a save or a start a person deliberately confirms takes effect even when another tab
  rejected the same offer moments earlier. That record then leaves both the pending and the declined
  list as consumed rather than staying declined, because the thing it offered exists now, and the
  Pipelines surface says so instead of showing a decline the product did not honour. The reverse
  ordering is R49's already-stated case, and both are the same rule read from either side: the
  durable mutation decides, and the proposal record only ever reports what became of the offer.
  This is deliberate rather than incidental. An approval's effect is external — a template file
  written, or a run started with agents launched — and no later status update can withdraw it
  safely, so a design in which Reject blocked a committed approval could only ever pretend to. It
  also keeps one meaning for a person's explicit save.
  Each of Reject, Delete, and approval claims the record with one conditional durable write on the
  state it expects (INV §5), so two tabs racing on the same offer produce exactly one effect and the
  loser is told what actually happened — consumed, already declined, or already gone — rather than
  failing silently or reporting a second success. Approval's claim is the only transition permitted
  to overwrite a decline. Because consumption follows its mutation and can never undo it (TS-09.R26),
  a crash between a committed mutation and its consumption mark leaves one offer still listed for
  content that already exists; that is the accepted failure mode, since the next identical proposal
  re-arms the same record (R50) and a person can reject the leftover offer. Nothing repairs it by
  guessing, and no recovery pass invents a decline or a consumption that no action wrote.
  A stale action — one sent from a view older than the record's current state — is refused with what
  happened rather than applied, and the surface refreshes to the durable state instead of retrying
  blind.

### 4.5 Running a stage without a person in the loop

- **R53** — The boundary is the result AgentDeck accepted, not the call the
  agent made. The assignment states that the agent's part in an assignment ends when AgentDeck
  **accepts** a result; that a refused call records nothing, leaves the attempt still owing a
  result, and leaves that agent as the only one who can supply it; and that a refusal AgentDeck
  classes as retryable is an instruction to correct the call and send it again rather than a signal
  to stop. Every refusal of `report_pipeline_stage_result` states which of the two it is: either the
  attempt still owes a result and this agent must send one, naming what to change, or its
  participation has already ended and work done since cannot be recorded — R47's existing terminal
  case, whose wording is unchanged. R47's `blocked` pause wording is likewise unchanged. This
  matters because a refusal FS-17.R1–R3 already class as retryable, `validation_failed` most
  reachably, otherwise meets an instruction that tells the agent its part is over: the agent stops,
  the attempt still owes a result, and only a person noticing can restart it.

- **R54** — A run waiting on an undecided permission request says so. When the
  current attempt's stage agent is holding an unanswered approval (FS-03.R43), the run carries an
  attention reason naming that wait, and it joins R29's needs-attention notification category
  alongside blocked, approval gate, launch failure, and crash. The reason clears when the request is
  approved, denied, or otherwise resolved, and the run returns to its ordinary presentation. This
  covers only the state AgentDeck can prove, because it is holding the request itself; it is not a
  general detector of a stage agent that has gone quiet for some other reason, which remains an
  unresolved product decision recorded in the handoff. Without this, removing the approval deadline
  would trade a stage that fails in three minutes for a run that waits in silence indefinitely.

- **R55** — A pause states what each action does before a person picks one.
  While **Continue** is disabled it names the continuation input it is waiting for, beside that
  control, exactly as R46 already requires of the start dialog. Wherever **Continue** and **Retry
  stage** are both offered, each states its consequence: Continue delivers the person's answer to
  the same stage agent as a new assignment, and Retry starts a fresh agent that carries only the
  bounded prior-attempt summaries R12 and R20 define. That statement appears at every pause offering
  the choice, not only at the recovered pause R48 added, because the ordinary `blocked` pause is the
  one a person meets most.

- **R56** — A stage's declared outputs reach a surface. Each execution-timeline
  attempt (R38) renders the outputs that attempt declared, beside its summary, details, and checks,
  so an output is read where the work that produced it is read. A finished run presents its named
  values expanded rather than collapsed. Output text is bounded on the surface exactly as the other
  reported fields are, and a long output remains inspectable rather than being silently truncated
  away. This adds no new field to the run payload: `report_outputs` already ships and is currently
  drawn nowhere, so a person whose run declares a final report has no place to read it.

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

- **A14** (R35/R36/R41) — Opening Pipelines lands on Runs; switching to Templates shows
  the template library with no run history and no start form on screen, and switching back shows no
  template editor. A run page and a template page each reload correctly from their own URL. *Verify:*
  `ui/src/features/pipelines` routing tests and J14.
- **A15** (R36/R37) — Start run collects template, name, project, goal, required inputs,
  and per-stage runtime in a dialog; a valid start opens the new run's page, and a start rejected
  for an undeclared effort level or a missing required input keeps the dialog open with the named
  field error and starts no process. *Verify:* Pipelines run-setup UI tests and J14.
- **A16** (R38) — A completed run whose validation failure routed through Fix and back
  through Review and Validate shows each of those visits as its own timeline entry in execution
  order, each naming its stage, visit number, runtime, outcome, and result; an interrupted attempt
  with no report is shown as unreported. *Verify:* run-detail UI test over a fake looping run and J14.
- **A17** (R39/R40) — For a stage agent that created two tasks, the attempt shows three
  cards — the stage agent and both assigned agents — a running card opens `/agent/{id}` while a
  finished one opens its archive transcript, an agent created by a delegated agent does not appear,
  and no card exposes a lifecycle action. When blocked continuation reuses one stage agent across
  two attempts, a task appears only under the attempt during which it was created. A run that
  finished while a delegated agent is still running states the distinct remaining-agent count even
  when the visible delegated cards are capped. *Verify:* run-detail UI tests over a fixture with
  delegated tasks, and J14.
- **A18** (R42) — A pending `save_template` proposal is actionable on Templates and
  counted on Runs, a pending `start_run` proposal is actionable on Runs and counted on Templates,
  and approving either still consumes it exactly once. *Verify:*
  `ui/src/features/pipelines/AgentDeckerBuilder.test.tsx` and J14.
- **A19** (R43) — A link to the combined page carrying a selected run opens that run's
  page, and a link naming a deleted run or template explains the absence and returns to the matching
  list. *Verify:* Pipelines routing tests.
- **A20** (R44) — Deterministic browser fixtures cover an empty and populated Runs ledger,
  a long looping run, and a maximum-shape 32-stage/64-declaration template. At the existing desktop
  floor, the run keeps live state/actions ahead of secondary setup and values, completed attempts
  remain compact until opened, the template editor mounts one selected stage form rather than all
  32 full forms, and the three-step start dialog retains values and keeps its navigation/actions
  available without horizontal page overflow. *Verify:* Pipelines visual-matrix fixtures and J14.
- **A21** (R45) — In a real browser, switching Runs/Templates, opening the start dialog,
  expanding an attempt, and receiving one new attempt each use one brief continuity transition;
  a background refetch leaves the rendered list/detail in place and does not replay those entrances.
  With reduced motion emulated, the same route, dialog, disclosure, and live-update sequence is
  fully usable with transform/entrance motion absent. *Verify:* component state-preservation tests,
  presentation-contract/static CSS checks, and J14 in normal and reduced-motion modes.
- **A22** (R36) — A fixture with 121 retained runs initially shows the first bounded page,
  states the total, loads the rest through **More runs** without duplicates, and reaches an explicit
  complete-history state. A run whose template was later edited or deleted still shows its frozen
  current-stage title in the ledger. *Verify:* run-list API/UI pagination tests and J14.
- **A23** (R39) — Reloading a run detail before the global live snapshot hydrates still
  renders the stage and delegated cards with bounded previews from the detail response; a delegated
  task whose agent state is missing renders one honest non-linking fallback card, while a later
  `state_update` refreshes an available card in place and updates its explicit live/archive route.
  *Verify:* run detail projection/API/UI tests and J14.
- **A24** (R46) — On a home with no valid template, Runs disables **Start run** and
  offers a link to Templates. With a template that declares a required named input, filling only the
  display name and goal leaves **Next** disabled beside a message naming that input, with the empty
  input marked; filling it enables **Next** and clears both. *Verify:*
  `ui/src/features/pipelines/PipelinesPage.test.tsx` and
  `ui/src/features/pipelines/RunStartForm.test.tsx`.

- **A25** (R47) — A rendered stage assignment states that the one result call ends
  the agent's part, that chat input during a blocked pause is out of band and cannot be recorded,
  and that the person's answer arrives as a new assignment; an accepted `blocked` result repeats the
  pause wording while an accepted `success`/`failure` result does not; and a second report from the
  same attempt is refused with a message saying its participation has ended and that work done since
  cannot be recorded. *Verify:* `internal/pipeline/blocked_chat_answer_test.go` and
  `internal/messaging/pipeline_tools_test.go`.

- **A26** (R48) — A run paused by restart recovery, by a failed stage launch, or by a
  failed stage resume shows no **Open agent** action, keeps **Retry stage**, and states that the
  stage agent can no longer report against the run; a run paused as `blocked` still links to its
  live stage agent. *Verify:* `ui/src/features/pipelines/RunBrowser.test.tsx`.

- **A27 (planned)** (R49/R50/R57) — Rejecting a pending proposal removes it from the pending
  list, lists it as declined with its decline time, and survives a reload; the pending count the
  other sub-destination shows drops by one. Deleting that declined entry removes the record, and it
  is still absent after a reload. Re-proposing byte-identical content after a reject, and again
  after a delete, each leaves exactly one pending offer rather than zero or two.
  Both orderings of the Reject-versus-approval race run for both proposal kinds, against the durable
  store rather than a mocked one: an approval that commits first leaves a later Reject with no
  declined entry, the refusal explained and the entry gone as consumed; a Reject that lands first
  does not prevent the approval, and the record ends listed as consumed in neither the pending nor
  the declined list while the saved template or started run exists exactly once. Concurrent Rejects,
  concurrent Deletes, and a Reject racing a Delete each produce one effect, with every loser told
  what happened — consumed, already declined, or already gone — and never a second success or a
  silent no-op. A mutation that commits while its consumption mark fails leaves exactly one listed
  offer for content that already exists, and the next identical proposal re-arms that same record
  rather than adding a second. A Reject or Delete the server refuses leaves the
  entry visible with its action retryable, and a stale action from an out-of-date view is refused
  with the durable state rather than applied. — `internal/state/pipeline_proposals_test.go`,
  `internal/server/pipeline_handlers_test.go`, and
  `ui/src/features/pipelines/AgentDeckerBuilder.test.tsx`.

- **A28 (planned)** (R51) — A pending 32-stage `save_template` proposal renders collapsed:
  its kind, template title, stage count, and pending age are present and its exact payload is not
  rendered until it is expanded, so the template library remains the surface's first content.
  Expanding renders the canonical payload unchanged, collapsing hides it again, and a reload returns
  every proposal to collapsed. The whole pending/declined × save/start matrix collapses the same way:
  a declined `save_template` record and a pending and a declined `start_run` record each summarize
  with their own fields, a declined record showing its decline time where a pending one shows its
  age, and a record at the largest size the limits module admits still leaves the template library
  and run ledger as the surface's first content. A `start_run` summary names its template's current
  title, follows a rename, and names the template id and says the template is gone once that template
  is deleted. A proposal whose payload cannot be summarized still lists with its
  kind and proposal id. — `ui/src/features/pipelines/AgentDeckerBuilder.test.tsx`; J14.

- **A29** (R53) — A rendered assignment states that the boundary is the
  accepted result, that a refused call records nothing and leaves the attempt still owing one, and
  that a retryable refusal means correct and call again. A `validation_failed` refusal of
  `report_pipeline_stage_result` says the attempt still owes a result and names what to change,
  leaves the run at `await_result` with the attempt untouched, and a corrected second call from the
  same agent is accepted. A report from an attempt whose participation has ended is still refused
  with R47's terminal wording. — `internal/pipeline/refused_report_retry_test.go` (the committed
  skipped reproduction, unskipped by the implementation) and
  `internal/messaging/pipeline_tools_test.go`.

- **A30** (R54) — A run whose stage agent holds an unanswered approval carries
  an attention reason naming that wait and emits the needs-attention notification; resolving the
  request clears the reason and the notification is not repeated; a run whose stage agent is busy
  with no pending request carries neither. — `internal/pipeline` manager tests,
  `internal/server/pipeline_handlers_test.go`, and
  `ui/src/features/pipelines/RunBrowser.test.tsx`.

- **A31** (R55) — At an ordinary `blocked` pause, the disabled **Continue**
  control names the continuation input it is waiting for, and both **Continue** and **Retry stage**
  state their consequence including that Retry uses a fresh agent; the recovered pause R48 defines
  still withholds Continue. — `ui/src/features/pipelines/RunBrowser.test.tsx`.

- **A32** (R56) — A completed run's timeline renders each attempt's declared
  outputs within that attempt's entry, and the run's named values are expanded without a click. An
  attempt that declared no output renders unchanged. — `ui/src/features/pipelines/RunBrowser.test.tsx`.

- **A33** `(planned)` (R58) — A fake run whose second stage is retried creates each stage agent
  with that stage's group label stored on it, putting the retried stage's two agents in one section
  and each other stage's agent in its own; those sections carry the member count, per-state summary,
  persisted collapse, and **Release group**; a member moved out with **Move to group** keeps both
  its run/stage card badge and its place on the run's page; and releasing a stage's group stops its
  members and leaves the run paused with recovery actions rather than finished. — a pipeline launch
  test asserting the composed group label plus `ui/src/components/grid/CardGrid.test.tsx`.

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
- The Pipelines destination supports manual editing and a provider/model-selected AgentDecker
  builder. AgentDecker submits a validated model-neutral draft and may request Save or Start, but
  each exact action requires a separate one-time human confirmation. This is a guided-flow guard,
  not a change to AgentDeck's same-user local-API trust boundary.
- The §4.3 split reorganizes the pipeline surface only. It changes no template, routing,
  approval, loop-bound, recovery, or run-state behavior, adds no agent-facing tool or payload, and
  adds no attention reason. Delegated agents are followed one hop from a stage agent and are shown
  for supervision, not treated as pipeline participants: they cannot report a stage result, do not
  gate a transition, and are not stopped when the run advances or ends. A stage still runs exactly
  one agent of its own at a time (R5, R9); genuine parallel stage execution remains out of scope.
- The split adds no per-surface item to FS-12, which carries no Pipelines entry today. FS-12's
  shared page-frame, route-heading, dialog, and repeated-surface rules govern the new pages as they
  govern the existing ones; R44–R45 define the Pipelines-specific hierarchy, disclosure, continuity,
  and reduced-motion behavior those shared rules do not.
- Runs start through the Pipelines page or local CLI/API, including AgentDecker invoking that surface
  after a user request; there is no start-run MCP tool or child pipeline in v1.
- Only needs-attention and completed pipeline notifications join the existing notification/mute
  surface.
- **Confirmed unattended-run boundary.** R53–R56 remove the four ways a run made a person do the
  control plane's work: an agent told to stop after a retryable refusal, a run that waits in
  silence, a pause that does not say what its two actions do, and a declared output that reaches no
  screen. They add no work-unit, checkpoint, or resumable-task concept, no partial-success carry
  across a Retry, no workspace isolation for a stage or its delegates, and no change to how a run's
  outcome is decided — those remain as §6 already records them. R54 covers only a wait AgentDeck is
  itself holding; detecting a stage agent that has gone quiet for any other reason stays an open
  product decision, as does how long a stopped agent remains unaddressable after its stage
  (FS-06.R29).

- **Confirmed stage-permission boundary.** R52 states the FS-03.R40 carve-out in pipeline terms and
  adds nothing else: no per-template, per-run, or per-stage autonomy setting, no change to what a
  stage agent's role policy means for workspace tools, and no agent-facing tool or payload change.

- **Confirmed run-grouping boundary.** R58 writes one ordinary group label at stage-agent
  creation and adds nothing else: no pipeline-owned group concept, no group kind or badge on the
  section, no automatic regrouping when a run advances or ends, no cleanup of the label when a run
  is deleted, and no dashboard section derived from the run/stage join. Grouping is per stage rather
  than per run, so a run of four stages shows four sections; a single section per run, and a section
  that reads a run's live state, remain open product decisions. Two runs that share both a display
  name and a stage title deliberately share one section, exactly as two hand-labelled groups of the
  same name do.

- **Confirmed proposal-decline boundary.** R49–R51 add a two-step decline (Reject, then Delete on
  the declined entry), a collapsed-by-default proposal summary, and nothing else. A decline is a
  human surface action only: it never reaches the proposing agent, never blocks content from being
  proposed again, and adds no per-content refusal list, no decline reason, no confirmation dialog,
  no separate retention bound for declined records, and no agent-facing tool or payload change.

## 7. Traceability

- Existing agent identity, launch, stop, crash, and backend/model rules: FS-01.
- Existing card/group and notification presentation: FS-02.
- Role/project/backend configuration timing: FS-04 and FS-09.
- Existing agent-to-agent messaging and token-bound identity: FS-06.
- Shared project-directory boundary: FS-00 and FS-11.
- Native pipeline state machine, persistence, tools, API, and restart contracts: TS-09.
- Split surface: TS-02 (targeted task-read index), TS-09 (attempt-window delegated-agent
  projection), TS-03 (run-list pagination and run-detail response), and TS-08.R7/R8 (presentation
  hooks for the new pipeline surfaces); R44–R45 are exercised through J14 and deterministic visual
  fixtures.
- Proposal review, decline, and delete (R49–R51, R57; A27–A28, all `(planned)`): TS-02.R29 owns the
  record's states, its conditional claims, and retention; TS-03.R36 owns the two routes, the
  two-collection list, and the typed refusals; TS-09.R32 owns the control-plane boundary and what
  the approval paths keep. The change is
  [`../../ready-changes/decline-pipeline-proposals.md`](../../ready-changes/decline-pipeline-proposals.md).
- Primary regression anchors: `internal/pipeline/{templates,manager}_test.go`,
  `internal/state/pipelines_test.go`, `internal/messaging/pipeline_tools_test.go`,
  `internal/server/pipeline_handlers_test.go`, and `ui/src/api/{pipelines,sse}.test.ts`.
- Archive containment: `internal/pipeline/manager_test.go`
  `TestStartRejectsProjectArchiveClaimBeforeDurableMutation`; `internal/server/archive_gate_test.go`.
