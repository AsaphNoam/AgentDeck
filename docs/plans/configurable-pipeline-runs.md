# Configurable pipeline runs — implementation plan

This plan sequences the active FS-14 / TS-09 change. The specifications remain authoritative.

## 1. Template and persistence foundation

- Add centralized limits, version-1 template/run types, canonical validation, and bounded diagnostics.
- Add owner-only atomic pipeline-template CRUD under `pipelines/{id}.json`.
- Add forward-only run/attempt/value/request schema and state-store operations for snapshots,
  revisions, reports, actions, lineage, deletion, and idempotent starts.
- Cover template validity, non-null collections, schema guards, both SQLite build variants, and
  restart reads.

## 2. Lifecycle and manager

- Extract server-owned launch/resume/stop services used by HTTP and pipelines without weakening the
  existing generation-scoped teardown contract.
- Add deterministic assignments and immutable run/stage/attempt associations before process launch.
- Implement the sequential manager, compare-and-swap action claims, quiescence/exit fan-out,
  approval/blocked/retry/stop controls, visit limits, notifications, and bounded startup recovery.
- Cover success/failure routing, repair loops, launch/crash/blocked recovery, race winners,
  idempotency, and restart boundaries with fake runtimes.

## 3. MCP, REST/SSE, and CLI

- Extend the scoped MCP authority with current-attempt results and AgentDecker-only proposal tools.
- Add template/run/control REST routes, structured errors, advisory workspace conflicts, and
  revisioned `pipeline_update` publication.
- Add thin `agentdeck pipeline` API-client subcommands.
- Cover spoofed/stale calls, proposal non-mutation, route/method/error contracts, replayed request
  ids, Host/Origin inheritance, and CLI contracts.

## 4. Product surfaces

- Add shared TypeScript schemas/API functions and SSE invalidation.
- Add the Pipelines navigation/page with template editor, start form, selectors/warning, run
  detail/history/actions/transcript links, and exact-payload proposal confirmations.
- Show immutable run/stage associations on ordinary agents and add the two pipeline notification
  categories through the existing notification path.
- Cover null-safe payloads, visible mutation errors, one-time confirmation invalidation, selectors,
  mocks, and shared-workspace/deletion behavior.

## 5. Completion

- Run FS-14.A1-A10 focused tests and the browser pipeline journey with fake runtimes.
- Run `make check-specs`, both Go test variants through `make test`, `make build`, UI tests/build,
  `make dist`, focused race tests, and `git diff --check`.
- Flip shipped `(planned)` markers/statuses, update traceability, remove this plan, finish the
  handoff/change state, add the exact human update to `BRIEFS.md`, and commit the verified result.
