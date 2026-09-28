# AgentDeck — Implementation handoff

**Live agent state.** Read the **Current position** and **Active change** below, then open the
requirements they name. Settled state is archived in `../archive/state/`: the dated
[`HANDOFF-through-2026-09-27`](../archive/state/HANDOFF-through-2026-09-27.md),
[`-25`](../archive/state/HANDOFF-through-2026-09-25.md),
[`-14`](../archive/state/HANDOFF-through-2026-09-14.md),
[`-13`](../archive/state/HANDOFF-through-2026-09-13.md),
[`-12`](../archive/state/HANDOFF-through-2026-09-12.md),
[`-11`](../archive/state/HANDOFF-through-2026-09-11.md),
[`-10`](../archive/state/HANDOFF-through-2026-09-10.md),
[`-09`](../archive/state/HANDOFF-through-2026-09-09.md),
[`-07`](../archive/state/HANDOFF-through-2026-09-07.md),
[`-06`](../archive/state/HANDOFF-through-2026-09-06.md) and
[`-03`](../archive/state/HANDOFF-through-2026-09-03.md) files, plus
[`HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md). Follow
[`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md); this file holds resumable current state only.

## Current position

- **Active change:** none. `add-mobile-remote-control` finished 2026-09-28 (see Review units).
- **Release:** `v0.6.0` is tagged and published; **Release state** carries its contents. `v0.5.0` and earlier
  are in the state archive, as are the units, findings and bug reports it closed.
- **Review units:** `add-studio-skin` (finished 2026-09-23) and `complete-studio-composition`
  (finished 2026-09-25) await `/review`; the operator shipped them unreviewed in `v0.6.0`. Review
  `add-studio-skin` against `e474d8a..1d78e1d` and `complete-studio-composition` against
  `9ae10ee..4bb5b2e`; `71c2810` is the latter's evidence-only handoff follow-up, not another unit.
  FS-12.A19–A23 and TS-08.R68 remain planned despite the shipped requirements.
  `share-creative-workspace-layout` finished 2026-09-28 and awaits `/review`; its substantive
  commits are `0b31c9a`, `ddab692` and the closure commit named
  `Finish shared workspace acceptance and reduced-motion fallback` (base `f79fb97`).
  Evidence: `docs/archive/reviews/implementation-share-creative-workspace-2026-09-28.md`.
  `add-mobile-remote-control` was reviewed 2026-09-28 across `db23ca9^..6ac9061` minus `36e656f`;
  its findings remain open below. `stop-telling-agents-to-poll` shipped outside this queue on the
  operator's explicit 2026-09-10 instruction; it can be added later.
- **Work units:** `rename-product-to-deckhand.md` and
  `drop-pipeline-recipient-refusal.md` are Waiting to start. `migrate-internal-actions-from-mcp.md` stays
  paused on its transport blocker.
- **Design units:** `Ideas being defined` entries may resume (the operator deleted the
  uncommitted Cursor backend draft on 2026-09-23); `New ideas`
  entries are available. The unaddressable-pipeline-agent idea was already shipped by FS-14.R74
  (`8d8ca6e`); on 2026-09-28 its stale FS-01/03/06/14/16 and TS-04 text was reconciled and the
  leftover refusal became `drop-pipeline-recipient-refusal.md` (FS-06.R37/A26, TS-04.R67).
  `docs/ideas.md` was pruned 2026-09-28: shipped agent re-arm/retry/inspection, fixed chat-reload,
  pagination and ACP-readiness items, and nudge-era liveness items were removed; small related
  entries were merged.
- **Open findings:** `add-mobile-remote-control` has four Must-fix and four Worth-fixing findings
  below. The injected-steer lifetime edge case is still named in prose
  but was never recorded as a finding; it needs `/investigate-bug` before `/fix` can take it.
  FilesTab and CommandsTab still copy silently via bare `writeText`.
- **Bug reports:** BR-6 closed 2026-09-28 (file links: rendered-file relative links, `name:line`,
  `:start-end`, case-variant roots, refusal logging). BR-7 closed 2026-09-28: live shared workers
  retain the shared reconnect path during server outages.
- **State:** Automated MCP contract verification is green.
- **Branch:** `main`.

## Active change

None. **Owed from `add-mobile-remote-control`:** FS-20.A1/A5/A6/A8 manual gates (real tailnet,
real Android phone and iPhone, `pmset -g assertions`); a PNG touch icon for iPhone Home Screen.
Local phone passes: `AGENTDECK_DEV_FAKE_TAILNET=localhost:4529 go run -tags dev
./scripts/stress-fixture`, then `PUT /api/remote {"enabled":true}` on loopback (TS-13 §5).
Observed pre-existing flake: `TestContextSharingStartsNoModelTurn` under full `go test ./...` load.

**Owed from archived entries** ([`HANDOFF-through-2026-09-25`](../archive/state/HANDOFF-through-2026-09-25.md)):
A46's real-browser J14 pass; the credentialed Codex 1.12.0 receipt (TS-06.R26) gating
FS-03.A41/A42 and FS-01.A20; Sky & Grove unviewed for Codex capabilities. Pre-existing, not Studio:
Sky & Grove tints the whole user event row; `--ad-shadow-project-edge` resolves at `:root`.

**Acceptance remains planned, not shipped:** FS-12.A19–A23 and TS-08.R68 still need the complete
route/state comparison. In this pass, the available Chrome browser adapter reported 1280×1125 after
its 1024×900 viewport request, so the populated built-app review does not establish true-1024
behavior; the in-app browser was unavailable to that run. A faithful desaturated cross-skin
comparison beyond the dashboard was also unavailable. fakeACP produced active and paused pipeline
states but not a genuine finished-run timeline. No product code or specifications changed in this
evidence pass.

**Release state:** `v0.6.0` is published on tag `24ab07b` (range `v0.5.0..main`, 49 commits). The
CI and Release macOS installer runs both succeeded; the GitHub Release carries the darwin/arm64
archive, `install.sh`, and a manifest declaring `0.6.0` whose SHA-256 and size match the uploaded
archive.
`make test` (both tag variants, including `make check-specs`), the UI suite (58 files, 481 tests),
and `make dist VERSION=0.6.0` pass; the local distributable reports `0.6.0` with `sqlite_fts5`. The
range changed nothing an operating agent must know, so `operating-agentdeck` is unchanged; README,
`install.sh` and `scripts/release/assemble.sh` pins still match. Two Studio review units shipped
unreviewed on the operator's explicit decision. Owed: the credentialed Claude and Codex
login/chat gates (TS-06.R21) and every real-browser journey; none may be described as verified.

**Available by role:** `/review` may take `add-studio-skin`, `complete-studio-composition` or
`share-creative-workspace-layout`.
`/work` may take `rename-product-to-deckhand` or `drop-pipeline-recipient-refusal`;
`/design-feature` may choose an available or resumable idea. Queues are independent.

## Decisions needing your input

- **API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
  provider model-ID ownership. Standardizing either is a compatibility change.
- **Failed pipeline-stage chat:** Confirm whether a pause after a failed launch or resume should
  keep withholding **Open agent**, matching restart recovery (FS-14.R48), or whether chat should
  remain reachable with a wider continuation contract.

## Blocked on human

None for shared creative-workspace implementation. Its approved system check passed after
correcting reduced-motion selector priority, and Reduce Motion was restored off.

## Review findings

### add-mobile-remote-control (2026-09-28) — **Fix model:** medium — Codex Terra or Claude Opus.

- **Must fix** — confirmed contract/security defect; TS-13.R5/R6, FS-20.R15, INV §8/§14.
  `internal/server/remote_routes.go:35,48,56` accepts `backend`, `model`, `effort`, and `fast` on
  phone session/task creation and applies no body filter to pipeline start. A crafted or stale
  paired client can therefore select runtimes instead of letting the Mac resolve the configured
  desktop defaults; pipeline requests can also supply non-phone runtime assignments. Restrict
  session/task fields to what the shipped forms send, validate the pipeline start's nested runtime
  assignments as empty/default on the remote chain, and add negative route tests proving every
  override is rejected before handler work.
- **Must fix** — confirmed reconnect defect; FS-20.R15/R17/R23, INV §1/§16.
  `ui/src/remote/connection.ts:58-79` ignores the stream's `__hydrated__` marker and only merges
  snapshot rows into the existing agent map. An agent removed or archived while the phone is
  offline therefore survives reconnect as running; **Ask AgentDecker** can send to that stale id
  instead of launching the resident, and an open conversation can show obsolete state. Rebuild the
  connection-scoped map through the hydration boundary, clear per-agent transcript counters for
  missing rows, and keep mutations gated until hydration completes. Test reconnect with one
  previously known agent absent from the new snapshot.
- **Must fix** — confirmed normal-scale omission; FS-20.R11/R18, TS-13.R10, INV §8/§10/§16.
  `internal/server/remote_home.go:98-121` takes one newest-first page containing active and terminal
  tasks/runs before classifying it. More than 200 newer task completions or 100 newer pipeline rows
  can hide an older interrupted/running task or paused/running run from both Home and push, despite
  **Needs you** requiring oldest-first coverage. Query bounded active/attention rows independently,
  then fill the completion budget, and test an older attention item behind more than one page of
  newer terminal history.
- **Must fix** — confirmed revocation race; FS-20.R8, TS-13.R9, INV §4/§5/§14.
  `internal/server/remote_routes.go:331-362` looks up a device before registering its request in the
  per-device cancellation set. If revoke deletes the row and runs `end(id)` in that gap, the request
  registers afterward and can leave an SSE/read open or reach a shared mutation with a credential
  that is already revoked. Give admission and revoke one linearized boundary (or register then
  revalidate the row), and use a barrier test to prove a request paused between lookup and tracking
  cannot survive revocation.
- **Worth fixing** — confirmed first-use race; TS-02.R37, TS-13.R11, INV §5/§9/§15.
  `internal/remote/push.go:39-67` generates and renames one fixed `vapid.json.tmp` without
  serialization. Concurrent first `/api/remote/self` reads can return different keypairs or one can
  fail rename, leaving a phone subscribed with a public key that the sender no longer owns.
  Serialize atomic initialization and test concurrent first callers all receive the one persisted
  pair.
- **Worth fixing** — confirmed owner-mode gap; TS-02.R37, TS-13.R12, INV §9/§14.
  `internal/remote/push.go:41-45,60-65` neither repairs an existing VAPID private-key file to `0600`
  nor protects against promoting a permissive leftover `.tmp`. Use the owner-only atomic-write
  helper or explicitly chmod existing and staged files, and test loading a permissive pre-existing
  key tightens its mode.
- **Worth fixing** — confirmed API-contract drift; TS-13 §3, INV §10/§11/§17.
  `internal/server/remote_home.go:17-27,127-130` and `ui/src/remote/api.ts:8-28` expose only the
  presentation string `stage`; the specified `stage_number` and `stage_count` fields never cross
  the Go/TypeScript boundary. Emit and parse the numeric fields, derive the displayed label in the
  phone UI, and add a JSON contract test for a staged run.
- **Worth fixing** — confirmed static-surface drift; TS-13.R14, TS-08.R73, INV §10/§14/§17.
  `internal/server/spa.go:45-69` remaps the HTML entry but serves every existing dist asset,
  including generated desktop entry files such as `assets/main-*.js` and `main-*.css`, through the
  unauthenticated tailnet static surface. Separate or manifest the phone asset graph and reject
  desktop-only entry assets while retaining shared chunks. The current `spa_test.go` oracle uses
  one generic asset and cannot catch this; derive a regression from the built desktop entry's actual
  asset references.

## Design consistency notes

- The paused direct-action change cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 cite
  `TS-04.R32–R39` and omit R40, the direct-action redaction clause. Align them when that change
  resumes.
- FS-17 §6's opening sentence should be scoped when its planned direct-cutover work resumes; it
  currently reads as covering a section that also contains planned R13–R19 boundaries.

## Changelog

- **2026-09-28 — Review add-mobile-remote-control (INV §1, §4, §5, §8–§11, §14–§17).** Four
  Must-fix and four Worth-fixing findings recorded: remote runtime overrides, stale reconnect
  hydration, attention rows lost behind terminal-history pages, a revoke/admission race, VAPID
  initialization and owner-mode gaps, missing numeric run-stage fields, and desktop-only assets on
  the tailnet static surface. Fix model: medium — Codex Terra or Claude Opus. Focused UI tests (26),
  `internal/remote`, `internal/state`, `internal/server`, and `make check-specs` pass; the first
  server run was blocked only by sandbox loopback permissions and passed when rerun with them.

- **2026-09-28 — Fix BR-6 file links (INV §11 protocol meaning; §14 filesystem boundary; §8
  surfaced errors).** Links inside a rendered file resolve against that file's directory
  (`resolveFromFile`, escapes still reach the server's refusal); `README.md:12` is a file link, not
  a scheme; `:start-end` cites its start line; an absolute link spelling the working directory in
  another case reads when that spelling is the same directory on disk (only the variant root is
  stat'ed); each file-read refusal is logged with path and code. FS-03.R51/R52/A34/A35, TS-03.R40,
  TS-05.R21 updated. The three skipped UI reproductions and two new Go tests failed before, pass
  after. `make test` (both variants), `make build`, UI 515 passed, UI build pass. BR-6 closed.

- **2026-09-28 — Finish add-mobile-remote-control (INV §2, §4, §5, §8, §10, §14, §15, §16).**
  Embedded `tsnet` node (Go 1.26.6, `tailscale.com` v1.102.5), tailnet chain with allowlist
  inventory test, node-bound cookie pairing, shared attention helper for Home and Web Push,
  keep-awake, desktop Remote tab, and the installable phone app. FS-20 R1–R29/A2–A4/A7 and TS-13
  (now Current), TS-02.R37, TS-03.R46, TS-05.R23, TS-06.R27, TS-08.R73, FS-00.R18 reconciled.
  `make test` (both variants), `make dist`, UI 510 passed/3 skipped, focused `-race` on remote
  paths, and a 390×844 fakeACP browser pass (found and fixed the desktop 1024px floor on the
  phone) pass. Real-device gates stay owed.
- **2026-09-28 — Fix BR-7 (INV §1 boundary-derived state; §16 bounded streams).** A worker port
  message now proves worker liveness for the current connection, preventing server outages from
  permanently multiplying per-tab streams. TS-03.R7 reconciled; silent-worker and failed-load
  fallbacks remain covered. The unskipped 90s-outage regression failed before the fix (two direct
  streams), then passed; initial-outage recovery and old-port teardown are also covered.
  Focused SSE tests: 26 passed; full UI: 484 passed/3 existing skips. `make dist`, both Go test
  variants and `make check-specs` pass. The first `make test` stopped on a concurrent FS-06 index
  status edit; after that session committed the correction, spec lint and both Go variants passed
  separately. BR-7 is closed; BR-6 remains open. FS-02.A27's six-tab real-browser check is still owed.
