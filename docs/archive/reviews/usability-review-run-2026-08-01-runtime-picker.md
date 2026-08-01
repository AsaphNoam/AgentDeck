# Usability review run — 2026-08-01 — per-chat runtime picker (FS-03.R23/R24/A9)

**Scope:** The per-chat runtime picker shipped in `ca100e0` — the chat-header backend/model/effort
selects and explicit Switch on a running chat agent, and the static identity for a stopped agent.
This is the runtime-switch browser journey the prior handoff recorded as the only unexercised one
(the earlier in-app browser rejected `window.prompt()`; the shipped picker uses inline selects, not
a prompt, so it is now driveable).

**Result:** PASS. All FS-03.A9 observations confirmed end-to-end in a real browser with **zero
console errors and zero page errors**. No findings.

## Harness

- **Binary:** `make build` (`bin/agentdeck`, `-tags sqlite_fts5`), commit `146f71e`.
- **Browser (rung 1):** Playwright `playwright-core` 1.62.1 driving installed Chrome-for-Testing
  (chromium-1228), headless. Console + pageerror captured throughout.
- **Fresh isolated state:** `AGENTDECK_HOME=.review/usability-20260801-runtime-picker/home`
  (seeded: `onboarding_complete`, one project with a real cwd, one role, empty layout).
- **Deterministic backend:** `fakeacp` (`internal/runtime/testdata/fakeacp`) reached via PATH shims
  `claude-agent-acp` and `codex-acp`, each `exec`ing fakeacp, so both `claude-acp` and `codex-acp`
  adapter types spawn the fake. `FAKEACP_SCENARIO=stream_text`.
- **Catalog:** `claude` (opus/sonnet, efforts low/medium/high/max, default medium) and
  `codex` (gpt-5.6-sol/gpt-5.5, efforts low/medium/high/xhigh, default medium).
- **Evidence:** `.review/usability-20260801-runtime-picker/run/` — `drive.mjs`, `drive-stopped.mjs`,
  `picker-report.json`, screenshots `01`–`08`.

## Journey — running chat agent (R23), fresh claude/sonnet agent `a_5ba181`

| Step | Action | Expected | Observed | Result |
|------|--------|----------|----------|--------|
| 1 | Open `/agent/<id>` | Running chat header renders backend/model/effort **selects** (not static); no Switch when unchanged | picker=true, claude/sonnet/medium, Switch absent | PASS |
| 2 | Model sonnet→opus | Differing selection reveals **Switch** | Switch appears, enabled | PASS |
| 3 | Click Switch | Applies; picker re-seeds to opus from authoritative record; Switch hides | model=opus, Switch gone | PASS |
| 4 | Effort medium→high | Switch reveals | Switch appears, enabled | PASS |
| 5 | Click Switch | Applied; Switch hides | effort=high | PASS |
| 6 | Backend claude→codex | Model resets to codex default, effort resets to its default (R23) | model=gpt-5.6-sol, effort=medium | PASS |
| 7 | Click Switch | Cross-backend switch applies via native resume | backend=codex, no error, Switch gone | PASS |

Server-side confirmation after the run: `GET /api/sessions/<id>` reported `backend=codex
model=gpt-5.6-sol effort=medium running=true` — every browser switch was actually applied by the
switch-runtime path, not just reflected in local UI state.

## Journey — stopped agent static identity (R24), agent `a_5ba181`

Stopped the agent (`running` became null), reloaded `/agent/<id>`:
- `hasPicker=false`, `hasSwitch=false`
- static identity text = `codex · gpt-5.6-sol · medium` (the last-switched runtime)
- zero console/page errors

PASS — matches the running-only rule; a non-running session shows read-only identity with no
editable picker or Switch.

## Notes / not exercised

- **no_change guard:** the UI reveals Switch only when the selection differs from the live runtime,
  so a user cannot submit an identical runtime from the header; the rejected/`no_change`
  error-and-snap-back branch (R23) is covered by `ChatPanel.test.tsx`, not this browser pass.
- **Catalog-missing identity (R24 second sentence):** covered by component tests; not reproduced in
  browser (would require mutating the catalog under a live agent).
- Effort delivery to a real provider and cross-backend native resume against real CLIs remain the
  standing credentialed release gates; the fake exercises the AgentDeck-side contract only.

## Matrix maintenance (USABILITY-REVIEW §7)

FS-03 already lists J7 (stop/resume/switch) and the header picker is an additional entry point to
the same switch-runtime operation; no new journey charter is required and no uncovered normal-use
surface was found. No missing-requirement candidate.
