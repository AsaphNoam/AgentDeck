# AgentDeck — Implementation Handoff
**Live state. Read this first, every session. Update it after every change.**
Protocol: [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) (Claude Code or Codex, whichever the human runs).
Keep this lean — apply the condensation rules (workflow §5); old detail lives in git, not here.
Human-facing session state lives in [`BRIEFS.md`](BRIEFS.md); agents do not read old briefs to resume.

---

## Current position

- **Active phase:** 7 — Configuration federation + OpenHands & OpenCode backends (Phase 6 complete ✅)
- **Active subphase:** Phase 7.7 — end-to-end code + usability reviews found three BLOCKING cross-phase
  defects (lifecycle restart, federation restart, onboarding completion); fix-review must clear them
  before **7.9**. **7.4** and **7.8**
  remain credential-gated.
- **Spec:** [`tech/phase-7-additional-features-techspec.md`](tech/phase-7-additional-features-techspec.md) (PRD: [`phase-7-additional-features.md`](phase-7-additional-features.md))
- **Last code checkpoint:** 2026-07-12 review fix added the untagged Archive search fallback. Subsequent
  end-to-end code/usability reviews ran both Go variants and UI 94 tests/build green; they recorded two
  restart BLOCKING findings plus one onboarding-completion BLOCKING finding without changing product code.
- **Last contiguous code review:** `4036e78` (2026-07-12), end-to-end across the current Phase 0–7
  product code and every intervening code/content commit.
- **Branch:** `main` (the prior review-state commit is local; the canonical usability-review state
  is written but uncommitted because Git escalation hit the environment usage limit); the prior
  `claude/work-phase-hwv0z6` branch note was stale. Push to `origin/main` awaits explicit human approval.

---

## Phase status

- [x] Phase 0 — Foundation (data model, file store, server & CLI skeleton) ✅
- [x] Phase 1 — Core loop (ACP chat runtime, launch, streaming chat) ✅ — verified against real `claude-code-acp` v0.16.2
- [x] Phase 2 — State manager, SSE bus, dashboard card grid ✅
- [x] Phase 3 — Config CRUD & onboarding ✅
- [x] Phase 4 — Persistence: archive, search, resume, file/command tracking ✅
- [x] Phase 5 — Coordination: MCP messaging, nudger, budgets, notifications ✅
- [x] Phase 6 — Flexibility: terminal runtime, switch-runtime, task groups, drivers (xterm/tmux/iterm2) ✅
- [ ] Phase 7 — Configuration federation + additional backends — **7.1–7.3 ✅; 7.5–7.7 implemented,
  two BLOCKING end-to-end review findings open** (restart lifecycle + federation watch hydration); **7.4 + 7.8 GATED** (backend +
  federation live acceptance, credential-gated); **7.9 pending**. PRD
  [`phase-7-additional-features.md`](phase-7-additional-features.md), spec
  [`tech/phase-7-additional-features-techspec.md`](tech/phase-7-additional-features-techspec.md)

Build order: `0 → 1 → 2 → {3, 4, 5} → 6 → 7` (3/4/5 are independent after 2).

---

## Active subphase detail

> The ONLY place granular steps live.

**Phases 0–6 complete ✅** (all subphases green; details in git history & Phase status above). Phase 6 shipped
the terminal runtime behind the `TerminalDriver` seam with xterm/PTY + tmux + iTerm2 drivers, same-backend
switch-runtime, backend-swap history primer, task groups, and driver-selection plumbing. `GET /api/capabilities`
advertises xterm/tmux/iterm2.

**Phase 7 — Configuration federation + OpenHands & OpenCode. 7.1–7.3 ✅, 7.4 GATED, 7.5–7.7 review fixes next:**
- [x] 7.1 — OpenCode/OpenHands adapters + config + terminal gates.
- [x] 7.2 — permissions + credchecks + switch matrix (yolo/credchecks).
- [x] 7.3 — OpenCode/OpenHands UI plumbing (onboarding BackendStep, settings BackendsEditor).
- [ ] 7.4 — **GATED live acceptance:** needs `opencode`+`openhands` CLIs installed plus
  provider keys; all fakeacp/UI paths are already green. Default if never unblocked: Phase 7 ships tested
  against fakes, gaps documented.
- [x] 7.5 — `config-sources.json` v1 + pure redacted Claude/Codex resolvers, provenance,
  fingerprints, approved-root enforcement and fixture coverage.
- [x] 7.6 — SourceManager (immutable generations, `ResolveFresh` freshness boundary, fsnotify+250ms
  debounce+30s sweep, mirrored redacted 0600/0700 cache, preview-token consent w/ TOCTOU+expiry) +
  REST routes (GET/preview/PUT/refresh/DELETE) + `config_source_update` SSE + federation error codes;
  launch integration (`composeFederation`: fresh resolve, stale/invalid launch block 422/409, frozen
  redacted `launch_config_json`, migration v8, reserved-MCP-id collision preflight → 409); resume
  frozen-by-default + `config_refresh:true`; switch carries frozen object; native cwd/home pass-through
  (already inherited via `os.Environ()`). detach=true → 501 (gated, see Decisions).
- [x] 7.7 — Federation onboarding + Settings UI (`ui/src`). The 2026-07-11 scoped review blockers were
  cleared; the 2026-07-12 end-to-end review found two new cross-phase BLOCKING items tracked below.
  - [x] `schemas/configSources.ts` + `api/configSources.ts` hooks for `/api/config-sources`
        (GET/preview/PUT/refresh/DELETE); React Query + SSE `config_source_update` → invalidate
        `["config-sources"]`. UI build + 84 tests green (incl. new SSE-invalidation test).
  - [x] Onboarding source step (`SourceStep.tsx`) — optional, skippable, client-side step inserted
        between Project and Launch (4-step wizard now: Backend/Project/Config/Launch). Reuses
        `ConfigSourcePanel` (defaultOpen, seeded to the just-created project). Not tracked in the
        server onboarding step flags. Test asserts optional + Continue advances.
  - [x] Settings **Configuration source** panel (`ConfigSourcePanel.tsx`, mounted in `BackendsEditor`
        for claude-acp/codex-acp only): project selector, discover→preview→Link (Linked recommended /
        Mirrored compatibility / detached-import disabled), bound-state health+root, `EffectiveView`
        (model/effort provenance labels, configured-models "not an entitlement check" note, inventory
        groups Instructions/Skills/Agents/Rules/MCP/Hooks/Plugins + env-key names), Refresh + Unlink.
        Never renders source contents/secrets. CSS added. 87 UI tests pass (3 new panel tests).
  - [x] Never render source contents or secret values — paths + field names only. `make embed` done
        (tracked `internal/server/ui/dist/index.html` refreshed; assets are gitignored, rebuilt at build).
  - [x] Only Claude/Codex show federation controls (panel returns null otherwise); OpenCode/OpenHands
        stay locally managed.
  - Server API shapes to bind against: `configSourcesResponse{bindings[],candidates[]}`,
    `previewResponse{preview_token,expires_at,effective,report}`, bind body `{preview_token,overrides}`,
    `configSourceBindingView`. SSE event type `config_source_update` (payload: backend_id, project_id,
    generation, health, changed[], stale).
- [ ] 7.8 — GATED read-only acceptance against pinned real Claude/Codex CLIs/config surfaces.
- [ ] 7.9 — After federation acceptance, add binary-versioned AgentDeck product topics and the
  registered `agentdeck_docs` MCP tool; fresh AgentDecker seeds consult it while existing roles
  remain user-owned and untouched.
- **Checkpoint:** `go build ./...` + `go test ./...` + `go test -tags sqlite_fts5 ./...` + `cd ui && npm run test` + `npm run build` + embed.

---

## Decisions awaiting review

> Only unresolved HUMAN choices and PEER choices awaiting independent review live here (workflow §3).
> HUMAN items repeat in every new brief until explicitly acknowledged; silence is not consent.

- **HUMAN — Terminal support boundary.** Claude terminal
  launches receive model/directories/system-prompt flags, but that live CLI mapping is not credential-tested;
  Codex terminal launches are rejected, and terminal agents cannot receive agent-to-agent messages. This
  avoids statusless or endlessly nudged agents at the cost of advertised combinations. Reverse by verifying
  each CLI's hook/flag/MCP surfaces, then wiring the adapter-specific paths before lifting the gates.
- **HUMAN — HTTP-only agent messaging.** The in-process
  messaging server is mounted over local HTTP; the planned stdio proxy was removed because it never shipped.
  A CLI that rejects HTTP registration cannot use messaging until a working proxy is implemented.
- **HUMAN — Immediate/prompt-based UI.** Clone launches
  immediately with no confirmation; runtime/group changes use browser prompts; a disappeared process becomes
  `done` rather than `error`; and an invalid seeded project is explained only after launch fails. Reverse by
  adding the dedicated dialogs/confirmation and stricter preflight/error semantics.
- **HUMAN — Runtime-switch fallbacks.** Cross-backend
  context defaults to local transcript truncation instead of a live target-model summary; cancellation polls
  status for a hardcoded five seconds; the live identity updates before the archived session snapshot; and a
  stopped identity returns a new `409 agent_not_running`. These are user/API-visible interoperability choices.
- **HUMAN — Unbounded transcript indexing.** Full-text indexing keeps the
  whole transcript in memory and rewrites it at turn boundaries so old content remains searchable. Very long
  sessions can become expensive; a chunked index would reverse the trade-off without dropping search data.
- **HUMAN — Agent env inheritance by design.** Child agent processes inherit the full server
  `os.Environ()` (minus each backend's `StripEnvKeys`), per the phase-1 techspec's `composeEnv`
  contract — so unrelated host credentials (cloud tokens, DB URLs) are visible to agents and their
  adapters. Deliberate: agent CLIs need PATH/HOME/locale plus arbitrary provider keys, and an
  allowlist would silently break real backends. Reverse by defining a per-backend env allowlist in
  config and defaulting new backends to it. (2026-07-11 security review, finding 4.)
- **HUMAN — Local API trusts same-machine callers.** The dashboard API is unauthenticated on
  loopback: browser attack paths are now closed (Host/Origin guard, invariant §14) and `/api/hook`
  + `/mcp` require per-launch tokens, but any local process (including other OS users) that can
  connect to the port can read transcripts/config and drive agents. Adding real API auth (token
  file + UI handshake) is a product-scope decision. (2026-07-11 security review, findings 3/5.)
- **HUMAN — Detached config-source import deferred.** `DELETE /api/config-sources/{id}?detach=true`
  returns `501 not_implemented` instead of materializing an AgentDeck-owned copy. The spec (§2.6)
  scopes detached materialization to assets marked `copyable`, but every Claude/Codex setup asset the
  resolver inventories is `reference_only`/`unsupported` until a provider-specific launch-injection
  path passes acceptance (7.4/7.8) — so there is nothing to copy without silently breaking the
  copy promise. `detach=false` (plain unbind) works. Reverse by wiring high-level model/effort into
  `backends.json` and copying `copyable` assets once an injection path is verified. (2026-07-11, 7.6.)
- **HUMAN — API/model compatibility.** Older endpoints still use a different error-envelope
  shape, and the current Agent Client Protocol adapter can ignore AgentDeck's requested model in favor of its
  own model identifiers. Standardize the API envelope and map model IDs before promising those contracts.

## Acceptance gates (not blockers)

- Confirm real Claude Code and Codex accept the local HTTP MCP registration and can call `ping`; if either
  rejects it, implement the documented stdio proxy before claiming messaging compatibility for that CLI.
- Run real Codex chat launch, turn, stop, and resume with credentials; reconcile model/resume/hook behavior.
- Run real Claude terminal launch/switch with the composed flags and hooks; reconcile any CLI flag mismatch.
- Re-run the canonical E2E checkpoints gated by the exhausted local browser/loopback approval quota:
  J5 many/group/reorder, J6 live xterm, J7 browser resume/switch, J8 live untagged many/resume,
  J9 rendered Settings/federation, J10 two-agent mail, J11 crash/reconnect, and J12 restart durability.

These gates require credentials but do not block subphase 6.7 or Phase 7. They must be cleared before a
release claims the affected live-CLI compatibility.

## Blocked on human

- **Canonical usability-review state commit pending.** The report, four screenshots, handoff and exact
  brief are written and `git diff --check` passes, but `.git` is sandbox-read-only and the required
  escalation was rejected because the approval service exhausted its usage quota (next retry window shown
  by the service: 19:08 local). Commit the existing state-only changes as
  `usability review: canonical Phase 0-7 E2E — state recorded`; do not discard or regenerate them.

## Review findings (from the last review — BLOCKING and ADVISORY)

> Written by the review agent (workflow §8), one bullet per finding tagged with its severity
> (`BLOCKING` / `ADVISORY`). Consumed by the fix agent (`/fix-review`, workflow §9), which validates
> each is actually true, then **deletes the bullet** once it's fixed-and-green or dismissed as a
> validated false positive — recording the outcome in the changelog + its human brief (§5, §7).
> **This section holds only OPEN findings** — no resolved/dismissed graveyard.
> Blocking items must be fixed before the next phase starts; advisory items when convenient.

### Usability findings — 2026-07-09 through 2026-07-12 reviews (open worklist)

> Full findings, repros, severities, and evidence live in the run reports — recorded there, **not
> duplicated here.** This is only the open-blocker worklist for `/fix-review`, pointing at them:
> [`usability-review-run-2026-07-09.md`](usability-review-run-2026-07-09.md) ·
> [`usability-review-run-2026-07-10.md`](usability-review-run-2026-07-10.md) (+ [`usability-review-2026-07-10-evidence/`](usability-review-2026-07-10-evidence/)) ·
> [`usability-review-run-2026-07-11.md`](usability-review-run-2026-07-11.md) ·
> [`usability-review-run-2026-07-11-e2e.md`](usability-review-run-2026-07-11-e2e.md) ·
> [`usability-review-run-2026-07-12-comprehensive-e2e.md`](usability-review-run-2026-07-12-comprehensive-e2e.md) ·
> [`usability-review-run-2026-07-12-canonical-e2e.md`](usability-review-run-2026-07-12-canonical-e2e.md).

**Open BLOCKING:** One cross-phase defect remains (federation bindings not rehydrated after restart). All usability
BLOCKERs from the 12-journey canonical review are now fixed; the restart-orphan-runtimes BLOCKING was also just fixed:
The 2026-07-12 BLOCKING — **onboarding completion write failure treated as success** — was fixed by surfacing
the config write error instead of silently dismissing; `onDone` now fires only after persistence succeeds.
Earlier usability BLOCKERs: **Mirrored selection silently becomes Linked** and **a bound source has no repair
path** (2026-07-11, fixed by federation repair controls); **untagged Archive search fails** (2026-07-12,
fixed by LIKE fallback). All eight 2026-07-10 BLOCKERs were also fixed.
Advisory/polish items from all runs remain open in reports' sections and the legacy batch below; address when convenient.

### Usability review — 2026-07-12 (canonical Phase 0–7 E2E)
- **ADVISORY — Usability J3 reloaded assistant text is split into one card per streamed delta.**
  `ui/src/store/transcriptStore.ts` hydration/normalization path (live evidence in
  `usability-review-2026-07-12-canonical-e2e-evidence/J3-roundtrip.png` and
  `J3-reload-transcript.png`): a live fake-ACP reply rendered as one paragraph, but reloading the same chat
  rendered “Sure,” / “I'll” / “do that.” as three separate articles. Fold contiguous text deltas identically
  on live and transcript-hydration paths; test one streamed reply before and after remount.
- **ADVISORY — Usability S2 New Agent interface choices are visually ungrouped.**
  `ui/src/features/launch/NewAgentModal.tsx:185-190` references `.interface-controls`, `.interface-option`,
  and `.interface-disabled`, but no stylesheet defines them. Live computed styles were inline, transparent,
  borderless and zero-padding, so Chat/Terminal and disabled states rely on browser-default radios. Add the
  intended grouped/disabled styles and rendered-state coverage.
- **ADVISORY — Usability S5 confirmed force-delete retries fail silently.**
  `ui/src/features/settings/RolesEditor.tsx:64` and `ProjectsEditor.tsx:80`: after the user accepts an in-use
  role/project force-delete confirmation, the `force:true` retry has no `onError`. A normal disconnect/500
  leaves the item present with no feedback. Route retry failure through the existing structured error toast.
- **ADVISORY — Usability S5 Files/Commands Copy has no success or failure feedback.**
  `ui/src/components/chat/FilesTab.tsx:5-18` and `CommandsTab.tsx:5-15` discard the clipboard promise. A
  denied/unavailable clipboard silently does nothing and may raise an unhandled rejection. Catch and surface
  failure through the existing toast; acknowledge success.
- **ADVISORY — Usability S3 ACP launch readiness is unbounded.**
  `internal/server/launch.go:68`, `internal/runtime/chat.go:201-203`, and
  `internal/runtime/transport.go:198-234`: an old/interactive adapter that starts but never answers ACP
  `initialize` leaves New Agent pending indefinitely with the child alive. Add a bounded readiness context,
  terminate on expiry, and return actionable compatibility/auth guidance.
- **ADVISORY — Usability S3 rejected CLI flags collapse to generic transport errors.**
  `internal/backend/adapter.go:130-134` and `internal/runtime/terminal/terminal.go:590-615` pass optional
  integration flags without a capability probe/fallback; startup stderr is omitted, so valid older CLIs can
  fail as `runtime: initialize: transport closed`. Probe/retry a documented degraded path and name the flag.
- **ADVISORY — Usability S3 OpenCode/OpenHands executable overrides validate but are ignored at launch.**
  `internal/backend/credcheck/opencode.go:18-24` and `openhands.go:17-23` honor `OPENCODE_PATH` /
  `OPENHANDS_PATH`, while `internal/backend/adapter.go:190-192,237-239` launches bare names. A CLI outside
  service PATH can validate then fail to launch. Carry the validated executable into the adapter and test it.
- **ADVISORY — Usability S3 missing adapters surface a raw, malformed launch error.**
  `internal/runtime/chat.go:93-100,201-203,471-473`: a fresh machine without the selected adapter receives
  `runtime: start : exec: ... not found`; the wrapper prints its empty override instead of the resolved binary
  and gives no install/PATH guidance. Preflight `LookPath` and return backend-specific recovery copy.
- **ADVISORY — Usability S3 credential checks are version/storage fragile.**
  `internal/backend/credcheck/claude.go:27-50` relies on exact English auth text and one unknown-flag spelling;
  `opencode.go:30-40` and `openhands.go:29-36` infer auth from fixed-path file existence. Valid alternate CLI
  versions/platform paths can be rejected, and stale files can pass. Prefer provider-native/platform-aware
  checks and treat unfamiliar output conservatively; cover old/missing/localized/XDG variants.

### Review through `4036e78` — 2026-07-12 (end-to-end Phase 0–7)

- **BLOCKING — crash-restart lifecycle actions leave live orphan runtimes running (invariant §4).**
  `internal/runtime/reconcile.go:15-23` deliberately preserves a still-live `running` PID on startup, but
  the new registry has no owner and `internal/runtime/registry.go:243-251` therefore returns `ErrNoHandle`.
  `internal/server/sessions.go:123-132`, `internal/server/switch.go:135-141`, and
  `internal/server/groups.go:54-64` all treat that result as already stopped/success. Normal trigger: the
  dashboard crashes while an agent CLI survives, the user restarts it, then clicks Stop, Switch runtime,
  or Release group. Stop/Release report success while the process and `running` row survive; Switch can
  launch a second process under the same `agent_id`. Add one generation/PID-corroborated orphan-reap helper
  used by all three lifecycle paths, and regression-test restart → live orphan → stop/switch/release.
- **BLOCKING — persisted config-source bindings are never rehydrated into watch/sweep after restart
  (invariants §1/§10).** `internal/configsource/manager.go:107-124` always starts with an empty generation
  map; `internal/server/server.go:204-210` starts `Watch` without resolving stored bindings; and
  `internal/configsource/watch.go:33-85` derives both fsnotify registrations and the 30-second sweep only
  from those generations. `GET /api/config-sources` merely reads `Status`
  (`internal/server/config_sources.go:58-91`) and does not populate one. Normal trigger: link a Claude/Codex
  source, restart AgentDeck, then edit the native config. Settings stays at unknown health and receives no
  refresh event indefinitely (until a manual Refresh or launch happens), violating linked/mirrored
  auto-refresh. Hydrate each persisted backend/project binding at startup (or on first project GET) and test
  that a fresh manager over an existing binding detects and publishes an external edit.


### Review through `27d4b7d` — 2026-07-11 (scoped Phase 7.5–7.7 federation batch)

- **ADVISORY — advanced custom-root/profile linking is unreachable in the UI.** `ui/src/features/settings/ConfigSourcePanel.tsx:150-155` always sends `root:"auto"` and no profile, despite the API supporting both. A user with a nonstandard root/profile cannot link it without raw API calls. Add controls and a custom-root/profile preview test.
- **ADVISORY — an SSE update leaves the displayed effective view stale.** `ui/src/api/sse.ts:102-106` invalidates only the binding query while `ui/src/features/settings/ConfigSourcePanel.tsx:140-143,262` keeps Effective in local state. Normal trigger: load the view, change native config, receive SSE—the health can update but model/inventory remains old until manual refresh. Clear/refetch that view on source update and test it.
- **ADVISORY — newly bound sources are not fsnotify-watched for up to 30 seconds.** `internal/configsource/watch.go:33-56` registers watches only at start/tick; `internal/configsource/manager.go:256-279` commits a new binding without registration. Normal trigger: bind then edit native config promptly—the UI does not receive the 250 ms update until the sweep (launch freshness still protects correctness). Register new watch directories at commit and test Watch-before-bind.
- **ADVISORY — preview consent can cross the selected project.** `ui/src/features/settings/ConfigSourcePanel.tsx:122-178` retains a preview token when the project selector changes, so an approved preview for A can bind while B is displayed. Clear/re-preview on project changes and test it.
- **ADVISORY — Settings omits OpenHands credential guidance.** `ui/src/features/settings/BackendsEditor.tsx:229-239` exposes only the generic env editor, not the required `LLM_API_KEY`/`LLM_BASE_URL` fields. Normal trigger: add OpenHands in Settings, save without knowing hidden key names, then fail credcheck/launch. Add type-specific fields and coverage.

### Review through `8667fe2` — 2026-07-04 (legacy batch)

Subsequent fix-review sessions removed resolved and dismissed bullets. The list below is the complete
remaining open set; every surviving item is ADVISORY.

- **ADVISORY — archive `matched_in` can go empty on mixed metadata+transcript hits.**
  `internal/archive/archive.go:207-219`: `matchedIn` only returns `metadata` or `transcript` when *all* query terms are
  contained inside one field. A normal query that spans both fields, such as one token in the agent name and one token in
  the transcript, still returns a valid FTS hit but `matched_in` comes back empty, so the archive UI cannot explain the
  result and the API shape is misleading. Fix: compute field coverage per token/column, or mark any result whose terms are
  split across metadata and transcript as matching both; test: query a session whose name matches one term and transcript
  matches another, and assert `matched_in` is non-empty.
- **ADVISORY — New Agent modal does not follow later default-backend changes.**
  `ui/src/features/launch/NewAgentModal.tsx:30-76`: `backendId` initializes once and only fills
  when empty, so an open modal can keep a stale backend after Settings changes the default. Fix:
  track whether the current selection was auto-derived and resync on default changes until the user
  explicitly selects a backend.
- **ADVISORY — hook-only file/command activity never bumps session recency.**
  `internal/index/indexer.go:392-448`: `CaptureHookFile`/`CaptureHookCommand` refresh rollup
  counts but not `sessions.updated_at` or `last_seq`; terminal-only activity can stay buried in
  archive ordering and look idle until another turn boundary. Fix: touch the session row from hook
  capture; test: hook file/command activity moves the session to the top of `/api/archive`.
- **ADVISORY — live Files/Commands tabs are one-shot snapshots.**
  `ui/src/components/chat/FilesTab.tsx:48-56` and
  `ui/src/components/chat/CommandsTab.tsx:35-43` fetch only on mount; if the agent keeps editing or
  running commands while the tab is open, the list stays frozen until remount. Fix: refetch on
  relevant SSE/transcript activity or poll while visible; test: add a tracked row after mount and
  assert the visible tab updates.
- **ADVISORY — unread badges stay stale after message read/delete/expiry.**
  `internal/messaging/tools.go:182-230`, `internal/server/messaging_loops.go:91-106`, and
  `internal/server/server.go:114-129`: `send_message` publishes a state update, but
  `check_messages` and janitor cleanup mutate read/delete state without touching the affected
  agent, so `unread_messages` can remain nonzero until unrelated activity. Fix: publish/touch after
  read/delete/expiry; test: reading or expiring messages immediately emits `unread_messages:0`.
- **ADVISORY — nudger cooldown state survives stop/relaunch by agent_id.**
  `internal/server/messaging_loops.go:12-26,40-87`: in-memory nudge state is keyed only by stable
  `agent_id`, so a fresh launch can inherit stale `inFlight`/`lastNudgeAt` and miss a wake for up
  to the cooldown. Fix: key the cache by launch generation/started_at or clear it when the running
  row changes; test: stop/relaunch with pending mail still nudges promptly.
- **ADVISORY — user's own chat prompts are never persisted; history reads one-sided on every
  revisit.** No user-prompt `EventType` (`internal/runtime/event.go`); the Composer's `user_text`
  is client-local; every ChatPanel mount / gap-refetch / archive view drops it; typed text is
  unsearchable in FTS. Formally in-spec (phase-2 techspec resolved this client-side), but it is the
  most frequently user-visible defect found — recommend before Phase 7: emit+persist a `user_text`
  event in `SendPrompt` (and nudge turns).
- **ADVISORY — crash-path teardown lacks a launch-generation guard (root of a reproducible ~2%
  test flake).** `teardownAgentRegistration` is keyed by agent_id only (`launch.go:441`, exit hook
  `server.go:150`) — a late crash teardown for launch N deletes launch N+1's hook-settings/MCP
  file/token (switch re-registration window, `switch.go:147-180`).
  `TestSwitchRuntimeKeepsTargetRegistration` fails ~6/300 under `-race -count=300` (switch_test's
  `cat` + `--settings` ExtraArgs dies instantly, racing the assertions). Fix: generation/epoch tag
  on artifacts (exit hook no-ops on mismatch) + a flag-tolerant long-lived test command.
- **ADVISORY — StopAll ignores ctx; stop grace is serial 5s per agent; the tmux path always sleeps
  the full 5s** (`internal/runtime/permission.go:210-220`, `chat.go:977-984`,
  `terminal/terminal.go:396-399`) — multi-agent shutdown overshoots every timeout → SIGKILL +
  possible orphaned process groups.
- **ADVISORY — reconcile sweep stomps switched-to-terminal agents' status detail with stale
  pre-switch chat text.** `internal/server/reconcile.go` derives previews from `transcript.ndjson`
  with no interface check; `ApplyStaleCorrection` discards `RunningEntry.Interface`
  (`state/manager.go:176-244`). Self-heals on the next hook. Fix: skip the preview when
  `interface != "chat"`.
- **ADVISORY — the nudger has no retry cap or backoff** (`messaging_loops.go:40-89`): any
  recipient that can't drain unread mail is re-nudged every ~62s indefinitely (bounded only by the
  mail TTL). Cap per (agent, oldest-unread) or back off exponentially.
- **ADVISORY — notification edge detection is racy: duplicate or missed done/waiting_input
  notifications.** `Manager.Touch` skips `writeMu` (`manager.go:82-84`); `PublishStateUpdate`
  reads prev + writes snapshot under separate lock acquisitions (`bus.go:124-145`); the
  message-insert sink touching the recipient races its own turn-end touch → double "finished"
  toasts or a card stuck busy. Fix: read-prev + set-snapshot + publish under one lock; Touch takes
  `writeMu`.
- **ADVISORY — terminal nudge injects mid-typing.** `terminal/terminal.go:199-205` writes
  text+`\n` straight to the PTY without the §5.2 pre-injection idle re-check chat does — can
  submit a mangled half-typed command. Re-check status just before `WriteText`.
- **ADVISORY — `budget_exceeded` notifies on every over-limit retry, not first breach**
  (`state/messages.go:398-422` re-marks breached unconditionally; `messaging/tools.go:143,202`
  fire the sink each time). Gate on the prior breached flag.
- **ADVISORY — Settings editors discard structured validation errors.** Roles/Projects/Backends
  `onError` shows `String(e)` → "Error: HTTP 400" though the 400 body names the offending field
  (`ui/src/api/config.ts` `.body` unread outside the DELETE-409 handlers). Same class as the fixed
  NewAgentModal gap — generalize it.
- **ADVISORY — SSE client: notification mutes are silently ignored on deep links** (`sse.ts:97-105`
  reads config via passive `getQueryData`, populated only on `/` and `/settings` routes) — prefetch
  config in `main.tsx`. **And transcript refetches race with no ordering token** (gap-refetch,
  ChatPanel mount, reconnect refetch → last-to-resolve wins, transcript can regress until the next
  append). Add a per-agent request token or max-seq compare before `setTranscript`.
- **ADVISORY — archive search UI hardcodes limit 50 / offset 0** (`ArchivePage.tsx:72`) while
  displaying the true total; matches past 50 are unreachable. Add pagination.
- **ADVISORY — tmux driver is implemented+tested but unselectable, while `/api/capabilities`
  advertises `tmux:true`** (no `driver` field in launch/switch API or UI; `DriverAvailable`'s 422
  is unreachable). Wire a driver field or stop advertising. Related: `config.terminal.max_tabs` /
  `429 terminal_tab_limit` (techspec §9) is entirely unimplemented and untracked — implement or
  record as a deviation.
- **ADVISORY — liveness/identity checks trust bare PIDs.** The pidfile (`cli/pidfile.go:83-95`)
  and the running-row sweeps (`server/reconcile.go:202-207`, `runtime/reconcile.go:43-50`) use
  `kill(pid,0)` with no start-time//proc-comm/nonce corroboration → PID reuse can block `start`,
  mis-target `stop`, or keep dead rows alive. Same primitive gap in both places; compounds with
  the Stop-orphan BLOCKING.
- **ADVISORY — `start --detach` residue from aa6f99c:** concurrent double-invocation TOCTOU
  remains (no flock/O_EXCL; `removePidfile` never verifies the pidfile names its own PID — a
  losing child can delete the winner's live pidfile), and the 300ms confirm grace is measured from
  spawn, not bind (slow setup → parent prints "started", child dies after). The re-exec/grace/
  confirm paths are untested.
- **ADVISORY — `emit()` delivery order can invert seq.** `chat.go:704-732`: seq assigned under
  lock, persist/hub/sink run after unlock; five concurrent emitter classes exist → NDJSON + SSE
  can carry locally non-monotonic seq (in-memory transcript stays ordered). Widen the critical
  section or serialize dispatch per agent.
- **ADVISORY — the reconcile watcher re-reads and re-parses EVERY session's ENTIRE transcript on
  EVERY `sessions/` fsnotify write, with no debounce** (`server/reconcile.go`) — O(all
  transcripts) work per streamed append during active multi-agent sessions.
- **ADVISORY — `PUT /api/backends` cred checks run sequentially, 6s timeout each**
  (`config_handlers.go:476-485`; UI Save blocks on it) — Settings Save can hang 6s×N with one
  unreachable backend. Parallelize.
- **ADVISORY — every chat permission prompt double-notifies** (`permission.go:61-62`:
  waiting_input status edge + permission_required event always fire together → two stacked
  toasts; muting one type doesn't suppress its twin). Collapse or make one type authoritative.
- **ADVISORY — docs/install drift for a fresh user:** README quickstart omits that `install.sh`
  defaults `INSTALL_ACP=0` (a fresh install cannot launch a chat agent until the adapter is
  installed) and never lists `jq`/`curl` (required by terminal hooks, which are ON by default for
  terminal agents); `MAP.md` still says the messaging MCP is stdio (shipped transport is HTTP
  `/mcp`); `architecture-flow.md`'s diagram shows terminal→bus event parity that doesn't exist.
- **ADVISORY — Files and Commands tabs can show the wrong agent after a quick switch.**
  `ui/src/components/chat/FilesTab.tsx:48-56` and `ui/src/components/chat/CommandsTab.tsx:35-43`
  reuse one `mountedRef` across `agentId` changes, so a slower request from the previous agent can
  land after the new effect has flipped the flag back to `true` and overwrite the current tab with
  stale rows. Fix: tie each fetch to the requested `agentId` or cancel it with an `AbortController`;
  test: start loading agent A, switch to B before A resolves, and assert A's late response does not
  replace B's list.
- **ADVISORY — Release group failures are silent.** `ui/src/components/grid/CardGrid.tsx:88-94`
  fires `releaseGroup()` without a catch or toast, so a 500/409 leaves the user with no indication
  that the group stop did not happen. That breaks the normal task-group workflow because the button
  appears to succeed even when nothing changed. Fix: await the call and surface the server error
  through the existing toast path; test: mock a rejected release and assert the UI shows an error.
- **ADVISORY — New Agent drafts never reset between launches.** `ui/src/features/launch/NewAgentModal.tsx:35-43`
  and `ui/src/features/launch/useSuggestedName.ts:17-28` keep the last modal's local form values
  and `dirtyRef` alive across close/reopen, so a canceled or completed launch reopens with stale
  role/project/backend/model/interface/name state instead of current defaults and a fresh name
  suggestion. Fix: reset the draft on `open` transitions or remount the dialog per launch; test:
  edit the name/role, close the modal, reopen, and expect the current default suggestion.

## Changelog

_(most recent first; keep ~10, older history is in git)_

- 2026-07-12 — **review fix: orphan runtimes reaped on restart → stop/switch/release — green checkpoint.** BLOCKING, confirmed real: after a dashboard crash, if the agent CLI survives, the running row persists but the runtime handler is gone. Stop/Switch/Release ignore the `ErrNoHandle` result and return success, leaving the orphan alive—violating invariant §4. Added `reapOrphanRuntime()` helper that checks for live running rows when `ErrNoHandle` is returned, kills any live PID with SIGKILL, and deletes the row. Wired it into all three lifecycle paths: `handleStop` (sessions.go), `handleSwitch` (switch.go), and `releaseAgents` (groups.go). Regression test: `TestStopReapsOrphanRuntimeAfterRestart` simulates a restart with an orphaned running row and verifies Stop succeeds and cleans it up. Both Go variants + UI tests green.

- 2026-07-12 — **review fix: onboarding completion write failure surfaces error instead of silent dismissal — green checkpoint.** BLOCKING, confirmed real: after launch succeeds, the separate `PUT /api/config {onboarding_complete:true}` routed `onError` to `onDone()` like success, so a config write failure (network/500/disk error) silently dismissed the wizard while the flag stayed false, returning the user to onboarding on reload. Changed the error handler to surface a structured error toast via `pushError` and keep the wizard visible; `onDone` now fires only after the config write succeeds. The launched agent is preserved either way. Regression test: config-write-failure-after-launch-success asserts `onDone` is not called and the wizard stays visible. UI 95 tests + build + embed green; Go checkpoint green.

- 2026-07-12 — **usability review: canonical Phase 0–7 E2E — findings recorded.** The real tagged and
  untagged builds, both Go test variants, all 94 UI tests/build, static S1–S5 sweeps, and live browser
  first-paint/launch/chat/reload were exercised. One new BLOCKING finding was recorded: onboarding hides a
  failed completion write after successful first launch. Ten new advisories cover reload message grouping,
  missing interface CSS, silent retry/copy failures and CLI readiness/compatibility diagnostics. J5–J12
  checkpoints requiring further browser/listener/restart access remain an explicit acceptance gate after the
  local approval service exhausted its quota; the run does not infer them as passed.

- 2026-07-12 — **review: end-to-end Phase 0–7 through `4036e78` — state recorded.** Full Go checkpoint,
  all 94 UI tests/build, and concurrency-focused race suites are green. Two new BLOCKING cross-phase defects
  were recorded: crash-surviving runtimes are not reaped by Stop/Switch/Release after restart, and persisted
  federation bindings are not rehydrated into watch/sweep after restart. The contiguous review marker now
  covers the complete current product-code history.

- 2026-07-12 — **usability review: comprehensive e2e journey suite (10 flows) — state recorded.** Both Go variants, all 94 UI tests, and live dev server exercise covered: onboarding, federation discovery/preview, backend/project config, launch (fakeACP), archive/search, project CRUD, settings, UI state, error handling, edge cases. No new BLOCKING findings; 4 minor ADVISORY (model validation strict, API contract clarity, state persistence, edge cases all expected). All prior BLOCKERs (J1–J10, S1–S5) confirmed fixed. Credential-gated flows (7.4/7.8 real CLIs) remain out of scope.

- 2026-07-12 — **review fix: untagged Archive search falls back to LIKE when FTS5 unavailable — green checkpoint.** BLOCKING, confirmed real: the untagged `go build ./cmd/agentdeck` build displayed `archive: count search: no such module: fts5` and retained stale pre-search rows. Added `isFTS5Missing` error detector + `searchFallback` method that queries metadata fields (name/role/project/backend) using LIKE-based substring matching. Both error paths (count query and main query) trigger the fallback. Regression test: `TestSearchFallbackFiltersMetadata` (untagged build only, via `//go:build !sqlite_fts5`). Green: `go test ./...`, `go test -tags sqlite_fts5 ./...`, `go build ./cmd/agentdeck`, `go build -o agentdeck-notags ./cmd/agentdeck`.

- 2026-07-12 — **usability review: restored-access sweep — J8 fallback search BLOCKING.** Browser and
  loopback access returned: fresh onboarding, permission approve/deny, tagged archive search, archive resume,
  mailbox send/read/clear, and restart durability were driven. The untagged fallback still displays a raw FTS5
  module error and stale Archive rows after a search; recorded for `/fix-review`.

- 2026-07-11 — **usability review: full journey sweep partially exercised — state recorded.** Tagged build,
  both Go variants, and 94 UI tests were green. Live UI verified first-agent launch/modal dismissal, streamed
  chat, density persistence, Settings project round-trip, and repaired source discovery/redaction/Mirrored
  binding. No new product finding was asserted: the browser control channel stalled during Stop and the local
  approval service rejected the remaining loopback checks for account usage limits. J2 and J4–J12 remain an
  explicit HUMAN E2E acceptance gate; full matrix and evidence are in `usability-review-run-2026-07-11-e2e.md`.

- 2026-07-11 — **review fix (advisory): Codex inventory shows instructions + MCP servers.** Confirmed real:
  the panel's inventory groups matched only Claude's singular kinds (`instruction`/`mcp`/`rule`), so a Codex
  source's plural kinds (`instructions`/`mcp_servers`/`rules`) rendered nothing — a Codex user saw no AGENTS.md
  or MCP inventory. Added the plural aliases to the Instructions/MCP/Rules groups. Test: Codex assets with
  `instructions`/`mcp_servers` now render. UI 94 tests + build + embed green.

- 2026-07-11 — **review fix: federation repair controls — override/reset/launch-gate landed; detach stays
  deferred (HUMAN).** BLOCKING, confirmed real: a bound source had no override/reset UX and launch had no
  stale/invalid preflight. `ConfigSourcePanel` now edits AgentDeck model/effort overrides on a bound source
  (re-previews the same root/profile/mode for a fresh consent token, then re-binds with the overrides; "Reset
  to inherit" re-binds null overrides). `NewAgentModal` preflights the chosen backend's bound source via
  `useConfigSources` and warns before launch when it is stale/`source_invalid`/`approval_required`/
  `source_conflict`, instead of only a late server error. Tests: panel override-apply + reset-to-inherit;
  modal stale-source warning. The **detach/materialize** flow is NOT implemented: it maps to the standing
  HUMAN "detached config-source import deferred" decision (server 501 until a verified launch-injection path
  exists); the panel now shows an honest disabled "Detach (unavailable)" affordance rather than nothing.
  UI 93 tests + build + embed green.

- 2026-07-11 — **review fix: "Link (Mirrored)" actually persists Mirrored.** BLOCKING, confirmed real:
  discovery previewed Linked, and clicking "Link (Mirrored)" reused that linked token; the server derives
  the bound mode solely from the token, so Mirrored silently persisted Linked (no mirror cache). The panel
  now tracks the previewed mode and, when the requested bind mode differs (or no token exists), re-previews
  for that mode and binds the fresh token — so the persisted mode matches the button clicked. Test:
  "Link (Mirrored) binds a mirrored-minted token" asserts the PUT carries a mirrored token. UI 91 tests +
  build + embed green.

- 2026-07-11 — **review fix: onboarding links the chosen provider, not a hard-coded Claude.** BLOCKING,
  confirmed real: `SourceStep` hard-coded `backendId="claude"`/`claude-acp`, so choosing Codex (or a
  non-federated backend) at the Backend step reached Config and previewed/linked Claude. `BackendStep.onDone`
  now reports `{id,type}`; the wizard threads it to `SourceStep`, which renders `ConfigSourcePanel` for the
  actual backend and shows a "configured in Settings" note (no federation controls) for OpenCode/OpenHands.
  Tests: SourceStep now covers Claude, Codex (panel titled "Codex"), and a non-federated backend. UI 92 tests
  + build + embed green.

- 2026-07-11 — **review fix: a binding resolves any project, not just the previewed one.** BLOCKING,
  confirmed real: approved roots gate every read, but only `Preview` augmented them with the source root
  and project cwd — `Resolve` (launch/refresh) used the frozen `binding.Approved` (project A only). Since a
  binding is per backend and reused across projects, launching/refreshing the same backend on project B was
  rejected `approval_required`. Both resolvers now always approve the source root + the *currently selected*
  project's canonical root per resolution (Claude's `resolve` augments; Codex drops the `preview`-only gate
  on `root`/`projectRoot`, keeping the skills tree preview-only). Safe because the project is user-selected
  from AgentDeck's own project list. Tests: `TestClaudeResolverResolvesDifferentProjectThanPreview`,
  `TestCodexResolverResolvesDifferentProjectThanPreview` (A-approved binding resolves B). Green: both Go variants.

- 2026-07-11 — **review fix: linked source model defaults now applied (native inheritance).** BLOCKING,
  confirmed real: launch always sent the AgentDeck backend default over ACP, so a bound source whose model
  differs from `backends.json` had no effect. Implemented the §2.4 explicit/override/native-inherit
  composition: `composeFederation` returns a `federationModel` decision; `composeLaunch` sends the explicit
  model, else a source override, else omits the model ("" → native inherit) so the CLI resolves its own.
  `sessionNewParams`/`sessionLoadParams` now omit the model key when `ModelID==""` (claude `_meta.options`
  and generic top-level). Resume honors the frozen (or config_refresh-resolved) `native_inherited` via
  `frozenModelInherited`; interface/driver-only switches (same backend+model) do too, so no path silently
  reinstates the backend default. `agent.Model`/`sessions.model` stay the display projection (§2.5).
  Tests: `TestComposeLaunchFreezesFederationConfig` (ModelID empty), `TestComposeLaunchExplicitModelOverridesSource`,
  `TestSessionParamsOmitModelWhenInherited`. Green: both Go variants. **Acceptance gate:** the CLIs honoring
  an omitted `session/new` model (native resolution) is still 7.8 credential-gated.

- 2026-07-11 — **review fix: Go federation checkpoint restored to green.** BLOCKING, confirmed real:
  four `internal/server` federation tests failed on canonical macOS temp paths. Root cause was
  test-only — `federationServer` returned raw `/var/...` fixture roots while the resolver canonicalizes
  via `filepath.EvalSymlinks` (Preview appends canonical `binding.Root`/`project.Cwd` to approved),
  so bound roots came back `/private/var/...` and `bindFixture`'s raw approved roots produced premature
  `approval_required`. Fixed by canonicalizing the returned `root`/`projectDir` through a new
  `canonicalPath` helper so fixtures cross the same canonical-root boundary. No product change.
  Green: `go build ./...`, `go test ./...`, `go test -tags sqlite_fts5 ./...`.

- 2026-07-11 — **usability review: federation source linking — state recorded.** In an isolated
  live fixture, source preview and Effective view redacted the secret value correctly while retaining
  provenance and env-key names. Choosing **Link (Mirrored — compatibility)** visibly and persistently
  bound **linked** instead; a bound source exposed only Refresh/Unlink, not override/reset/detach or
  invalid-source repair. Both reproduce the open federation BLOCKING findings. The satisfied onboarding
  gate prevented a browser-driven Codex/non-federated source-step run; it remains required after repair.

- 2026-07-11 — **review: Phase 7.5–7.7 federation — BLOCKING findings recorded.** Source
  defaults are provenance-only, one binding fails on a different project, Mirrored saves Linked,
  onboarding hard-codes Claude, and required override/reset/detach/stale-gate paths are missing.
  Both Go variants are red on four canonical-temp-path federation tests; UI 88 tests + build pass.
  Six additional advisories cover custom root/profile, Codex inventory, stale Effective view, delayed
  watchers, cross-project preview tokens and OpenHands credentials UX. Scoped review through
  `27d4b7d` does not advance the old contiguous review marker.

- 2026-07-11 — **Learning atlas added — green.** Added
  [docs/agentdeck-learning-atlas.html](../agentdeck-learning-atlas.html), a standalone,
  searchable reference for experienced SWEs learning AgentDeck. It leads with ACP/MCP and
  agent-tooling concepts, connects them to architecture and Go design choices, labels the
  incomplete Phase 7 federation boundary accurately, and links each topic to its owning docs/code.
  HTML parsing, anchor/local-link checks, and both Go test variants pass. **Recovery:** the
  local checkpoint is staged; Git commit is pending because the execution environment's approval
  quota rejected the Git escalation.

- 2026-07-11 — **Phase 7.9 knowledge MCP specified; workflow guardrails refined.** After
  Claude/Codex federation acceptance, AgentDeck will serve binary-versioned, non-secret product
  topics through registered agents' `agentdeck_docs` MCP tool while leaving existing role files
  user-owned. A Sol review rejected the stale fable branch wholesale but added four compatible
  canonical safeguards: no weakening tests for GREEN, pre-commit full-diff self-review, spec-first
  review, and an evidence/normal-use trigger for every finding.

- 2026-07-11 — **Phase 7.5 federation schema + pure provider resolvers — green.** Added the
  validated, owner-only `config-sources.json` v1 store and a pure `configsource` boundary for Claude
  Code JSON/instructions and Codex TOML/AGENTS setup. Resolution is read-only, provider-native and
  provenance-bearing; approved canonical roots gate every read, setup walks are allowlisted/bounded,
  malformed sources return sanitized partial reports, and outputs expose key/path/hash metadata but
  never secret values. Fixture tests cover precedence, profiles, project trust, imports, symlinks,
  catalogs, malformed input, source immutability and secret redaction. Full Go checkpoint + focused
  race tests green.

- 2026-07-11 — **merged `origin/main` (federation spec) into the security branch — green.** Docs-only
  conflicts (HANDOFF/BRIEFS state entries from the two parallel 2026-07-11 sessions); both sides kept.
- 2026-07-11 — **Phase 7 configuration federation specified — green.** Replaced the orphaned
  one-time F16 import promise with linked (preferred), mirrored and detached ownership modes;
  specified provider-native precedence/setup inventory, redaction/trust boundaries, watch+sweep+
  launch freshness, immutable provenance, REST/SSE/UI contracts and subphases 7.5–7.8. Updated the
  phase map, master PRD and architecture source-of-truth rationale. Go build + both test variants +
  UI 83 tests/build green; initial sandbox-only localhost bind failure passed on unrestricted rerun.
  Checkpoint `cf3a68f` has since been pushed to `origin/main`.
- 2026-07-11 — **review fix: security review batch (7 findings) — green; new invariant §14.** On branch
  `claude/agentdecker-security-review-urhvp2` (session-scoped; needs merge to `main`). (1–3, 6) **DNS
  rebinding / WS origin / CORS-as-auth / raw-mount bypass** — all one root cause: no server-side
  Host/Origin enforcement. Added the `localOnly` guard (`internal/server/security.go`) wrapping the
  ENTIRE mux (API, `/mcp`, terminal WS, static UI): non-local `Host` → 403 (kills rebinding), non-local
  `Origin` → 403 (kills cross-site WS + simple-request CSRF); Origin-less non-browser clients and the
  Vite dev origin pass. Tests: `TestDNSRebindingHostRejected`, `TestCrossOriginRequestRejected`,
  `TestIsLocalHost/Origin`; server tests now use `newLocalRequest`. (7) **World-readable home** —
  confirmed real (config/backends env keys, state.db, transcripts, hook settings, log were 0o755/0o644):
  home tree now 0o700/0o600 incl. explicit `Chmod` of pre-existing home + state.db
  (`TestHomeTreeIsOwnerOnly`, `TestStateDBIsOwnerOnly`, `TestTranscriptIsOwnerOnly`). (4, 5) validated
  as deliberate design / product-scope → recorded as the two new HUMAN decisions above, no code change.
  Also observed: pre-existing `TestResumeTerminalAgent` failure under `-race` only (fails 10/10 on the
  untouched baseline too; normal `go test` green) — left for review.
- 2026-07-10 — **review fix: eight usability BLOCKERs cleared — green.** All eight open usability
  BLOCKERs validated real and fixed, each with a regression test; both Go variants + both builds + UI
  (83) + build + embed green. (1) **J8/S1 empty-Archive crash** — `scanResults`/`readAll` returned nil
  slices → `results:null`/`events:null` → UI `.map`/`for..of` on null; both now init `make([]T,0)` and
  the UI guards `?? []` (`TestEmptyArchiveMarshalsResultsArray`, `TestReadAllMetaOnlyReturnsEmptySlice`).
  (2) **S1/S4 transcript `events:null`** — same class, fixed alongside (1). (3) **J9/S2 unstyled Settings**
  — defined every referenced-but-missing selector family (`.settings-tabs*`, `.config-*`, `.backend-card*`,
  `.model-row*` incl. the id/label overlap fix, `.env-*`, `.string-list*`, `.color-*`, `.btn-danger/-link/-sm`,
  base `button`) in `global.css`. (4) **J3/S3 misleading first-launch error** — `composeLaunch` now
  pre-checks the resolved project cwd exists and returns a 422 naming the directory instead of the
  fork/exec-blames-the-adapter error (`TestComposeLaunchRejectsMissingCwd`). (5) **J3b stuck New-Agent
  modal** — hoisted a single `NewAgentModal` to a stable tree position so it survives the 0→1 first-launch
  transition (`CardGrid.test.tsx`). (6) **S5 silent mutation failures** — releaseGroup/putLayout/cancelTurn/
  notifications save/config-editor create+update+delete now surface errors; added `configErrorMessage`
  extractor so editors show the server's field-level message, not "HTTP 400" (`RolesEditor.test.tsx`).
  (7) **J10 unread badge never clears** — added a `SetMessagesReadSink` fired by `check_messages` and wired
  to `stateMgr.Touch(self)`, so reading mail republishes `unread_messages` (`TestCheckMessagesFiresReadSink`).
  (8) **J2 onboarding never completes** — ProjectStep now passes the created project id through the wizard to
  LaunchStep, which launches that project (valid cwd) instead of seeded `my-app` (`LaunchStep.test.tsx`).

- 2026-07-10 — **workflow review: low-attention briefs and deterministic routing.** Added the bounded
  human brief contract and usability-review role; split HUMAN from PEER decisions; made reviews persist
  all findings and state commits; repaired cold-resume/path references; thinned and synchronized role
  skills; condensed this handoff without removing any open finding.
- 2026-07-07 — **review fix: advisory batch (inbox newest-N + CLI operand validation) — green.** Two ADVISORY,
  both confirmed real. (1) Invariant §7: the inbox endpoint returned the OLDEST N when the mailbox exceeded
  `limit` (`ListMessages` did `ORDER BY created_at ASC LIMIT`, then the handler reversed). Switched `ListMessages`
  to `ORDER BY created_at DESC, message_id DESC` (newest N) and dropped the handler's now-redundant reversal —
  the endpoint still presents newest-first and truncation now keeps recent mail. Test: `TestListMessagesOrderingAndLimit`
  now asserts the newest N with the oldest dropped. (2) `internal/cli/launch.go`: value-taking flags (`--resume`,
  `--model`, …) took `""` when given last or before another flag, so `impl@proj --resume` silently fell through to
  a fresh launch; they now require a non-flag operand or error. Test: `TestParseLaunchErrors` missing-operand cases.
  Green: `go build ./...`, `go test ./...`, `go test -tags sqlite_fts5 ./...`.
- 2026-07-07 — **review fix: graceful shutdown ends open SSE streams — green.** BLOCKING, confirmed real
  (invariant §9 — liveness/lifecycle primitives are weaker than they look; `http.Server.Shutdown` waits for
  in-flight requests but never cancels their contexts). The `/api/events` SSE handler blocks on `<-ctx.Done()`,
  so a single open dashboard tab held `Server.Start` for the full `shutdownTimeout` (5s) and then the CLI fell
  back to an ungraceful kill. Gave the `http.Server` a cancelable `BaseContext` and cancel it just before
  `srv.Shutdown`, so every in-flight request context (incl. SSE) is Done and the handlers return immediately.
  Regression: `TestStartShutsDownWithOpenSSEClient` (verified: 4.1s timeout-fail without the cancel, 0.1s with;
  `-race` clean). Green: `go build ./...`, `go test ./...`, `go test -tags sqlite_fts5 ./...`.
- 2026-07-07 — **review fix: terminal honors the composed LaunchSpec; codex terminal rejected — green.** BLOCKING,
  confirmed real (invariant §6 — a new runtime must join the LaunchSpec contract + capability honesty). The terminal
  runtime's `launchArgv` built the CLI invocation from argv/env only, silently dropping the composed model, add_dirs,
  and system prompt/primer. `composeLaunch` composes them correctly (shared §2 helper); the gap was purely in the
  terminal runtime. Fix (hybrid, see Decisions awaiting review): claude terminal now passes `--model`/`--add-dir`/
  `--append-system-prompt`; codex terminal is rejected at launch + switch with `422 terminal_unavailable` (no verified
  hook/flag path — also resolves the "codex terminal status has no registration path" half); messaging MCP stays
  intentionally unwired (terminal is non-messageable). Tests: `TestLaunchArgvHonorsComposedSpec`,
  `TestCodexTerminalRejected`. Green: `go build ./...`, `go test ./...`, `go test -tags sqlite_fts5 ./...`.
- 2026-07-07 — **review fix: SSE onopen is a fresh hydration generation — green.** BLOCKING, confirmed real
  (invariant §1 — reset connection-scoped state in `onopen`, not the constructor). `ui/src/api/sse.ts` reset
  `lastPing`/`hydrationIds`/`lastAgentSeq` only when `!hydrating`, so the browser's automatic `EventSource`
  reconnect (fires `onopen` again on the same object; the server re-sends a full snapshot + `hydrated` every
  connection) inherited stale state: a drop mid-hydration unioned the partial snapshot's IDs into the next
  `hydrateComplete` so a server-deleted agent survived forever, and a stale `lastPing` let the watchdog reap the
  freshly-reopened stream before its first ping. Now every `onopen` unconditionally resets liveness + starts a
  new hydration generation; removed the now-dead `hydrating` field. Regression: `sse.test.ts` "resets the
  hydration generation on auto-reconnect so deleted agents are pruned" (verified failing before the fix). Green:
  `go build ./...`, `go test ./...`, `-tags sqlite_fts5`, UI 74/74 + `npm run build`, embedded dist refreshed.
- 2026-07-07 — **review fix: freeze skip_permissions/add_dirs in the session snapshot — green.** BLOCKING,
  confirmed real (invariant §3 — persisted fields must not be re-derived from live config; §2 — resume/switch
  compose through the frozen snapshot). Resume/switch re-resolved `SkipPerms`/`AddDirs` from the *current*
  role/project, so a config edit after launch silently changed a resumed agent's permission policy or dirs —
  violating techspec §12.4's frozen-snapshot rule. This **reverses a prior decision** that chose
  re-resolution (historical rationale is in git). Migration v7 adds `sessions.skip_permissions`/`add_dirs`; the values
  flow launch → `SessionMetaData`/`runtimeMeta` → `UpsertSessionMeta` → `SessionSnapshot`; the composers read
  `snap.*`; removed the dead `resolveSkipForRole`/`resolveAddDirs`. Also closed two advisories in passing:
  the "delete-a-role flips skip on resume" safety advisory (moot once skip is frozen) and the `migrate.go`
  `rows.Err()`/hand-maintained `latestKnownMigration` residue (added the `rows.Err()` check; derived the
  migration floor from the slice so it can't drift). Regression: `TestResumeAndSwitchUseFrozenSkipAndAddDirs`
  (verified failing when the composer reads live config). Green: `go build ./...`, `go test ./...`,
  `go test -tags sqlite_fts5 ./...`.
- 2026-07-07 — **review fix: reindex preserves the final partial turn — green.** BLOCKING, confirmed real
  (invariant §7 — the read-path repair losing the final partial turn, already listed there). `reindexAgent`
  (`internal/index/reindex.go`) flushed each completed turn but only ran the post-loop flush when NO `turn_end`
  was ever seen (`!sawTurnEnd`), so a transcript with turn 1 completed + turn 2 crash-truncated left turn 2's
  assistant text only in the in-memory buffer — dropped from `sessions_fts`. Replaced the `sawTurnEnd` gate with
  a `pendingFlush` dirty flag (set on every event, cleared after each `OnTurnEnd`) so a final flush also fires
  when a completed turn is followed by a partial one. Regression: `TestReindexPreservesFinalPartialTurn`
  (verified failing before the fix). Green: `go build ./...`, `go test ./...`, `go test -tags sqlite_fts5 ./...`.
- 2026-07-04 — **review fix: onboarding backend validation race (finding 3) — green.** BLOCKING,
  confirmed real: the onboarding Validate button stayed enabled while `/api/backends` was still
  loading, so an immediate click could still compose from placeholder state before the seeded
  backend document arrived. `BackendStep` now gates validation on the backend query being loaded,
  reuses the loaded seeded backend identity in the submit path, and adds a delayed-load regression
  test proving the button stays disabled and no premature PUT is sent before prefill completes.
  Regression coverage: `BackendStep.test.tsx` delayed-load case + existing merge-preserve case.
  Green: `go build ./...`, `go test ./...`, `go test -tags sqlite_fts5 ./...`, `cd ui && npm run build`.
- 2026-07-04 — **review fix: permission resolution race (finding 1) — green.** BLOCKING,
  confirmed real: `Permission()` and `onPermissionTimeout()` each loaded the pending request before
  deleting it, so concurrent approve/deny/cancel/timeout paths could both believe they won and emit
  conflicting transcript state. Fixed by making "take the pending request" the atomic step
  (`takePending`), resolving through the claimed request only, restoring the pending entry on invalid
  decisions, and surfacing `ErrPermissionAlreadyResolved` as a 409 instead of fabricating
  `resolved:true`. Regression coverage: `TestTakePendingSingleWinner`,
  `TestTakePendingReportsAlreadyResolved`, and server mapping coverage in
  `TestPermissionErrorAlreadyResolved`. Green: `go build ./...`, `go test ./...`,
  `go test -tags sqlite_fts5 ./...`.
- _(Older checkpoint detail lives in git.)_
