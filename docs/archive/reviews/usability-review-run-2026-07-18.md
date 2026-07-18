# Usability Review Run — 2026-07-18

**Scope:** Fresh independent usability review after an interrupted Claude run left no report,
handoff checkpoint, or working-tree changes. The review focused on the shipped core-interface
redesign and on journeys that the 2026-07-16 review did not exercise.

**Review surface:** production `sqlite_fts5` binary from `make build`; a separately built untagged
fallback binary; the development-only deterministic visual matrix; the env-driven fake ACP peer;
isolated `AGENTDECK_HOME` directories and loopback ports; and the in-app browser's Playwright path
(browser ladder rung 1). Evidence lives under the ephemeral review directory
`/tmp/agentdeck-usability-20260718.kA088X/evidence/`. Product code and specifications were not
changed.

## Executive summary

1. **BLOCKER / Must fix — onboarding is ejected after backend validation.** A fresh user can pass
   the Backend step and briefly reach Project, but the next 10-second config poll sees the seeded
   role/project plus the now-valid backend, declares onboarding satisfied, removes the wizard, and
   shows an empty Dashboard. Config and Launch cannot normally be completed.
2. **MAJOR / Must fix — archived assistant replies replay as fragment bubbles.** The live fake-ACP
   response `Sure, I'll do that.` renders as one message, but Archive and resumed views render its
   three stored stream deltas as three separate assistant messages. Real provider streams can
   contain many more deltas, making history difficult to read.
3. The new core interface otherwise rendered coherently in the production binary and full visual
   matrix: local fonts and assets loaded, baseline and high-variance fixtures had the same semantic
   structure, dense Settings pages had no horizontal overflow, and no product console errors were
   observed. One browser error came only from this review browser not implementing native
   `prompt()` when Switch runtime was attempted; that is an environment limitation, not a finding.

## Journey matrix

| Journey | Verdict | Observed evidence |
|---|---|---|
| J1 Install & first paint | **PASS** | Real tagged build succeeded. A fresh isolated home rendered the styled shell and designed empty Dashboard with Instrument Sans, the intended neutral canvas, no overflow, and no console errors. Evidence: `J1-fresh-dashboard.png`. |
| J2 Onboarding end-to-end | **FAIL — BLOCKER** | Missing-adapter guidance was specific and actionable, and successful validation advanced to Project. Within the next config poll the wizard disappeared before Project/Config/Launch could finish. Reproduced twice; evidence: `J2-step-backend.png`, `J2-after-backend-validation.png`, `J2-after-config-poll.png`. Real signed-in-provider validation remains skipped. |
| J3 First launch + chat | **PASS** | New Agent launched the fake backend; a user prompt and streamed reply rendered as one live conversation; user text persisted and was searchable. Evidence: `J3-populated-dashboard.png`, `J3-chat-roundtrip.png`. |
| J4 Permission prompt | **PASS (approve/deny), timeout not exercised** | Prompt entered waiting-input, showed permission toasts, Approve created the sentinel and resolved inline, and a later prompt could be denied. The production timeout is three minutes and was not waited out. Evidence: `J4-permission-prompt.png`. |
| J5 Grid & layout | **PASS (core)** | Columns changed 3→5 and survived reload and two server restarts; stopped cards and the empty composition stayed coherent. Drag reorder, groups, and delete-from-order were not separately driven. |
| J6 Terminal runtime | **BLOCKED(real CLI)** | Terminal framing and states were inspected in the visual matrix and UI tests, but typing/resize/detach/reattach was not claimed without the pinned real Claude terminal dependency and credentials. |
| J7 Stop / resume / switch | **PASS resume; switch blocked** | Server restart stopped the agent; Archive Resume preserved agent id/name/backend/model/history and returned to the agent view. Native Switch runtime could not be driven because this review browser does not implement the product's intentional browser-native `prompt()` flow. |
| J8 Archive & search | **FAIL — MAJOR** | Tagged search found the persisted user prompt and Resume worked. Untagged search matched metadata and returned a clean empty result for transcript-only text. Archived and resumed transcript replay fragmented one assistant reply into three bubbles. Evidence: `J8-user-prompt-search.png`, `J8-fragmented-archive-transcript.png`. |
| J9 Settings & config | **PASS (core)** | Roles, Projects, and dense Backends surfaces rendered coherently; Project resource path stayed read-only; empty title produced inline `title is required`; backend content had no horizontal overflow. Notifications was represented in the shared visual system but not separately mutated. Evidence: `J9-settings-roles.png`, `J9-settings-backends.png`. |
| J10 Multi-agent + messaging | **NOT EXERCISED** | Two agents were run, but real MCP send/nudge/read and unread-badge clearing were not driven end-to-end. Existing automated coverage was not substituted for browser evidence. |
| J11 Failure & recovery | **PASS (core)** | Server stop changed the shell to reconnecting; restart returned it to open with accurate done cards and preserved layout. A deliberate fake-agent crash rendered partial output, `process exited`, a completion toast, and an error card. Invalid project title surfaced inline. Evidence: `J11-agent-crash.png`. |
| J12 Restart durability | **PASS for exercised state** | Agent identities/history, Archive rows, user-prompt search, and 5-column density survived restart. Unread-message durability was not exercised because J10 was not run. |

## Findings

### 1. BLOCKER / Must fix — config polling ejects the user from onboarding

- **Where:** `ui/src/features/onboarding/OnboardingGate.tsx:15-29` and the 10-second refetch in
  `ui/src/api/config.ts:181-186`.
- **Repro:** fresh isolated home with the Claude adapter absent → open Backend → make the adapter
  available → Validate & Continue → Project appears → wait for the next config poll.
- **Expected:** the non-dismissible wizard remains mounted through Project → Config → Launch, then
  closes only after the first agent launches and `onboarding_complete` is saved.
- **Observed:** because roles and `my-app` were seeded, successful backend validation makes server
  `onboarding.satisfied=true`; the poll replaces the wizard with an empty Dashboard before the
  remaining steps complete.
- **Relevant requirements:** FS-04.R16–R20 and A10. FS-04.R23's rendering rule conflicts with that
  walkthrough once the seeded project makes `satisfied` true, so the fix should reconcile the spec.
- **Suggested test/fix:** latch an already-open wizard until its own Launch completion, and add a
  gate test whose config query transitions from unsatisfied to satisfied while Project is visible.

### 2. MAJOR / Must fix — replayed assistant deltas render as separate messages

- **Where:** `ui/src/store/transcriptStore.ts:58-74` normalizes replayed events but does not perform
  the consecutive `assistant_text` merge implemented for live SSE at lines 108-115.
- **Repro:** fake ACP `stream_text` → send one prompt → observe one live reply → stop/restart → open
  the inactive Archive row or Resume it.
- **Expected:** the recorded response renders with the same message boundary as the live response.
- **Observed:** `Sure, `, `I'll `, and `do that.` render as three assistant articles; a provider that
  streams smaller deltas would produce many fragment bubbles.
- **Relevant requirements:** FS-03.R4/R12 and FS-05.R14.
- **Suggested test/fix:** make full-transcript folding merge consecutive assistant deltas identically
  to live append, with a store test and an archived-view render test using at least three chunks.

## Static sweeps

- **S1/S4:** no new nil-collection crash was reproduced. The previously fixed incomplete-backend
  fallback loaded successfully in dense Settings, and the rendered/API collection paths used here
  stayed null-safe.
- **S2:** `npm run check:styles` passed all 25 contract/style checks. The production UI and visual
  matrix showed no missing-style regression; baseline and high-variance matrix modes retained the
  same headings, 18 buttons, and seven agent-state cards.
- **S3:** the missing-adapter branch now gives provider-specific human guidance. Real CLI version and
  signed-in branches remain environment-gated.
- **S5:** mutations exercised in the browser surfaced errors. Transcript loads in `ChatPanel` and
  `ArchiveAgentPage` still use silent read-path catches; no failure was induced, so these remain risk
  leads rather than findings.

## Verification

- `make check-specs` and the tagged `make build` path passed. Go emitted a non-fatal sandbox warning
  while writing its module stat cache; the binary was produced and run successfully.
- UI presentation checks passed (25/25), UI tests passed (101/101), and the production UI build
  completed. Vite reported its existing large-chunk warning.
- The untagged binary built and its metadata-search fallback was exercised in the browser.
- All review-owned servers and the development visual-matrix server were stopped at the end.

## Coverage limits

- J4's three-minute permission timeout, J5 drag/group/delete variants, J6 live xterm operation,
  J7 browser-native switch prompts, J9 notification mutation, and J10 MCP messaging were not claimed
  as passing.
- Real Claude/Codex/OpenCode/OpenHands credentials, terminal hooks, and federation acceptance remain
  the manual gates already recorded in the handoff.
