# Usability Review Run — 2026-07-26 rerun after the week's fixes

**Scope:** rerun the complete J1–J14 matrix at `c59dd2c`, with extra attention on the sixteen
findings fixed after the preceding week review. The release-style `sqlite_fts5` binary, an untagged
fallback binary, and a fresh fake ACP peer were built. Fresh, seeded, and lived-in fixtures were
created under `.review/usability-20260726-rerun/`; no real provider session or credential was used.

**Review surface:** browser ladder rung 1 through J5, with screenshots, DOM, computed-style, console,
real pointer-drag, loopback API, and on-disk evidence. The in-app browser stalled on J5's native
confirmation dialog and could not reconnect. The execution account then reached its usage limit and
refused every further loopback server start, so the rest of J5 and J6–J14 are explicitly blocked.
Rung-2 component tests and focused non-listening Go regressions were run as supporting evidence, not
used to infer that blocked real journeys passed. Product code and specifications were not changed.

## Executive summary

1. **No new usability finding was confirmed.** Every browser step that completed behaved as
   specified, with zero unexpected console errors.
2. **The recently fixed onboarding and permission paths pass in the real app.** Missing and signed-
   out guidance, the `--no-color` compatibility retry, Set up later, ordinary first launch, restart
   persistence, and distinct Approved/Denied/Cancelled/Timed out history all passed.
3. **First paint, chat, and exercised layout behavior pass.** A truly fresh home is styled; streamed
   deltas replay as one reply; density, collapse, pointer-drag reorder, reload persistence, and the
   group-release API behave correctly.
4. **The rerun is incomplete.** The J5 server-restart check and J6–J14 real journeys are blocked by
   the browser stall and the execution-account limit. They are not reported as passed.
5. **Offline evidence is green.** All 139 UI tests, 25 presentation-contract tests, the UI build,
   specification checks, and focused tagged/untagged regressions for the recent fixes passed. The
   full Go suite could not run in the restricted sandbox because its `httptest` listeners were
   denied; focused non-listening tests passed with a writable review cache.

## Journey results

### J1 — Install & first paint (`fresh/`, port 48111)

| Step | Result | Evidence |
|---|---|---|
| Release-style build and fresh server start | **PASS** | binary version output and server log |
| Fresh browser paint | **PASS** — styled 720px onboarding dialog, product font, shell background/borders | `.review/usability-20260726-rerun/evidence/J1/01-first-paint.png` |
| Console | **PASS** — zero errors | browser console capture |

### J2 — Onboarding (`fresh/`, ports 48111–48113)

| Step | Result | Evidence |
|---|---|---|
| Missing adapter guidance | **PASS** | `J2/01-missing-adapter.png` |
| Installed but signed-out guidance | **PASS** | `J2/02-signed-out.png` |
| Adapter rejects optional `--no-color` as `unknown flag` | **PASS** — retried bare status command and advanced | `J2/03-flag-fallback-ready.png` |
| Project → optional config → first fake launch | **PASS** | `J2/04-onboarding-complete.png` |
| Set up later | **PASS** — only completion changed; seeded project/catalog retained; no session | `J2/05-set-up-later.png` plus API responses |
| Restart persistence | **PASS** — wizard stayed closed and prior agent remained visible as done | `J2/06-restart-persisted.png` |
| Real credentialed provider branch | **SKIPPED(existing human authorization gate)** | — |

### J3 — First launch and chat (`seeded/` + fake ACP, port 48121)

| Step | Result | Evidence |
|---|---|---|
| Launch from New Agent | **PASS** — idle card appeared | browser DOM |
| Prompt/stream/idle round trip | **PASS** | `J3/01-chat-roundtrip.png` |
| Reload replay | **PASS** — two transcript articles and exactly one folded assistant reply | browser DOM |
| Console | **PASS** — zero errors | browser console capture |

### J4 — Permissions (`seeded/` + fake ACP, ports 48122–48123)

| Step | Result | Evidence |
|---|---|---|
| Approve | **PASS** — resolved **Approved**, composer returned to Send | `J4/01-approve-deny-cancel.png` |
| Deny | **PASS** — resolved **Denied**, no stuck turn | same |
| Cancel while pending | **PASS** — resolved **Cancelled**, no actionable stale chip | same |
| Timeout | **PASS** — resolved **Timed out** live and after reload | `J4/02-timeout.png` |
| Replay preserves all decisions | **PASS** | browser DOM after reload |

### J5 — Grid and layout (`seeded/` + fake ACP, port 48124)

| Step | Result | Evidence |
|---|---|---|
| Empty state | **PASS** — fresh completed-onboarding home rendered `No running agents` and New Agent | J2 Set up later browser state |
| Three-card grouped layout | **PASS** | browser DOM |
| Density 3/16 → 2/24 | **PASS** | `J5-layout-before.json` and browser DOM |
| Collapse and reload | **PASS** | browser DOM; persisted `layout.json` |
| Real pointer-drag reorder | **PASS** — drop announcement named both cards; saved order changed | browser DOM and `J5-layout-before.json` |
| Release group through native confirm | **BLOCKED(browser stalled on native confirmation dialog)** | — |
| Group release backing operation and stopped result | **PASS** via API | `J5-release.json`, `J5-sessions-after.json` |
| Server-restart persistence | **BLOCKED(execution account refused further loopback server starts)** | persisted `layout.json` remains 2/24 with reordered ids |
| Remove an id already in saved order | **BLOCKED(no remaining server/browser execution)** | — |

### J6–J14

Every real journey below is **BLOCKED(execution account refused further loopback server starts after
the browser stall)**. Supporting tests are listed so their status is not confused with a browser
pass.

| Journey | Real result | Supporting evidence only |
|---|---|---|
| J6 Terminal runtime | **BLOCKED**; real Claude/xterm remains separately credential-gated | `TerminalTab` tests pass; fixed terminal argv remains a static S3 lead |
| J7 Stop/resume/switch | **BLOCKED** | lifecycle/switch UI tests pass; no real process observation this run |
| J8 Archive/search, both builds | **BLOCKED** | both binaries built; tagged and untagged archive/index regressions pass; Archive UI tests pass |
| J9 Settings/config editing | **BLOCKED** | role/project/backend/source/notification editor tests pass |
| J10 Messaging | **BLOCKED** | no two-agent live transport run this rerun |
| J11 Failure/recovery | **BLOCKED** | cancellation-escalation and validation-focused tests pass |
| J12 Restart durability | **BLOCKED** | persisted fixture files inspected; no restarted server available |
| J13 Annotate & assign | **BLOCKED** | DiffBlock, tray, stale-source, SSE, archive, and index regressions pass |
| J14 Pipelines | **BLOCKED** | dotted seeded Codex model, stopped-stage recovery, start diagnostics, run browser, and builder tests pass |

## Findings

No confirmed Must-fix or Worth-fixing usability finding was produced. Static leads below were not
promoted because the corresponding running journey could not reproduce them.

## Static sweeps

- **S1 serialization:** clean. No real/mocked collection-shape mismatch survived comparison.
- **S2 CSS wiring:** clean. Independent 302-reference/347-selector comparison found no missing or
  materially orphaned selector after dynamic and third-party classes were reconciled; all 25
  presentation-contract tests pass.
- **S3 external CLI variance:** six source leads, not findings: untimed installer readiness, an
  unfamiliar Claude status-command failure classified as failed instead of skipped, unbounded ACP
  initialize/load handshakes, OpenCode/OpenHands probe-vs-launch executable drift, untimed tmux
  commands/modern-flag assumptions, and fixed Claude terminal argv without rejected-flag fallback.
  They map to J1/J2/J3/J6/J7 and the existing provider/terminal acceptance gates.
- **S4 null hostility:** clean. No unguarded server-derived collection use survived comparison.
- **S5 error surfacing:** three source leads, not findings: onboarding/project creates flatten
  structured `400` diagnostics; forced role/project delete retries omit `onError`; Files/Commands
  clipboard writes drop rejections. They map to J2/J3/J8/J9 and need running repro before promotion.

## Verification

- `make check-specs`: **PASS**.
- release-style FTS5 binary, untagged binary, and fake ACP build: **PASS**.
- UI: **34 files / 139 tests PASS**; **25 presentation tests PASS**; production build **PASS**.
- Focused recent-fix regressions: pipeline dotted model id, all permission outcomes and cancellation
  escalation, display-name/project validation, stopped-stage pipeline recovery, tagged Archive
  pagination/search semantics, annotation/index boundaries, and FTS5 degradation: **PASS**.
- Focused untagged Archive/index regressions: **PASS**.
- Full `make test`: **BLOCKED(environment)** — the restricted sandbox denied Go cache writes and
  IPv6 loopback listeners used by `httptest`; this is not recorded as a product test failure.

## Coverage and next run

The next usability rerun must start at J5's server-restart/delete variants, then drive J6–J14 in
order with new isolated homes. J8 still needs both build variants; J12 remains last and reuses prior
journey state. Real-provider, terminal, and federation acceptance remain their existing human-gated
release checks.
