# AGENTS.md — AgentDeck

Guidance for coding agents working in this repository.

## Read order

Before implementation, review, fix-review, or usability-review work, read:

1. [`docs/features/HANDOFF.md`](docs/features/HANDOFF.md) — current state and relevant requirement IDs.
2. The change file named by the handoff, if a change is in progress.
3. [`docs/specs/README.md`](docs/specs/README.md) — the specification constitution and index.
4. The feature (`FS-*`) and technical (`TS-*` / `INV`) requirements named by the change file, handoff,
   or request.
5. [`docs/features/AGENT-WORKFLOW.md`](docs/features/AGENT-WORKFLOW.md) — the canonical role protocol.

The workflow explains the process; feature and technical specs define the product and architecture.
`MAP.md`, ADRs, archive files, plans, handoffs, briefs, and review reports provide navigation,
rationale, history, or sequencing but do not override an FS/TS requirement.

## Roles

- **Implement:** workflow §§1–6 and §10. Update the relevant specification before a behavior or
  architecture change, then keep working until done or a real blocker.
- **Review:** §7. Check that code matches the specifications and that the specifications cover the
  shipped behavior; record every real finding.
- **Fix review:** §8 and §10. Validate before editing; update the relevant specification when a fix
  changes behavior or fills missing coverage.
- **Usability review:** §9 plus `USABILITY-REVIEW.md`. Exercise FS acceptance criteria without
  changing product code or specs.

`docs/ideas.md` holds new ideas and known product improvements. `docs/ready-changes/` holds changes
that are specified and ready to start. `HANDOFF.md` records only the change already in progress.
Agents never choose future work for themselves.

Every role updates the live handoff and stores a short, plain-language human update in
[`docs/features/BRIEFS.md`](docs/features/BRIEFS.md). The stored update is the entire final response.

## Repository rules

- Read the matching [`INVARIANTS.md`](docs/features/INVARIANTS.md) class before a hot-spot change.
- Shared verification is specified by TS-06 and workflow §2: `make check-specs`, `make build`,
  `make test`, `make dist`; the server remains loopback-only.
- Never edit `internal/server/ui/dist/**`; generate it from `ui/src` with `make embed`.
- Preserve dirty-tree work unless the user explicitly authorizes discarding it.
