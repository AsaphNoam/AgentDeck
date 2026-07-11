# AGENTS.md — AgentDeck

Guidance for coding agents working in this repository.

## Canonical workflow

Read [`docs/features/AGENT-WORKFLOW.md`](docs/features/AGENT-WORKFLOW.md) and
[`docs/features/HANDOFF.md`](docs/features/HANDOFF.md) before implementation, review, fix-review,
or usability-review work. The workflow is the single behavioral contract; do not recreate it here.

- **Implement:** follow §§1–7. Continue subphase by subphase to GREEN checkpoints.
- **Review:** follow §8. Review the other agent's recorded code range and persist every finding.
- **Fix review:** follow §9. Validate each finding before changing code; fix real ones and dismiss false positives.
- **Usability review:** follow §10. Exercise normal user journeys without changing product code.

Every role updates the agent-facing handoff and closes with the same focused human brief — optimized to
rebuild the reader's mental model, not to hit a word count — stored in
[`docs/features/BRIEFS.md`](docs/features/BRIEFS.md). The exact stored brief is the entire final response.

## Project orientation

- [`docs/features/INVARIANTS.md`](docs/features/INVARIANTS.md) — paid-for bug classes; read the
  matching section before touching a hot spot.
- [`MAP.md`](MAP.md) — planning index and load-bearing concepts.
- [`docs/agent-dashboard-prd.md`](docs/agent-dashboard-prd.md) — master product requirements.
- [`docs/features/`](docs/features/) — feature PRDs and implementation tech specs.
- Build/test wrappers: `make build`, `make test`, `make dist`. The server binds only to `127.0.0.1`.
