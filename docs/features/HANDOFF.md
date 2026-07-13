# AgentDeck — Implementation handoff

**Live agent state.** Read this first, then open the governing requirements named below. Historical
phase state is archived in [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md).
Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) and keep this file limited to resumable current state.

## Current position

- **Active change:** none; the SDD foundation migration is complete. A human must select the next
  idea/candidate/gap in [`../product-backlog.md`](../product-backlog.md); a work-phase agent does
  not choose one autonomously.
- **Governing contracts:** [`../specs/README.md`](../specs/README.md) and the FS/TS/INV items selected
  for the next change.
- **State:** GREEN documentation checkpoint. `make check-specs`, shell syntax, twin work-phase skill
  comparison, and `git diff --check` passed on 2026-07-13. The preceding SDD checkpoint also passed
  `make test`, `make build`, `make vet`, all 95 UI tests, and the UI production build.
- **Last contiguous code review:** `4036e78` (2026-07-12). The later code checkpoints fixed every
  BLOCKING finding from that review; no BLOCKING finding is currently open.
- **Branch:** `main`.

## Active work detail

No implementation plan is active. For the next item, replace this paragraph with:

```md
- **Source:** I<n> | B<n> | G<n> from `docs/product-backlog.md`
- **Stage:** Discovery | Implementation
- **Governing contracts:** FS-nn.Rk, TS-nn.Rk, INV §n
- **Done when:** <observable completion and required verification>

- [ ] <bounded next step>
```

Only a human request to **consider**, **design**, or **build** authorizes the corresponding stage.
Discovery drafts a proposal but changes no product code. Implementation starts only after its FS/TS
delta is adequate. Plans sequence work; specs define truth.

## Decisions awaiting review

These are shipped boundaries documented in the specs, not blockers. A future reversal needs an
explicit spec delta; remove an item when the human accepts the current contract or queues that delta.

- **HUMAN — Local API authentication:** TS-05.R3 documents the current same-machine trust boundary. Decide
  whether a token/UI handshake is worth the added setup and compatibility cost.
- **HUMAN — Child-process environment:** TS-05.R8 documents full environment inheritance minus backend strip
  keys. Decide whether to trade provider compatibility for allowlists.
- **HUMAN — Terminal and messaging support boundary:** FS-07/TS-04 document Claude-only terminal support and
  non-messageable terminal agents pending real-CLI verification.
- **HUMAN — Detached federation import:** TS-07.R11 and FS-08 keep detach unshipped until a verified copy/
  injection contract exists.
- **HUMAN — API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
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

No BLOCKING findings. Open product/quality debt is grouped in
[`../product-backlog.md`](../product-backlog.md); only a human-selected item enters this handoff.

## Recent changelog

_(Newest first; durable product truth is in FS/TS and history is in git.)_

- 2026-07-13 — SDD foundation complete: authoritative FS/TS/INV contracts, lifecycle, archive
  manifest, traceability lint, local hook, CI, role workflows, and GREEN verification landed.
- 2026-07-13 — Idea intake separated from specifications: product backlog now has explicit
  Inbox/Discovery/Ready-to-build/candidate/gap lanes, and work-phase requires an active selected item.
- 2026-07-12 — Federation bindings hydrate on restart so watch/sweep detects external edits.
- 2026-07-12 — Restart-orphaned runtimes are reaped by Stop/Switch/Release.
- 2026-07-12 — Onboarding completion write failures remain visible and retryable.
- 2026-07-12 — Canonical Phase 0–7 usability review recorded; no remaining usability BLOCKER.
- 2026-07-12 — End-to-end code review through `4036e78` recorded two restart blockers, since fixed.
- 2026-07-12 — Untagged Archive search falls back to metadata `LIKE` when FTS5 is unavailable.
- 2026-07-11 — Configuration federation 7.5–7.7 shipped with resolver, manager, API, launch, and UI.
