# Usability Review Run — 2026-07-18 (post-fix)

**Scope:** Independent rerun of the shipped product after the onboarding-continuity and transcript-
replay fixes. The complete journey matrix was considered; every non-credentialed browser-facing
journey was exercised, with explicit limits below.

**Review surface:** production `sqlite_fts5` binary from `make build`; a separately built untagged
fallback binary; deterministic fake ACP and fake interactive Claude peers; isolated
`AGENTDECK_HOME` directories and loopback ports; the development-only visual matrix; and the in-app
browser's Playwright path (browser ladder rung 1). J7's native-prompt switch action used rung 2 after
the review browser rejected `prompt()`. Evidence lives under
`/tmp/agentdeck-usability-20260718.9cekrL/evidence/`. Product code and specifications were not
changed.

## Executive summary

1. **MAJOR / Must fix — denying a permission can leave the completed turn stuck busy.** The denial
   is recorded and `turn_end` is durable, but a fast reject response races the normal idle write;
   the later permission-handler write wins and leaves the card busy and the composer on Cancel.
   Reproduced with two fresh agents.
2. The two fixes under review pass in the running app. Onboarding stayed on Project through the
   10-second config refresh and completed through Launch; live, archived, and resumed views all
   rendered one streamed assistant reply as one message.
3. Grid reorder/density/groups, tagged and fallback Archive search, Settings round-trips, two-agent
   messaging, unread clear and restart persistence, fake live terminal input/resize/reattach,
   server reconnect, and agent-crash recovery all behaved coherently.
4. The presentation matrix retained the same nine headings, 18 buttons, and seven agent-state cards
   in baseline and high-variance modes with no horizontal overflow. Production journeys had no
   product console errors; the one logged error was the review browser's unsupported native
   `prompt()` implementation during J7.

## Journey matrix

| Journey | Verdict | Observed evidence |
|---|---|---|
| J1 Install & first paint | **PASS** | The real tagged build succeeded. A fresh isolated home rendered the styled onboarding shell with Instrument Sans, non-default dialog/control geometry, no horizontal overflow, and zero console errors. Evidence: `J1-fresh-onboarding.png`. |
| J2 Onboarding end-to-end | **PASS (credential branch gated)** | Missing-adapter guidance named the Claude adapter and recovery. With the deterministic adapter, Backend advanced to Project; the wizard remained mounted beyond the next 10-second poll, then Project, optional Config, and first Launch completed. All generated JSON re-read successfully. Real signed-in provider validation was not invoked. Evidence: `J2-missing-adapter.png`, `J2-project-before-poll.png`, `J2-project-after-poll.png`, `J2-onboarding-complete.png`. |
| J3 First launch + chat | **PASS** | The onboarding launch produced a live card. The accepted user prompt persisted and one three-delta fake response rendered as one reply with busy→idle and context usage. Evidence: `J3-chat-roundtrip.png`. |
| J4 Permission prompt | **FAIL — MAJOR** | Approve rendered resolved, created the fake tool sentinel, and returned idle. Deny rendered resolved and the transcript recorded `turn_end`, but two fresh agents remained server/UI `busy` with Cancel after completion. The three-minute timeout was not waited out. Evidence: `J4-permission-prompt.png`, `J4-deny-stuck-busy.png`. |
| J5 Grid & layout | **PASS (shipped actions)** | Density changed to five columns/12px gap, Focus collapsed, cards drag-reordered, and all persisted across reload and repeated server restarts. The empty dashboard rendered a real empty state. The shipped UI has no session-delete action, so delete-from-saved-order was not separately applicable. Evidence: `J5-layout-before-restart.png`, `J5-layout-after-restart.png`, `J5-reorder-after-restart.png`, `J5-empty-dashboard.png`. |
| J6 Terminal runtime | **PASS with deterministic CLI** | A fake interactive `claude` ran through the real PTY/xterm path. Typed input rendered, resizing retained it, and Transcript→Terminal detach/reattach replayed scrollback. The pinned real-Claude terminal gate remains separate. Evidence: `J6-terminal-input.png`, `J6-terminal-resized.png`, `J6-terminal-reattach.png`. |
| J7 Stop / resume / switch | **PASS core; UI prompt blocked** | Server stop/restart made sessions inactive; Archive Resume preserved id/name/model/history. The review browser rejected the intentional native `prompt()` switch UI, so the API fallback switched the resumed agent from Sonnet to Opus through native resume and the running UI reflected `claude · opus-4-7`. |
| J8 Archive & search | **PASS** | Tagged search found the persisted user prompt; archived and resumed views showed one user bubble and one folded assistant reply. The untagged build found metadata and returned a clean empty result for transcript-only text without a 500 or stale rows. Evidence: `J8-tagged-search.png`, `J8-archive-replay.png`, `J8-untagged-metadata-search.png`. |
| J9 Settings & config | **PASS** | Empty role submission showed field errors; a project title survived reload; the shared resource path was read-only; dense Backends had no horizontal overflow; and muting Done persisted after reload. Evidence: `J9-settings-backends.png`. |
| J10 Multi-agent + messaging | **PASS** | Two fake chat agents connected over the real HTTP MCP route. Send woke the idle recipient, the card showed Mail 1, and `check_messages` cleared it immediately. Evidence: `J10-unread-badge.png`, `J10-unread-cleared.png`. |
| J11 Failure & recovery | **PASS core** | Server stop showed reconnecting and restart returned accurate stopped cards. A deliberate agent crash preserved user/partial assistant text and produced an error card. Invalid project data named the title field, and Stop on the crashed agent was idempotent. Evidence: `J11-server-disconnected.png`, `J11-crash-chat.png`, `J11-crash-card.png`. |
| J12 Restart durability | **PASS** | Dragged order, five-column/12px density, agents, switched model, archived history, and a pending Mail 1 badge all survived restart. Evidence: `J12-unread-after-restart.png`, `J5-reorder-after-restart.png`. |

## Findings

### 1. MAJOR / Must fix — a denied permission can overwrite normal turn completion back to busy

- **Where:** J4 deny step, `internal/runtime/permission.go:91-94` and the asynchronous prompt
  completion in `internal/runtime/chat.go:333-366` (fixture seeded + fake ACP, port 44174).
- **Repro:** launch a permission-scenario agent → send a prompt → Deny → wait one second; repeat with
  a fresh agent.
- **Expected:** the resolved denial lets the peer finish; `turn_end` transitions the agent to idle
  and the composer returns to Send (FS-03.R8, R15, A4, A8).
- **Observed:** `permission_resolved` and `turn_end` are durable, but status remains
  `busy / PermissionResolved` and the UI remains on Cancel. Reproduced twice.
- **Evidence:** `J4-deny-stuck-busy.png`; both API confirmations returned a busy status after a
  transcript ending in `turn_end`.
- **Suggested test/fix:** prevent the post-response permission write from winning after the prompt
  goroutine's idle write (for example, order/guard the transition by the active turn generation),
  and add a fast-deny integration regression that asserts idle after `turn_end`.

## Static sweeps

- **S1/S4:** server collection responses exercised here remained arrays/maps compatible with their
  UI consumers; no nil-collection or null-hostility failure was reproduced.
- **S2:** all 25 style/presentation contract checks passed. The baseline and high-variance browser
  matrix kept identical semantic structure and no overflow.
- **S3:** missing-adapter guidance and deterministic ACP/terminal paths behaved as documented. Real
  provider-version, credential, and federation variance remains the explicit manual gate.
- **S5:** exercised mutations surfaced validation/action results. Silent transcript read catches
  remain a risk lead only; no read failure was induced. The J4 finding is an observed runtime state
  race, not a source-only sweep claim.

## Verification

- Tagged `make build`, the separately built untagged binary, and the deterministic peers built and
  ran successfully. Every review-owned JSON document passed `jq empty`.
- `make test` passed both untagged and `sqlite_fts5` Go variants after rerunning with loopback/cache
  access; the first sandboxed attempt was infrastructure-blocked, not a product failure.
- UI presentation checks passed 25/25, UI tests passed 104/104, and the production UI build passed.
  Vite emitted its existing large-chunk warning.
- `make dist` rebuilt the embedded release interface and tagged binary successfully.

## Coverage limits

- J4's production three-minute permission timeout was not waited out.
- J7's browser-native switch prompts were blocked by the review browser; API plus rendered-state
  fallback evidence was used and the browser limitation was not treated as a product finding.
- Real Claude/Codex/OpenCode/OpenHands credentials, pinned real terminal flags/hooks, and federation
  compatibility were not invoked; the handoff's manual acceptance gates remain unchanged.
- The macOS release installer was not installed into a second clean host home in this run; the real
  source/release build and distribution paths were exercised.
