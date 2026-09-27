# AgentDeck — archived handoff state through 2026-09-27

Settled state moved out of [`../../features/HANDOFF.md`](../../features/HANDOFF.md) when `v0.6.0`
was cut. Nothing here is live state; the handoff carries the resumable position, including any debt
these entries still owe.

## Closed units and reports at v0.6.0

- `tighten-chat-actions-and-triage-notes` (reviewed 2026-09-26, fixed 2026-09-27),
  `simplify-agent-and-automation-setup` (reviewed and fixed 2026-09-26),
  `simplify-pipeline-run-detail` (reviewed 2026-09-25, no findings),
  `adopt-modern-codex-acp-capabilities` (reviewed and fixed 2026-09-23), and
  `persistent-pipeline-orchestration` (2026-09-13) are closed; fix commits are not new units.
- The 2026-09-26 usability review's J14 stale run-status **Must fix** was fixed and closed
  2026-09-26.
- BR-1, BR-2, BR-3, and BR-4 are investigated, fixed and closed. Pinned Claude model delivery
  through `_meta` works; an ACP model `currentValue` is adapter configuration evidence and no
  execution-model oracle (TS-04.R54). BR-4 (2026-09-22, "the main project page looks off, the cards
  are stretched and stuck to the bottom") was fixed the same day.

## Changelog

**Changelog — 2026-09-27 (review 2026-09-26 + fix: chat actions and stale-note triage, closed):**
INV §17 — **Annotate selection** trims its excerpt again while Copy keeps the exact selection
(FS-03.R63 now says so); the padded-selection test fails pre-fix. INV §2 — header and selection copy
share `ui/src/lib/copyText.ts` (registered). All checks pass; no browser re-run. FilesTab and
CommandsTab still copy silently via bare `writeText` (outside this unit).

**Changelog — 2026-09-26 (fix: usability review J14 run status, closed):** INV §1 — the
dispatcher-confirmed stage start now moves a dispatch-pending run `queued → running` and republishes
it (FS-14 §3, R37); a test helper that forced `running` was removed (INV §17). Regression tests fail
pre-fix; no spec change. `make test`, `make build`, focused `-race` pass; no browser re-run.

**Changelog — 2026-09-26 (implementation: chat actions and stale-note triage):** Clone now opens
the forked conversation; selected transcript text has Copy beside Annotate; the agent header copies
the stable thread id and uses a compact left identity/runtime band. Six unresolved product/security
questions were recorded under **Ideas being defined**. The built product was exercised against
fakeACP: Clone kept history, both copy paths copied the expected text/id, and the header, rollback
error and Core/Sky & Grove/Studio staged/error matrix stayed inside 1024px and 1440px without
overflow. `make test`, `make build`, the UI suite, UI embed build, spec and diff checks pass.

**Changelog — 2026-09-26 (review + fix: compact agent and automation setup, closed):** INV §1
wizard backend from the loaded catalog (FS-04.R49); INV §10 scoped launch shows its fixed project
(FS-01.R37); INV §8 **Customize runtimes** opens for missing assignments (FS-14.R80). All checks pass.

## Release state — v0.5.0

`v0.5.0` was published and verified on tag `8ab84d3`. `make test` (both tag variants, including
`make check-specs`), the UI suite (54 files, 437 tests), and `make dist VERSION=0.5.0` passed; the
local distributable reported `0.5.0` with `sqlite_fts5`. The CI and Release macOS installer runs both
succeeded, and the GitHub Release carried the darwin/arm64 archive, `install.sh`, and a manifest
declaring `0.5.0` whose SHA-256 and size matched the uploaded archive. No credentialed or
real-browser journey was run for that release. The standing acceptance-gate checklist was retired
from the handoff on the operator's explicit decision during that release; the underlying
verification debt is recorded in [`HANDOFF-through-2026-09-13`](HANDOFF-through-2026-09-13.md).
