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

The run was **interrupted twice by a monthly-spend limit** and resumed twice; on completion **every
journey J1–J12 and all five sweeps S1–S5 were driven** against the running app (J6 is a documented
coverage gap — not mockable). Blockers were orchestrator-verified (curl + browser screenshot).

| Journey | Fixture | Result |
|---|---|---|
| J1 Install & first paint | fresh | **PASS** (styled shell, zero console errors, `run/J1/1-firstpaint.png`) **+ BLOCKER**: Archive-from-fresh crashes the dashboard (`run/J1/2-archive.png`). |
| J2 Onboarding | fresh | **PASS w/ caveats** — with mock creds satisfied it goes straight to the dashboard (no wizard); forced-render wizard is styled except the primary CTA button (POLISH); **completion is blocked** because LaunchStep launches the default `my-app` project whose cwd doesn't exist → 502 (ties to J3/cwd BLOCKER). |
| J3 Chat round-trip | seeded+fakeacp | **PASS** — launch, model choice, streamed reply, busy→done transitions all observed (§0 proof; `run/J3/`). |
| J3b First-launch modal | seeded+fakeacp | **MAJOR (new)** — launching the *first* agent leaves the New-Agent modal stuck open (overlay covers the page). |
| J4 Permission flow | seeded+fakeacp | **PASS** approve/deny/sentinel + status match; **MINOR** double-toast; **MINOR** stopped agent keeps a live Approve/Deny prompt. |
| J5 Grid & layout | seeded | **PASS** — grid stays sane on stop; reorder/density/group-collapse persistence confirmed via J12. (No per-card remove in the UI — only Stop; empty state covered by J1.) |
| J6 Terminal | seeded | **COVERAGE GAP** — not mockable with fakeacp (interactive PTY CLI). Modal correctly disables Terminal for non-claude backends. |
| J7 Stop/resume/switch | lived-in+fakeacp | **PASS** — resume & switch preserve the frozen model (opus-4-7 not reset to default); stop clean; model dropdown swaps per backend. **MINOR** Codex-not-installed shows a raw exec error. |
| J8 Archive & search | lived-in (tagged+untagged) | **PASS** tagged search + resume-from-archive; **BLOCKER** empty/no-match `results:null` (verified); **MAJOR** untagged no-FTS5 build returns raw 500 and leaves stale rows visible; **MINOR** pagination fixed at limit 50. |
| J9 Settings & config | seeded | **BLOCKER** whole surface unstyled (verified); **PASS** round-trip + merge-preserve via the UI; **MAJOR** validation/duplicate errors show bare "HTTP 4xx" discarding the field-naming body; **MINOR** stale server error lingers under a field. |
| J10 Multi-agent messaging | seeded+fakeacp | **PASS** list_agents/send_message/nudge; **MAJOR** unread badge never clears after read (server counter stays stale); **MINOR** no `notification` event for mail; **MINOR** terminal-recipient wording; token only in a 0600 file (observability gap). |
| J11 Failure & recovery | seeded+fakeacp | **PASS** — SSE reconnect accurate, crash→error card, `ignore_cancel` stop escalates <0.2s, all garbage inputs 422/404 (no 500s). **MINOR** force-stopped card labeled "Done/thinking stopped". |
| J12 Restart durability | lived-in | **PASS** — custom order + density + group-collapse and all agents survive a server restart. |

Static sweeps **S1–S5 all COMPLETE** (S2+S5 first pass, 17 findings; S1/S3/S4 second pass, 10 findings).

---

## 4. Findings (this run)

Severity bar: *a first-time or daily user hits this*. Blockers were orchestrator-verified. Findings are
marked **[NEW]** (not in the 2026-07-09 list), **[CONFIRMED]** (known-open, reproduced this build), or
plain (this run's detail on an existing item).

### BLOCKER
- **[CONFIRMED] J8/S1 — Empty / no-match Archive crashes the whole dashboard.** Tagged build,
  `GET /api/archive` and `?q=<no-match>` both return `{"results":null}` (orchestrator curl) → `ArchivePage`
  `.length`/`.map` on null → ErrorBoundary "Something went wrong in dashboard." `run/J1/2-archive.png`.
  Fix: `[]Result{}` when empty + `results ?? []`.
- **[CONFIRMED] J9/S2 — The entire Settings surface renders unstyled.** Tabs are raw buttons; Backends model
  rows overlap the id into the default label ("sonnet-4-6default"). `run/J9/02-backends-tab.png`. Undefined
  class families: `.settings-tabs*`, `.config-editor/-list/-form/-badge/-cwd/-slug/-excerpt/-empty`,
  `.backend-card/-*`, `.model-row/-*`, `.env-editor/-row/-key/-value`, `.btn-danger/-link/-sm`,
  `.string-list*`, `.color-picker/-swatch/-channel`, `.sensitive-wrap`, `.form-hint`.
- **[CONFIRMED] J2/J3 — Fresh first launch fails with a misleading error.** Seeded default project `my-app`
  has cwd `~/Projects/my-app` (missing on a fresh machine); Go can't chdir so exec fails and the message
  blames the adapter: `runtime_start_failed: …/claude-code-acp: no such file or directory` (`run/J2/06`).
  **[NEW] wrinkle:** onboarding LaunchStep launches `my-app` (not the project the user just created in
  ProjectStep), so **onboarding can never complete** — `onboarding_complete` never flips. Fix: pre-check the
  resolved cwd → cwd-named error; ship a default cwd that exists / create on first run; LaunchStep should
  launch the just-created project.

### MAJOR
- **[NEW] J3b — First-agent launch leaves the New-Agent modal stuck open.** On an empty dashboard, launching
  the first agent creates it but the modal never dismisses: `.dialog-overlay` persists (opacity 1,
  pointer-events auto, covers the viewport ≥6s), blocking clicks on the new card / nav until Escape or
  reload. Root cause: `CardGrid` renders two `<NewAgentModal>` in mutually-exclusive branches
  (`ids.length===0` vs the grid); the first launch swaps 0→1 and unmounts the open instance mid-mutation, so
  its `onSuccess→onClose` never fires. A *second* launch closes cleanly. No success feedback; the live
  Launch button invites a duplicate launch. `run/J3b/05-series-final.png`, `06-second-agent.png`. Fix:
  hoist a single modal instance above the branch, or key it so it survives the 0→1 transition.
- **[NEW] S1/S4 — A just-launched (meta-only) agent's transcript marshals `events:null` → the chat panel
  throws.** `transcript/reader.go readAll` returns a nil slice when the file holds only a `session_meta`
  record → `handleTranscript` emits `"events":null`; `transcriptStore.foldTranscript` does `for (const r of
  raw)` → "not iterable". The `.then/.catch` sites swallow it (transcript blank) but `refetchOpenTranscript`
  is awaited without a local catch. Fix: `readAll` init `out := []Event{}`; `foldTranscript(raw ?? [])`.
- **[NEW] J10 — Unread badge never clears after mail is read.** alice→bob `send_message`; bob card shows
  "Mail 1"; bob `check_messages` (mark_read) returns remaining:0; badge still "Mail 1" and a fresh SSE
  snapshot still reports bob `unread_messages:1` — the server counter is stale and no `unread:0` update is
  ever emitted. `run/J10b/step3_badge_cleared.png`. (Elevates the 2026-07-09 stale-badge advisory to a
  reproduced MAJOR.)
- **[NEW] J8 — Untagged (no-FTS5) build returns a raw 500 on search AND leaves stale rows visible.**
  `agentdeck-notags`: `/api/archive?q=…` → HTTP 500 `no such module: fts5`, rendered verbatim in
  `.archive-error` while the previous result rows stay on screen (looks like results + an error).
  `run/J8/2-search-auth.png`. Fix: degrade (LIKE scan / "search unavailable on this build") and clear rows.
- **[CONFIRMED] J9/S5 — Validation & duplicate errors surface as bare "HTTP 4xx", discarding the body that
  names the field.** New role with an existing slug → "Error: HTTP 409"; blank model provider → "Error: HTTP
  400" — the server sends a specific message on `.body` but the editors call `setError(String(e))`.
  `run/J9b/5-dup-role-409.png`, `7-backend-400.png`. (Delete-in-use 409 *is* handled well — a named confirm
  dialog.) Also the general S5 set: `CardGrid` `void putLayout`/`void releaseGroup`, `NotificationsEditor` no
  `onError`, delete `onError` 409-only, `Composer` `void cancelTurn`.
- **[NEW] S3 — The Phase-7 backend cred-checks repeat the claude fragility class → onboarding gate can wedge.**
  `credcheck/claude.go` any non-zero exit (renamed/absent subcommand) → `failed` not `skipped`;
  `codex.go` hardcodes `GET /v1/models` (Azure/gateways 404 → `skipped`, a valid key reads unusable);
  `opencode.go` hardcodes `~/.local/share/opencode/auth.json` (ignores `XDG_DATA_HOME` + macOS path → a
  logged-in mac user reads `not_logged_in`); `openhands.go` treats mere existence of `settings.json` as `ok`
  (false PASS). Each can leave `computeBackendStep` un-Done for a correctly-configured user.

### MINOR / POLISH
- **[NEW] J7** — launching a Codex chat with no `codex-acp` installed shows the raw
  `exec: "codex-acp": … not found in $PATH` (accurate but developer-facing; no setup guidance). `run/J7/04`.
- **[CONFIRMED] J4** — one permission request fires **two** toasts (`waiting_input` + `permission_required`).
  `run/J4/01c-toasts.png`.
- **[NEW] J4** — a stopped agent still shows a live Approve/Deny prompt; clicking it → "Failed to send
  decision — the agent may have stopped." (stale but diagnosable). `run/J4/04c`.
- **[NEW] J10** — no `notification` SSE event fires for incoming mail (badge relies solely on
  `state_update.unread_messages`, which goes stale per the MAJOR above).
- **[NEW] J10** — `send_message` to a running *terminal* agent → "recipient_not_found: No live agent matches"
  — sane but misleading (the agent IS live, just not a messaging target).
- **[NEW] J11** — a user force-stopped card is labeled "Done" (green) with sub-detail "thinking / stopped"
  (the stop itself works in <0.2s; label only). `run/J11/step3_stopped_card.png`.
- **[NEW] J9** — a stale server error ("HTTP 409") lingers under a field alongside a fresh client-side
  validation error, because client-side blocking skips the `setFormError("")` reset. `run/J9b/6`.
- **[CONFIRMED] J8** — archive UI hardcodes limit 50 / offset 0 while showing the true total; matches past 50
  are unreachable (no pagination UI).
- **[NEW] S1** — `handlePutConfig` normalizes `notifications` only when the body includes it; a PUT that
  changes only default project/role can echo `"muted":null` in its response.
- **[NEW] S4** — `BackendsEditor` `Object.entries(backend.models)` / `Object.keys(...)` throw on a
  model-less backend (`models` marshals null; no zod parse coerces it); `Object.entries(cfg.backends)` same
  class one level up. Fix: `?? {}`.
- **[POLISH][NEW] J2** — onboarding primary CTA (`.form-actions button`) is a browser-default button
  (no `button` or `.form-actions button` rule). `run/J2/01-wizard-backend.png`.
- S2 direction-b: **zero dead selectors** — no CSS drift.

### Notable PASSES (an unexercised journey and a passing one must be distinguishable)
- J1 first paint: styled, zero console errors. J3: full mock chat round-trip. J4: approve/deny/sentinel +
  status all correct. J7: frozen-config invariant holds across resume & switch; modal swaps model list per
  backend and disables Terminal for non-claude. J8: tagged search + resume-from-archive. J9: round-trip +
  merge-preserve through the UI. J10: list_agents/send_message/near-instant nudge. J11: SSE reconnect,
  crash→error, stop escalation, garbage-input handling all correct. J12: full layout + agent durability.

### API-layer notes (not user-reachable today, but latent)
- `PUT /api/backends` **replaces** the whole document (a subset curl body dropped the other 3 backends);
  safe only because the editor re-sends the full set. A future partial writer would destroy seeded state
  (INVARIANTS §3). The messaging per-launch token lives only in `<HOME>/mcp/<agent_id>.mcp.json` (0600) —
  no API/UI surface exposes it, so agent-messaging is effectively unobservable to a normal user.

### Coverage gaps (matrix maintenance §7) — several answer the human's charter directly
- **Terminal runtime (J6)** not mockable with fakeacp (interactive PTY CLI) — needs a fake interactive/PTY
  binary. **Agent-initiated messaging** needs a scriptable tool-calling fake (fakeacp never calls the MCP
  tools). **Per-turn budgets (F11)**, **Files/Commands tabs (F10)**, and **CLI parity** have no charter.
  **Terminal driver selection** (tmux/iterm2) has no UI picker; iterm2 is macOS-only.

### Evidence
Committed under [`usability-review-2026-07-10-evidence/`](usability-review-2026-07-10-evidence/) — one
subfolder per journey (`J1 J2 J3 J3b J4 J5 J7 J8 J9 J9b J10 J10b J11 J12`, 93 screenshots). The live harness
(binaries, fakeacp shim, fixtures, full `run/`) lived under a gitignored `.review/` and is not committed.
