# AgentDeck — Implementation handoff

**Live agent state.** Read this first, then open the relevant requirements named below. Historical
phase state is archived in [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md).
Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) and keep this file limited to resumable current state.

## Current position

- **Active change:** none; the SDD foundation migration is complete. Changes approved to start live
  in [`../ready-changes/`](../ready-changes/README.md), not here. An agent does not choose work from
  [`../ideas.md`](../ideas.md) on its own.
- **Relevant requirements:** [`../specs/README.md`](../specs/README.md) and the FS/TS/INV items selected
  for the next change.
- **State:** Documentation checks passed. `make check-specs`, shell syntax, twin work-phase skill
  comparison, and `git diff --check` passed on 2026-07-14. The preceding SDD update also passed
  `make test`, `make build`, `make vet`, all 95 UI tests, and the UI production build.
- **Last reviewed code:** `4036e78` (2026-07-12). The later fixes addressed every must-fix finding
  from that review; no must-fix finding is currently open.
- **Branch:** `main`.

## Current change detail

No change is in progress. When an agent starts a change from
[`../ready-changes/`](../ready-changes/README.md), replace this paragraph with:

```md
- **Change:** <descriptive-file-name>
- **Why now:** direct human request or a link to `docs/ideas.md`
- **Relevant requirements:** FS-nn.Rk, TS-nn.Rk, INV §n
- **Done when:** <observable completion and required verification>

- [ ] <bounded next step>
```

Defining an idea drafts a proposal but changes no product code. A ready change is created after its FS/TS
update is adequate; the handoff begins only when implementation actually starts. Plans order work;
specifications define what the product must do.

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
- **Detached federation import:** TS-07.R11 and FS-08 keep detach unshipped until a verified copy/
  injection approach exists.
- **API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
  provider model-ID ownership. Standardizing either is a compatibility change.

## Acceptance gates

- Run pinned, credentialed Claude and Codex chat/MCP/resume checks before claiming those combinations.
- Run pinned Claude terminal flags/hooks and live xterm journeys before claiming full terminal support.
- Run pinned OpenCode/OpenHands launch/credential checks before claiming those backends beyond fakes.
- Run the Phase 7 federation discovery/precedence/refresh/launch/resume matrix against real Claude and
  Codex installations before promoting FS-08/TS-07 from Partial.

## Blocked on human

None.

## Review findings

No must-fix findings. Future ideas and known product improvements are in
[`../ideas.md`](../ideas.md); ready changes live in
[`../ready-changes/`](../ready-changes/README.md).

## Recent changelog

_(Newest first; durable product truth is in FS/TS and history is in git.)_

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
