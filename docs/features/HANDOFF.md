# AgentDeck — Implementation handoff

**Live agent state.** Read this first, then open the relevant requirements named below. Historical
phase state is archived in [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md).
Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) and keep this file limited to resumable current state.

## Current position

- **Bug investigation:** The 2026-08-27 all-200/no-page-load incident is fixed. Same-origin dashboard
  tabs now share one long-lived SSE connection, leaving the browser's HTTP/1.x pool available for
  REST queries; the related config-source, transcript-reconciliation, and card-preview amplification
  paths are bounded as described in the changelog.
- **Review state:** Every review and usability finding through 2026-08-27 is closed in code, tests,
  or an explicit specification boundary. No open review finding remains.
- **Active change:** None. Agent-facing retry classification and structured result delivery is shipped;
  FS-17 and TS-04.R30–R31 are Current.
- **State:** Automated MCP contract verification is green. Pinned Claude/Codex live-provider checks
  remain owed before claiming those adapters accept structured results.
- **Usability state:** The v0.2.2 → v0.2.3 user-facing delta was driven through a real browser on
  2026-08-27. Its recorded project-warning, task-supervision, default-role, parked-action, and
  attention-copy findings are fixed. FS-02.A24 is closed; FS-04.A22 remains narrowed to the native panel.
- **Last reviewed code:** `895348e` (2026-08-26).
- **Branch:** `main`.

## Active change

**State:** none in progress.

## Changelog

- **2026-08-27 — design review:** Reviewed the waiting Mermaid chat-rendering change against FS-03,
  TS-08, the shipped Markdown/presentation/archive seams, Mermaid 11.17.2, and every invariant
  class. The existing `AssistantText` and presentation adapters are the right extension points, and
  the closed-fence state is recoverable from react-markdown's source-position metadata. Three
  implementation-gating findings remain: Mermaid image nodes can make an attacker-chosen request
  before returned SVG can be sanitized, a main-thread render cannot be interrupted by the promised
  elapsed-time budget, and an already-rendered SVG does not observe a later skin change. No product
  code, specification, or change file was edited.

- **2026-08-27 — fix:** Closed all four Pipelines design-review findings (INV §7/§8/§10).
  FS-14.R36/R39 now require frozen human stage titles, complete attempt-agent cards, correct
  same-agent continuation attribution, and retained-history pagination; R44–R45 and A20–A23 make
  the focused hierarchy, progressive disclosure, stable refetch state, restrained motion, and
  reduced-motion result observable. TS-03.R29–R30 define the additive detail/list contracts,
  TS-09.R28 replaces the project-wide task read with the capped attempt-window projection, and
  TS-02.R26 owns its one composite read index. J14 and the ready change cover the dense, paginated,
  live-update, fallback, and reduced-motion evidence. The stale effort note is removed.

- **2026-08-27 — design review:** Reviewed the waiting Pipelines split against FS-12, FS-14,
  TS-03, TS-08, TS-09, the shipped frontend/server seams, and every invariant class. The separation
  into Runs and Templates is sound, but four implementation-gating findings remain: the requested
  high-end interaction outcome has no observable contract, the list/card data shapes cannot render
  all promised human-facing content, retained run history silently stops at the first page, and the
  delegated-work response cap does not bound the work used to build it. One stale effort sentence
  is recorded separately as a consistency note. No product code, specification, or change file was
  edited.

- **2026-08-27 — design:** Defined the Pipelines surface split. FS-14.R25 is superseded by R35–R43
  and A14–A19; TS-09.R28 specifies the derived one-hop delegated-agent projection composed at the
  HTTP boundary with no new schema; TS-03.R29 specifies the additive capped run-detail block and its
  `task_update` refetch rule. FS-14, TS-09 and TS-03 are now Partial and J14 covers the split
  surface. Queued `docs/ready-changes/split-pipelines-surface.md`; the source idea is removed.
  `make check-specs`, the twin-skill comparison and `git diff --check` are green.

- **2026-08-27 — fix:** Closed all thirteen open findings. Browser tabs share one SSE stream and
  six-tab REST capacity is specified (INV §1/§8); unchanged config generations no longer publish,
  session writes reconcile incrementally after a debounce, and background streams retain bounded
  card previews (INV §7/§8). Project warnings stay visible, task supervision names and links real
  work, defaults/actions/copy match the shipped contracts, the lifecycle flake waits for ownership
  to settle (INV §5/§8), and FS-16/FS-17 now state their verified boundaries accurately (INV §4/§10/§15).

- **2026-08-27 — bug investigation correction and reproduction:** Built a deterministic production-UI
  stress fixture with one Claude Haiku-labelled orchestrator, six workers, and 7,000 streamed ACP
  deltas. A real Chromium reproduced the field symptom at the sixth same-origin tab: the shell and
  SSE opened, REST queries never reached the server, and the page remained on `Loading project…`.
  Closing one existing tab released an SSE connection and the stalled tab completed in 127 ms. This
  confirms browser HTTP/1.x connection starvation as the root cause. The earlier default-home
  analysis inspected a different computer and is not incident evidence; its config-source and
  transcript findings remain independent scaling defects only.
- **2026-08-27 — usability review:** Drove the user-facing changes released between `v0.2.2` and
  `v0.2.3` through a real Chromium against the release binary on isolated fixtures: dependent work
  end to end, the dashboard attention count, the task-concurrency budget, browser-local chat drafts,
  the directory-browse control, and the projects-canvas context menu. One **Must fix** and five
  **Worth fixing** findings recorded below; no blocker. The FS-02.A24 right-click gate is closed and
  the FS-04.A22 gate is narrowed to the native panel alone. J15–J17 were added to the journey matrix
  because FS-15, FS-16 and FS-17 shipped citing no journey at all. Full run:
  [`../archive/reviews/usability-review-run-2026-08-27-release-delta.md`](../archive/reviews/usability-review-run-2026-08-27-release-delta.md).
- **2026-08-26 — review:** Verified the fixes for the nine findings against the tree. Seven are
  closed with real regression coverage, including a three-level chain test that also exposed the
  restart re-evaluation having been a no-op because `TasksInStates` never populated `Arms`. Three
  residual items are recorded below. `make test`, `make build`, `make check-specs`,
  `git diff --check`, `npm test` (251 passing), and `npm run build` are green;
  `TestRetryRunsAgainOnTheSameAssignee` failed once in three full-suite runs and did not reproduce.
- **2026-08-26 — work:** `/work` found no active change and no waiting ready change, so no
  implementation started. The repository remains clean and ready for the next designed change.
- **2026-08-26 — fix:** Closed all nine review findings. Source evaluation now publishes and
  recursively propagates across pipeline and restart boundaries (INV §1/§10/§15); task start
  generations are confirmed at effect time and failed wake confirmation settles the attempt (INV
  §4/§5); context authorization has one transaction-safe predicate (INV §2); task and dashboard
  errors surface accurately (INV §8); and agent-tool retry coverage and its SDK boundary now match
  FS-17 (INV §10). The two review-named pipeline conflict codes are HTTP-only, not MCP refusals.
- **2026-08-26 — review:** Reviewed `e1e827b`, `b121fd0`, and `895348e` against FS-02, FS-16, FS-17,
  TS-03, TS-04, TS-05, TS-10, and every invariant class. Two **Must fix** and seven **Worth fixing**
  findings recorded below; `make check-specs`, the Go suite for the touched packages, `tsc --noEmit`,
  and `npm test` all pass, while `git diff --check` reports one trailing-whitespace line.
- **2026-08-26 — implementation:** Shipped the agent-facing tool result contract. Every MCP refusal
  now carries a centralized four-class retry hint, successful and refused JSON objects are mirrored
  into structured content without changing the text block, and registration-derived tests cover the
  complete tool surface and deferred output-schema boundary.
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
- [ ] Run J2/J9/J16 in a real macOS browser to confirm the native folder panel opens in front,
  selects, and cancels (FS-04.A22). Narrowed on 2026-08-27: a real browser confirmed the **Browse…**
  controls are present and enabled for `cwd` and the pending `add_dirs` entry in both the Settings
  project form and the New project modal, and that the onboarding wizard renders styled. Only the
  native `osascript` panel itself is still unverified, and it needs a human at the machine.
- [x] **Closed 2026-08-27.** A real Chromium confirmed a right-click anywhere on the projects
  canvas opens **New project** (FS-02.A24): eight background points including the padding frame on
  every edge and corner, while a card right-click still opens the card menu, and the menu opens a
  styled create modal. Evidence in the J16 section of
  [`../archive/reviews/usability-review-run-2026-08-27-release-delta.md`](../archive/reviews/usability-review-run-2026-08-27-release-delta.md).
- [ ] Run a task start, an assignment turn, and a reported result against the pinned Claude and Codex
      adapters before claiming dependent work works with real providers (FS-16 §6).
- [ ] Run one successful and one refused MCP tool call through pinned Claude and Codex adapters before
      claiming they accept structured tool results without losing the text block (FS-17.A6).
- [ ] Run the Phase 7 federation discovery/precedence/refresh/launch/resume matrix against real Claude and
  Codex installations before promoting FS-08/TS-07 from Partial.

## Blocked on human

Live-provider acceptance is waiting for human authorization because it invokes real provider sessions
and creates disposable local configuration homes. On 2026-07-15 this machine has Claude Code 2.1.202,
the retired `claude-code-acp`, Codex CLI 0.142.5, and `codex-acp` 1.1.2 installed; the new
`claude-agent-acp`, OpenCode, and OpenHands are not installed globally.

## Review findings

- **Must fix** — Mermaid can make an attacker-chosen network request before the specified sanitizer
  runs (FS-03.R38, TS-08.R40, INV §8). A person who opens an assistant message containing a
  flowchart image node such as `A@{ img: "https://attacker.example/pixel" }` causes Mermaid to load
  that URL while laying out the diagram, before `render()` returns the SVG string to the proposed
  sanitizing insertion seam. Mermaid 11.17.2 officially supports URL-backed image nodes, and its
  upstream image-policy proposal identifies both the eager preload and later SVG emission; strict
  mode and post-render DOMPurify therefore cannot uphold the design's "no network request" promise
  or prevent disclosure to the remote host. Specify a pre-render URL/resource rejection boundary
  that cannot be weakened by diagram configuration, and add an acceptance case proving no request
  occurs, not merely that the final DOM lacks the URL.
- **Must fix** — The promised render-time budget cannot stop a diagram that blocks the browser thread
  (FS-03.R38, INV §8). A person who opens a pathological but size-compliant assistant diagram can
  have the chat panel freeze inside Mermaid parsing/layout/rendering. The planned dynamic import and
  a Promise timeout can choose not to insert a late result, but they cannot pre-empt synchronous work
  already executing on the main thread; the change defines neither an interruptible worker boundary
  nor a purely structural bound proven to cap render work. Specify an enforceable availability
  boundary and test a worst-case input against it, or narrow the product promise to a source/shape
  bound whose protection is demonstrated.
- **Worth fixing** — A visible diagram keeps the old skin's baked SVG colors after appearance changes
  (FS-03.R37, TS-08.R40). A person who switches between Core and Sky & Grove while a chat or archive
  transcript remains mounted sees surrounding UI update immediately while the diagram retains its
  prior palette until another event, reload, or remount happens. `AppearanceRoot` only mutates the
  document's `data-skin`; `AssistantText` does not subscribe to that marker, and
  `resolvePresentationColors` only reads values when markup is generated. Define reuse of the
  existing presentation-color observer (or an equivalent shared rerender signal) for diagrams and
  test a mounted diagram across a skin-marker change.
