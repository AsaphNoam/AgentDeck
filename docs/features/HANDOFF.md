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
- **Work units:** `rename-product-to-deckhand.md` is waiting to start.
  `migrate-internal-actions-from-mcp.md` stays paused on its transport blocker.
- **Review units:** `open-and-annotate-any-local-file` is available.
- **Fix units:** none available.
- **Design units:** available and resumable entries remain in `docs/ideas.md`.
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

None.

## Review findings

None.

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

- **2026-09-30 — Feature design: exact context and expanded-card runtime metadata.** Specified an
  additive optional used/total token pair through ACP, durable status/session state, AgentState SSE,
  and the shared context meter, with truthful percentage-only fallback. Expanded scoped-project
  agent cards reuse their existing backend/model/effort metadata without adding controls or changing
  project summaries. The incumbent expanded-card fixture confirmed the header needs a separate,
  wrapping metadata line rather than a denser action row. The change is waiting to start; no product
  code changed. Spec, twin-skill, and whitespace checks pass.

- **2026-09-30 — Implementation: open and annotate any local text file.** The conversation file
  viewer now accepts absolute, traversal, symlinked, and `.git` paths under the approved local trust
  policy while keeping regular-file, UTF-8, size, local-origin, and remote-route bounds. Source and
  rendered file selections join the existing annotation tray as backward-compatible file anchors;
  cards, transcript persistence, search, self/agent delivery, reload, and legacy annotations keep
  one shared path. Both Go test modes, the production Go build, all 556 UI tests, the UI build, and
  an isolated real-browser outside-file selection/send passed with no console errors.

- **2026-09-30 — Feature design: unrestricted conversation file viewing and annotation.** Specified
  the operator-approved local trust policy: absolute file links and direct local requests may read
  any regular UTF-8 file AgentDeck can read, while relative links keep the session working directory
  and the tailnet route stays denied. File selections now have a ready design as backward-compatible
  path/line annotations through the existing tray, transcript event, search and delivery flow;
  rendered Markdown keeps a path-only anchor. No product code changed.

- **2026-09-30 — Review fix: Claude same-backend model switching (INV §1/§2/§11/§12/§17).**
  Claude chat now explicitly applies and verifies the selected model after `session/new` or
  `session/load`, so native resume cannot restore the transcript's prior model over a same-backend
  switch. Activated the load-path regression and aligned FS-09/TS-04 with the adapter contract.
  `make test` and `make build` pass; the `claude-model-switch-resume` review unit is closed.

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
