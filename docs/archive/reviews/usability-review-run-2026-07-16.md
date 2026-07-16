# Usability Review Run — 2026-07-16

**Scope:** Behavior-driven journeys from `USABILITY-REVIEW.md`, focused on the core first-run and
daily paths plus the newly shipped project shared resources (FS-11). Driven against the running app,
not the diff. This run does not claim PASS for any journey it did not observe (see Coverage).

**Review surface:** production `sqlite_fts5` binary (`make build` → `bin/agentdeck`) and an additional
untagged fallback binary; Playwright driving the installed Google Chrome (browser ladder rung 1:
screenshots + console-error capture); deterministic fake ACP peer registered as the `claude-acp`
backend through a PATH shim (`claude-agent-acp` answers the credential probe and execs `fakeacp` for
the ACP session); isolated `AGENTDECK_HOME` fixtures per journey; localhost API/state checks; and the
Phase A static sweeps S1–S5 (delegated, read-only). Product code was not changed.

Screenshots and driver scripts were written to a review-owned temp run directory
(`/tmp/agentdeck-usability-*/run/`); it is ephemeral. Every finding below is reproducible from its
steps alone (fixture + steps + expected vs observed), per the evidence-discipline rule.

## Executive summary (top findings)

1. **MAJOR / Must fix (NEW) — a hand-edited `backends.json` missing the `backends` key kills the whole
   dashboard.** The server serves `{"backends":null}` for a syntactically valid but incomplete file
   (no default-fallback), and `Settings → Backends` calls `Object.entries(cfg.backends)` unguarded →
   `TypeError` → the app-level ErrorBoundary replaces the entire dashboard with "Something went wrong
   in dashboard." This is the historical nil-map → UI-crash escape class, reproduced live.
2. **MAJOR / Must fix (extends a known advisory) — the durable transcript never records the user's
   prompts.** Only assistant/tool/turn events are persisted; there is no user-message event type. The
   chat panel shows your text optimistically, but after any reload/resume/archive the durable
   transcript replaces the view (FS-03.R12) and your side of the conversation is gone. Because user
   text is not in the transcript, FS-05.R5 transcript search cannot find a session by what you asked.
   The 2026-07-12 run logged the reload-disappearance as MINOR; the archive-one-sided and
   unsearchable-user-text dimensions raise the impact.
3. **MINOR / Worth fixing (newly observed) — credential-check failures show raw machine codes with a
   misleading fix hint.** Both branches render e.g. `Credential check: skipped — cli_not_installed.
   Please check your settings and try again.` and `failed — not_logged_in`. The snake_case codes and
   the generic "check your settings" misdirect (a missing binary / not-being-logged-in is not a
   settings problem, and the copy names no next step).
4. **MINOR / Worth fixing — the config-source panel renders unstyled.** `ConfigSourcePanel`'s
   `source-unbound` / `source-bound` / `source-preview` / `source-field-value` / `src-override-*`
   classes have no CSS definitions; the `.source-unbound` element was observed with no
   border/padding/background. Adds to the previously-noted `interface-controls` / `cred-chip` cluster.
   Degrades (functional) rather than crashes.

## Journey matrix

| Journey | Verdict | Observed evidence |
|---|---|---|
| J1 Install & first paint | **PASS** | Real `make build` binary; fresh isolated home; UI rendered (`#root` populated) with **zero** console/page/failed-request errors and a genuinely styled shell (Inter font, `#f8fafc` body, styled buttons, 291 CSS rules / 2 sheets — non-default computed styles). Fresh state boots into the onboarding wizard. |
| J2 Onboarding end-to-end | **PASS (walk) + finding** | Full wizard walked backend → project → config → launch → dashboard with a launched agent at Idle, **zero** console/page errors on every step. Credential branches observed: `cli_not_installed` (Claude adapter absent) and `failed — not_logged_in` (toggle shim) → raw-code copy finding. The logged-in-with-real-credentials pass is ENV-DEPENDENT and was satisfied deterministically via the fake backend, not a real provider (SKIPPED as a real-CLI check). |
| J3 First launch + chat | **PASS** | Sent a prompt; assistant reply "Sure, I'll do that." streamed and rendered in the transcript; zero console errors. Idle→busy→idle was not separately sampled (the fake reply completes faster than the poll window); completion is confirmed by the rendered turn. |
| J4 Permission prompt | **NOT EXERCISED** | Requires the `permission` fakeacp scenario and a live approve/deny/timeout drive; not run this session. |
| J5 Grid & layout | **PASS (core)** | Empty grid renders a real "No running agents" empty state. Density (Columns 3→5) persisted across page reload **and** a server restart (`/api/layout` = `perRow:5` after restart); zero console errors. Drag-reorder of cards was not exercised (headless drag is flaky). |
| J6 Terminal runtime | **NOT EXERCISED** | Live xterm typing/resize/detach/reattach not driven this session. |
| J7 Stop / resume / switch | **NOT EXERCISED** (identity preserved not separately driven) | Stop/restart persistence was observed incidentally (archive + layout survive restart); the resume/switch identity/model/add_dirs matrix was not driven. |
| J8 Archive & search | **PASS (both builds) + finding** | Tagged build: archive lists the stopped session with correct inactive metadata; transcript search matches assistant words (`sure`, `do that`) but **no** user-prompt words (`hello`, `agent`, `please`, `respond`) → transcript finding. Untagged fallback build: metadata search (`atlas` → `matched_in:["metadata"]`) works, transcript-only term (`sure`) returns 0 without erroring (FS-05.R6). Archived transcript view renders read-only with only the assistant side. |
| J9 Settings & config (+ FS-11) | **PASS + finding** | Project title round-trips across reload; empty-title submit surfaces "title is required" and holds the dialog open (no silent no-op). FS-11: the "Shared resources directory" field is present, **read-only**, and shows a real owner-only absolute path outside the repo (`…/project-resources/<id>`). Backends nil-map crash reproduced here (finding 1). |
| J10 Multi-agent + messaging | **NOT EXERCISED** | Two-agent send_message / nudge / unread badges not driven this session. |
| J11 Failure & recovery | **NOT EXERCISED** | Server-kill reconnect / agent-crash card / garbage-into-every-form not driven this session. |
| J12 Restart durability | **PARTIAL** | Layout density and the archived session both survived a real server restart (observed in J5/J8); the full "everything the UI showed is still true" sweep was not driven. |

## Findings (detail)

### 1. MAJOR / Must fix — incomplete `backends.json` → `backends:null` → dead dashboard (NEW)
- **Where:** `internal/config/backends.go` `ReadBackends` + `internal/server/handlers.go` (a valid-but-
  incomplete file is neither `ErrNotFound` nor `ErrCorrupt`, so the `DefaultBackends()` fallback is
  skipped); `ui/src/features/settings/BackendsEditor.tsx:87` `Object.entries(cfg.backends)` unguarded
  (also `:256` `Object.entries(backend.models)` for a `models:null`).
- **Repro:** complete onboarding (so `onboarding_complete` is set), then hand-edit `backends.json` to
  `{"version":2}` (the product documents config as hand-editable JSON), restart, open `Settings →
  Backends`. `GET /api/backends` returns `{"version":2,"backends":null}`; the tab throws
  `TypeError: Cannot convert undefined or null to object` at `Object.entries`; the app-level
  ErrorBoundary shows "Something went wrong in dashboard" (whole dashboard, not just the tab).
- **Expected:** an incomplete backends file is normalized to defaults (like the missing/corrupt cases)
  and/or the UI guards `cfg.backends ?? {}`; the surface should not die, and any error should name the
  offending file.
- **Relevant reqs / class:** TS-03 serialization contract; FS-04 backends; INV nil-collection class.

### 2. MAJOR / Must fix — user prompts are not in the durable transcript (extends the 2026-07-12 advisory)
- **Where:** `internal/runtime/chat.go` `SendPrompt` emits no user event; `internal/runtime/event.go`
  has no user/prompt event type (only `assistant_text`, `tool_call`, `tool_result`, `diff`,
  `permission_*`, `session_meta`, `turn_end`, `error`, `backend_switch`).
- **Repro:** launch, send "Hello agent, please respond.", get a reply. `GET
  /api/sessions/<id>/transcript` contains only `assistant_text` + `turn_end`. Archive view of the
  session shows only the assistant text. `GET /api/archive?q=<assistant word>` matches; `q=<user
  word>` (`hello`/`agent`/`please`/`respond`) returns 0.
- **Expected:** the durable transcript includes the user's prompts (FS-03.R7 shows them optimistically;
  FS-03.R12 replaces the view with the durable transcript on reconnect → user text vanishes), and
  FS-05.R5's promised transcript-content search covers what the user typed.
- **Missing requirement:** FS-03 §2.1 does not list a user-prompt transcript event; the fix should add
  one (persist user prompts) or FS-03/FS-05 should explicitly document the omission.

### 3. MINOR / Worth fixing — credential-failure copy shows raw codes and a misleading remediation
- **Where:** `ui/src/features/onboarding/steps/BackendStep.tsx` cred-status render.
- **Repro:** onboarding backend step with the pinned CLI absent → `Credential check: skipped —
  cli_not_installed. Please check your settings and try again.`; not-logged-in → `failed —
  not_logged_in`.
- **Expected:** human copy naming the specific problem and next step (install the adapter / run guided
  sign-in / add the API key), not a snake_case code plus "check your settings."
- **Relevant req:** FS-04.R17 (copy unspecified — candidate to pin).

### 4. MINOR / Worth fixing — config-source panel renders unstyled
- **Where:** `ui/src/features/settings/ConfigSourcePanel.tsx` — `source-unbound`, `source-bound`,
  `source-preview`, `source-field-value`, `src-override-model`, `src-override-effort` (0 CSS defs).
- **Repro:** `Settings → Backends` → the per-backend config-source panel; `.source-unbound` computes
  with no border/padding/background/radius. Adds to the previously-noted `interface-controls` /
  `cred-chip` undefined cluster.
- **Expected:** the referenced classes have styling (or are removed). Functional, so low severity.

## Static sweeps (Phase A) — risk leads

- **S1/S4 (reproduced as finding 1):** `backends.json` nil-map path is the live crash. Other historical
  nil-slice hotspots (layout order, sessions/roles/projects/archive/files/commands) are defended.
- **S2 (reproduced as finding 4):** `ConfigSourcePanel` undefined-class cluster; the wizard shell
  itself is styled (no full unstyled-soup regression).
- **S3 (routed to code-review / `/fix`, ENV-DEPENDENT, not reproduced live):**
  `internal/backend/credcheck/claude.go:27` the `--no-color` fallback only re-triggers on the exact
  substring `unknown option '--no-color'` (other phrasings still fail a valid login); `:42` logged-out
  detection is substring-only, so a differently-worded logout falls through to `Status:"ok"` (false
  pass). Same class as the historical `--no-color` escape.
- **S5:** essentially clean; the only leads are two read-path `.catch(()=>{})` swallows on transcript
  loads (`ChatPanel.tsx`, `ArchiveAgentPage.tsx`) — a failed transcript load shows blank with no error.

## §7 maintenance note

The §3 matrix does not explicitly name FS-11 (project shared resources), shipped since the last run.
Its user-facing surface (the read-only "Shared resources directory" field in the project edit form,
and the retained-path notice on project delete) is a normal-use surface; J9 exercised the read-only
field this run and it behaved per spec. A future matrix edit should fold the FS-11 field and the
delete-retention notice explicitly into the J9 charter.

## Coverage & honesty

- Exercised with browser + API evidence: J1, J2 (full walk), J3, J5 (core), J8 (both builds), J9 (+FS-11).
- Partial: J12 (restart persistence observed for layout + archive only).
- Not exercised this session (bounded budget; each needs extra fixtures/drivers): J4 permission flow,
  J6 terminal xterm, J7 resume/switch identity matrix, J10 multi-agent messaging, J11 failure/recovery.
  These are recorded as NOT EXERCISED, not PASS.
- The real-provider credential pass (J2 logged-in branch) is a manual/credentialed gate; it was
  satisfied deterministically via the fake backend and is not claimed as a real-CLI pass.

## Environment note (disclosure)

While preparing a fixture I ran `agentdeck dashboard stop` without a scoped `AGENTDECK_HOME`, which
targets the default `~/.agentdeck` pidfile and stopped the user's own running dashboard (brief
downtime). State lives on disk in `state.db`, so I restarted it on its normal port 4317 and it is
running again with its prior state; it was restarted with the freshly built `bin/agentdeck` rather
than the installed `~/.local/bin/agentdeck`. All subsequent server stops were scoped to review homes.
