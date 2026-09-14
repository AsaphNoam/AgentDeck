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

**Changelog — 2026-09-13 (fix: BR-1 OpenCode/OpenHands delivery):** The undetermined finding
becomes recorded compatibility evidence instead of an open finding. FS-09.A6 and its deviation note
name the two out-of-schema members (top-level `systemPrompt` on both, top-level `model` on
OpenCode), state that OpenHands' model arrives through `LLM_MODEL` so its copy is redundant, and
require delivery to be checked at the effective provider rather than at the emitted request.
TS-04.R47 now says "unchanged" is not "working". Neither member is removed: R47's reason holds while
the CLIs are uninstalled. No code change; the machine-readable record is the R54 out-of-schema table
added with finding 2.

**Changelog — 2026-09-13 (fix: BR-1 provider-contract oracles):** TS-04.R54 states the three rules
that separate a contract claim from a restatement of AgentDeck's own intent. The pinned ACP
session-request member set is now transcribed from the protocol schema and compared against
`sessionNewParams`/`sessionLoadParams`, with each backend's surviving out-of-schema members declared
and backend types enumerated from the adapter registry (new `backend.Types`, one `registry` slice
replacing `For`'s parallel union). `fakeacp` decodes both session requests through that member set,
so it drops what the pinned peer drops. R46 now labels `configOptions.currentValue` adapter
configuration evidence, not provider execution evidence, and FS-09.A15 cites the schema check while
leaving live honoring to gated A16. INV §12/§17 gained the BR-1 entries (INV §12/§17/§2).
`internal/runtime/acp_session_schema_test.go` holds both checks; the fake-peer one fails against the
pre-fix fake with `[model systemPrompt]`.

**Changelog — 2026-09-13 (fix: BR-1 live-gate durability):** Workflow §4 makes a failed acceptance
gate or live-provider check live state: recorded under `## Review findings` in the §7 format before
the role that ran it closes, with the archived run record as supporting evidence rather than the
only trace. A blocked state-file write leaves the role explicitly blocked under §3 instead of
archiving the finding, and a commit or review carrying an acceptance run record first reconciles
every Must-fix in it against the live findings. §16.6 repeats the rule where release acceptance
reports are produced.

**Release state — `v0.4.3` (superseded by `v0.5.0`):** Published and verified on tag `8ad5261`.
Release and CI runs passed, the local distributable reported `0.4.3` with `sqlite_fts5`, and the
GitHub Release carried the darwin/arm64 archive, `install.sh`, and a manifest declaring `0.4.3` with
its SHA-256. That release shipped with five open Must-fix findings on the operator's explicit
decision; all five were closed on 2026-09-13, before `v0.5.0`.

**Acceptance gates — retired 2026-09-13.** The standing checklist of nine unrun manual gates
(pinned real-provider stage-result/file-edit approval, post-fix credentialed Claude and Codex
checks, pinned Claude terminal flags/hooks and live xterm, pinned OpenCode/OpenHands launch and
credential checks, real macOS native folder panels, real-browser permission-pane and drag-refusal,
the Phase 7 federation matrix, real-browser worktree journeys, and the six-tab same-origin dashboard
check) was removed from the live handoff on the operator's explicit decision during the `v0.5.0`
release. None of them was ever run. Removing the list retires the tracking, not the underlying
verification debt: no credentialed or real-browser journey in it has been exercised, and no agent
may describe any of them as verified.
