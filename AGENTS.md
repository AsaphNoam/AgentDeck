# AGENTS.md — AgentDeck

Guidance for coding agents working in this repository.

## Read order

Before implementation, review, fix-review, or usability-review work, read:

1. [`docs/features/HANDOFF.md`](docs/features/HANDOFF.md) — resumable current state and governing IDs.
2. [`docs/specs/README.md`](docs/specs/README.md) — the specification constitution and index.
3. The feature (`FS-*`) and technical (`TS-*` / `INV`) requirements named by the handoff or request.
4. [`docs/features/AGENT-WORKFLOW.md`](docs/features/AGENT-WORKFLOW.md) — the canonical role protocol.

The workflow is the single process contract; the feature and technical specs are the only product/
architecture authority. `MAP.md`, ADRs, archive files, plans, handoffs, briefs, and review reports
provide navigation, rationale, history, or sequencing but do not override an FS/TS requirement.

## Roles

- **Implement:** workflow §§1–7 and §11. Draft the spec delta before behavior/contract changes,
  then continue checkpoint-sized steps until done or a STOP condition.
- **Review:** §8. Check code→spec and spec→code across the recorded range; persist every finding.
- **Fix review:** §9 and §11. Validate before editing; spec-gap or behavior-changing fixes update
  the governing spec in the same GREEN checkpoint.
- **Usability review:** §10 plus `USABILITY-REVIEW.md`. Exercise FS acceptance criteria without
  changing product code or specs.

Every role updates the live handoff and stores the exact bounded human brief in
[`docs/features/BRIEFS.md`](docs/features/BRIEFS.md). The stored brief is the entire final response.

## Repository rules

- Read the matching [`INVARIANTS.md`](docs/features/INVARIANTS.md) class before a hot-spot change.
- Shared verification is specified by TS-06 and workflow §2: `make check-specs`, `make build`,
  `make test`, `make dist`; the server remains loopback-only.
- Never edit `internal/server/ui/dist/**`; generate it from `ui/src` with `make embed`.
- Preserve dirty-tree work unless the user explicitly authorizes discarding it.
