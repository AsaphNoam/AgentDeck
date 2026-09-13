# AgentDeck — archived handoff state through 2026-09-13

Settled changelog entries moved out of [`../../features/HANDOFF.md`](../../features/HANDOFF.md) to
keep its session-start header inside budget. Nothing here is live state; the handoff carries the
resumable position.

## Changelog

**Changelog — 2026-09-13 (fix: persistent-pipeline-orchestration):** Cleanup gained one stage/run
convergence contract that pages and cancels scoped members, holds the cursor until every
release/yield settles, re-drives the run after a settled effect, and offers repair on `finishing`
too (INV §5/§15/§16). Inherited closure became one SQL fence applied by admission, retry, re-arm and
both wait paths; stage acceptance now fences on run state rather than a caller-read revision, so a
report after Stop cannot un-stop a run, and a failed startup reconcile keeps the stop (INV
§2/§5/§15). Stage reports require a matching execution handle and reject a standing yield (INV
§5/§11). Managed-work authority derives from the live standing-stage binding, not creation
provenance, and assignments now carry the managed child and prior accepted results (INV §2/§10).
Report sharing resolves the task-backed result directly (INV §10). The run projection separates
stage succession from delegated work, surfaces retained cleanup, and is bounded and batched (INV
§7/§8/§11/§16). Deferred-only mail no longer starts a turn (INV §5/§15). Also removed the dead v1
report/lifecycle engine and its skipped tests, shared one task-row insert, refcounted the per-run
lock, added one ordered run-cursor accessor, and moved the report branch to the control plane that
owns runs so it publishes the run update it commits (INV §1/§2/§16/§17). `make test`, `make build`,
the UI suite and spec checks pass; credentialed provider and browser gates remain unverified.

**Changelog — 2026-09-13 (workflow):** Clarified verified slice commits versus final closure,
required checkpoint/resumption notes and stable delegation ownership, and added an actionable-work
check before ending a turn. This administrative update leaves the product work and role queues intact.
Diff checks pass; `make check-specs` reports 14 existing finding-label errors, also present in `HEAD`.
Skill frontmatter is unchanged; its validator could not run because the available Python lacks PyYAML.
