# AgentDeck — Implementation handoff

**Live agent state.** Read this first, then open the relevant requirements named below. Historical
phase state is archived in [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md).
Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) and keep this file limited to resumable current state.

## Current position

- **Active change:** none.
- **State:** paused — no active change. The release-path review findings are fixed and verified;
  credentialed provider acceptance remains a manual gate.
- **Last reviewed code:** `d260f93` (2026-07-15), across the continuous range after `4036e78`.
- **Branch:** `main`.

## Decisions needing your input

These are shipped boundaries documented in the specifications, not blockers. A future reversal needs
an explicit specification update; remove an item when the human accepts the current rule or queues
that update.

- **Local API authentication:** TS-05.R3 documents the current same-machine trust boundary. Decide
  whether a token/UI handshake is worth the added setup and compatibility cost.
- **Child-process environment:** TS-05.R8 documents full environment inheritance minus backend strip
  keys. Decide whether to trade provider compatibility for allowlists.
- **Terminal and messaging support boundary:** FS-07/TS-04 document Claude-only terminal support and
  non-messageable terminal agents pending real-CLI verification.
- **API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
  provider model-ID ownership. Standardizing either is a compatibility change.

## Acceptance gates

- [ ] Run pinned, credentialed Claude and Codex chat/MCP/resume checks before claiming those combinations.
- [ ] Run pinned Claude terminal flags/hooks and live xterm journeys before claiming full terminal support.
- [ ] Run pinned OpenCode/OpenHands launch/credential checks before claiming those backends beyond fakes.
- [ ] Run the Phase 7 federation discovery/precedence/refresh/launch/resume matrix against real Claude and
  Codex installations before promoting FS-08/TS-07 from Partial.

## Blocked on human

Live-provider acceptance is waiting for human authorization because it invokes real provider sessions
and creates disposable local configuration homes. On 2026-07-15 this machine has Claude Code 2.1.202,
the retired `claude-code-acp`, Codex CLI 0.142.5, and `codex-acp` 1.1.2 installed; the new
`claude-agent-acp`, OpenCode, and OpenHands are not installed globally.

## Review findings

None.

## Recent changelog

_(Newest first; durable product truth is in FS/TS and history is in git.)_

- 2026-07-15 — Defined the waiting project shared-resources change: every project will receive an
  AgentDeck-owned owner-only folder outside its repository, injected consistently into agent
  launches and retained after project deletion. It is ready to start and is not active work.
- 2026-07-15 — Fixed the release-path review findings (INV §9): bootstrap and updater lock claims
  now cover resolution/download through activation, the stable shim is fsynced then atomically
  renamed, and the arm64 macOS release workflow runs release/CLI coverage plus a bootstrap journey.
  `make check-specs`, `make test`, `make build`, and `make dist` passed.
- 2026-07-15 — Review through `d260f93` recorded three must-fix macOS release defects: full-operation
  installer/update contention is not serialized, the stable shim is written in place, and release CI
  omits required delivery checks. Shared specification, Go (both variants), build, and distribution
  checks passed.
- 2026-07-15 — Shipped the Apple-silicon macOS GitHub Releases installer: verified private Node and
  Claude/Codex ACP runtime, guided sign-in, stable shim, explicit update/rollback, no-start mode,
  release assembly/publish workflow, and release documentation. Automated checks are green; real
  provider sign-in remains credential-gated.
- 2026-07-15 — Claude chat and credential checks now target the pinned official
  `@agentclientprotocol/claude-agent-acp` package; source installs enforce its Node 22 floor.
- 2026-07-15 — Defined the waiting macOS arm64 GitHub Releases installer change: a private Node and
  Claude/Codex ACP runtime, optional guided sign-in, explicit update/rollback, checksums, and no
  signing/notarization. It is ready to start and does not make the release installer active yet.
- 2026-07-15 — Added a collaborative feature-design workflow that turns one idea into confirmed
  planned specifications and a ready change without starting implementation.
- 2026-07-14 — Codex backends can opt into `autosync_models`: on startup AgentDeck add-only merges
  the Codex CLI's `models_cache.json` into the catalog (FS-09.R28/A8). Claude autosync stays an idea.
- 2026-07-15 — Confirmed detached federation import remains deliberately unshipped: `detach=true`
  returns `501 not_implemented`; source assets remain reference-only until a verified provider launch-
  injection design exists. It is a known capability gap, not a human decision awaiting resolution.
- 2026-07-14 — New Agent modal now defaults the name to just the (capitalized) role instead of
  `Role-project` (FS-01.R1 auto-suggest; format not pinned).
- 2026-07-14 — Project ids are now server-derived from the title (`slug(title)-<timestamp>`); the
  Settings and onboarding project forms no longer ask for a slug (FS-04.R31/A11).
- 2026-07-14 — Replaced letter-number future-work labels with plain-language ideas, known
  improvements, ready changes, and current-change records.
- 2026-07-14 — Limited Claude and Codex workflow skills to their explicit slash-command triggers.
- 2026-07-14 — Added archive notices explaining that old process labels are historical and must
  not be followed; older live briefs now carry the same context.
- 2026-07-14 — Removed repeated user-intent classification from agent instructions; only the
  no-self-prioritization rule remains.
- 2026-07-14 — Simplified agent instructions: removed specialist process labels while keeping
  stable requirement IDs and plain-language human updates.
- 2026-07-13 — SDD foundation complete: authoritative FS/TS/INV contracts, lifecycle, archive
  manifest, requirement-link lint, local hook, CI, role workflows, and verification landed.
- 2026-07-14 — Changes waiting to start moved out of the handoff; the handoff now records only the
  change in progress.
- 2026-07-12 — Federation bindings hydrate on restart so watch/sweep detects external edits.
- 2026-07-12 — Restart-orphaned runtimes are reaped by Stop/Switch/Release.
- 2026-07-12 — Onboarding completion write failures remain visible and retryable.
- 2026-07-12 — Canonical Phase 0–7 usability review recorded; no remaining usability BLOCKER.
- 2026-07-12 — End-to-end code review through `4036e78` recorded two restart blockers, since fixed.
- 2026-07-12 — Untagged Archive search falls back to metadata `LIKE` when FTS5 is unavailable.
- 2026-07-11 — Configuration federation 7.5–7.7 shipped with resolver, manager, API, launch, and UI.
