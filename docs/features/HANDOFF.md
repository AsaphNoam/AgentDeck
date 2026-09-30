# AgentDeck — Implementation handoff

**Live agent state.** Read **Current position** and **Active change**, then open the requirements
they name. Settled state through 2026-09-30 is archived in
[`HANDOFF-through-2026-09-30`](../archive/state/HANDOFF-through-2026-09-30.md); older epochs remain
beside it. Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md).

## Current position

- **Active change:** none.
- **Release:** `v0.7.0` is tagged at `1fe78c1` and published. The 74-commit range from `v0.6.0`
  shipped mobile remote control, shared creative-workspace composition, reliable file links and
  shared-stream recovery, pipeline-recipient cleanup, and centralized launch-support choices. CI
  and the macOS installer workflow passed. The GitHub Release carries the 293,094,990-byte
  `darwin-arm64` archive, `install.sh`, and a `0.7.0` manifest whose size and SHA-256 match the
  archive asset. The release audit changed no operator guidance or pinned component version.
- **Work units:** `rename-product-to-deckhand.md` is Waiting to start.
  `migrate-internal-actions-from-mcp.md` stays paused on its transport blocker.
- **Review and fix units:** `claude-model-switch-resume` is available with one confirmed Must-fix
  finding and a skipped reproduction test.
- **Design units:** unrestricted conversation file viewing and annotation is being defined;
  available and other resumable entries remain in `docs/ideas.md`.
- **Branch:** `main`.

## Active change

None.

## Acceptance gates still owed

- FS-20.A1/A5/A6/A8: real tailnet, Android, iPhone, and `pmset -g assertions` checks. The iPhone
  Home Screen experience also still lacks a PNG touch icon. The 390px fake-provider browser pass
  covered A3/A4/A7/A9, except the fast-mode picker and Continue on an approval pause.
- TS-06.R21: credentialed Claude and Codex login/chat checks.
- TS-06.R26: the credentialed Codex 1.12.0 receipt gating FS-03.A41/A42 and FS-01.A20.
- FS-02.A27: six-tab real-browser shared-stream check; A46's real-browser J14 pass; Sky & Grove
  with Codex capabilities.

## Blocked on human

- **Unrestricted conversation file viewing and annotation:** the operator confirmed that a
  hand-edited local request may read any OS-readable regular text file without per-read
  confirmation, then added file annotations through the existing tray. FS-03.R64–R65/A45–A46 and
  FS-13.R25/A16 draft the combined behavior. Before technical design, confirm that a file
  annotation is a first-class path/line anchor with no required transcript-event sequence; rendered
  Markdown records path plus excerpt without a synthetic line; and sending durably copies the
  selected path and excerpt into the source transcript/search and chosen prompt or mail target.

## Review findings

### claude-model-switch-resume — **Fix model:** medium — Codex Terra or Claude Opus.

From the 2026-09-30 investigation of Claude same-backend model switching.

**Report (verbatim).** “switching between Claude models doesn't work (whichever model you swap to,
it changes to sonnet 5), unless you move from codex to that Claude model, then it works, but
changing again from sonnet to opus will go to sonnet, and specifically sonnet 5 even though the
backend is configured with sonnet5.5.” No AgentDeck version, separate log, or fuller environment
description was supplied. The current checkout pins `claude-agent-acp` 0.75.1.

- **Must fix** (confirmed) (FS-01.R13–R15/A8, FS-09.R21/R29, TS-04.R13/R47;
  INV §1/§2/§11/§12/§17) — a Claude-to-Claude model switch persists and reports the requested model
  but resumes the provider session on its previous model. `internal/backend/adapter.go:188-195` says
  Claude can switch model on native resume, so `internal/server/switch.go:148-162` preserves the
  prior native session and `internal/runtime/chat.go:1203-1266` drives `session/load`. AgentDeck
  includes the requested model in Claude's `_meta` options, but its shared post-session configuration
  step at `internal/runtime/chat.go:2430-2436` applies the model only for `codex-acp`. The bundled
  adapter proves why that is insufficient: `acp-agent.js:5708-5735` loads the transcript's model
  hint, `:5960-5965` passes AgentDeck's options into SDK construction, then `:7217-7265` explicitly
  chooses the resumed transcript model and skips `setModel` when no environment/settings override
  exists. Normal-use trigger: select another model for any running Claude chat agent. The dashboard
  records the new identity while the provider keeps the old model; a prior `sonnet` alias therefore
  resolves back to Sonnet 5 even when the selected catalog entry points elsewhere. Codex-to-Claude
  works because it has no compatible native session and takes the fresh `session/new` path instead.
  Existing switch/parameter tests pass because they assert persisted identity and request shape, not
  the adapter's applied model. The skipped
  `internal/runtime/chat_test.go:TestResumeClaudeAppliesRequestedModelAfterSessionLoad` reproduces
  the missing post-load model call and fails when unskipped. Fix at the existing session-configuration
  seam: after a successful Claude load, apply and verify the selected model before effort/fast, or
  deliberately use the primer path if native model change cannot be honored; update TS-04's stale
  claim that Claude gains nothing from post-session model delivery.

## Decisions needing your input

- **API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
  provider model-ID ownership. Standardizing either is a compatibility change.
- **Failed pipeline-stage chat:** decide whether a pause after failed launch/resume keeps
  withholding **Open agent**, matching restart recovery, or permits chat under a wider continuation
  contract.

## Design consistency notes

- At the next presentation review, decide whether the remaining crisp asymmetric radii on technical
  surfaces are deliberate under FS-12.R52.
- When the paused direct-action change resumes, align TS-01.R25 and TS-03.R32 with TS-04.R40 and
  scope FS-17 section 6's opening sentence to the intended planned boundary.
- The injected-steer lifetime edge case needs `/investigate-bug` before `/fix`; FilesTab and
  CommandsTab still copy silently through bare `writeText`.

## Changelog

- **2026-09-30 — Bug investigation: Claude same-backend model switching.** Confirmed that
  same-backend Claude switches resume the transcript's prior provider model while AgentDeck records
  the newly selected one. Cross-backend Codex-to-Claude switches work because they create a fresh
  Claude session. Added a skipped regression that fails on the missing post-load model-setting call;
  no product code or specification changed.

- **2026-09-30 — Bug investigation: conversation links outside the working directory.** Confirmed
  the refusal is the shipped security contract rather than a regression: the request supplies only
  a path, the server derives the sole readable root from the session working directory, and existing
  tests prove absolute, traversal, and symlink escapes are refused. Broadening the viewer is a new
  feature/security-policy change; its readable-root scope is awaiting the operator's decision.

- **2026-09-30 — Release preparation: v0.7.0.** Confirmed the minor version and audited the
  `v0.6.0..main` range. The shipped `operating-agentdeck` package already matches the agent-facing
  behavior; README install claims and pinned release components remain current. Archived the prior
  live handoff epoch. Both Go test variants, all 555 UI tests, the presentation contract and the
  versioned arm64 `sqlite_fts5` distributable pass. Credentialed and real-device gates remain owed.

- **2026-09-30 — Release: v0.7.0 published.** CI and the macOS release workflow passed. The
  published archive, installer and manifest are attached to the GitHub Release, and the manifest's
  archive size and SHA-256 match GitHub's asset metadata. Credentialed provider and real-device
  gates remain owed and are not represented as verified.
