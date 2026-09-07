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
