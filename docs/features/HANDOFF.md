# AgentDeck — Implementation handoff

**Live agent state.** Read this first, then open the relevant requirements named below. Historical
phase state is archived in [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md).
Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) and keep this file limited to resumable current state.

## Current position

- **Release:** `v0.3.0` is published. The tag is on `95136ce`, `main` is pushed through `89014a5`,
  and release run `33296747765` succeeded in 3m23s: the archive, `install.sh`, and `manifest.json`
  are attached, the manifest's `sha256` matches GitHub's asset digest, and `latest` resolves to
  `v0.3.0`. The `v0.2.3..v0.3.0` range was read for agent-facing change and the embedded
  `operating-agentdeck` package needed none, because it shipped in that range's last commit and
  already states FS-14.R47's stage boundary; the README layout block was corrected to name
  `cache/agent-skills/`. The credentialed Claude and Codex checks are not covered by that run and
  remain owed (TS-06.R21). A customized `agentdecker` role is deliberately not migrated (FS-04.R44),
  so it keeps the superseded product manual beside the current skill; nothing user-facing says so.
- **Bug investigation:** The 2026-08-28 pipeline `stale_assignment` report is closed. Its three
  findings are all fixed: the refusal code was corrected earlier, the boundary is now stated to the
  stage agent in the assignment, the accepted result, and the refusal (FS-14.R47), the restart pause
  no longer offers a dead-end **Open agent** (FS-14.R48), every refusal is logged with the fields
  that separate its conditions, and `OnTurnEnd`/`OnExit` re-read the run under the run lock. The
  product decision the first finding needed was taken as the smaller of the two named options —
  state the boundary rather than build an in-chat continuation route, and withhold the dead-end
  action rather than widen Continue — and is flagged in the human update for confirmation. The
  earlier 2026-08-27 all-200/no-page-load incident stays fixed, and the shared SSE stream is now
  replayed to a joining tab instead of restarted for every tab.
- **Review state:** The continuous `52d01c4..da2db77` range is reviewed against its requirements and
  every invariant class, and all five of its findings are closed. Open findings are now the four
  from the MCP-migration design review, which no code change may act on until the human resolves
  its two **Must fix** items, plus one **Worth fixing** load-dependent flake in the task-cancel
  release assertion found while verifying that fix. The refused-drag pointer needs a real-browser
  computed-cursor pass and the six-tab shared-stream check remains an acceptance gate. Two behavior
  choices made by an earlier fix still need human confirmation below.
- **Active change:** None. Active-project navigation tabs are shipped and their review findings are
  closed (FS-02.R53/A35, FS-12.R39/A15, TS-08.R44). Thin AgentDecker and the shared AgentDeck operating skill are finished
  and verified (FS-18, FS-04.R44/A24, TS-11). Expandable dashboard chat panes are finished and verified
  (FS-02.R46–R52/A29–A34, FS-03.R39/A23, FS-12.R38/A14, TS-03.R31, TS-08.R41–R43).
  Running-first card placement shipped on 2026-08-28 (FS-02.R45/A28, FS-12.R37/A13). The Pipelines
  surface
  split is finished and committed (`9114df7`, with its usability fixes in `69c2f99`) and is ready
  for an independent review; its change file is removed, and FS-14 is the authority on what
  shipped.
- **State:** Automated MCP contract verification is green. Pinned Claude/Codex live-provider checks
  remain owed before claiming those adapters accept structured results.
- **Usability state:** The new Pipelines pages and the changed dashboard grid were driven through a
  real Chromium on 2026-08-30 against a `make dist` build of the shipped tree (no product code
  differs between that build point and `v0.3.0`). The three fixes that had never been in a browser —
  attempt badge on the timeline rule, run rail stacking after the timeline at the 1024 floor, and
  the one-shot entrance on an appended timeline entry with reduced motion removing it — all pass.
  So do A14, A18's cross-destination counts, A19, A20 on the 32-stage template, A22, A24 and A26.
  J5's owed items are now closed: running-first placement, the live start/stop boundary crossing in
  both directions, in-drag geometry inside one block, and the refused cross-block drop were all
  driven, along with the pane cap, its least-recently-used eviction, the evicted pane's draft, pane
  persistence, the terminal-card exemption, and the pane name link. Three of its four findings are
  closed; the attempted pointer treatment for the refused cross-block drag was reopened by code
  review because the card under the pointer overrides it (FS-02.R53).
  Still owed: A25's stage-boundary wording (needs a live report cycle `fakeacp` cannot drive),
  A18's consumption on approval, and A32's unknown-agent and cross-project id cases. The earlier
  v0.2.2 → v0.2.3 delta remains closed: FS-02.A24 is closed and FS-04.A22 remains narrowed to the
  native panel. Full run:
  [`usability-review-run-2026-08-30-new-pages.md`](../archive/reviews/usability-review-run-2026-08-30-new-pages.md).
- **Last reviewed code:** `da2db77` (2026-08-31). Advanced across the continuous range
  `52d01c4..da2db77`, which was read end to end against its requirements and every invariant class.
- **Branch:** `main`.

## Active change

**Change:** `migrate-internal-actions-from-mcp.md` — design reviewed 2026-08-31, still
**Waiting to start**. Implementation must not begin: two Must-fix design findings are open.

**State:** Active-project navigation is shipped, the continuous range through its implementation is
reviewed, and all five findings from that review are fixed. The four MCP-migration design findings
remain open. Its two Must-fix items are the private loopback transport being unreachable from a Codex
agent's own sandboxed shell, and the launch credential the design promotes to the action token
already being published as `agent_generation` over the dashboard API. Two Worth-fixing items cover
the unowned supersession of FS-06/FS-15/FS-16 and the authenticated round trip for `describe`.

**Next:** Resolve the two Must-fix design findings with the human through `/design-feature` before
any migration code is written. The refused-drag pointer now needs its real-browser computed-cursor
pass under the acceptance gates below.

## Changelog

- **2026-08-31 — fix (FS-02.R53/A35, FS-12.R39/A15, TS-08.R44, FS-18.A1/A5, FS-04.A24,
  TS-11.R4/R6; INV §6, §8, §10, §13):** Closed all five shipped-code findings from the
  `52d01c4..da2db77` review. **INV §10/§13** — the refused cross-block drag marked only the group
  stack, so the card under the pointer kept its own `cursor: pointer` and the promised in-flight
  refusal never reached the pointer; the refused state now covers every descendant of the stack with
  one wildcard rule rather than an opt-out list that would drift as card controls are added, and it
  outranks the expanded card's own header cursor. jsdom evaluates no CSS, so the new case reads the
  stylesheet the way `ProjectDashboard.test.tsx` already does and the browser half stays an
  acceptance gate. **INV §10** — the active-project overflow handled Escape only on its `+n` button,
  so the normal keyboard path (Tab into a project link, press Escape) left the disclosure open; the
  handler moved to the disclosure container and the regression now focuses a menu item first.
  **INV §10** — the ready-change index still linked the change file the implementation commit
  deleted, presenting finished work as selectable; the waiting list is corrected. **INV §8/§10** —
  the migration's "read failure" fixture exercised JSON decode corruption, not an `os.ReadFile`
  error, and one root guard skipped that independent case along with the permission-based write
  case; the three cases are now separate, corruption and a genuine read-I/O failure (a directory at
  the role path) always run, and only the write fixture skips as root. FS-18.A5 names the decode and
  I/O cases separately to match. **INV §6/§10** — the operating-knowledge lifecycle matrix resumed
  and switched only a chat agent, so a terminal-specific regression in either composer could stay
  green; terminal resume and terminal runtime switch rows joined it for both the available and
  unavailable package. One new finding is recorded below rather than fixed: the task-cancel case
  asserts a completed release the specification only guarantees through recovery, and it failed once
  under full-package load. `make check-specs`, both Go test variants, `make build`, the 340-case UI
  suite, the UI production build, and `git diff --check` pass.

- **2026-08-31 — design review (`migrate-internal-actions-from-mcp.md`; FS-06.R1/R2/A1/A3, FS-15,
  FS-16, FS-17.R13–R19/A7–A11, TS-01.R25, TS-03.R32, TS-04.R32–R40, TS-05.R18–R19, TS-06.R23,
  TS-11.R11–R12; INV §2/§4/§10/§12/§14):** Reviewed the waiting MCP-migration design through the
  over-engineering, extension, and research lenses. Four findings recorded, two of them Must fix. The
  transport assumption fails a real check: Codex 0.142.5 under the default `workspace-write` sandbox
  cannot open a loopback TCP connection from a spawned command, and AgentDeck mirrors the user's
  `config.toml` into its private `CODEX_HOME` unchanged, so `agentdeck action` would be unreachable
  for every Codex agent — while today's MCP path works because the unsandboxed CLI process makes the
  call. Separately, `generation` defaults to the launch token and is persisted and served as
  `agent_generation`, so merging hooks and actions onto that one secret publishes full action
  authority. The remaining two cover FS-06/FS-15/FS-16 still mandating the internal MCP with no
  supersession note, and `describe` routing compiled-in registry data through an authenticated HTTP
  round trip. Two consistency notes recorded. The change stays Waiting to start; no specification,
  change file, or product code was modified.

- **2026-08-31 — feature design (FS-17.R13–R19/A7–A11; TS-01.R25; TS-03.R32;
  TS-04.R32–R40; TS-05.R18–R19; TS-06.R23; TS-11.R11–R12; INV §§1–6, 8–15):** Validated that
  AgentDeck's in-process internal MCP adds model-visible schema and protocol coupling without an
  interoperability benefit, while MCP remains appropriate as a supported provider/user extension
  protocol. Specified one packaged `agentdeck action` CLI over a private loopback action adapter and
  provider-neutral typed registry, reusing the existing generation-scoped launch credential. The
  fifteen actions, structured results, authority, domain behavior, and external MCP federation stay
  unchanged. The implementation freezes parity first, validates Claude/Codex/OpenCode/OpenHands,
  and removes the internal MCP path before release with no shipped fallback. The ready change is
  `migrate-internal-actions-from-mcp.md`; its phased implementation plan is under `docs/plans/`.

- **2026-08-31 — review (FS-02.R53/R54/A35/A36, FS-12.R39/A15, FS-18.A1/A5,
  FS-04.A24, TS-08.R44, TS-11.R4/R6; INV §1–§15):** Reviewed the continuous
  `52d01c4..da2db77` range in both directions. Five findings are open: Escape closes the project
  overflow only from its trigger; the refused-drag state never overrides the card under the
  pointer; the ready-change index retains a dead link to the finished change; the migration
  I/O fixture substitutes corrupt JSON for a real read failure and skips all cases as root; and the
  knowledge-overlay matrix never resumes or switches a terminal agent. The code otherwise matches
  the active-project membership, ordering, current-context, presentation, pipeline action, layout
  error, Mermaid sanitization, and runtime-overlay requirements. The skipped pre-implementation
  design review was evaluated as a local workflow choice: this pass found no excess abstraction,
  parallel mechanism, or contradicted seam to turn into a product finding, but the independent
  before-build sequence cannot be reconstructed after implementation. INV §§1–3, 6–11, and 13 had
  applicable surfaces; §§6, 8, and 10 produced the findings above and the others produced none.
  §§4, 5, 12, 14, and 15 had no applicable surface. Specification checks, production build, the
  focused 61-test UI set, both Go test variants, the full UI suite, and the distributable build
  pass; the first sandboxed Go run failed only because it could not use the host cache or bind
  loopback test ports and passed unchanged with those permissions.

- **2026-08-31 — work (FS-02.R54/A36, FS-12.R39/A15, TS-08.R44; INV §1/§2/§8/§10/§13):**
  Added compact active-project navigation to the persistent header. One pure derivation combines
  the project catalog, hydrated agent projection, and current project/agent route; it filters
  stopped, archived, and unavailable entries, retains current context, alphabetizes with the id
  tie-breaker, and produces the five-link visible/overflow split without retained state. The
  feature-owned component supplies full accessible titles, accent identity, a structural selected
  marker, and an Escape/outside/selection-closing `+n` disclosure. The shell now uses a four-region
  non-wrapping grid. Focused regressions, the zero/one/overflow visual matrix, all UI tests, both Go
  variants, production and distributable builds, specification/style checks, and `git diff --check`
  pass. Real-browser inspection at 1024 and 1440 confirmed no shell overflow in Core or Sky & Grove.

- **2026-08-30 — feature design (FS-02.R54/A36, FS-12.R39/A15, TS-08.R44; INV §1, §2,
  §8, §10, §13):** Specified compact active-project navigation immediately after the shell's
  primary tabs. Configured non-archived projects qualify only while they have a non-archived
  `running` agent, except that the current project remains visible across its last agent stopping
  until the operator leaves its project or agent route. Title/id alphabetical order supplies five
  direct links; when the selected project falls later it replaces the fifth, and `+n` contains the
  alphabetized remainder. The visual contract uses smaller restrained rounded tabs, project accent
  plus a non-color selection cue, full accessible names for truncated labels, no motion, and a
  four-region header that holds at the 1024px floor in Core and Sky & Grove. The design reuses
  `useProjects`, the hydrated agent store, current route matching, and `--ad-project-accent`; it adds
  no measurement, recency state, persistence, API/server shape, dependency, token, public hook, or
  second row. `active-project-navigation-tabs.md` is Waiting to start.

- **2026-08-30 — fix (FS-03.R38/A21, FS-02.R45/R53/A28/A35, FS-14.R48/A26, FS-12.R37/A13,
  FS-18.A1/A5, FS-04.A24; INV §6, §7, §8, §10, §13):** Closed all seven open findings from the
  2026-08-30 code review and usability run. **INV §8** — the diagram sanitizer judged CSS before
  decoding it, so `u\72 l(...)`/`@\69 mport` survived the literal-text strip; style elements and
  style attributes are now decoded first and dropped whole when a URL-bearing token remains, with a
  renderer case that fails against the old regex. **INV §7/§8** — a failed `GET /api/layout` left
  `loaded` false forever, silently disabling layout persistence for the session; the failure now
  surfaces through `pushError` and still arms saving. **INV §10** — `launch_failed` and
  `resume_failed` pauses also leave no running stage agent, so R48's withholding of **Open agent**
  widened to them with wording that names the failure; the spec item and A26 widened with it.
  **INV §13** — `.pipeline-state-launch_failed`, `-resume_failed`, `-restart_recovery`, and
  `-restart_awaiting_quiescence` had no selector, so interrupted attempts fell back to the neutral
  badge; failures now read at error salience and interruptions at waiting salience. The refused
  cross-block drag now states the refusal in flight through the pointer instead of an unexplained
  snap-back, specified as new FS-02.R53/A35. **INV §6/§10** — FS-18.A1's lifecycle-composition
  matrix now covers all three composers across seven lifecycles with the package available and
  unavailable, asserting the effective process parameters once and their absence from frozen session
  metadata; removing the overlay call from `resume.go` fails it. **INV §10** — the exact migration
  gained corrupt-read and read-only-directory write-failure fixtures that compare the role bytes
  before and after. `make check-specs`, `make build`, both Go test variants, the UI suite, and the
  UI build pass.

- **2026-08-30 — review (FS-18, FS-04.R44/A24, FS-03.R38/A21, TS-11, INV
  §1–§15):** Reviewed the continuous `43e5feb..52d01c4` range end to end. The package installer,
  conditional runtime-only overlay, exact migration, lifecycle wiring, and release/workflow state
  match their specifications, but three findings are open below. One **Must fix**: Mermaid CSS is
  scrubbed with a literal `url`/`@import` regex, so a CSS-escaped identifier survives sanitization
  and can still resolve as a remote request in the browser. Two **Worth fixing** acceptance gaps:
  FS-18.A1's lifecycle matrix is tested only at the overlay helper and terminal argument layer, and
  FS-18.A5's read/write-error migration fixtures are absent. Every invariant class was swept:
  §§6, 8 and 10 produced these findings; applicable surfaces under §§1–5, 7, 9 and 11–15 produced no
  new finding. `make check-specs`, `make build`, the full default and `sqlite_fts5` test variants,
  and `make dist` pass; the first sandboxed test attempt could not use the Go cache or bind a
  loopback test port, then passed unchanged with those required permissions.

- **2026-08-30 — usability review (FS-14.R43/R45/R46/R48, A14/A18–A22/A24/A26; FS-02.R45–R52,
  A28–A34; FS-12.A13):** Drove the new Pipelines pages and the changed dashboard grid through a real
  Chromium against a `make dist` build of the shipped tree. Twenty-nine steps passed, including all
  three fixes that had never been in a browser (attempt badge on the timeline rule, run rail
  stacking after the timeline at the 1024 floor, one-shot entrance on an appended entry with
  reduced motion removing it) and every J5 item that was owed for running-first placement and the
  chat panes. Four findings are open: two Must fix (a failed layout read silently disables layout
  persistence for the session; a launch- or resume-failed pause still offers a dead-end **Open
  agent**) and two Worth fixing (failure states render in the neutral badge; a refused cross-block
  drag gives no reason). No product code, specification, or journey matrix changed. Coverage gaps
  between the J5/J14 charters and FS-02/FS-14 acceptance items are recorded in the run file.

- **2026-08-30 — release (FS-10, TS-06.R13–R22, TS-11.R1/R8):** Cut `v0.3.0` for the 55-commit
  `v0.2.3..HEAD` range under the new §16 role. No open findings blocked it; the user accepted the
  20-commit unreviewed range (`43e5feb..HEAD`) and chose a minor version for the bundled operating
  skill, split Pipelines surface, expandable chat panes, Mermaid chat rendering, and running-first
  card placement. Step 2 found the embedded package already correct for the range and recorded that
  as a result: the one agent-facing change after the design froze, FS-14.R47's stage participation
  boundary, is already stated in `build-and-run-pipelines.md`, and the mail budget of 15 and CLI
  launch forms still match the code. The README `~/.agentdeck/` layout was missing the
  `cache/agent-skills/` root TS-11.R2 introduced and is fixed. Both Go variants, 50 UI test files,
  `make dist VERSION=0.3.0`, and `git diff --check` pass; the binary reports `0.3.0` built with
  `sqlite_fts5`. The user authorized the push: `main` went out through `89014a5` (17 commits, which
  swept in another session's usability-review state commit), the tag followed, and release run
  `33296747765` published a verified archive, checksum manifest, and installer.

- **2026-08-30 — workflow (TS-11 §5, FS-10, TS-06.R13–R22):** Added the release role the shared
  operating skill was waiting on. Workflow §16 fixes the release range at the previous `vX.Y.Z` tag,
  blocks on open **Must fix** findings, requires the range to be read for agent-facing change and
  the embedded `internal/agentknowledge/operating-agentdeck/**` source to be refreshed under
  TS-11.R1/R8 ownership and exclusions, extends the same test to the README and the pinned
  `install.sh`/`assemble.sh` component versions, confirms the version with the user, verifies with
  the §2 checks plus `make dist`, and leaves assembly and publication to release CI behind explicit
  push authorization with the credentialed provider gates named as owed. Byte-identical `/release`
  launchers were added to `.claude/skills` and `.agents/skills`, and AGENTS.md now routes the role.
  Documentation-only: `make check-specs`, the twin comparison, and `git diff --check` pass.

- **2026-08-30 — work (FS-18, FS-04.R44/A24, TS-11, INV §1/§2/§4/§6/§8/§10/§15):** Shipped the
  release-matched `operating-agentdeck` package and thin AgentDecker role. Startup stages, syncs,
  verifies, and atomically publishes owner-only `.agents` and `.claude` views; a failure logs a
  warning, suppresses every availability signal, and leaves exact migration for a later retry.
  `applyKnowledgeOverlay` is the one fresh/resume/switch seam, using runtime-only add-dir, prompt
  suffix, and environment fields so no managed value enters the frozen session snapshot. Chat and
  every terminal driver consume the same effective parameters; iTerm transfers secret environment
  values through a bounded owner-only FIFO so its visible command and filesystem carry no secret
  data. Only the production SHA-256 of the immediately preceding AgentDecker seed migrates, with all
  other role fields preserved; fresh PM/teammate prompts and tool descriptions no longer duplicate
  cross-workflow guidance. Package, exact-fixture, failed-install retry, persistence, primer, and
  terminal regressions are green with both Go test variants, specification checks, production and
  distributable builds, and `git diff --check`. An independent audit closed its iTerm secret and
  migration-fixture findings; its old-cache note was rejected because FS-18 requires verified
  replacement of an older cache. Pinned adapters 0.59.0/1.1.2 are installed, but logged-in live
  provider discovery remains a manual gate.

- **2026-08-29 — feature design follow-up (FS-18.R2/R7/R11, FS-04.R44, TS-11.R4–R6/R8/R10):**
  Closed the shared-skill design review on the user's classification. Package verification now
  precedes exact AgentDecker migration; an unavailable package leaves the legacy prompt untouched
  for a later retry, and the thin seed prompt refers only to current operating guidance rather than
  claiming the bundled skill exists. The other two review items remain small required alignment
  cleanups: overlay directory/prompt additions use runtime-only fields that session persistence
  cannot see, and fresh PM/teammate seeds drop duplicated coordination mechanics and the numeric
  budget without migrating user-owned roles. Acceptance coverage now joins install failure to the
  exact-role fixture and later successful retry. The impossible comparison-error case and unrelated
  INV §11 citation were removed. The change is Waiting to start and ready for implementation.

- **2026-08-29 — design review (FS-18, FS-04, TS-11, INV §1/§2/§6/§8/§10/§15):** Three Must-fix
  findings block the waiting shared-skill change. The proposed helper adds the overlay to
  `LaunchSpec.AddDirs` and its prompt, but those are the frozen fields `runtimeMeta` writes back to
  the session; after one successful start, a later install-failed process can therefore resume with
  the stale directory/pointer that TS-11.R10 says must be absent. The exact AgentDecker migration is
  independent of package availability and its target prompt unconditionally tells the role to use
  the bundled skill, so an install failure can both remove the legacy manual and direct the agent to
  unavailable knowledge. Finally, the seeded PM and teammate prompts still own messaging tool names,
  wake behavior, recipient addressing, and the numeric budget assigned exclusively to
  `coordinate-work.md`, leaving two stale release-mismatched sources. Pinned `codex-acp` 1.1.2 and
  Claude 2.1.238 support skill discovery from additional roots, so no discovery finding remains.
  Two consistency notes record an impossible comparison-error fixture and a misapplied INV §11
  citation. No product code, specification, or change file was edited; the change remains Waiting
  to start.

- **2026-08-29 — feature design revision (FS-18, TS-11):** Tightened the shared operator skill to
  three runtime-operation references. Messaging budgets now belong only to coordination guidance;
  blocked/Continue/proposal behavior belongs to pipeline guidance; release maintenance is excluded
  from the product skill. Secure installation remains atomic and owner-only, but failure now logs a
  warning, starts AgentDeck, and suppresses every discovery/pointer claim for that dashboard
  process. The ready change remains waiting to start.

- **2026-08-29 — fix (INV §1/§2/§4/§5/§7/§8/§10):** Closed every open finding: the twelve from the
  2026-08-29 review of `6a16126..43e5feb`, the three from the 2026-08-28 bug investigation, and the
  seven remaining from the 2026-08-28 review of `790c01c`.

  The Must fix needed a product call and got one — state the blocked-stage boundary rather than
  build an in-chat continuation route, and withhold the dead-end action rather than widen Continue.
  New FS-14.R47 puts the boundary in the assignment, the accepted result, and the refusal; new
  FS-14.R48 withholds **Open agent** on a restart pause and names Retry. Refusals are now logged at
  Warn with the run, attempt, caller and attempt agent/generation, pending action and code — the set
  that separates the conditions the report conflated — and `OnTurnEnd`/`OnExit` re-read the run
  under the run lock as `Report` already did (INV §5).

  Two amplification defects in the shared SSE transport (INV §1/§4/§8): a joining tab restarted the
  one shared stream, costing every other tab a full re-hydration and dropping their deltas, and the
  worker never removed a port. The worker now replays a bounded retained snapshot to the joining
  port alone and drops a port that says goodbye, closing the stream with the last one (TS-03.R7).

  Two artifacts built twice were unified (INV §2): the card preview kept opposite ends of a message
  on the client and the server, now both the tail clipped by code point; and Retry eligibility, hand-
  duplicated across the language boundary and already drifted once, is now projected by the store as
  `retry_eligible` (new TS-10.R22).

  In the grid, each running/stopped block gained its own sortable context so an in-drag preview
  cannot cross the boundary FS-02.A28 promises it will not, an expanded card stays in its block's
  set so neighbours see its two-column footprint (FS-02.R47), and pane focus cycling binds once at
  the grid container so it can leave a group section (FS-02.R50).

  Smaller: Mermaid's scratch node is torn down on a draw failure; the run page keys `RunDetail` by
  run id; the run-list projection no longer claims a run-detail read it never made; an agent-target
  task row no longer opens with a stray separator; the reconcile regression now proves its own name;
  the tracked build caches and the emitted worker chunk are untracked; and TS-06 §6 names the stress
  fixture. Coverage that acceptance items named but no test provided was added for the pane change,
  the delegated-agent cards, the looping timeline, and the proposal counts, and FS-02.A27 was
  narrowed to what its suite proves. Both Go test variants, the 329-case UI suite,
  `make check-specs`, the UI typecheck, `make build` and `git diff --check` are green. Two browser
  checks remain owed and are recorded as acceptance gates.

- **2026-08-29 — feature design (FS-18, TS-11):** Specified a thin AgentDecker role backed by one
  product-managed `operating-agentdeck` skill available to every launched role. The ready change
  covers exact-only migration of the historical prompt, job-oriented progressive references,
  native and direct-path discovery, one lifecycle composition seam, and a four-way release
  maintenance classification. It remains waiting to start; no product code changed.

- **2026-08-29 — fix (INV §8):** Closed the false `stale_assignment` Must-fix finding. A report from
  the run's own current attempt after its prior result was accepted now returns the shared
  `already_reported` code and points to the human Continue boundary; genuine caller or generation
  mismatches still return `stale_assignment`. The previously skipped field-bug reproduction is now
  an active regression, and both Go test variants plus the product build are green.

- **2026-08-29 — fix (INV §1/§4):** Closed the pane transcript-retention Must-fix finding. Raw
  events now live only in a constant-time append tail while an authoritative transcript request is
  in flight; the tail is cleared when the newest request settles, the last surface unregisters, or
  the agent is removed. The permanent folded transcript still preserves streamed deltas and resolved
  permissions across refetches. Both Go test variants, the 302-case UI suite, specification checks,
  and production builds are green.

- **2026-08-29 — fix (INV §2/§8):** Closed the template deep-link Must-fix finding. A failed
  template-library request now renders a load failure and the transport message instead of claiming
  the template was deleted; a missing record after a successful read keeps the deletion guidance.
  The new acceptance regression and the full 301-case UI suite are green with both Go test variants
  and production builds.

- **2026-08-29 — fix (INV §8):** Closed the diagram sanitizer Must-fix finding. Renderer-produced
  inline `style` attributes now have remote `url(...)` and `@import` references removed at the same
  DOMPurify seam that already scrubs style-element text. The safety regression covers both the
  generated attribute and a zero-request assertion; both Go test variants, the 300-case UI suite,
  the product build, and the UI production build are green.

- **2026-08-29 — review:** Read the whole unreviewed range `6a16126..43e5feb` against the
  specifications and every invariant class, clearing the backlog of three commits that had only
  ever had design and usability reviews. Fifteen findings, three of them Must fix. The diagram
  sanitizer forbids URL attributes but not `style=""`, and DOMPurify treats `style` as URI-safe by
  default, so an author `classDef` carrying `url(https://…)` defeats FS-03.R38's no-network
  guarantee at the configuration level. A failed `/api/pipelines` fetch makes the template page
  claim the template was deleted, where its own twin `RunDetail` distinguishes 404 from any other
  read failure — the two paths drifted inside one commit series. The new pane store keeps a second,
  unfolded copy of every transcript event with no cleanup on any lifecycle boundary and a full array
  copy per streamed delta, on exactly the four-pane streaming workload the feature creates. The
  remainder are smaller: pane keyboard cycling cannot leave a group section, expanded and
  cross-block drags compute in-drag geometry over a list that does not match what is rendered,
  a draw-stage diagram failure leaks a node onto `document.body`, the delegated-agent and
  proposal-count UI has no frontend test behind the acceptance items that name one, and several
  traceability and tracked-artifact gaps. Both Go test variants, the 289-case UI suite,
  `make check-specs`, `git diff --check` and all nine skill twins are green at `43e5feb`; every new
  `rows.Next()` loop in the range checks `rows.Err()`. One earlier finding is corrected: the tracked
  `internal/server/ui/dist/index.html` is covered by a `.gitignore` negation that has existed since
  the first commit, so only the emitted worker chunk is genuinely tracked outside the rule.

- **2026-08-29 — work:** Shipped expandable chat panes on project dashboards. Up to four chat cards
  can expand in place, persist across reloads, retain per-agent drafts, cycle by keyboard, and keep
  their transcript scrolling isolated from the page. The shared SSE client now registers multiple
  open agents with reference-counted teardown, reconnect and gap recovery fan out only to registered
  agents, and stale transcript responses cannot erase newer streamed events or resolved permissions.
  Expanded cards leave dnd-kit sorting, keep activation on the header rather than pane controls, and
  silently evict the least-recently-used pane at the cap. Server layout validation and round-trip
  coverage, 300 UI tests, `make check-specs`, `make build`, `make test`, `make dist`, and whitespace
  checks are green. A real Chromium pass at 1280×800 verified fixed 640px pane geometry, two-column
  span, stable neighboring cards and page position during streaming, internal long-transcript
  scrolling, focus cycling, persistence after reload, and rendering under Core and Sky & Grove.

- **2026-08-29 — design-feature:** Resolved the 2026-08-29 design review of the expandable chat
  pane change. All five findings and all three consistency notes are closed in the specifications;
  none was rejected, because each named a real consequence that survived checking against the tree.
  Two changed the design rather than its wording. `AgentCard` carries `onClick`/`onContextMenu` on
  its outer `<article>` and the pane composes inside it, so an ordinary Send, permission decision,
  or autocomplete accept would have collapsed the pane or opened the card menu; new FS-02.R52 and
  A34 move both handlers to the card header for an expanded card, and TS-08.R43 records why that
  structural boundary was chosen over per-control `stopPropagation` — an opt-out list is the INV
  §2/§10 drift shape, since every control added later must remember to join it and a miss fails
  silently. And `setTranscript` replaces an agent's whole slice, so a refetch resolving after newer
  deltas drops them or reverts a resolved permission chip; TS-03.R31 now specifies the two repairs
  FS-03's own §6 deviation already named — a per-agent request token (INV §1's canonical pattern,
  already used by `FilesTab`/`CommandsTab`) and seq reconciliation that retains events newer than
  the response's maximum — so the four-fold fan-out closes that advisory instead of multiplying it,
  for the agent screen as well as the panes. The three lower-severity findings tightened evidence
  and definitions: FS-02.A29's geometry claims moved to J5 because jsdom evaluates no grid sizing,
  stretch, overflow, or scroll position (INV §13); R48 now names the three events that mark a pane
  as used, including a pointer press, because the transcript's scroll region is not focusable and a
  reader would otherwise lose the pane they were reading; and A30 gained the running→stopped
  transition, which every previously named case would have passed while closing the pane. The
  consistency notes closed as FS-02.R46's corrected host, TS-08.R42's narrowed scroll-region claim
  (`.annotation-tray-body` and `.composer-picker` also scroll), and new FS-12.R38/A14 scoping
  R15's no-new-shortcut clause to the presentation change it was written for.

  One defect the review did not raise is also fixed, and it was the more damaging one. R49 said an
  expanded id outside the grid's current scope is dropped from the next save — but `CardGrid` mounts
  only at `/project/:project`, so *every* id belongs to some other project the moment the operator
  opens a different one, and the debounced `PUT` would have wiped the first project's arrangement on
  arrival at the second. R49 and A32 now retain an out-of-scope id unrendered and write it back
  unchanged; only an unknown or archived agent is pruned. That same correction disposes of the
  original wrong claim that the projects home hosts an agent grid: it renders project cards, and
  after FS-02.R29's project-first split the project dashboard is the only agent-card surface.
  FS-12 joins FS-02, FS-03, TS-03, and TS-08 as Partial. No product code changed. `make
  check-specs`, the skill-twin comparison, and `git diff --check` are green.

- **2026-08-29 — design review correction:** Removed the projects-home Must-fix finding after the
  user clarified that "projects page" means the agent-card dashboard reached after selecting a
  project (`/project/:project-id`), not the root project-card catalog. That intended host is the
  existing `CardGrid` mount and is buildable. The misleading "projects-home and scoped project
  grid" wording remains a consistency note for follow-up design cleanup. Two Must-fix and three
  Worth-fixing findings remain; no product code, specification, or change file was edited.

- **2026-08-29 — design review:** Reviewed the waiting expandable-chat-pane change against FS-02,
  FS-03, FS-12, TS-03, TS-08, every invariant class, the affected grid/chat/SSE/layout code, and the
  incumbent Core visual matrix at 1280×800. Three Must-fix and three Worth-fixing findings are
  recorded below. The promised projects-home host does not exist after FS-02.R29's project-first
  split; an expanded pane composed inside `AgentCard` inherits root click/context-menu handlers that
  would collapse it or open the card menu during ordinary chat interaction; and concurrent
  transcript refetches can replace newer SSE deltas because `setTranscript` is an unguarded replace.
  Lower-severity gaps leave actual pane geometry without browser evidence, least-recent focus
  undefined, and the stopped-pane promise without acceptance coverage. Two page-only consistency
  notes record the `only scroll region` overstatement and FS-12.R15's stale no-shortcut clause. No
  product code, specification, or change file was edited; the change remains Waiting to start and
  is blocked from implementation until follow-up feature design resolves the Must-fix items.

- **2026-08-29 — design-feature:** Specified expandable chat panes on the agent card grid. A
  chat-interface card now expands in place into a transcript-plus-composer pane spanning two grid
  columns, up to four at once, so an operator can read one agent, answer a blocked one, and keep the
  rest of the grid's state on screen instead of paying a route round-trip per agent. Card-body click
  toggles expansion, which supersedes FS-02.R8's navigate clause; right-click, the context menu's
  **Open chat**, and `/agent/:id` itself are untouched, and the pane's name links to that page. The
  `/ux` trigger passed — this changes an established, many-times-per-hour task — and its walkthrough
  changed the design twice: a card expanding inside `repeat(perRow, 1fr)` would be *one column* wide
  and so would worsen the space complaint it was meant to fix, and a pane that grows the page rather
  than scrolling internally would let one agent's stream move another agent's reader. Both are now
  pinned by FS-02.R47 and TS-08.R42, which also record that `.card-grid` must gain `align-items:
  start` (it declares only `display: grid` today, so the default `stretch` would inflate every
  collapsed card sharing a pane's row) and that `grid-auto-flow` must never become `dense`, since
  dense packing reorders items and FS-02.R47 forbids expansion changing order. Four decisions were
  the user's and are recorded as chosen: click-to-expand over a separate chevron; a four-pane cap
  that silently collapses the least-recently-focused pane, which loses nothing because drafts are
  already per-agent and browser-local (FS-03.R36); persistence in `layout.json` beside order,
  density, and group collapse; and `Ctrl+Alt+ArrowDown`/`ArrowUp` focus cycling. Two exclusions were
  proposed and accepted: no card ever auto-expands from a notification or state change, so the grid
  never reflows while it is being read, and expansion belongs to the card grid, so it works on the
  projects home as well as a scoped project. Terminal-interface cards keep navigating, because the
  pane deliberately carries no terminal. The load-bearing technical finding is that `ui/src/api/sse.ts`
  holds a single `openAgentId`: only that agent's deltas are appended and only its sequence gaps are
  refetched, so "several live chats" is precisely what today's client cannot do. TS-03.R31 turns it
  into a reference-counted set whose registration returns its own teardown (INV §4), keeps the set
  alive across reconnect while `lastPing`/`hydrationIds`/`lastAgentSeq` keep resetting on `onopen`
  (INV §1), and bounds the reconnect refetch fan-out to the same four panes so it cannot re-create
  the origin connection-pool exhaustion TS-03.R7 exists to prevent (INV §7). `layout.json` gains one
  additive `expanded` list; a file written before this change reads unchanged and needs no
  migration. TS-08.R41/R43 hold the pane to composing the shipped `TranscriptView` and `Composer` —
  both already `agent_id`-parameterized with per-instance scroll refs — rather than growing a second
  chat surface (INV §2), and reuse the existing collapsed-section filter to keep an expanded id out
  of dnd-kit's sortable list. FS-02 and FS-03 add R46–R51/A29–A33 and R39/A23; FS-02, FS-03, TS-03,
  and TS-08 move to Partial with every new item tagged `(planned)`. No product code changed.
  `make check-specs`, the skill-twin comparison, and `git diff --check` are green.

- **2026-08-28 — workflow:** Retuned `/ux` for AgentDeck's actual small, experienced internal
  operator set. Automatic use now starts with a no-artifact trigger check and runs a full pass only
  when work changes an established task or introduces an unfamiliar or consequential decision,
  ambiguous state, recovery path, long-running operation, or AI uncertainty; ordinary user-facing
  additions and familiar interactions stay in their normal workflow. The task frame now defaults to
  one primary repeated-use task, adds a second only for a materially different high-risk branch,
  and explicitly favors speed, density, keyboard flow, predictability, learned shortcuts, and
  control over hypothetical-novice simplification. Walkthrough questions use the operator's real
  knowledge and habits; onboarding and rediscoverability apply only when the task makes them real.
  Real-browser validation now runs only when rendered interaction, timing/state, recovery, or an
  unresolved design risk can change the judgment, plus explicit standalone critique; behavior fully
  established by acceptance tests does not earn the harness cost. The canonical workflow, routing
  summary, and Claude/Codex launchers agree; skill twins, YAML parsing, specification checks, and
  whitespace checks are green.

- **2026-08-28 — workflow:** Added the twinned first-party `/ux` skill and canonical workflow §15.
  It accompanies `/design-feature` automatically before meaningful user-facing behavior is
  confirmed, joins implementation only when acceptance materially depends on task flow, state,
  consequence, recovery, long-running work, or AI interaction, and remains available as a focused
  read-only critique. The workflow distills cognitive walkthroughs, NN/g heuristics and focused
  patterns, Microsoft HAX, CLI Guidelines, Good Services, and the non-duplicative PAIR caution
  against implying human understanding into task framing, a two-question walkthrough, behavioral
  contract hardening, and focused real-product validation. Findings must name the actual task,
  observed friction, consequence, evidence, and a proportionate repair. `/design` continues to own
  visual composition and polish while sharing task frames and browser passes where the lenses
  overlap; `/usability-review` continues to own the product-wide acceptance matrix. Independent
  forward tests against the Pipelines split, Mermaid rendering, and running-first card placement
  recovered the existing blocked-run chat/Continue failure, produced no Mermaid false positive,
  and isolated cross-boundary drag feedback as an unverified J5 risk rather than a finding. The
  internal SharedWorker fallback correctly did not trigger the skill. Specification checks, all
  skill-twin comparisons, YAML frontmatter parsing, and whitespace checks are green. The bundled
  skill validator itself could not start because the host Python lacks PyYAML; its covered
  frontmatter/name/placeholders were checked directly instead.

- **2026-08-28 — work:** Shipped running-first card placement. Inside every group section of an
  agent card grid, running agents now render before stopped ones and the persisted manual order
  survives inside each of those two blocks, so a card crosses the boundary the moment its agent
  starts or stops with no reload. `running` is the sole test: the live `state` values still express
  themselves through salience alone and never move a card, which is why FS-12.R10's "without
  changing order" clause is narrowed by FS-12.R37 instead of being contradicted. Nothing new is
  persisted — `layout.json` keeps exactly the order, density, and per-group collapse state it held
  before, and a same-block drag still commits the identical flat manual order it committed before
  the split existed. Two things the split forced: dnd-kit now receives the order the cards actually
  render in, skipping collapsed sections whose cards mount no sortable node, because its indices and
  rect transforms are derived from that list; and a drop onto the other running/stopped block
  returns before `arrayMove` and before any layout write, so manual drag cannot override the
  boundary or make a hidden change (INV §10). Seven new `CardGrid.test.tsx` cases cover placement,
  the live flip, the registry order under a collapsed section, salience-without-movement, Ungrouped
  staying last, the same-block payload, and the cross-block no-request; five fail against the
  pre-change component. FS-02 and FS-12 have no `(planned)` items left, so both move to Current in
  their headers and the spec index. `make test` (both Go variants), `make build`, `make embed`,
  `make check-specs`, `git diff --check`, `tsc --noEmit`, `npm test` (289 passing) and `npm run
  build` are green. J5's real-browser half is owed: no browser was available in this session.

- **2026-08-28 — docs:** Replaced `/design`'s rigid one-repair/one-confirmation stopping rule with a
  quality gate. Agents still batch rendered findings, but they continue repairing and re-rendering
  while material in-scope problems remain; they stop when the direction and material findings are
  satisfied, without drifting into subjective polish. The twinned launcher and canonical workflow
  use the same rule.

- **2026-08-28 — implementation:** Added the twinned first-party `/design` skill and canonical
  workflow §14 after researching Impeccable, Emil Kowalski's design/animation skills, Anthropic's
  frontend-design skill, and designer-skills' design-review flow. Automatic discovery is deliberately
  narrow: it applies to new screens, redesigns, meaningful composition/styling, motion, polish, and
  critique, while routine frontend wiring and style-preserving fixes stay in their normal role. The
  workflow records one compact direction, derives specificity from AgentDeck's operator and
  lifecycle/coordination states, gates motion by purpose and frequency, extends TS-08's tokens,
  primitives, hooks, appearances, and visual matrix, and requires a bounded real-browser critique
  that continues while material in-scope problems remain and stops before subjective polish. It adds
  no product code, design-system layer, vendored skill pack, detector, script, screenshot baseline,
  or runtime dependency. Standalone critique remains read-only, and automatic invocation never
  selects or enlarges work. Independent forward tests covered an open Pipelines redesign, an
  incidental API/UI field, and a read-only Dashboard critique; the skill twins, YAML frontmatter,
  specification checks, and whitespace checks are green.

- **2026-08-28 — fix:** Closed the Tasks Retry finding from the `790c01c` review (INV §10). The
  view now gates Retry on the same predicate the server uses — `interrupted`, or `dependency_failed`
  with no `unsatisfiable` arm — instead of on `interrupted` alone. A task parked because its three
  start attempts were spent, or because its target became ineligible, gets back the repair FS-16.R23
  names for it; a task parked by an arm that can never be satisfied still shows Re-arm and no Retry,
  so the view never offers a control that would be refused. No behavior beyond the specification
  changed, so this restores FS-16.R23/R25 rather than altering them; A11 gains the UI half of its
  verification, which had no home before. A new regression covers both parked kinds in one list and
  fails against the pre-fix gate. `make test`, `make build`, `make embed`, `make check-specs`,
  `git diff --check`, `tsc --noEmit`, `npm test` (282 passing) and `npm run build` are green.

- **2026-08-28 — fix:** Closed the one Must-fix finding from the `790c01c` review (INV §6/§8/§12).
  The shared-worker SSE transport now has a reachable failure path: construction runs inside
  `try`/`catch`, the `SharedWorker` object's `error` event is handled, and a stream that never opens
  within the liveness window is demoted rather than reconnected into. All three routes fall back to
  the direct `/api/events` stream for the rest of the session, so a browser that exposes
  `SharedWorker` but cannot run it — or a worker asset that fails to load — no longer leaves the
  dashboard on `connecting` with no live data, no error and no retry. TS-03.R7 now states that
  sharing is best-effort and names the fallback, which was previously unspecified. `sse.test.ts`
  gains a `SharedWorker` stub and four cases: port fan-out including a ping satisfying the liveness
  window, and one per fallback route; each fails against the pre-fix code. Bookkeeping while
  verifying: the FS-02.A27 finding is narrowed to the un-run six-tab browser check now that the
  transport has tests, and a new finding records that `index.html` and one worker asset are
  force-tracked under the ignored embed directory. `make test`, `make build`, `make embed`,
  `make check-specs`, `git diff --check`, `tsc --noEmit`, `npm test` (281 passing) and `npm run
  build` are green.

- **2026-08-28 — review:** Read the shipped diff of `790c01c` (the thirteen dashboard/SSE and
  usability fixes) against FS-02, FS-04, FS-08, FS-16, FS-17, TS-03, TS-07, TS-10 and every
  invariant class. Eight findings recorded below: one **Must fix** and seven **Worth fixing**. The
  headline fixes are real — tabs do share one SSE stream, reconciliation is debounced to the changed
  session, card previews no longer clone the whole transcript map, and the six usability items are
  closed — but the shared-stream transport ships with no failure path and no test, and the Tasks
  Retry gate hides the repair FS-16.R23 requires. Two findings changed after the user reviewed them
  on 2026-08-28: the narrowed config-source publication gate (`changedFields` suppressing every
  change outside model/effort/assets, so an open Settings → Sources panel keeps a superseded view
  until reload) is the user's accepted boundary and is removed, leaving the amended FS-08.R15 as
  written; and the Tasks Retry finding drops to **Worth fixing** because the blank Re-arm form does
  return the task to `ready`, so the defect is discoverability and FS-16.R23 conformance rather than
  unrecoverable work. Classes with no finding: §3 (the create-then-edit switch in `ProjectsEditor` is
  correct), §5 (the reconcile debounce timer's stop/drain/reset is race-free), §7
  (`lastAssistantPreview` returning empty on an unreadable path cannot blank a card, because
  `ApplyStaleCorrection` refuses an empty detail), §9, §11 (`changedFields` returns a non-nil slice),
  §13 (the diff ships no new className), §14 (no new route; the worker's `/api/events` request is
  same-origin and inherits `localOnly`) and §15. `make check-specs`, `git diff --check`, `go test`
  for `internal/{configsource,server,state}`, `tsc --noEmit` and the 277-test UI suite are all green.
  `Last reviewed code` stays at `6a16126`: this review read one named commit, not a continuous range.

- **2026-08-28 — docs:** Corrected the review-state bookkeeping. `Last reviewed code` moves from
  `895348e` to `6a16126`, the last code commit actually read — `bbbdc90` verified it and did not
  advance the pointer. Review state now names the four commits whose diffs have had no §7 code
  review (`790c01c`, `c35ff8c`, `9114df7`, `69c2f99`), so the stale pointer stops implying that
  the twenty-plus commits behind it are all unreviewed when most are review sessions themselves.

- **2026-08-28 — bug investigation:** Diagnosed the field report that a blocked pipeline stage agent
  got `stale_assignment` when it tried to continue. Reproduced the most reachable route locally: a
  `blocked` report leaves the stage agent live and idle and the run offers **Open agent** beside
  **Continue**, so answering the question in that chat makes the agent's next report land on
  `internal/pipeline/actions.go:198`, which returns `stale_assignment` / "caller is not the current
  stage attempt" for a caller that *is* the current attempt under the current generation — a retry
  class of `never` (FS-17) on a false reason, discarding a full turn of work. Refusing is specified
  (FS-14.R19); the code, the message, and the silence around them are not. Four findings recorded:
  the misclassified refusal, the unspecified blocked-pause boundary (needs a user product
  decision), the total absence of refusal logging, and an unlocked-read compare-and-swap in
  `OnTurnEnd`/`OnExit` that can park a run at `await_quiescence` (INV §5, probable). No product code
  or specification changed; the only tree change is the skipped reproduction test.

- **2026-08-28 — docs:** Retired the completed *Projects page problems* and 2026-08-10
  play-session ideas at the user's direction, and closed out the Pipelines redesign paperwork: its
  finished change file is removed and it no longer appears under *Changes waiting to start*, with
  FS-14 left as the authority on what shipped. The card drag-and-drop and Content-Security-Policy
  ideas were checked against the tree and kept: drag listeners are still bound to the handle button
  alone and no CSP exists in the server, the UI shell, or the specs.

- **2026-08-28 — fix:** No open review findings. `## Review findings` was empty and the
  working tree was clean, so no code, test, or specification changed. The committed Pipelines
  redesign and its six usability fixes are still waiting for their independent code review.

- **2026-08-28 — fix:** Closed all six Pipelines usability findings. The attempt badge sits on the
  timeline rule instead of off the left window edge and the run rail no longer hoists itself above
  the timeline at the desktop floor (INV §13). The start path explains every gate it applies: with
  no valid template **Start run** is refused with a link to Templates, and a disabled step control
  names the missing value and marks the empty required input (INV §8/§10) — new behavior, specified
  as FS-14.R46/A24. A newly appended timeline entry now plays the one brief entrance FS-14.R45/A21
  already promised, tracked from the ids present at first render so neither the initial list nor a
  background refetch replays it (INV §10). A deleted run explains itself in product language, while
  any other read failure says so honestly and keeps its transport detail (INV §8).

- **2026-08-28 — usability review:** Drove the split Pipelines surface through a real Chromium
  against a `make dist` build of the working tree, on three isolated fixtures: an empty home, a home
  where one run was started end to end through the dialog, and a seeded home carrying a seven-attempt
  repair loop, the approval/blocked/crashed pause branches, 121 retained runs, 26 delegated tasks on
  one attempt, a second-hop task, a task whose agent record is gone, a 32-stage template, and one
  pending proposal of each kind. Zero blockers; five **Worth fixing** findings and one polish item
  recorded below. Routing, the execution timeline, attempt agent cards and their live/archive routes,
  the delegated cap and true counts, bounded run history to an exact end, frozen stage titles after
  template deletion, the focused editor, action gating, legacy links, background-refetch stability,
  reduced motion and the Sky & Grove appearance all behaved as specified with no console error.
  Full run:
  [`../archive/reviews/usability-review-run-2026-08-28-pipelines.md`](../archive/reviews/usability-review-run-2026-08-28-pipelines.md).

- **2026-08-28 — work:** Shipped the Pipelines redesign. Runs and Templates now have separate,
  addressable list/detail experiences; run supervision is timeline-first with frozen setup, named
  values, complete paginated history, and reload-safe stage/delegate agent summaries; template
  authoring is a focused one-stage workspace; and start-run uses a stable three-step dialog. The
  additive run projections use one indexed, bounded task read and preserve the existing pipeline
  control plane. The full UI and Go suites, spec checks, release build, and real-browser checks at
  the supported desktop floor are green; browser QA also found and closed a runtime-dialog footer
  overflow before completion.

- **2026-08-27 — fix:** Closed the active-first Ordering design finding (INV §10). FS-02.R45/A28
  now require the rendered running-first id order to drive drag geometry, same-block drops to map
  back to the unchanged flat manual order, and cross-block drops to perform no reorder or layout
  write. J5 and the ready change cover the visible in-drag boundary and persistence behavior. The
  ready change now names all six `AgentStatus` values, including `unknown`; the broader cross-group
  and whole-card drag problems remain separately out of scope.

- **2026-08-27 — design review:** Reviewed the waiting active-first Ordering change against FS-02,
  FS-12, the shipped card-grid/layout/SSE seams, dnd-kit's pinned sortable implementation, and every
  invariant class. The render-time `running` partition cleanly extends the existing group projection
  and needs no server or persistence change, but the design leaves `SortableContext` keyed to the
  differently ordered flat manual list. One implementation-gating drag finding and one consistency
  note remain. No product code, specification, or change file was edited.

- **2026-08-27 — work:** Shipped Mermaid rendering in assistant messages (FS-03.R37–R38,
  TS-08.R40). A closed ```mermaid fence becomes a themed diagram; an open one stays a code block
  until its closing delta arrives, decided from react-markdown's source span. `renderers/mermaid.ts`
  owns the 50,000-code-unit bound, the refused external-image grammar, strict initialization, and a
  directly declared DOMPurify pass that also strips remote references from the generated `<style>`;
  `MermaidDiagram.tsx` is the only markup-insertion seam, now enforced by a new `renderer-markup`
  rule in the presentation audit plus its manifest entry and a repository check. Mermaid and
  DOMPurify load on demand, so the initial bundle grows only by the host code (1,813.87 → 1,818.43
  kB). J3's diagram steps were driven through a real Chromium against the production server and
  embedded UI on the deterministic `diagram_stream`/`diagram_injection` fake-ACP scenarios: open
  fence stayed a code block, the closing delta promoted it, the source toggle round-tripped, a live
  skin change regenerated the mounted diagram, reload and the archived transcript matched, and the
  injected script/HTML-label case rendered inert with no console error and no external request.

- **2026-08-27 — fix:** Closed all three Mermaid design-review findings (INV §8/§10). The design
  now rejects Mermaid's pinned external-image grammar before rendering, directly owns DOMPurify,
  replaces the unenforceable main-thread timeout with an exact 50,000-code-unit source bound, and
  reuses the existing presentation observer to regenerate mounted diagrams after a skin change.
  It explicitly avoids a worker, iframe renderer, or custom parser; acceptance tests cover the
  no-request preflight, size fallback, and live palette change.

- **2026-08-27 — design review:** Reviewed the waiting Mermaid chat-rendering change against FS-03,
  TS-08, the shipped Markdown/presentation/archive seams, Mermaid 11.17.2, and every invariant
  class. The existing `AssistantText` and presentation adapters are the right extension points, and
  the closed-fence state is recoverable from react-markdown's source-position metadata. Three
  implementation-gating findings remain: Mermaid image nodes can make an attacker-chosen request
  before returned SVG can be sanitized, a main-thread render cannot be interrupted by the promised
  elapsed-time budget, and an already-rendered SVG does not observe a later skin change. No product
  code, specification, or change file was edited.

- **2026-08-27 — design:** Defined two small UI changes and held a third. FS-03.R37–R38 and
  A20–A22 specify Mermaid rendering in assistant messages, gated on the closed fence because every
  delta re-renders the whole message, with a code-block fallback and one sanitizing injection seam;
  TS-08.R40 puts the renderer on the existing R13 adapter seam as a dynamically imported chunk
  pinned at or above the release fixing the known diagram HTML-injection defect. FS-02.R45 and A28
  order running agents before stopped ones inside each group while preserving the manual order, and
  FS-12.R37 narrows R10's ordering clause, which forbade it. FS-02, FS-03, FS-12 and TS-08 are now
  Partial; J3 and J5 cover the new evidence. Queued `mermaid-diagrams-in-chat.md` and
  `active-agents-before-stopped.md`. Edit-a-sent-message was **not** specified: ACP v1 as
  implemented has no rewind method, the transcript and FTS documents are append-only, and the only
  rewind-shaped seam is the lossy switch primer, so the user chose to hold it with those findings
  recorded in `../ideas.md`. The absent Content-Security-Policy is recorded there too.
  `make check-specs`, the twin-skill comparison and `git diff --check` are green.

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
- **Failed pipeline-stage chat:** Confirm whether a pause after a failed launch or resume should
  keep withholding **Open agent**, matching restart recovery (FS-14.R48), or whether the chat should
  remain reachable with a wider continuation contract.
- **Refused card drag feedback:** Confirm whether the cross-block refusal should remain an in-flight
  pointer signal (FS-02.R53) or whether snap-back alone is the intended behavior. The shipped pointer
  implementation currently has an open wiring finding below.

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
- [ ] Drag a running card over the stopped block in a real browser and confirm the computed cursor
      on the card under the pointer states the refusal, clears when the pointer returns to its own
      block, and clears when the drag ends (FS-02.A35, J5). jsdom evaluates no CSS, so the unit
      cases cover only the marked state and the stylesheet rule.
- [ ] Run a task start, an assignment turn, and a reported result against the pinned Claude and Codex
      adapters before claiming dependent work works with real providers (FS-16 §6).
- [ ] Run one successful and one refused MCP tool call through pinned Claude and Codex adapters before
      claiming they accept structured tool results without losing the text block (FS-17.A6).
- [ ] Run the Phase 7 federation discovery/precedence/refresh/launch/resume matrix against real Claude and
  Codex installations before promoting FS-08/TS-07 from Partial.
- [ ] Run the six-tab same-origin dashboard check against a `make dist` build (FS-02.A27). The
  transport half is now covered by `ui/src/api/sse.test.ts` and A27 has been narrowed to say so;
  the browser half has never been run against a build carrying the shared stream.
  `scripts/stress-fixture` (TS-06 §6) is the fixture.
- [x] **Closed 2026-08-30.** A real Chromium run covered J5's running-first placement, live
  running/stopped boundary crossings in both directions, in-drag geometry within one block, refused
  cross-block drop, and the expanded pane's two-column drag footprint (FS-02.A28, FS-02.R47).
  Evidence is in the J5 section of
  [`../archive/reviews/usability-review-run-2026-08-30-new-pages.md`](../archive/reviews/usability-review-run-2026-08-30-new-pages.md).

## Blocked on human

Live-provider acceptance is waiting for human authorization because it invokes real provider sessions
and creates disposable local configuration homes. On 2026-07-15 this machine has Claude Code 2.1.202,
the retired `claude-code-acp`, Codex CLI 0.142.5, and `codex-acp` 1.1.2 installed; the new
`claude-agent-acp`, OpenCode, and OpenHands are not installed globally.

## Review findings

- **Worth fixing** (FS-16.R3/R4, TS-10.R15/R19; INV §15) — `internal/server/task_http_test.go:244`
  asserts the cancel response already carries `pending_release=false` and an empty runtime claim,
  but `finishInterruptedRelease` only clears them when its `StopStage` succeeds; a failed stop is
  specified to log and leave the release for recovery (TS-10.R19/R15). Observed once on 2026-08-31
  during a full `internal/server` run under load: the response carried `RuntimeClaim:created
  PendingRelease:true` and the case failed. It passes alone, twenty times under `-race`, and on a
  repeat full-package run, so it is a load-dependent flake rather than a new regression. Decide
  which side is wrong — either the cancel path owes a completed release before it answers, or the
  case should assert the recovery-completed state instead of the synchronous one — and record it in
  FS-16/TS-10 rather than loosening the assertion.

- **Must fix** (FS-17.R18/A9, TS-04.R33/R34/R36/R39, TS-06.R23; INV §12) — the design's private
  loopback transport is unreachable from a Codex agent's own shell. Normal-use trigger: a Codex chat
  agent is woken for mail, or launched for a pipeline stage, and runs
  `$AGENTDECK_ACTION_CLI action check_messages`. That command executes inside Codex's sandbox, which
  cannot open a TCP connection to the dashboard's loopback port, so the agent gets a provider-owned
  "couldn't connect" instead of its mail — and every Codex agent silently loses messaging, tasks,
  context links, and pipeline reporting. Verified on this machine with Codex 0.142.5:
  `codex sandbox -- curl http://127.0.0.1:<port>/` returns `curl: (7) Failed to connect`, the same URL
  outside the sandbox returns 200, `-c 'sandbox_mode="danger-full-access"'` returns 200, and
  `-c 'sandbox_workspace_write.network_access=true'` does **not** lift it. `~/.codex/config.toml` here
  carries the default `sandbox_mode = "workspace-write"`, and `internal/config/codexprofile.go:46`
  mirrors that `config.toml` verbatim into AgentDeck's private `CODEX_HOME` without writing any
  sandbox key, so launched `codex-acp` agents inherit it. The shipped MCP path is not affected because
  the token rides in the `mcpServers` session param (`internal/server/messaging_registration.go:36`,
  `internal/runtime/chat.go:1450`) and the unsandboxed CLI process makes the HTTP call; the migration
  moves that call into a sandboxed shell child. Because TS-04.R39 and FS-17.R19 forbid a per-provider
  fallback and forbid shipping if any adapter cannot run the client, this is the difference between
  the migration shipping and being abandoned after the registry, routes, CLI, and overlay are built.
  Prove the loopback-shell-exec path for all four adapters before phase 3 and record the evidence in
  the change file; if it does not hold, own the mitigation in the design — a transport the sandbox
  permits, or an AgentDeck-owned sandbox/escalation policy in the managed `CODEX_HOME` — rather than
  leaving it to R39's release gate.
- **Must fix** (TS-05.R18/R19, FS-17.R16/A8; INV §4/§14) — the launch secret the design promotes to
  the action credential is already published as `agent_generation`. On a fresh launch,
  `internal/server/launch.go:305` sets `generation = token`, the minted launch token itself
  (`internal/server/switch.go:409` does the same). That generation is persisted to
  `pipeline_attempts.agent_generation` (`internal/state/schema.go:243`, `internal/state/pipelines.go:515`),
  marshalled as `agent_generation` (`internal/state/types.go:200`), and parsed by the dashboard client
  (`ui/src/schemas/pipeline.ts:106`), so it is served over the unauthenticated same-user API. Today
  that leak only enables forged hook events. Normal-use trigger after the change: any AgentDeck agent
  or local process reads a pipeline run's attempts, recovers another agent's action token, and sends
  mail, creates or cancels tasks, reads context links, and reports stage results as that agent —
  precisely what FS-17.R16 promises cannot happen. A8 as written checks captured process parameters,
  frozen session rows, generated provider configuration, logs, and transcripts; it never names the
  pipeline attempt row or an API payload, so the acceptance passes while the credential is public.
  Require the generation identifier to be derived independently of the credential — the pipeline and
  task paths already pass an explicit `Generation` — and extend A8 to the attempt row and every
  API/SSE payload carrying `agent_generation`.
- **Worth fixing** (FS-06.R1/R2/A1/A3, FS-15, FS-16, FS-17.R13/R19; INV §10) — the specifications that
  own the actions still mandate the mechanism the change removes, and nothing supersedes them.
  FS-06.R1 requires every launched or resumed chat agent to receive the reserved `agentdeck-messaging`
  MCP server, FS-06.R2 states the MCP server exposes exactly three coordination tools, FS-06.A1/A3
  verify HTTP MCP registration through a named registration test, FS-15 derives caller identity "from
  the live MCP session", and FS-16 calls the task surface "scoped MCP tools". TS-04 §5 carries an
  explicit planned-supersession note; FS-06, FS-15, and FS-16 carry none, and the change file's
  relevant-requirements list omits all three. Normal-use trigger: the agent implementing phase 5 must
  delete FS-06.A3's shipped, passing registration coverage and contradict FS-06.R1/R2 with no
  requirement authorising it, so the change either stalls on a shipped-requirement conflict or
  silently drops verified acceptance. Add the supersession note to each owning spec and cite FS-06,
  FS-15, and FS-16 in the change file.
- **Worth fixing** (TS-04.R33/R34, TS-06.R23; INV §2) — `describe` routes compiled-in data through an
  authenticated HTTP round trip. TS-06.R23 makes the action client the exact running AgentDeck binary
  and TS-04.R32 puts the registry — name, description, resolved schema — in that same binary, so
  `GET /api/agent-actions/{action}` and R34's projection of it add a route, an authentication path,
  and a live-server dependency for data the client already holds. Normal-use trigger: an agent told by
  the runtime overlay to run `agentdeck action describe check_messages` receives a transport or
  authentication refusal instead of the contract whenever the dashboard is restarting or its
  generation has just ended, and the contract it was pointed at is the one thing it cannot read.
  Resolve `describe` from the in-process registry and drop the GET route, or record in the design why
  the round trip is required.

Design findings from the 2026-08-31 review of `migrate-internal-actions-from-mcp.md` are the four
items above. The user resolved the `agentdeck-shared-skill` design review: verified installation now
precedes exact AgentDecker migration and the thin prompt no longer claims an unavailable skill.
Runtime-only overlay fields and fresh PM/teammate prompt cleanup remain included implementation
alignment, not review findings. Browser-only evidence is recorded as acceptance gates above, not as
findings.

## Design consistency notes

- The change file cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 both cite `TS-04.R32–R39` and
  omit R40, the direct-action redaction clause. One range is wrong; the three should agree.
- FS-17 §6 opens with "The contract is shipped. Live-provider compatibility remains tracked as
  acceptance gate A6," which reads as covering the whole section, but §6 now also carries the planned
  direct-cutover boundary for R13–R19. Scope the opening sentence to R1–R12.
