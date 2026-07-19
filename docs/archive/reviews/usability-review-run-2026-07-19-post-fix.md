# Usability Review Run — 2026-07-19 post-fix

**Scope:** The full non-credentialed J1–J12 matrix after the cancelled-permission fix, against
`191c22a` with isolated review-owned homes. Live-provider and real-Claude terminal gates remained
unauthorized and were not invoked.

**Review surface:** release-style `sqlite_fts5` binary (`make dist`) plus an untagged fallback
binary for J8; the in-app browser's Playwright/DOM, screenshot, computed-style, and console APIs
(browser ladder rung 1); current fake ACP peer through a PATH shim; isolated `fresh/`, `seeded/`,
and `lived-in/` fixtures; loopback API/state checks. Evidence is in
`/tmp/agentdeck-usability-2026-07-19-rerun/run/`. Product code and specifications were not changed.

## Executive summary

1. **No new Must-fix or Worth-fixing finding.** Every runnable product path completed without a
   user-impact failure.
2. **The cancelled-permission fix is verified in the built app.** Approve, deny, the real
   three-minute timeout, and cancel all resolved the prompt; cancel returned the agent to Send,
   stayed resolved after reload, and a second decision returned a clean `409`.
3. **The permission-deny race remains fixed.** Deny returned the running agent to `idle` and the
   composer to Send while withholding tool execution.
4. **Coverage limits are explicit.** J6 and the credentialed J2/provider branches were skipped as
   human-gated. This in-app browser reports native `prompt()` as unsupported and cannot execute
   `confirm()` flows, so the affected J5/J7/J9 UI actions were marked blocked; their backing API
   operations passed and the resulting UI state was observed, but those clicks are not claimed.

## Journey results

| Journey | Result | Evidence |
|---|---|---|
| J1 Install & first paint | **PASS** — fresh home, styled fixed wizard (`Instrument Sans`, non-default canvas/dialog surfaces), zero console errors. | `run/J1-first-paint.png` |
| J2 Onboarding | **PASS** for missing-adapter guidance, signed-out guidance, passing credential shim, project creation, optional Config skip, first launch, persisted config/model merge, and non-dismissible styled wizard. Real signed-in provider branch **SKIPPED(human-gated credentials)**. No Back action exists in the current specified wizard, so the protocol's generic Back-path phrase had no action to drive. | `run/J2-onboarding-complete.png` |
| J3 First launch + chat | **PASS** — empty state, suggested name, launch, durable user prompt, folded streamed reply, reload, and idle card. Permission waiting in J4 supplied the observable non-idle state. | `run/J3-roundtrip.png` |
| J4 Permissions | **PASS** — pending prompt, approve with tool sentinel, deny without sentinel and idle recovery, actual 180-second timeout, cancel with durable resolution, reload, and clean double-fire rejection. The first cancel automation attempt reloaded before dispatch; the required stable confirm attempt passed and server logs showed the cancel request. | `run/J4-timeout-resolved.png`, `run/J4-cancel-confirmed.png`, `run/J4-double-fire.json` |
| J5 Grid & layout | **PASS** — three cards, group collapse/reload, gap change, real drag reorder, stopped member, and server-restart persistence. Empty grid passed in J3. Release-group UI action **BLOCKED(browser lacks native confirm)**; the release API remains covered by its acceptance test and was not inferred as a UI pass. | `run/J5-layout-after-stop.png` |
| J6 Terminal runtime | **SKIPPED(human-gated real Claude terminal)** — fake ACP cannot exercise the PTY/xterm path and authorization was not provided. | — |
| J7 Stop / resume / switch | **PASS** for archive resume, preserved id/name/model prompt hash/add-dirs, resumed chat, API runtime switch, rendered switched model, stop, and archived durability. Context-menu switch/stop clicks **BLOCKED(browser lacks native prompt/confirm)**; API results were not relabeled as those UI clicks. | `run/J7-stopped-after-switch.png` |
| J8 Archive & search | **PASS** — empty/three-session variants; FTS metadata, transcript and user-prompt hits with snippets/tags; no-match state; untagged metadata fallback and expected lack of transcript search; resume covered by J7. | `run/J8-fts-empty-search.png` |
| J9 Settings | **PASS** — role and project edits round-tripped; shared-resources path was present and read-only; empty cwd stayed unsaved with `cwd is required`; backend model rename kept every backend/model; source panel computed styles were present; notification mute persisted. Forced-delete confirmation/retry **BLOCKED(browser lacks native confirm)**. | `run/J9-backends-saved.png` |
| J10 Multi-agent messaging | **PASS** — per-agent MCP initialize/list/send, sender Sent state, recipient Mail 1 badge, idle nudge transcript, durable message body, recipient check, and immediate unread clear. | `run/J10-send.json`, `run/J10-unread.png`, `run/J10-check.json`, `run/J10-read-cleared.png` |
| J11 Failure & recovery | **PASS** — server disconnect showed reconnecting; restart restored open SSE and accurate cards; killed adapter and mid-turn crash became error/stopped; unknown role returned clean `422`; stopping a non-running agent returned a clean no-op. Invalid project-form input was driven in J9. | `run/J11-disconnected.png`, `run/J11-agent-crash.png`, `run/J11-bad-launch.json` |
| J12 Restart durability | **PASS** — two-agent identity, read message state, nudge transcript, archive rows, two-turn Migrator history, and switched `opus-4-7` model survived independent restarts with no false busy. | `run/J12-messaging-restart.png`, `run/J12-archive-restart.png` |

## Static sweeps

- **S1/S4 serialization and null hostility:** no actionable lead. The one mock-fidelity mismatch is
  omitted empty `layout.groups` versus mocked `{}`; the running J5 path safely normalized it through
  reload and restart. Server-normalized collection responses and guarded UI first-touch paths held.
- **S2 CSS wiring:** the production presentation contract passed. Dynamic command status and
  syntax-language labels remain bounded presentation leads, not reproduced product problems.
- **S3 external CLI variance:** exact-string retry/auth-output variants in the Claude probe and
  direct-Claude/tmux/iTerm version variance remain risk leads for the human-gated J2/J6 checks.
- **S5 error surfacing:** force-delete retry, clipboard denial, and desktop-permission rejection are
  static leads. Native confirmation and browser-permission injection were unavailable in this run,
  so none was promoted to a finding.

## Coverage notes

- The generic J2 charter says to exercise a Back path, but FS-04 specifies a forward-only
  Backend → Project → Config → Launch wizard and the running UI exposes no Back action. This is a
  matrix/spec wording gap, not an observed acceptance mismatch.
- Native browser dialogs are an explicit shipped boundary in FS-04. The in-app browser's own
  unsupported-dialog error was excluded from product console-error counts and is recorded as
  blocked coverage rather than a product defect.
- The fake ACP peer reuses one tool-call id across turns, which makes earlier permission chips adopt
  the newest resolution in this harness. This known harness limitation was not treated as product
  evidence.

## Not exercised

Pinned authenticated Claude/Codex chat, MCP and resume; real Claude terminal flags, hooks and xterm;
OpenCode/OpenHands provider launches; real federation pass-through; and native prompt/confirm UI
actions in a browser that supports them.
