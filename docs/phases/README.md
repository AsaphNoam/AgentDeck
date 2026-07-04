# AgentDeck — Phased Delivery Plan

This folder breaks the [master PRD](../agent-dashboard-prd.md) into **dependency-ordered, independently buildable phases**. Each phase has its own PRD with scope, deliverables, detailed requirements, and acceptance criteria. A phase is considered "done" only when its acceptance criteria pass and it leaves a working, demoable slice of the product. The architecture these phases build toward is recorded in [../architecture-decisions.md](../architecture-decisions.md).

## Guiding principles

- **Vertical slices, not horizontal layers.** Every phase from 1 onward ends with something a user can run and see, not just an internal layer.
- **The server is the contract.** Human-edited config lives as plain JSON files; machine state lives in a single SQLite `state.db` the server solely writes. Producers (hooks via `/api/hook`, runtimes) and consumers (UI via SSE) are decoupled through the server, not a shared file layout.
- **Stable identity first.** `agent_id` never changes; everything that "swaps" (model, backend, interface, resume) is layered on top of it. Getting this right in Phase 0/1 is what makes Phases 6 cheap.
- **Defer the optional.** Terminal runtime comes after the core chat/runtime spine. Phase 7 is a future-phase candidate slot, not a required polish dependency.

## Phase map

| Phase | Title | Features | Depends on |
|-------|-------|----------|------------|
| [0](phase-0-foundation.md) | Foundation: data model, file store, server & CLI skeleton | — (substrate) | — |
| [1](phase-1-core-loop.md) | Core loop: ACP chat runtime, launch, streaming chat | F4, F3 (minimal) | 0 |
| [2](phase-2-state-dashboard.md) | State manager, SSE bus, dashboard card grid | F1 | 1 |
| [3](phase-3-config-onboarding.md) | Config CRUD & onboarding | F5, F6, F12 | 1, 2 |
| [4](phase-4-persistence-archive.md) | Persistence: archive, search, resume, file/command tracking | F9, F10 | 1, 2 |
| [5](phase-5-coordination.md) | Coordination: MCP messaging, nudger, budgets, notifications | F8, F11 | 1, 2 |
| [6](phase-6-flexibility.md) | Flexibility: terminal runtime, switch-runtime, task groups | F7, F2 | 1, 2, 4 |
| [7](phase-7-future-phase.md) | Future phase: candidate-driven post-core work | Candidate backlog | Selected candidate |

## Dependency graph

```
        ┌──────────────┐
        │ 0 Foundation │
        └──────┬───────┘
               ▼
        ┌──────────────┐
        │ 1 Core loop  │
        └──────┬───────┘
               ▼
        ┌──────────────┐
        │ 2 State + UI │
        └──┬───┬───┬───┘
           ▼   ▼   ▼
        ┌────┐┌────┐┌────┐
        │ 3  ││ 4  ││ 5  │   (parallelizable after 2)
        │Cfg ││Pers││Coord│
        └────┘└─┬──┘└─┬──┘
                ▼     ▼
              ┌──────────┐
              │ 6 Flex   │
              └────┬─────┘
                   ▼
              ┌──────────┐
              │ 7 Future │
              └──────────┘
```

Phases 3, 4, and 5 all sit on top of Phase 2 and have no hard dependencies on each other, so they can be built in parallel or reordered by priority. Phase 6 wants resume (Phase 4) in place. Phase 7 is candidate-driven; choose its dependencies from the selected candidate in [`phase-7-feature-candidates.md`](phase-7-feature-candidates.md).

## Milestone-to-PRD mapping

The master PRD §8 milestones map 1:1 onto Phases 1–7 here, with Phase 0 added as the foundation that §8 assumes implicitly.

## How to use these docs with a coding agent

Brief one agent per phase. Each phase PRD is self-contained: it restates the relevant data shapes, the REST/SSE surface it adds, and a verifiable acceptance checklist. Point the agent at the master PRD for the full picture and the phase PRD for the slice to build.

For **autonomous, quota-limited delivery** across many short sessions (Claude Code ⇄ Codex, one at a time), don't brief by hand — point the agent at [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) and [`HANDOFF.md`](HANDOFF.md). Claude Code: run the `/work-phase` skill. Codex: repo-root [`AGENTS.md`](../../AGENTS.md) routes there. The agent builds subphase by subphase to GREEN checkpoints and keeps the handoff current so the next session resumes cold.
