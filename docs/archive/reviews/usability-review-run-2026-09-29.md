# Usability review run — 2026-09-29

## Scope and outcome

- **Scope:** J1 first paint on a fresh home, and the owed FS-20 phone-size fakeACP browser pass
  (A3 Home/permission/conversation, A4 phone actions, A9 Re-arm editor, runtime picker, diff
  annotation, **Show earlier**).
- **Baseline:** `d9b18a8` on `main`.
- **Outcome:** one confirmed **BLOCKER / Must fix** (S1-class serialization mismatch) that stops
  every phone conversation and decision. J1 passed. The rest of the phone pass is BLOCKED behind it.
- **Evidence:** [screenshots](usability-review-2026-09-29-evidence/).
- **Second pass (same day, after `86502cd` fixed the hydration blocker):** see
  [Second pass](#second-pass--after-86502cd) below. One new **BLOCKER** (phone Start pipeline), three
  **MINOR**.

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

---

## Second pass — after `86502cd`

- **Baseline:** `86502cd` on `main`; `make embed` then the same dev-tagged stress fixture (port 4612,
  fake tailnet `localhost:4529`), extra fakeACP models `tool_flow`, `permission`, `hold_turn`,
  `hold_turn`+steering. Same headless Chrome at 390 × 844, rung 1. Zero console errors throughout.
- **Fixture objects:** two permission agents, hold/steer agents, a 70-round `tool_flow` agent
  (350 events), four tasks with prerequisites, a two-stage template with a required input, a
  desktop-started run on `my-app` (cwd pointed at a scratch dir).

| Area | Result | Observed | Evidence |
|---|---|---|---|
| Link | PASS | Pairs, reaches connected (no banner), updates live. | — |
| A3 Home | PASS | Needs you / Moving (grouped by project, "Stage 1 of 2") / Since you last looked from agents, tasks and runs, all live. See MINOR 3. | [home](usability-review-2026-09-29-evidence/r2-16-home-mixed.png) |
| A3 decision | PASS | Card shows tool, command and summary; Approve and Deny each produce the matching `permission_resolved` on the Mac. | [card](usability-review-2026-09-29-evidence/r2-02-decision.png) |
| A3 conversation | PASS | Send; held follow-up with Withdraw while busy; Steer (steering runtime); Cancel turn; Stop; Resume. Held follow-up is sent after cancel. The second turn's instant cancel comes from fakeACP's latched cancel channel, not the product. | [held](usability-review-2026-09-29-evidence/r2-04-held.png), [steer](usability-review-2026-09-29-evidence/r2-06-steer.png) |
| Show earlier | PASS | 350 events: 30 rounds → 60 → all 70, then the control goes away. | — |
| Tool calls / diffs | FAIL (MINOR 1) | Diffs render with a line-range selector; tool results render expanded raw JSON outside the collapsed tool line. | [transcript](usability-review-2026-09-29-evidence/r2-07-transcript-top.png) |
| A4 Ask AgentDecker | PASS | Running `agentdecker` gets the message; project without one launches one and sends; bad project path refuses with the Mac's reason and keeps the typed text. | — |
| A4 New task | PASS | Name/role/instruction → task page, running. | — |
| A4 Start pipeline | FAIL (BLOCKER) | Shared-workspace conflict shows and keeps input, but **Start anyway** and a conflict-free project both fail with "run cannot start". | [refused](usability-review-2026-09-29-evidence/r2-14-start-pipeline-refused.png), [conflict](usability-review-2026-09-29-evidence/r2-12-run.png) |
| A4 retry task / run | PASS | Interrupted task Retry → running; paused run Retry stage → queued; run page moves to paused live. | — |
| A4 continue paused run | SKIPPED | fakeACP cannot report stage success, so no approval pause could be reached. | — |
| A9 Re-arm | PASS with MINOR 2 | Opens on the current arms; a task prerequisite changed to another task plus an added named signal applied exactly on the Mac. Empty-outcome and cycle sets are refused without mutation and the draft stays. | [done](usability-review-2026-09-29-evidence/r2-10-rearm-done.png), [refused](usability-review-2026-09-29-evidence/r2-11-rearm-invalid.png) |
| A9 Replace orchestrator | PASS | Preselected Claude/Haiku/medium; incomplete choice (Codex, no model) is blocked; Opus/high launches the replacement with those values. Fast toggle SKIPPED: no fast-capable model configured. | [picker](usability-review-2026-09-29-evidence/r2-15-replace-open.png) |
| A9 annotate | PASS | Selecting a diff line opens the form; This agent (new turn), Another agent (reserved-sender mail), New task (launches a teammate on defaults, delivers). A stopped target refuses with "target agent is not running", keeps the draft, and the draft survives a reload. | [form](usability-review-2026-09-29-evidence/r2-17-annotate-form.png), [failed](usability-review-2026-09-29-evidence/r2-18-annotation-failed.png) |
| A7 unreachable | PASS | Fixture stopped: "Mac unreachable since …", last known state, actions disabled. | [unreachable](usability-review-2026-09-29-evidence/r2-19-unreachable.png) |

### Findings

```
SEVERITY: BLOCKER
WHERE: A4 Start pipeline (my-app, conflict-free; also stress after Start anyway)
REPRO: New work → Start pipeline → template, required input, goal → Start run
EXPECTED: run starts on the configured default runtimes (FS-20.R15, FS-14.R80)
OBSERVED: "run cannot start"; server 422 "the standing orchestrator requires a configured backend and model"
EVIDENCE: usability-review-2026-09-29-evidence/r2-14-start-pipeline-refused.png
```

`ui/src/remote/NewWorkScreen.tsx` submits `orchestrator: {backend:"", model:"", …}`; FS-14.R80
says accepting defaults still supplies explicit resolved assignments, which the desktop's
`RunStartForm.tsx` does. The phone also hides the diagnostic, so the person can't tell why.

```
SEVERITY: MINOR (1)
WHERE: phone agent screen, any agent with tool results
REPRO: open the tool_flow agent
EXPECTED: tool calls collapsed to one line that expands (FS-20.R13)
OBSERVED: each result prints expanded raw JSON under the collapsed line, repeating the diff
EVIDENCE: usability-review-2026-09-29-evidence/r2-07-transcript-top.png
```

```
SEVERITY: MINOR (2)
WHERE: phone task page, Re-arm
REPRO: submit a valid re-arm; then one with no outcome ticked; then one that forms a cycle
EXPECTED: confirmation on success; plain-language refusals
OBSERVED: no confirmation; "satisfying_outcomes is required", "state: task arms would create a cycle"
EVIDENCE: usability-review-2026-09-29-evidence/r2-10-rearm-done.png, r2-11-rearm-invalid.png
```

```
SEVERITY: MINOR (3)
WHERE: phone Home
REPRO: stop a running task's agent
EXPECTED: one consistent story for that task
OBSERVED: the task is under Needs you as "was interrupted", and its same-named agent is under Since you last looked as "finished"
EVIDENCE: usability-review-2026-09-29-evidence/r2-16-home-mixed.png
```

### Notes (not findings)

- The decision card names the agent ("Perm A needs permission"), not `role@project` as FS-20.R12
  words it; the project appears on the Home card. This may be specification drift.
- FS-20.A9's fakeACP phone-size browser pass has now run. Only the fast-mode picker and Continue on
  an approval pause stay unexercised.
