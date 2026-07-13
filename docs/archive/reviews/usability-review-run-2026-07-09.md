# Usability Review — Run Report (2026-07-09)

Behavior-driven review per [`USABILITY-REVIEW.md`](USABILITY-REVIEW.md). Evidence is observed
behavior of the **running** binary, not the diff. Built via the real build line
(`go build -tags sqlite_fts5`) plus the untagged fallback for J8; UI embedded via `make embed`.
Fixtures under a review-owned `AGENTDECK_HOME` temp root; deterministic agent = `fakeacp` symlinked
onto PATH as `claude-code-acp`. Browser rung used: **Playwright driving the environment's Chromium**
(screenshots + console-error capture) for every visual checkpoint.

## Executive summary (top 5)

1. **BLOCKER (J8/S1) — the Archive page crashes the whole dashboard on a fresh install.** Empty or
   no-match archive returns `results: null`; the UI does `results.length` → `TypeError: Cannot read
   properties of null` → ErrorBoundary "Something went wrong in dashboard." Reachable by a first-time
   user clicking **Archive** in the nav. Exact class as the original `order: null` escape. *Verified.*
2. **MAJOR (J9/S2) — the entire Settings/config surface renders unstyled.** Roles/Projects/Backends
   editors and their forms reference `.config-*`, `.settings-tabs*`, `.backend-card`, `.model-row`,
   `.env-row` etc. that are defined in **no** stylesheet; tabs are default browser buttons and the
   Backends editor's controls **overlap** ("sonnet-4-6default"). The wizard escape migrated here.
   *Verified.*
3. **MAJOR (J3/S3) — first launch from the shipped defaults fails with a misleading error.** The
   seeded default project ships `cwd: "~/Projects/my-app"`, which does not exist on a fresh machine.
   The launch fails `runtime_start_failed: fork/exec …/claude-code-acp: no such file or directory` —
   blaming the ACP binary, not the missing directory — so the user cannot self-diagnose. (`~` *is*
   expanded; the dir simply doesn't exist.) *Verified.*
4. **MAJOR (S5) — core mutating actions swallow failures silently.** release-group and cancel-turn
   are bare `void mutation()` with no `.catch`; notifications save has no `onError`; role/project
   delete only handles 409; config editors show "HTTP 400" while discarding the field-level body.
   A failed action looks like it worked. INVARIANTS §8. *Static-confirmed; behavior-consistent.*
5. **ADVISORY (S3) — the claude credcheck is fragile to CLI variance (re-opens the `--no-color`
   class).** Unknown-flag fallback is gated on one exact English substring; `auth status` subcommand
   presence is assumed; exit-0 + absence of two phrases is treated as "ok" (a false PASS).
   ENV-DEPENDENT.

## Checkpoint matrix

| # | Journey | Fixture | Verdict | Notes |
|---|---------|---------|---------|-------|
| J1 | Install & first paint | fresh | **PASS** | 0 console/page errors, 202 CSS rules, styled shell. `run/J1-fresh/paint.png` |
| J2 | Onboarding wizard | fresh | **SKIPPED(env)** | Server auto-seeds working defaults → `onboarding.satisfied=true` → wizard not shown; forced-unsatisfied + real-CLI branches are credential-gated. |
| J3 | First launch + chat | seeded+fakeacp | **PASS** | launch→prompt→stream→turn_end via fakeacp; chat surface styled. Transcript `events:null` on a meta-only session does **not** crash (guarded `.catch`). |
| J4 | Permission flow | seeded+fakeacp | **PASS** | prompt renders; Approve → sentinel written, `permission_resolved`+`turn_end`, state→idle, no stuck/double prompt. `run/J4-*` |
| J5 | Grid & layout | seeded | **PARTIAL PASS** | Empty state ("No running agents") ✓, populated grid + columns/gap/group controls ✓ styled. Reorder-persist-across-restart not driven. |
| J6 | Terminal runtime | seeded | **SKIPPED(env)** | Terminal launches the *interactive* CLI (`claude`) under a PTY; not present in this env (fakeacp is ACP-only). |
| J7 | Stop/resume/switch | lived-in+fakeacp | **PASS (resume)** | Resume preserves identity (backend/model/interface). Switch verb not driven. |
| J8 | Archive & search | lived-in | **BLOCKER (empty) + PASS (populated)** | Empty→crash (finding 1). One-session list + `q=Atlas` search return correct results and render. **Untagged build:** search returns raw `no such module: fts5` (finding 7). |
| J9 | Settings & config | seeded | **MAJOR** | Unstyled config surfaces (finding 2). Round-trip save not fully driven. |
| J10 | Multi-agent messaging | seeded+fakeacp×2 | **NOT RUN** | Deferred (budget). |
| J11 | Failure & recovery | seeded+fakeacp | **PASS** | Kill server → connection dot green→orange, grid holds last state; restart → SSE reconnects, state accurate. Garbage-form-input not driven. `run/J11/*` |
| J12 | Restart durability | left by J3–J11 | **PASS** | After server restart the agent + status persist and re-render accurately. |

## Findings (severity-ordered)

### BLOCKING

- **J8/S1 — Empty/no-match Archive crashes the dashboard.**
  `internal/archive/archive.go:155` `scanResults` returns `var out []Result` (nil) when there are no
  rows → `Search` returns `Results: nil` → JSON `results: null`. `ui/src/features/archive/ArchivePage.tsx:74,115,120`
  does `setResults(resp.results)` then `results.length`/`results.map` with no guard. MSW mock returns
  `results: []` (ArchivePage.test.tsx:47) so tests never see it. **Repro (fresh fixture):** start
  server, open `/archive` (or any query with no match) → ErrorBoundary "Something went wrong in
  dashboard." **Evidence:** `run/J8-archive-empty/paint.png`; console `TypeError: Cannot read
  properties of null (reading 'length')`; `curl /api/archive` → `{"total":0,…,"results":null}`.
  **Fix direction:** return `[]Result{}` when empty (as `files_commands.go` already does), or
  `resp.results ?? []` in the UI.

- **J9/S2 — Settings/config surfaces render unstyled.**
  The project uses hand-written global CSS (`ui/src/styles/global.css`), no Tailwind. Settings and its
  editors reference classes defined nowhere: `.settings-tabs`, `.settings-tabs-list`,
  `.settings-tab-content`, `.config-editor`, `.config-list*`, `.config-form`, `.backends-editor`,
  `.backend-card`, `.backend-*`, `.model-row`, `.env-row`, `.string-list`, `.btn-danger/-link/-sm`,
  etc. (`SettingsPage.tsx:11-24`, `RolesEditor.tsx`, `ProjectsEditor.tsx`, `BackendsEditor.tsx:183-223`,
  `ModelRow.tsx:36-91`). **Repro (seeded):** open `/settings` → tabs are default buttons, roles list
  is a bare bulleted `<ul>`; Backends tab → labels/inputs **overlap** ("sonnet-4-6default"). **Evidence:**
  `run/J9-settings/paint.png`, `run/J9-backends/click1-Backends.png`. **Fix direction:** define the
  referenced selectors (mirror the already-styled `.dialog-*`/`.wizard-*` families).

- **J3/S3 — First launch from shipped defaults fails with a misleading error.**
  Seeded default project `projects/my-app.json` ships `cwd: "~/Projects/my-app"` (nonexistent on a
  fresh machine). Launch → `{"error":{"code":"runtime_start_failed","message":"runtime: start :
  fork/exec …/claude-code-acp: no such file or directory"}}` — the message names the adapter binary,
  not the missing directory (Go reports the `cmd.Dir` ENOENT against the exec target). **Repro (seeded):**
  point default project at a missing dir, POST `/api/sessions` with defaults. **Verified** the same
  launch succeeds once the dir exists (so `~` expansion works — the issue is the missing dir + the
  misleading message). **Fix direction:** pre-check the resolved cwd exists and return a
  cwd-specific error (`project directory <path> does not exist`); ship a default project cwd that
  exists (or create it on first run), or leave `default_project` unset until onboarding sets a real one.

- **S5 — Core mutating actions swallow failures silently (INVARIANTS §8).**
  `ui/src/components/grid/CardGrid.tsx:94` `void releaseGroup(...)` (also discards the
  `stopped[].ok=false` partial-failure body); `ui/src/components/chat/Composer.tsx:37` `void
  cancelTurn(agentId)`; `ui/src/features/settings/NotificationsEditor.tsx:20` `putConfig.mutate(...)`
  with no `onError` (checkbox snaps back silently); `RolesEditor.tsx`/`ProjectsEditor.tsx` delete
  handlers only branch on `status===409` with no `else`; config editors (`RolesEditor:38/44`,
  `ProjectsEditor:50/59`, `BackendsEditor:176`) render `String(e)` = "HTTP 400" and throw away the
  structured `.body` that names the field. **Repro:** make any of these calls return 500/409/offline
  → no toast, no feedback. **Fix direction:** route every mutation error through `pushError`; read
  `e.body.error.message`/`e.body` like `NewAgentModal.tsx` already does.

### ADVISORY

- **S3 — claude credcheck fragile to CLI wording/subcommand variance (re-opens `--no-color`).**
  `internal/backend/credcheck/claude.go:30` gates the flag-drop fallback on the exact substring
  `unknown option '--no-color'`; any other phrasing (localized, "unrecognized option", no quotes)
  skips the retry → a valid login reports **failed**. `:43,54` assumes the `auth status` subcommand
  exists and treats any non-zero exit lacking two phrases as failed. `:46-50` treats exit-0 + absence
  of "not logged in"/"not authenticated" as **ok** (false PASS for a logged-out CLI with different
  wording). ENV-DEPENDENT. Fix: match on a broad unknown-flag regex; map unknown-subcommand/unexpected
  exit to `skipped`, not `failed`; require a positive logged-in signal for `ok`.

- **S3 — `--settings` is appended to the claude-code-acp launch with no capability check/fallback.**
  `internal/backend/adapter.go:130` + `internal/runtime/chat.go:99` unconditionally append
  `--settings <path>`; an older/variant adapter that rejects it fails **every** launch, and the error
  doesn't name the flag. ENV-DEPENDENT. Fix: probe support or degrade to launch-without-hooks with a
  clear message.

- **J8 (untagged build) — no-FTS5 fallback surfaces a raw internal error on search.**
  On a plain `go build` (no `sqlite_fts5`), `/api/archive?q=…` returns 500 `archive: count search: no
  such module: fts5`; the ArchivePage prints that raw string. Mitigated: `install.sh`/`make build`
  both carry the tag, so no shipped binary hits it — but the fallback should degrade (LIKE scan or a
  "search unavailable on this build" message) rather than 500. **Evidence:** `run/J8-untagged-search/2-after-search.png`.

- **S1 — Hand-edited config with missing collection maps crashes editors.**
  A `backends.json` backend with no `models` key, or `config.json` `notifications` with no `muted`,
  parses fine (no corrupt-fallback) and serializes `null`; `BackendsEditor.tsx:240,268`
  `Object.entries/keys(backend.models)` and `NotificationsEditor.tsx:51,55` `notifications.muted[…]`
  throw. Lower likelihood (requires hand-edit). Fix: normalize nil maps on read, or `?? {}` at the
  consumer.

- **S2 — Secondary undefined classes (partial degradation).**
  `.transcript-item`, `.turn-end`, `.assistant-message`, `.tool-call`, `.permission-error`
  (chat renderers), `.app-logo` (header), `.interface-controls/-option/-disabled`, `.gate-loading`
  are referenced but undefined; parent wrappers are defined so degradation is partial (missing
  spacing/emphasis). POLISH: `.onboarding-overlay` redundant.

- **S5 — Secondary error-surfacing gaps.**
  `CardGrid.tsx:41` layout autosave `void putLayout(...)` no catch (silent persistence loss on
  reorder/density); onboarding `LaunchStep/ProjectStep/BackendStep` use `setError(String(e))`
  (discard body); permission/send catch blocks show fixed strings discarding the real `err.message`.

### Verified non-finding (documented)

- **Transcript `events:null` does NOT crash the chat panel.** S1 flagged a meta-only transcript
  (`internal/transcript/reader.go:49` returns nil when `include_meta=false` filters the seq:0 record)
  as a BLOCKER. In the running app the ChatPanel fetch (`ChatPanel.tsx:50`) is wrapped in
  `.catch(() => {})`, so `foldTranscript(null)` throws internally and is swallowed; `events` stays
  `[]`. No crash, no console error. Downgraded from BLOCKER. (Cosmetically the API should still emit
  `[]`.)

## Coverage gaps (§7 — matrix must track the product)

- **Phase 7 (OpenCode/OpenHands backends) shipped user-facing surfaces with no journey charter added**
  — the four-type Backends editor, BackendStep's `LLM_API_KEY`/`LLM_BASE_URL` fields for
  `openhands-acp`, and NewAgentModal's terminal-gating for non-claude backends. §7 maintenance
  requires a charter per shipped surface. (This run confirmed the four types render in the Backends
  editor, but no dedicated journey exists.)
- **F10 File & command tracking** has no dedicated journey; the live Files/Commands tabs (with open
  advisories) go behaviorally unexercised.
- **F2 task-group create + release-group failure** and **F11 notification toasts** (done/waiting_input,
  mutes) are not journey checkpoints.

## Notes for the human

- J2 (onboarding wizard) and J6 (terminal) and the J2 real-CLI credential branches are ENV-DEPENDENT
  and were SKIPPED in this sandbox (no logged-in `claude`/interactive CLI). J10 (messaging) and the
  reorder-persist / switch / garbage-input sub-checks were deferred for budget, not run.
- All BLOCKER repros were spot-replayed by the orchestrator before reporting (per §5). The single
  BLOCKER (empty archive) reproduces cleanly.
