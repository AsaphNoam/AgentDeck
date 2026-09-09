# AgentDeck — handoff state settled at the `v0.4.2` epoch (2026-09-07)

Archived under AGENT-WORKFLOW §16.7. Everything below stood in the live handoff when `v0.4.2`
was cut and is settled history. The live file is [`../../features/HANDOFF.md`](../../features/HANDOFF.md).
Earlier epochs are in [`HANDOFF-through-2026-09-06.md`](HANDOFF-through-2026-09-06.md),
[`HANDOFF-through-2026-09-03.md`](HANDOFF-through-2026-09-03.md), and
[`HANDOFF-pre-sdd.md`](HANDOFF-pre-sdd.md).

## Release notes for `v0.4.2`

The release range `v0.4.1..main` contains six commits. The annotation tray now docks as a
right-hand column beside a wide transcript, falls back to its floating overlay in narrow
transcript regions, and remembers a per-source collapsed state. Docked drafts have readable
anchors, excerpts, and taller instruction fields. A self-targeted annotation's machine prompt is
suppressed from both live and replayed transcript presentation while the underlying event remains
stored, delivered, returned by the API, and searchable. Mermaid diagrams retain their generated
theme and local SVG references while rejecting network URLs, and compact diagrams keep their
intrinsic size within bounded transcript and viewport dimensions.

The annotation change was reviewed on 2026-09-07. Its two findings were fixed: removing the last
draft now removes the complete browser-local tray record, including the collapse flag, and the
presentation contract correctly exposes collapsed state without a `data-variant`. The unit is
closed; no review findings remain. The range changes no agent-facing behavior, so the shipped
`internal/agentknowledge/operating-agentdeck/` package required no refresh. README, `install.sh`,
and `scripts/release/assemble.sh` claims were not falsified.

Automated release checks passed for the tagged tree: both Go variants and spec checks via `make
test`, UI tests, and `make dist VERSION=0.4.2`. The distributable binary reports `0.4.2` and is
built with `sqlite_fts5`. Credentialed provider and other real-browser acceptance gates remain
owed and are not represented as verified by this release.

## Settled change units

- `dock-the-annotation-tray-and-quiet-its-prompt` — implemented, reviewed, fixed, and closed;
  FS-13.R20–R23/A12–A14 and TS-08.R53–R54 are shipped.
- Mermaid rendering investigation and fixes — diagnosed, fixed, and covered by focused tests;
  no specification change was needed because the fixes restore existing requirements.

The prior `v0.4.1` archive contains all earlier release-range history, decisions, and closed units.

## Settled post-`v0.4.2` changelog entries

Moved out of the live handoff once their units closed, so the session-start header stays within
budget. The next release archive supersedes this section.

**Changelog — 2026-09-08 (fix):** Closed all six `chat-session-configuration` review findings
(INV §1, §5, §8, §11, §12, §15, §17). The ACP configuration decode now keeps each option's reported
`currentValue` and is re-read from every response that carries one, so applying a model consults the
peer's rebuilt option list rather than the pre-model one, and a setting AgentDeck required is checked
against the value the peer itself reports instead of the bare RPC envelope. The live
`session-config` route takes the shared exclusive lifecycle claim, and a combined request that fails
partway now persists what actually applied before returning the error. Runtime failures are typed
(unsupported, unavailable, rejected, ignored) and mapped to distinct route envelopes instead of one
`runtime_start_failed`. The live session's fast-mode advertisement is decoded per generation, stored
on the `running` row (migration 24), projected into agent state, and rendered by the chat header, so
a launch that asked for fast mode and did not get it now says the model does not offer it rather
than showing a dead toggle. Added regression coverage with acceptance IDs for the rebuilt-option-list
and ignored-setting paths, partial-apply persistence, lifecycle serialization, unavailable-fast
header states, and task/pipeline requested-versus-applied fast mode; the option-list regression was
confirmed to fail against the pre-fix discard. TS-02.R30, TS-03.R37, and TS-04.R46 updated. Full Go
suite, SQLite-FTS variant, targeted `-race` runs, vet, all UI tests, UI build, style/presentation
checks, and spec checks pass.

**Both pinned adapters were driven live during this fix**, but only for setup/configuration calls.
See **Bug investigation reports** for the recorded probes. Post-session configuration works at the
ACP adapter layer for Claude as well as Codex. The run's stronger Claude model-delivery conclusion
was retracted by the 2026-09-09 review because it never sent a prompt and relied on a stale
adapter-local field.

**Changelog — 2026-09-08 (review):** Reviewed `chat-session-configuration` across its design and
implementation range. The ordered provider-setting path discards the required updated option list,
the live mutation is not serialized with lifecycle changes, a combined request can partially apply,
and the UI cannot explain an unhonored fast request. Error typing and required task/pipeline and
negative UI acceptance coverage are also incomplete. The unit stays open for fix. CLI and launch
surface propagation, requested-versus-applied persistence, migrations, archive/index projections,
terminal rejection, and task/pipeline wiring had no additional finding. The invariant sweep found
no class-6 surface because the change extends existing adapters and runtimes rather than adding one;
all other triggered classes were checked. Both Go variants, Go vet, all UI tests, the UI build, style
checks, presentation contract, and spec checks pass; these findings are gaps the current suite does
not exercise.

**Changelog — 2026-09-08:** Implemented `chat-session-configuration`. Fast mode now flows through
the model catalog, launch API and CLI, task and pipeline assignments, applied agent/session state,
archive projections, and capability-gated UI controls. Chat launches and resumes apply one ordered
model → effort → fast session-configuration sequence; Codex no longer relies on the ignored ACP
session model parameter. The chat header separates staged backend/model controls from immediate
effort/fast settings, and `POST /api/sessions/{id}/session-config` persists live changes without a
process or native-session restart. Added fake-provider sequence coverage, a live-route persistence
test, catalog/migration/UI coverage, and the header state to the visual matrix. The full Go suite,
SQLite-FTS suite, UI tests/build, and rendered desktop matrix check pass. Credentialed provider
gates remain open as recorded below.
