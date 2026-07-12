# Usability Review Run — 2026-07-12 (Canonical Phase 0–7 E2E)

**Scope:** The complete 12-journey matrix in `USABILITY-REVIEW.md`, across Phases 0–7. This run
supersedes the earlier API-only “10 flows” characterization; it does not claim PASS for a visual or
restart checkpoint that was not observed.

**Review surface:** production `sqlite_fts5` binary, an additional untagged binary, deterministic
fake ACP, three isolated `AGENTDECK_HOME` fixtures, the in-app Chromium browser while available,
localhost API/state checks, static sweeps S1–S5, and focused fallback tests. Product code was not
changed. Both Go test variants, all 94 UI tests, and the production UI build were green before the
journeys. Evidence: [`usability-review-2026-07-12-canonical-e2e-evidence/`](usability-review-2026-07-12-canonical-e2e-evidence/).

## Executive summary

1. **MAJOR / BLOCKING — onboarding completion failures are hidden.** A successful first launch is
   followed by a separate config write; that write's error handler deliberately calls the success
   callback, dismissing onboarding without an error or retry. Reload can reopen the wizard.
2. **MINOR — reloaded chat history changes shape.** The optimistic user prompt disappears (confirming
   an existing advisory), and one streamed assistant message reloads as three separate message cards.
3. **MINOR — New Agent interface choices are visually ungrouped.** Live computed styles confirm the
   referenced interface-control classes have no CSS definitions.
4. **MINOR — several failure paths remain silent or developer-facing.** Force-delete retry and clipboard
   failures are swallowed; missing/old CLI startup failures can hang or collapse to raw transport errors.
5. **Coverage is partial, not green.** Browser control stalled on the Stop confirmation and the localhost
   approval service then exhausted its quota. J5–J12 checkpoints that required a new listener, restart,
   or browser were marked BLOCKED/GATED; no product verdict was inferred from the harness failure.

## Journey matrix

| Journey | Verdict | Observed evidence |
|---|---|---|
| J1 Install & first paint | **PASS** | Real tagged + untagged builds succeeded. Empty isolated home rendered a styled shell and empty state with zero browser console errors. Computed shell/main/button styles were non-default. `J1-first-paint.png`. |
| J2 Onboarding end-to-end | **PARTIAL / GATED** | Logged-in local Claude branch auto-satisfied correctly while preserving `onboarding_complete:false`; restricted-PATH fixture reported `cli_not_installed`, survived backend validation and restart, and all JSON re-read. Missing-CLI wizard copy/navigation was browser-blocked. |
| J3 First launch + chat | **PASS with findings** | Fake-ACP agent launched, card reached idle, prompt streamed “Sure, I'll do that.” with zero console errors. After reload the user prompt vanished and the assistant deltas rendered as three cards. `J3-roundtrip.png`, `J3-reload-transcript.png`. |
| J4 Permission prompt | **PARTIAL PASS** | Runtime approve/deny/timeout/cancel tests and PermissionPrompt render passed; the live browser branch was blocked after the Stop-dialog stall. |
| J5 Grid & layout | **PARTIAL / BLOCKED** | Empty grid and first-agent state rendered. Reorder/many/groups/collapse and server-restart persistence were blocked when a separate listener could not be approved. |
| J6 Terminal runtime | **PARTIAL PASS** | Terminal argv/spec, PTY status, persistence, orphan-stop, TerminalTab render tests passed. Live xterm typing/resize/detach/reattach was browser-blocked. |
| J7 Stop / resume / switch | **PARTIAL PASS** | Live Stop API moved the fake session from active to archived; runtime resume/switch/identity focused coverage was green in the baseline. Browser resume/switch and crash-restart variants were blocked. |
| J8 Archive & search | **PARTIAL PASS** | Live tagged archive returned the stopped session with correct inactive metadata. Untagged fallback, empty-array contract, handler, and all seven ArchivePage tests passed. Many-state, live untagged, and archive-resume were quota-gated. |
| J9 Settings & config | **PARTIAL / BLOCKED** | Live project and config writes round-tripped; static S2/S5 sweeps completed. Full rendered form matrix and federation state variants required a new blocked listener. |
| J10 Multi-agent messaging | **PARTIAL PASS** | Baseline server suite plus focused runtime nudge/message tests were green. Two live fake agents and browser unread-badge transitions were blocked. |
| J11 Failure & recovery | **BLOCKED** | Live crash/reconnect/process-kill journey required restart/bind access after quota exhaustion. No product result inferred. |
| J12 Restart durability | **BLOCKED** | Required restart of state left by J3–J10 could not be completed after localhost approval exhaustion. Existing code-review restart blockers remain separately open. |

Browser rung: Playwright/in-app Chromium for J1/J3; approved fallback rung 2 (live API plus targeted
render/integration tests) elsewhere. Visual checkpoints with no browser are explicitly BLOCKED.

## Findings

### BLOCKING (MAJOR)

**B1 — S5/J2 onboarding completion write failure is treated as success.**

- **REPRO:** complete the final onboarding Launch step; let session creation succeed; make the following
  `PUT /api/config {onboarding_complete:true}` fail (disconnect, disk error, or 500).
- **EXPECTED:** keep the successful agent, show a specific error and retry for persisting completion.
- **OBSERVED:** `LaunchStep.tsx:53-56` routes `onError` to `onDone()`, dismissing the wizard exactly like
  success. The only completion bit stays false, so onboarding may return on reload.
- **FIX:** surface the structured error and offer retry; call `onDone` only after the config write succeeds.

### ADVISORY (MINOR)

**A1 — J3 reloaded transcript splits one assistant message into delta cards.** Live round-trip rendered
one assistant paragraph; reload rendered “Sure,” / “I'll” / “do that.” as three separate articles.
Normalize/fold contiguous text deltas identically on live and hydration paths. Evidence:
`J3-roundtrip.png`, `J3-reload-transcript.png`.

**A2 — J3 user prompts vanish after reload.** Live evidence confirms the existing open advisory: the
optimistic user message is not persisted, so revisiting the chat yields one-sided history. This is not
duplicated in `HANDOFF.md`; the existing bullet remains the fix-loop source.

**A3 — S2 New Agent interface controls have no styling.** `.interface-controls`, `.interface-option`, and
`.interface-disabled` are referenced but undefined. Live computed styles showed inline, transparent,
borderless, zero-padding labels. Add the grouped/disabled styles and a rendered-state test. Evidence:
`J3-launch-modal.png`.

**A4 — S5 confirmed force-delete retries can fail silently.** `RolesEditor.tsx:64` and
`ProjectsEditor.tsx:80` omit `onError` on the confirmed `force:true` retry. A disconnect/500 after the user
confirms appears ignored. Route the retry through the same error toast/form state.

**A5 — S5 Files/Commands Copy gives neither success nor failure feedback.** Clipboard promise rejections
are bare `void` calls in `FilesTab.tsx:5-18` and `CommandsTab.tsx:5-15`. Catch denial/unavailability and
show the existing toast; ideally acknowledge success.

**A6 — S3 ACP startup has no readiness timeout.** An installed but old/interactive adapter that never
answers `initialize` leaves the launch request and child process pending indefinitely
(`launch.go:68`, `chat.go:201-203`, `transport.go:198-234`). Add a bounded startup context and cleanup.

**A7 — S3 rejected CLI flags collapse to generic transport errors.** Unconditional integration flags
(`adapter.go:130-134`, `terminal.go:590-615`) have no capability probe/fallback, and startup stderr is not
surfaced, so a valid old CLI often becomes `runtime: initialize: transport closed`. Add compatibility
probing/degraded retry and name the rejected flag.

**A8 — S3 OpenCode/OpenHands path overrides validate but are ignored at launch.** Credential checks honor
`OPENCODE_PATH` / `OPENHANDS_PATH`, while adapters execute bare names (`adapter.go:190-192,237-239`). Use
the validated executable for launch and test a CLI outside service PATH.

**A9 — S3 missing adapters produce raw, malformed launch errors.** A fresh install without an adapter can
receive `runtime: start : exec: ... not found`; the wrapper prints an empty command and gives no backend
install/PATH guidance (`chat.go:93-100,471-473`). Preflight the selected binary and return actionable copy.

**A10 — S3 credential probes remain version/storage fragile.** Claude auth parsing relies on exact English
phrases and one exact unknown-flag spelling (`credcheck/claude.go:27-50`); OpenCode/OpenHands infer auth from
fixed-path file existence (`opencode.go:30-40`, `openhands.go:29-36`). Use platform-aware/provider-native
status where possible and treat unfamiliar output conservatively.

## Static sweep disposition

- **S1:** collection/null contracts currently pass. One boundary typing mismatch remains between raw
  `RuntimeEvent` and `getTranscript`'s declared render type; J3's live reload finding is the user-visible
  manifestation and is recorded above rather than duplicated.
- **S2:** A3 recorded. Other undefined wrapper/modifier selectors had no independently demonstrated normal-use
  break and were omitted under the review bar.
- **S3:** A6–A10 recorded; real-version/provider branches remain acceptance-gated.
- **S4:** PASS — archive, transcript, files/commands, layout/config collections are null-guarded end to end.
- **S5:** B1, A4, and A5 recorded.

## Acceptance gates

- Re-run J5 many/group/reorder, J6 live xterm, J7 browser resume/switch, J8 live untagged many/resume,
  J9 rendered settings/federation, J10 live two-agent mail, J11 crash/reconnect, and J12 restart durability
  when browser/localhost approval capacity is restored.
- Existing credential gates remain: real Claude/Codex MCP + launch/resume/terminal behavior and real
  OpenCode/OpenHands binaries/provider keys (Phase 7.4/7.8).

## Verification

- `make test`: both Go variants pass.
- UI: 94/94 tests pass; production build passes.
- Focused runtime permission/messaging and terminal lifecycle suites pass.
- Focused server integration rerun was sandbox-blocked and escalation quota-rejected; the full baseline
  server suite had already passed before the review.
