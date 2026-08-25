# AgentDeck — Implementation handoff

**Live agent state.** Read this first, then open the relevant requirements named below. Historical
phase state is archived in [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md).
Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) and keep this file limited to resumable current state.

## Current position

- **Review state:** The review through `76b1493` found ten **Must fix** and three **Worth fixing**
  findings in the shipped dependency-aware work. All are resolved in `e1e827b` and `b121fd0`; those
  two fix commits still need independent review.
- **Active change:** None. The dependency-aware work change is shipped; FS-16 and TS-10 are Current.
- **State:** Automated verification used the fake ACP adapter. A pinned Claude/Codex live-provider
  task pass remains owed before claiming the feature works with real providers.
- **Last reviewed code:** `76b1493` (2026-08-24).
- **Branch:** `main`.

## Active change

**State:** none in progress.

## Changelog

- **2026-08-25 — fix:** Closed all thirteen dependent-work review findings. Task starts now retain
  the runtime's true generation, cancellation and start serialize per task, failed dependencies
  propagate fully, dispatcher transitions publish after commit, failed cleanup retains ownership,
  attachment creation is atomic and bounded, and the Tasks UI and API cover their specified paths.
- **2026-08-25 — design:** Defined FS-17 (agent-facing tool result contract) and TS-04.R30–R31, and
  queued `docs/ready-changes/agent-tool-retry-classification.md`. Investigation of the "richer
  agent-facing orchestration API" idea found most of it already shipped; the remainder is trimmed in
  `../ideas.md`. The task-graph query exclusion (FS-16 §6, TS-04.R29) was reaffirmed by the user.

## Decisions needing your input

These are product decisions needed for a future change or shipped boundaries whose reversal needs
an explicit specification update. Remove an item when the human resolves it or queues that update.

- **API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
  provider model-ID ownership. Standardizing either is a compatibility change.

## Acceptance gates

- [ ] Run pinned, credentialed Claude and Codex chat/MCP/resume checks before claiming those combinations.
- [ ] Run pinned Claude terminal flags/hooks and live xterm journeys before claiming full terminal support.
- [ ] Run pinned OpenCode/OpenHands launch/credential checks before claiming those backends beyond fakes.
- [ ] Run J2 and J9 in a real macOS browser to confirm the native folder panel opens in front,
  selects, and cancels (FS-04.A22); component tests stand in until then.
- [ ] Run J5 in a real browser to confirm a right-click anywhere on the projects canvas — including
  the padding frame below and beside the cards — opens **New project** (FS-02.A24); the stylesheet
  assertion stands in until then.
- [ ] Run a task start, an assignment turn, and a reported result against the pinned Claude and Codex
      adapters before claiming dependent work works with real providers (FS-16 §6).
- [ ] Run the Phase 7 federation discovery/precedence/refresh/launch/resume matrix against real Claude and
  Codex installations before promoting FS-08/TS-07 from Partial.

## Blocked on human

Live-provider acceptance is waiting for human authorization because it invokes real provider sessions
and creates disposable local configuration homes. On 2026-07-15 this machine has Claude Code 2.1.202,
the retired `claude-code-acp`, Codex CLI 0.142.5, and `codex-acp` 1.1.2 installed; the new
`claude-agent-acp`, OpenCode, and OpenHands are not installed globally.
