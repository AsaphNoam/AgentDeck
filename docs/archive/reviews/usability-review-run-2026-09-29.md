# Usability review run — 2026-09-29

## Scope and outcome

- **Scope:** J1 first paint on a fresh home, and the owed FS-20 phone-size fakeACP browser pass
  (A3 Home/permission/conversation, A4 phone actions, A9 Re-arm editor, runtime picker, diff
  annotation, **Show earlier**).
- **Baseline:** `d9b18a8` on `main`.
- **Outcome:** one confirmed **BLOCKER / Must fix** (S1-class serialization mismatch) that stops
  every phone conversation and decision. J1 passed. The rest of the phone pass is BLOCKED behind it.
- **Evidence:** [screenshots](usability-review-2026-09-29-evidence/).

## Setup

- **Builds:** `make embed && make build` (`sqlite_fts5`) for J1; the phone pass used the documented
  dev-tagged stress fixture (`AGENTDECK_DEV_FAKE_TAILNET=localhost:4529 go run -tags "dev sqlite_fts5"
  ./scripts/stress-fixture --port 4612 --workers 2 --chunks 20`), remote control enabled with
  `PUT /api/remote {"enabled":true}` on loopback, and extra `claude` models whose env selects the
  fakeACP `tool_flow` and `permission` scenarios.
- **Browser rung:** 1 — headless Chrome for Testing (Playwright cache) driven over CDP, phone
  metrics 390 × 844 with touch emulation. The desktop app's in-app browser pane was used for J1's
  first look only: it refuses the fixture's self-signed certificate, and its SharedWorker cannot
  reach the network (see Risk leads).
- **Pairing:** code created on loopback (`POST /api/remote/pairings`), entered on the phone's Pair
  screen, allowed on loopback. The phone's pair, allow and paired-Home transitions worked.

## Journey matrix

| Journey | Result | Observed coverage | Evidence |
|---|---|---|---|
| J1 Install & first paint | PASS | Fresh `AGENTDECK_HOME`, tagged build, styled shell (Instrument Sans, themed background), zero console errors, SSE reached **Local link** in real Chromium. Onboarding was correctly skipped: the computed gate is satisfied by the host's logged-in Claude CLI and the seeded project. | [first paint](usability-review-2026-09-29-evidence/J1-first-paint-pane.jpg) |
| FS-20 A3 Home | FAIL | Home rendered at 390px and updated live (a new permission request appeared under **Needs you** without reload), but the banner stays **Reconnecting…** indefinitely, including after a full reload. | [home](usability-review-2026-09-29-evidence/02-home.png), [needs you](usability-review-2026-09-29-evidence/03-home-needs-you.png) |
| FS-20 A3 permission / conversation | FAIL | Opening the waiting agent shows "This agent is not on the Mac any more."; no decision card, transcript, Approve/Deny or composer. | [agent screen](usability-review-2026-09-29-evidence/04-permission-card.png), [after reload](usability-review-2026-09-29-evidence/05-agent-after-reload.png) |
| FS-20 A4 phone actions | BLOCKED | Blocked by the finding below. | — |
| FS-20 A9 Re-arm, runtime picker, diff annotation, Show earlier | BLOCKED | Blocked by the finding below; every one of these controls is disabled while the link is not `connected`. | — |

## Findings

```
SEVERITY: BLOCKER
WHERE: FS-20 A3 (stress fixture, dev tag, phone https://localhost:4529, 390px)
REPRO: pair a phone → launch a chat agent that requests permission → open it from Needs you
EXPECTED: live link, decision card with Approve/Deny, transcript and composer
OBSERVED: permanent "Reconnecting…"; agent screen says "This agent is not on the Mac any more."
EVIDENCE: usability-review-2026-09-29-evidence/03-home-needs-you.png, 04-permission-card.png
```

Cause, confirmed from the wire: the server's hydration marker is
`{"agent_id":"__hydrated__","data":{"hydrated":true}}` (`internal/bus/bus.go` `HydratedMarker`),
but `ui/src/remote/connection.ts` looks for `data.agent_id === "__hydrated__"`, so it returns early
on the marker, never flushes `hydratingAgents`, never sets `connected`, and buffers every later
agent row forever. The regression came with `680775d` (second-pass mobile review fix). The unit test
`ui/src/remote/PhoneApp.test.tsx:156` emits `{data:{agent_id:"__hydrated__"}}`, a shape the server
never sends, so the suite is green against a server that doesn't exist.

## Risk leads (not findings)

- The desktop app's in-app browser pane keeps the desktop dashboard on **Reconnecting** forever: its
  SharedWorker is alive and posts `error` for every attempt, but the worker's EventSource never
  reaches the server. Real Chromium connects. Because a worker `error` message counts as "alive"
  (BR-7's outage rule), the tab never falls back to the direct stream. Environment-specific; worth
  a look only if a real browser ever blocks worker network the same way.

## Coverage gap (§7)

- The journey matrix has no FS-20 phone charter; this run followed FS-20 A3/A4/A9 and TS-13 §5
  directly. A phone journey (pair, Home, decision, conversation, Re-arm, Replace orchestrator
  runtime, diff annotation, Show earlier, unreachable) should be added when USABILITY-REVIEW.md is
  next maintained.
