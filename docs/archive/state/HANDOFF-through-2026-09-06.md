# AgentDeck — handoff state settled at the `v0.4.1` epoch (2026-09-06)

Archived under AGENT-WORKFLOW §16.7. Everything below stood in the live handoff when `v0.4.1` was
cut and is settled history: finished changes, closed review units, closed findings, and decisions
the operator resolved. The live file is [`../../features/HANDOFF.md`](../../features/HANDOFF.md).
Earlier epochs are in [`HANDOFF-through-2026-09-03.md`](HANDOFF-through-2026-09-03.md) and
[`HANDOFF-pre-sdd.md`](HANDOFF-pre-sdd.md).

## Changelog through `v0.4.1`

- **2026-09-05 — fix: proposal Reject/Delete findings (INV §5, INV §8, INV §17):** Delete's
  conditional claim now requires `declined_at != '' AND consumed_at = ''`, so the ordinary
  Reject → approval → stale Delete order loses with the durable `consumed` refusal instead of
  erasing the record the approval consumed (INV §5); TS-02.R29 states that predicate and why the
  unconsumed clause belongs in it, and its two neighbouring claims are corrected to the
  empty-string convention the code actually uses. A storage regression declines, consumes, refuses
  the Delete, and reads the row's own marks rather than the projection that hides a consumed record;
  the server refusal table gains the consumed-Delete case and proves a second stale Delete still
  answers `consumed` rather than gone (INV §17). Reject/Delete failure text moved from the proposal
  card to the builder panel that outlives the durable refetch, and the panel stays mounted while a
  message is unread, so a consumed refusal that empties both collections still explains itself
  (INV §8). The UI cases now model the list state each refusal really produces — consumed in
  neither collection, already-declined in the declined one — and all three fail against the previous
  card-owned message. Both Go variants, the full server package, all UI tests, and the production UI
  build pass. The proposal Reject/Delete unit is closed.
- **2026-09-04 — review: collapse, reject, and delete pending pipeline proposals (INV §§2, 5,
  7–11, 13–17):** The storage shape, one-query projection, permissive per-record UI narrowing,
  collapsed placement above the template library, route wiring, bounded summaries, and
  post-commit event publication match the requirements. Two normal multi-tab paths remain wrong.
  Delete's conditional claim accepts a row that approval already consumed after Reject, so a stale
  Delete erases that consumed record instead of losing with the durable `consumed` state. Separately,
  Reject/Delete failures live inside the proposal card while their settled hook awaits a durable
  refetch; when that refetch removes or moves the card, the required refusal message disappears with
  it. The tests omit the consumed-Delete state and model a consumed Reject refusal while incorrectly
  keeping its proposal pending, so neither regression is exposed. The proposal unit remains open
  with two must-fix findings. Focused state, pipeline, server, and UI tests pass; the broader server
  package run is blocked in this sandbox by an unrelated IPv6 `httptest` listen denial. Invariant
  classes 1, 3–4, 6, and 12 have no applicable changed surface.
- **2026-09-04 — fix: task launch-specification effort findings (INV §2, INV §9, INV §11, INV
  §17):** TS-10.R23 now describes the topology that ships: selecting the concrete backend/model
  target and resolving the effort over it are two shared seams, launch composition calls the
  selection seam, resolves the backend's bound configuration source, then calls the effort seam with
  that source's override, and the two authoring paths call the one composed helper that runs both
  back to back (INV §2). TS-10 §3 states the exact `TEXT NOT NULL DEFAULT ''` shape of the effort
  columns and says why a nullable column would fail a task's read, and a new
  `PRAGMA`-driven assertion in `internal/state` pins that shape on the migrated database rather than
  on the migration text (INV §9, INV §11, INV §17). FS-16.A18 is now proven at the provider
  boundary: a task-dispatch case captures the `session/set_config_option` call the adapter actually
  receives against a bound source carrying a lower effort override, asserts the explicit task effort
  wins there, and its second case leaves the task's effort unset so the override takes force —
  making "beats the override" non-vacuous. That case fails when the provider-facing
  `runtime.LaunchSpec.Effort` is dropped, which the pre-existing persisted-projection case did not.
  Every materially touched dispatcher, HTTP, MCP, and Tasks-view test now carries the exact
  `FS-16.A18` comment TS-06.R6 requires. The task launch-specification effort unit is closed.
- **2026-09-04 — review: task launch-specification effort (INV §§2, 3, 7–11, 14–15, 17):** Runtime
  behavior is sound across HTTP, MCP, persistence, dispatch, and the Tasks view, but TS-10.R23 still
  claims launch composition calls the composed triple resolver when federation requires it to call
  the shared selection and effort seams around source resolution; TS-10 §3 does not state the
  shipped `TEXT NOT NULL DEFAULT ''` storage shape; and FS-16.A18 is not independently proven at the
  composed provider boundary or against a bound-source override, with its materially touched tests
  also lacking the exact acceptance id TS-06.R6 requires. The task-effort unit remains open with
  three worth-fixing findings. Shared construction, persisted-value handling, read/migration safety,
  UI errors, SQLite behavior, wiring, serialization, HTTP guarding, and effect ordering otherwise
  satisfy their triggered invariants; classes 1, 4–6, 12–13, and 16 have no applicable changed
  surface. Both Go variants, the tagged build, all UI tests, and the production UI build pass. The
  proposal Reject/Delete review unit remains available independently.
- **2026-09-04 — review: pipeline stages group their own agents (INV §§1, 2, 10, 11, 15, 17):** No
  findings. The stage label is composed once as the agent name, passed through the existing launch
  seam as the ordinary persisted group, and rendered by the existing dashboard grouping behavior;
  the durable run/stage association remains the only membership authority. Retry coverage proves a
  repeated stage reuses the label, the server boundary proves the label is stored, and the grid plus
  ordinary-stop coverage proves the promised group and recovery behavior. Lifecycle, shared-helper,
  wiring, boundary-meaning, effect-ordering, and independent-test invariants are satisfied; classes
  3–9, 12–14, and 16 have no applicable changed surface. The full Go test matrix, tagged build, UI
  tests, and UI production build pass. The stage-agent grouping review unit is closed; the other two
  review units remain available.
- **2026-09-04 — build: pipeline stages group their own agents (FS-14.R58, A33; TS-09.R33):** Every
  agent a run launches now arrives carrying its stage's label — the stage title and the run's
  display name — as its ordinary task group, so a stage's work reads as one collapsible dashboard
  section with the count, per-state summary, persisted collapse, and **Release group** any group
  has, and a retried or loop-revisited stage collects its later agents in the same section. The
  whole mechanism is `LaunchStage` passing the string `stageExecution` already composes for the
  agent's name as the existing `launchRequest.Group`, so the convention has one home and the name
  and label cannot drift (INV §2). No new field, column, migration, API shape, or grid code, and no
  pipeline awareness in the dashboard. FS-14.R16 is unchanged: the run/stage association stays the
  only authority for run membership, and clearing a label removes an agent from a section, never
  from its stage. With their last `(planned)` items shipped, FS-14 and TS-09 are back to
  **Current**; the ready change file is removed.
- **2026-09-04 — fix: workflow/queue repair findings (INV §10, INV §17):** `/design-feature` again
  takes an idea a person names from any `docs/ideas.md` section, including `Known things to
  improve`, and moves that recorded entry through design instead of duplicating it; automatic
  selection with no argument stays limited to `New ideas` and `Ideas being defined` (INV §10, both
  launchers and workflow §11). The launcher contract check now asserts the complete no-op rule
  ("report that and do not make an empty commit.") and the positive any-unit review selection
  instead of fragments an opposite instruction would satisfy, matching rules across line wraps, and
  `scripts/check-launcher-contract-test.sh` proves each guarded rule fails on a mutated launcher or
  workflow copy that states its opposite (INV §17). `make check-specs` runs that mutation test;
  TS-06 records both scripts. The workflow/queue-repair unit is closed.
- **2026-09-04 — review: workflow/queue repair (INV §§10, 17):** The independent role queues and
  mirrored Claude/Codex launchers are internally consistent, but the design launcher no longer
  recognizes an explicitly named entry under `Known things to improve`, and the launcher contract
  check can accept contradictory review rules. The originating unit remains open with two findings.
- **2026-09-04 — build: collapse, reject, and delete pending pipeline proposals (FS-14.R49–R51, R57,
  A27–A28; TS-02.R29; TS-03.R36; TS-09.R32):** A pending proposal now lists collapsed — kind,
  template title, stage count or run name and goal, and how long it has been pending — and expands
  to the exact canonical payload an approval acts on, so a 32-stage draft no longer pushes the
  template library off the screen. **Reject** moves an offer to a **Declined** list with its decline
  time and a **Delete**; migration 22 adds `pipeline_proposals.declined_at`, and Reject, Delete, and
  consumption are each one conditional claim whose `WHERE` names the state it expects, so a race
  produces one effect and every loser is told what actually happened (INV §5). The durable mutation
  still wins: consumption matches `consumed_at` alone and may overwrite a decline, and the approval
  paths gained no proposal id, no pre-check, and no cross-store transaction. Re-proposing identical
  content clears a decline as well as a consumption, so a re-proposal leaves exactly one pending
  offer. TS-02 is back to **Current**; the ready change file is removed.
- **2026-09-04 — build: task launch-specification effort (FS-16.R2, R27–R28, A18; FS-09.R49, A22;
  TS-10.R23, §3):** A task that launches its own agent can now name the reasoning effort that agent
  runs at, over `create_task`, `POST /api/tasks`, and the Tasks create form. Migration 21 adds
  `tasks.effort`; the dispatcher passes it as the existing `launchRequest.Effort`, so `resolveEffort`
  and `config.ValidateModelEffort` stay the only precedence and validation code. The same change
  validates a launch specification's backend, model, and effort when the task is created — resolving
  the install defaults for an omitted field — instead of letting a bad specification spend all three
  start attempts, and rejects an effort named beside an existing-agent target. Launch composition was
  refactored onto the shared `selectLaunchTarget`/`resolveTargetEffort` seam the authoring paths use,
  so selection and effort validation each exist once (INV §2). FS-16 and TS-10 are back to
  **Current**; the ready change file is removed.
- **2026-09-04 — design: pipeline stages group their own agents (FS-14.R58, TS-09.R33):** A run now
  labels every stage agent it launches with that stage's title and the run's display name as an
  ordinary task group, so a stage's work lands in one collapsible dashboard section with the
  existing count, collapse, and **Release group** behavior instead of scattering through Ungrouped.
  The whole mechanism is one composed string on the existing `launchRequest.Group` at one call site;
  a guard for reused agent ids and one for empty labels were both dropped after the code showed them
  unreachable. FS-14.R16 is unchanged: the run/stage association stays the only authority for run
  membership. FS-14 and TS-09 were already Partial; the work is queued as
  `docs/ready-changes/pipeline-run-agent-groups.md` and is not active.
- **2026-09-04 — design: task launch-specification effort (FS-16.R27–R28, FS-09.R49, TS-10.R23):**
  Specified an optional reasoning effort on a task's launch specification, offered by `create_task`,
  `POST /api/tasks`, and the Tasks create form, stored on the task row and applied as FS-09.R41's
  explicit request when the task's agent is launched. The same design validates a task's backend,
  model, and effort at creation instead of letting a bad specification spend all three start
  attempts, and rejects an effort supplied with an existing-agent target. FS-16 and TS-10 moved to
  Partial; the later implementation is now an available review unit.
- **2026-09-04 — workflow repair: terminal review units (INV §10):** Review state now follows a
  substantive change from implementation through its review findings and fixes. Review reports,
  finding-fix commits, release records, and handoff/archive/queue bookkeeping cannot re-enter the
  review queue, and an empty queue produces no state commit. Role queues are independent and may
  each hold multiple units; an invocation may choose any available unit without chronological or
  cross-queue gating. Claude and Codex launchers carry the same rules, and the launcher check
  enforces independent selection, administrative exclusions, no-op exit, fix closure, and twin
  parity. The stale raw commit ledger and settled finding were removed from this handoff.

## Decisions the operator resolved on 2026-09-05

Resolved on 2026-09-05: the refused card drag stays as shipped (FS-02.R53's in-flight pointer
signal); worktree restore after a consented checkout deletion stays as shipped (FS-19 §3, no
re-materialization); the stalled-`await_result` detection item is withdrawn, because the
2026-09-02 report's nineteen hours were operator absence rather than a missing signal — reopen it
only if a run is ever seen parked with a live, healthy, idle stage agent; and the permanently
unaddressable pipeline agent is not a decision but a defect, now queued as a new idea in
`docs/ideas.md`.
