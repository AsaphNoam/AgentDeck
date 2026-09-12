# Persistent pipeline orchestration through durable tasks

**State:** Paused
**Why:** Operator request of 2026-09-11, refined and confirmed 2026-09-12; promoted from
“Rethink the pipeline experience on top of durable tasks” in `docs/ideas.md`.
**Relevant requirements:** FS-00.R17, FS-06.R30–R36, FS-14.R61–R78 (R60 superseded), FS-16.R30–R38,
TS-09.R35–R50, TS-10.R25–R37, TS-05.R22; INV §1, INV §2, INV §3, INV §4, INV §5,
INV §7, INV §8, INV §9, INV §10, INV §11, INV §14, INV §15, INV §16, INV §17.

## Outcome

One standing orchestrator owns every ordered stage assignment and explicitly reports its result.
Optional dedicated stage coordinators are managed children and the normal contact for their stage
work. Orchestrators dynamically create implementation/review/repair work; durable tasks own execution,
waiting and outcomes. Transient cleanup failures reconcile automatically without human babysitting.
The requested mail extension adds waking/deferred send and direct bounded content delivery;
coordinator FYIs reuse deferred mail without an extra synchronization turn.

## Included work

- Replace the old pipeline execution engine with the shared task dispatcher/result path; add durable
  wait/yield, scoped task inspection/repair, managed-child provenance and coordinator handoffs.
- Keep run-wide authority with the standing owner; preserve subordinate coordination and durable
  awareness of material direct interventions through deferred mail, not a separate notice queue,
  wake or coordinator delivery watermark. Use ordinary same-identity stop/resume and re-deliver
  assignments/results, visibly reporting native context restoration failure.
- Fence stage/run closure before descendant cancellation; retry cleanup durably with bounded
  backoff and surface human attention only for persistent/non-recoverable conditions.
- Update template, builder, UI/API/CLI, context report selectors, permissions, runtime knowledge and
  acceptance fixtures together. Remove the old executor/routes/report paths and historical wake veto.
- Apply the explicitly authorized clean legacy pipeline reset; preserve unrelated tasks, agents,
  transcripts and configuration. Keep existing project boundaries. No legacy converter, child DAG,
  cyclic task engine, cross-project delegation, continuously live process lease or direct-action
  transport migration is included. Dedicated coordination is the one configured child choice;
  internal decomposition and repair loops remain the agents' responsibility.

## How we will know it works

FS-06.A20–A25, FS-14.A35–A45 and FS-16.A20–A24 cover ordered explicit completion, stage-local handoffs, managed
coordinator authority, direct intervention awareness, dynamic multi-repository work within one project,
budget-one nested waiting, recovery/replay, cancellation races, automatic cleanup and legacy reset.
Run the applicable TS-06 closure matrix and rendered supervision journeys. TS-09.R47 also requires
bounded packaged Claude/Codex continuation probes; fake-provider checks alone do not prove native
conversation restoration. All requirements remain planned until implementation and verification.

## Evidence and design constraints

The shipped task dispatcher (`internal/server/task_dispatcher.go`) already owns created/woke/borrowed
claims, admission, resume and turn-end release. `internal/runtime/activation_kinds.go` provides task
activation; `internal/messaging/task_tools.go` currently lacks created-work inspection and agent repair.
`internal/runtime/chat.go` Resume attempts native session/load and may fall back to a fresh session,
so durable handoff remains necessary. `internal/runtime/runtime.go` exposes agent-scoped Cancel;
borrowed-work cancellation needs the guarded generation/turn seam specified by TS-10.R32.
Existing pipeline delegation is already dynamic: its constraint is stage-agent lifecycle, not a
mandatory child DAG. `internal/state/messages.go` currently unconditionally coalesces an activation
with each send; `runPromptTurn`/`StartActivation` in `internal/runtime/chat.go` are the direct-content
delivery seams. Mail is currently deleted 24 hours after read or seven days after creation, requiring
the explicit deferred-retention decision below. These verified local seams motivate convergence; no unverified provider
limitation or provider upgrade motivates another execution mechanism.

## Waiting on

The 2026-09-12 efficient-mail extension's feature confirmation, deferred-mail retention decision
in FS-06 §6, and matching technical delivery/confirmation contract. The earlier hierarchy, cleanup
and runtime-lifetime decisions remain confirmed. TS-09.R50 and TS-10.R36's old separate notice
design is withdrawn and must not be implemented while this unit is paused.
