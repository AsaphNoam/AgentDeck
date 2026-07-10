# Usability review run — 2026-07-10 (mock-driven / config-dependent focus)

Second behavior-driven top-to-bottom `/usability-review`. Theme (per the human's charter): **exercise the
features that normally require a loaded Claude Code / Codex config — creating a chat, choosing models,
sending messages, inter-thread actions — by standing the agent CLI in with a deterministic mock, so every
feature reaches usability testing instead of being SKIPPED as ENV-DEPENDENT.**

Prime directive held: findings come from the **running** app, not the diff.

---

## 0. The mock that unlocks the config-dependent surface (proven, reusable)

Prior runs marked the chat/permission/switch/messaging journeys ENV-DEPENDENT and skipped them (no real
`claude-code-acp` + login). This run closes that gap with the repo's own deterministic ACP peer,
`internal/runtime/testdata/fakeacp`, wired as the adapter binary:

**Recipe (verified end-to-end before any journey ran):**
1. `go build -o claude-code-acp ./internal/runtime/testdata/fakeacp`; put its dir first on `PATH`.
   The server launches a chat agent via `exec.Command("claude-code-acp", …)` resolved through `PATH`
   (`internal/runtime/chat.go::spawnCmd`), so the shim is used with no code change. Unknown flags the
   real adapter would take (`--settings`, `--model`, `--resume`) are ignored by the shim.
2. The launched process inherits the server's env (`os.Environ()`), so the scenario is selected by
   starting the **server** with `FAKEACP_SCENARIO=<name>` (one scenario per server; give each journey its
   own port + home). Scenarios: `stream_text`, `tool_flow`, `permission[_approve|_deny|_timeout]`
   (+ `FAKEACP_SENTINEL`), `crash_midturn`, `ignore_cancel`, `big_frame`, `malformed_then_valid`.
3. Onboarding cred-checks pass because the shim is literally named `claude-code-acp`.

**Proof of the round-trip (curl + browser):** launch → `POST /prompt` produced, on `/api/events`:
`state_update busy/"thinking"/UserPromptSubmit` → three `new_message` `assistant_text` deltas
("Sure, ", "I'll ", "do that.") → `state_update idle/Stop` → `new_message turn_end/end_turn`, and the
dashboard card rendered the streamed text with a Done badge and context %. So chat creation, model
selection, message send, streamed render, and status transitions ARE usability-testable under the mock.

**What the mock still cannot cover (honest boundaries — reported as coverage gaps, not passes):**
- **Terminal runtime (J6):** launches the *interactive* `claude` CLI in a PTY, not the ACP adapter — the
  shim does not stand in. Needs a separate tiny fake interactive/PTY binary.
- **Agent-initiated messaging (J10):** fakeacp streams canned text and never *calls* the MCP tools
  (`send_message`/`check_messages`), so agent→agent traffic must be injected through the messaging
  API/`/mcp` to observe the UI reaction; a fully autonomous multi-agent conversation needs a scriptable
  tool-calling fake.
- **Real-CLI variance (J2 cred branches):** not-logged-in / old-flag branches stay ENV-DEPENDENT.

## 1. Harness

- Binaries built the ship way: tagged `-tags sqlite_fts5` (real build line from `install.sh`/`make`), plus
  the untagged no-FTS5 fallback for J8. UI built (`npm run build`) + embedded (`make embed`) — the checkout
  shipped only a 394-byte placeholder `internal/server/ui/dist/index.html`, so the UI had to be built for
  any visual test (noted; not a product finding).
- Fixtures under a review-owned `AGENTDECK_HOME` temp root: `fresh/` (empty), `seeded/` (config + a REAL
  project cwd so launch doesn't hit the missing-`~/Projects/my-app` blocker, 0 agents),
  `lived-in/` (seeded + 3 scripted-fakeacp archived chat sessions).
- Browser: Playwright (`playwright-core`) driving the environment's Chromium
  (`/opt/pw-browsers/chromium-1194`); console-error + pageerror capture; `waitUntil:'load'` (SSE never
  idles). Screenshots under `.review/run/`.
- Orchestration: main thread = orchestrator only; static sweeps S1–S5 + journeys J1–J12 fanned to
  subagents with isolated ports/home copies.

---

## 2. Coverage-gap analysis (journey matrix §3 vs MAP feature→phase table) — done before running

| Feature (MAP) | Phase | Journey coverage | Gap |
|---|---|---|---|
| F1 dashboard grid | 2 | J1, J5 | ok |
| F2 terminal runtime | 6 | J6 | **can't mock** (interactive PTY CLI) — coverage gap |
| F3 chat streaming | 1+2 | J3 | ok (now mock-covered) |
| F4 launch modal | 1+3 | J2, J3 | ok |
| F5 config CRUD | 3 | J9 | ok |
| F6 onboarding | 3 | J2 | cred branches ENV-DEPENDENT |
| F7 switch runtime | 6 | J7 | ok (now mock-covered) |
| F8 messaging | 5 | J10 | partial — agent-initiated traffic needs a tool-calling fake |
| F9 archive/search | 4 | J8 | ok (tagged + untagged) |
| F10 file/command tracking | 4 | (implicit in J3/J8) | **no dedicated charter** — Files/Commands tabs untested surface |
| F11 nudger/budgets/notifications | 5 | J4, J10 | **budgets (per-turn 15, budget_exceeded) untested**; notification mute delivery thin |
| F12 config/onboarding | 3 | J2, J9 | ok |
| F13 future candidate | 7 | — | n/a |

Additional shipped surfaces without a dedicated charter (flagged for matrix maintenance, §7):
- **Task groups** (Phase 6.6): release-group, collapsible sections, move-to-group — only partially in J5.
- **Terminal driver selection** (tmux/iterm2): no UI picker (known advisory); iterm2 is macOS-only (can't
  test on this Linux host).
- **Context-menu actions** clone / rename / move-to-group — only delete/stop touched in J5.
- **Multi-backend launch** (Codex/OpenCode/OpenHands): choosing a non-claude backend at launch — folded
  into J7 checkpoint 4.
- **CLI parity** (`agentdeck launch/resume/reindex`): the CLI is a first-class user surface (CLI≡modal)
  with no journey — untested.

---

## 3. Checkpoint matrix

**Run cut short:** a monthly-spend limit terminated 7 of 8 journey/sweep subagents mid-run. One sweep
(S2+S5) completed fully; the rest returned partial results. The orchestrator (main thread) directly
verified the two headline BLOCKERs (curl + browser screenshot). Coverage is therefore PARTIAL — the
matrix below distinguishes verified / partial / not-reached honestly.

| Journey | Fixture | Result |
|---|---|---|
| J1 Install & first paint | fresh | **PARTIAL-PASS** — first paint styled, zero console errors (subagent). Archive-from-fresh reproduces the BLOCKER (verified, screenshot `run/J1/2-archive.png`). |
| J2 Onboarding | fresh | NOT-REACHED (subagent died before Settings/onboarding drive). |
| J3 Chat round-trip | seeded+fakeacp | **PARTIAL-PASS** — launch + modal + streamed reply captured (`run/J3/01-04`); the mock round-trip is independently proven (§0). Unconfirmed possible finding: New-Agent dialog overlay persisted after submit and intercepted the card click. |
| J4 Permission flow | seeded+fakeacp | NOT-REACHED (same subagent died before J4). |
| J5 Grid & layout | seeded | NOT-REACHED (subagent died after J1). |
| J6 Terminal | seeded | **COVERAGE GAP** — not mockable with fakeacp (interactive PTY CLI). |
| J7 Stop/resume/switch | lived-in+fakeacp | NOT-REACHED (subagent died writing its Playwright script). |
| J8 Archive & search | lived-in (tagged+untagged) | **BLOCKER VERIFIED** (empty/no-match `results:null`, orchestrator curl). Untagged fallback + resume-from-archive NOT-REACHED. |
| J9 Settings & config | seeded | **BLOCKER VERIFIED** — whole Settings surface unstyled; Backends model rows overlap ("sonnet-4-6default"), tabs are raw buttons (`run/J9/02-backends-tab.png`). Round-trip/merge checks NOT-REACHED. |
| J10 Multi-agent messaging | seeded+fakeacp | **PARTIAL-PASS** — messaging reachable & functional via `/mcp` + token: `list_agents` showed `bob@reviewer@my-app`, `send_message` succeeded (`m_4ae0ac`). Badge-clear / nudge / stale checks NOT-REACHED. |
| J11 Failure & recovery | seeded+fakeacp | NOT-REACHED (subagent died locating the SSE indicator). |
| J12 Restart durability | lived-in | NOT-REACHED. |

Static sweeps: **S2+S5 COMPLETE** (17 findings). **S1/S3/S4 PARTIAL** (subagent died after enumerating
the Go-side nil-slice set, before the UI cross-check).

---

## 4. Findings (this run)

Severity bar: *a first-time or daily user hits this*. Blockers below were orchestrator-verified.

### BLOCKER (verified)
- **J8 — Empty / no-match Archive crashes the whole dashboard.** On the tagged build, `GET /api/archive`
  and `?q=<no-match>` both return `{"results":null}` (orchestrator curl, seeded fixture, 0 sessions).
  Clicking **Archive** on a fresh install throws in `ArchivePage` (`results.length`/`.map` on null) →
  ErrorBoundary "Something went wrong in dashboard." Evidence: `run/J1/2-archive.png`. Reproduces the
  2026-07-09 known-open finding — **still unfixed.** Fix: return `[]Result{}` when empty; `results ?? []`
  in the UI.
- **J9 / S2 — The entire Settings surface renders unstyled.** Backends/Roles/Projects/Notifications tabs
  are raw browser buttons; the Backends editor's model rows overlap the id into the default label
  ("sonnet-4-6default", "gpt-5.5default", "sonnet-4-5default"); env/model widgets collapse to a raw stack.
  Evidence: `run/J9/02-backends-tab.png`. S2 enumerated the undefined class families:
  `.settings-tabs*`, `.config-editor/-list/-form/-badge/-cwd/-slug/-excerpt/-empty`, `.backend-card/-*`,
  `.model-row/-*`, `.env-editor/-row/-key/-value`, plus shared `.btn-danger/-link/-sm`, `.string-list*`,
  `.color-picker/-swatch/-channel`, `.sensitive-wrap`, `.form-hint`. Reproduces the 2026-07-09 known-open
  finding — **still unfixed.** Fix: define these selectors (mirror the styled `.dialog-*`/`.wizard-*`).

### MAJOR (S5 — silent / misleading mutation failures, static-confirmed against running behavior)
- `CardGrid.tsx:41` `void putLayout` and `:94` `void releaseGroup` — reorder/density/group-collapse
  persistence and Release-group fail **silently**; change is lost on reload with no error.
- `NotificationsEditor.tsx:20` `putConfig.mutate` with no `onError` — a notification toggle that fails to
  save snaps back silently.
- `RolesEditor.tsx:53-64` / `ProjectsEditor.tsx:66-77` delete `onError` handles only `409`; any 500/403/
  network delete failure (and the `force:true` retry) is swallowed.
- Config editors + onboarding steps (`RolesEditor:38,44`, `ProjectsEditor:50,59`, `BackendsEditor:176`,
  `ProjectStep:41`, `BackendStep:100`, `LaunchStep:53`) surface `String(e)` = "HTTP 500", discarding the
  `err.body.error.message` that names the offending field — `NewAgentModal` already does this correctly;
  generalize it.

### MINOR / POLISH (S2/S5)
- New-Agent modal runtime picker (`.interface-controls/-option/-disabled`) undefined — partial degradation
  choosing chat vs terminal.
- Secondary chat/renderer classes undefined but co-occur with a defined parent (`.transcript-item`,
  `.turn-end`, `.assistant-message`, `.tool-call`, `.permission-error`, `.app-logo`, `.grid-view`).
- `Composer.tsx:37` `void cancelTurn` — a failed Cancel is silent.
- `PermissionPrompt.tsx:19-21` / `Composer.tsx:16-18` show a fixed "the agent may have stopped" string,
  discarding the real error message.
- S2 direction-b: **zero dead selectors** — no CSS drift to clean up.

### Coverage gaps (matrix maintenance, §7) — several directly answer the human's charter
- **Terminal runtime (J6) is not usability-testable with the current mocks** — it launches the interactive
  `claude` CLI in a PTY, not the ACP adapter. Needs a tiny fake interactive/PTY binary registered as the
  interactive command. Today the whole terminal surface is only reachable with a real login.
- **Agent-initiated messaging (J10) needs a scriptable tool-calling fake.** fakeacp streams canned text and
  never calls `send_message`/`check_messages`; agent→agent traffic had to be injected via `/mcp`. The
  autonomous nudge/badge/budget loop can't be driven without a fake that calls the MCP tools.
- **Per-turn budgets (F11: budget 15, `budget_exceeded`) — no journey.**
- **Files/Commands tabs (F10) — no dedicated charter**, despite multiple standing advisories about them.
- **CLI parity (`agentdeck launch/resume/reindex`) — no journey**, though CLI≡modal is a shipped contract.
- **Terminal driver selection** (tmux/iterm2) has no UI picker; iterm2 is macOS-only (untestable here).

### Unconfirmed (subagent died before confirming — do NOT treat as fact)
- **J3 — New-Agent dialog overlay may persist after submit and intercept the next card click.** Seen once
  in `run/J3/04-after-launch.png`; the subagent was terminated before a second repro. Re-drive to confirm.

### Evidence index
`run/J1/{1-firstpaint,2-archive,3-back}.png`, `run/J3/{01-dashboard,02-modal,03-modal-filled,04-after-launch}.png`,
`run/J9/{01-roles,02-backends,03-projects,04-notifications}-tab.png`. Committed copies:
[`usability-review-2026-07-10-evidence/`](usability-review-2026-07-10-evidence/) (same `J#/…png` layout).
The live harness (binaries, fakeacp shim, fixtures, full `run/`) lived under a gitignored `.review/` and is
not committed.
