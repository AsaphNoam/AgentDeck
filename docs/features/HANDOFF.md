# AgentDeck — Implementation handoff

**Live agent state.** Read this first, then open the relevant requirements named below. Historical
phase state is archived in [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md).
Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) and keep this file limited to resumable current state.

## Current position

- **Bug investigation:** The 2026-08-28 pipeline `stale_assignment` report is diagnosed and open for
  `/fix`: answering a blocked stage agent in its own chat, instead of through **Continue**, gets its
  next report refused as `stale_assignment` even though it is the run's current attempt. Reproduced
  by the committed skipped test in `internal/pipeline/blocked_chat_answer_test.go`. Four findings
  below; the second needs a product decision. The earlier 2026-08-27 all-200/no-page-load incident
  is fixed. Same-origin dashboard tabs now share one long-lived SSE connection, leaving the
  browser's HTTP/1.x pool available for REST queries; the related config-source,
  transcript-reconciliation, and card-preview amplification paths are bounded as described in the
  changelog.
- **Review state:** The 2026-08-29 code review read `6a16126..43e5feb` — the whole unreviewed
  range, including the three commits that had never had their shipped diff read (Mermaid rendering,
  the Pipelines split and its fixes) and the expandable panes that landed during the session. It
  recorded fifteen findings below, three of them **Must fix**: unsanitized `style=""` attributes in
  diagram markup, a failed template fetch reported as a deletion, and the panes' unbounded
  second transcript copy. The 2026-08-29 design review of expandable dashboard chat panes is closed:
  all five findings and all three consistency notes are resolved in the specifications. The four
  bug-investigation findings are open, joined by the residue of the 2026-08-28 review of `790c01c`
  — its Must fix and the Tasks Retry gate are closed, and two of its open items were re-confirmed
  against the current tree by this review.
- **Active change:** None. Expandable dashboard chat panes are finished and verified
  (FS-02.R46–R52/A29–A34, FS-03.R39/A23, FS-12.R38/A14, TS-03.R31, TS-08.R41–R43).
  Running-first card placement shipped on 2026-08-28 (FS-02.R45/A28, FS-12.R37/A13). The Pipelines
  surface
  split is finished and committed (`9114df7`, with its usability fixes in `69c2f99`) and is ready
  for an independent review; its change file is removed, and FS-14 is the authority on what
  shipped.
- **State:** Automated MCP contract verification is green. Pinned Claude/Codex live-provider checks
  remain owed before claiming those adapters accept structured results.
- **Usability state:** The split Pipelines surface was driven through a real Chromium on
  2026-08-28 against the working tree's own release build. Every FS-14.A14–A23 item was exercised;
  A14, A16–A20, A22 and A23 passed outright, and the A15 and A21 gaps are now fixed in code and
  tests. The badge, rail-order and append-motion fixes are unverified in a real browser: no browser
  run has been made since them. Expandable dashboard panes passed their J5 real-browser charter at
  1280×800 under both Core and Sky & Grove, including fixed geometry, internal transcript scrolling,
  stable page position during streaming, keyboard cycling, persistence, and four-pane composition.
  J5 is still owed for running-first card placement: the live
  start/stop boundary crossing, the in-drag geometry inside one block, and the refused cross-block
  drop have unit coverage but no real-browser run — this session had no browser available.
  The earlier v0.2.2 → v0.2.3 delta remains closed: FS-02.A24 is closed and FS-04.A22 remains
  narrowed to the native panel.
- **Last reviewed code:** `43e5feb` (2026-08-29). Advanced across the continuous range
  `6a16126..43e5feb`, which was read end to end in one pass; the backlog of three never-reviewed
  commits named in earlier handoffs is now cleared.
- **Branch:** `main`.

## Active change

**Change:** None.

**State:** Expandable dashboard chat panes are finished and verified, and the whole
unreviewed range now has a code review behind it.

**Next:** Run `/fix` on the three **Must fix** review findings, starting with the diagram
`style=""` sanitizing gap, then the remaining Worth-fixing items and the four bug-investigation
findings (un-skip `internal/pipeline/blocked_chat_answer_test.go` first). The second
bug-investigation finding still needs the user's product decision before it can close.

## Changelog

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

From the 2026-08-29 review of `6a16126..43e5feb` — the first read of the shipped diff for Mermaid
rendering (`c35ff8c`), the Pipelines split (`9114df7` + `69c2f99`), the four small fixes, and the
expandable dashboard chat panes (`43e5feb`). Both Go test variants, the 289-case UI suite,
`make check-specs`, `git diff --check` and all nine skill twins are green at `43e5feb`; every new
`rows.Next()` loop in the range checks `rows.Err()` (INV §7 clean).

- **Must fix** (confirmed) Diagram sanitizing leaves `style=""` attribute values untouched, so a
  diagram can still make a network request. `sanitizeDiagram`
  (`ui/src/components/chat/renderers/mermaid.ts:40-46`) forbids `href`/`xlink:href`/`src`/`srcset`
  but not `style`, and DOMPurify carries `style` in its `DEFAULT_URI_SAFE_ATTRIBUTES`
  (`ui/node_modules/dompurify/dist/purify.cjs.js:740`), which short-circuits its URI check for that
  attribute (`:2016`). The custom `stripRemoteStyleReferences` hook (`mermaid.ts:27-30`) only
  rewrites `<style>` *element* text, never `style=""` attribute values, so an author `style`/
  `classDef` directive carrying `url(https://…)` — which Mermaid writes into the node's `style`
  attribute — survives both layers. FS-03.R38 and TS-08.R40 state without qualification that
  rendering makes no network request and that URL attributes are removed before insertion; TS-08.R40
  specifically names runtime-generated markup as the reason the sanitizing seam must be complete.
  The `<style>`-element hook shows the authors knew about the CSS `url()` class and did not extend
  it to attributes. Verified at the sanitizer-configuration level by reading both files; not
  reproduced end-to-end in a browser. Fix: add an `afterSanitizeAttributes` hook that strips
  `url(...)`/`@import` from any `style` attribute value, or pass `ADD_URI_SAFE_ATTR` so DOMPurify's
  own URI check runs on `style`. Test: extend the sanitizer case in `AssistantText.test.tsx` with an
  SVG carrying `style="fill:url(https://example.invalid/x)"` and assert both the scrub and a
  `fetch`/network spy (FS-03.R38, TS-08.R40, **INV §8**).

- **Must fix** (confirmed) A failed template fetch tells the operator the template was deleted.
  `PipelineTemplatePage` (`ui/src/features/pipelines/PipelinesPage.tsx:131-143`) derives `seed` from
  `templates.data`, then renders "This template is gone. It may have been deleted in another tab."
  whenever `seed` is null and loading has ended. A transient 500 or network blip on `/api/pipelines`
  leaves `data` undefined and hits that same branch, so a reload of a bookmarked
  `/pipelines/templates/{id}` — exactly the deep link FS-14.R43 exists for — reports a deletion that
  did not happen, offers no retry, and never surfaces `templates.error`. Its twin got this right in
  the same commit series: `RunDetail` (`ui/src/features/pipelines/RunBrowser.tsx:89-101`) checks
  `detail.error instanceof PipelineAPIError && detail.error.status === 404` and shows the transport
  message for anything else. Two paths deriving "this resource is unavailable", already drifted.
  FS-14.A19's template half is therefore both wrong and untested — no case in
  `PipelinesPage.test.tsx` drives a `/api/pipelines` error. Fix: mirror `RunDetail`'s 404 check;
  test by mocking `/api/pipelines` to 500 and asserting the copy differs from "This template is
  gone." (FS-14.R43/A19, **INV §2/§8**).

- **Must fix** (confirmed) The pane transcript store keeps a second, unfolded copy of every event
  forever, and appends to it quadratically. `43e5feb` added `rawByAgent`
  (`ui/src/store/transcriptStore.ts:6,102`) to answer "which events are newer than this refetch".
  Unlike `byAgent`, it is never folded — `appendRenderedEvent` coalesces consecutive assistant
  deltas into one bubble, while `rawEvents.push(event)` (`:141`) retains every single delta — and
  every append copies the whole growing array first (`:110`). A 2,000-delta response therefore costs
  ~2,000,000 element copies and 2,000 retained objects per open agent. Nothing ever clears it: there
  is no delete path in the store, `registerOpenAgent`'s teardown (`ui/src/api/sse.ts:79-89`) only
  decrements the open count, and the removal branch (`sse.ts:96-105`) clears the agent, annotation
  and draft stores but not the transcript store. This lands on the exact workload the feature
  creates — up to four panes plus the agent screen streaming at once on a dashboard left open for
  days — and on the surface that already produced a freeze incident (`0a817f9`). INV §1 names this
  shape for `annotationStore`: browser-local state needs boundary handling plus its own retention
  bound. Fix: bound `rawByAgent` to the tail actually needed for seq reconciliation, and drop the
  agent's entry on removal and when the last consumer unregisters (FS-03.R39, TS-03.R31,
  **INV §1/§4**).

- **Worth fixing** (confirmed) `Ctrl+Alt+Arrow` pane cycling cannot leave a group section.
  `cyclePaneFocus` is bound to the per-group grid — `grouped.map(...)` renders one
  `<div className="card-grid" onKeyDown={cyclePaneFocus}>` per section
  (`ui/src/components/grid/CardGrid.tsx:202`) — and it collects panes with
  `grid.querySelectorAll("[data-agent-pane]")` on that one element, returning early when
  `panes.length < 2`. FS-02.R50 says the bindings move to "the next expanded pane" with order
  following "the panes as displayed" and do nothing only with "fewer than two panes expanded", and
  FS-02.R48's limit of four is a whole-grid limit. With one pane expanded in each of two task
  groups (FS-02.R18), both bindings silently do nothing. The keyboard tests
  (`CardGrid.test.tsx:169-182`) only ever render one section, so nothing catches it. Fix: bind the
  handler once at the grid container and collect panes across sections; test with two groups
  (FS-02.R50, **INV §10**).

- **Worth fixing** (confirmed) An expanded card is dropped from `SortableContext` while still
  mounting a sortable node, so in-drag geometry is computed over a list that omits it. `sortableIDs`
  now filters expanded ids out (`ui/src/components/grid/CardGrid.tsx:107-112`), but `AgentCard`
  still calls `useSortable({ id, disabled: expanded })` unconditionally
  (`ui/src/components/grid/AgentCard.tsx:16`) and the expanded card still occupies
  `min(2, perRow)` grid columns. `rectSortingStrategy` derives every other card's preview transform
  from the `items` array, so dragging any collapsed card near a pane shifts neighbours as though the
  two-column pane were not there. The committed order is unaffected — `onDragEnd` uses the full
  `ids` list and `disabled` keeps the pane from being `over` — so this is in-drag feedback only.
  FS-02.R47 specifies that an expanded card is not draggable but says nothing about dragging past
  one, so this is a specification gap as well. No test covers a drag with a pane expanded. Fix:
  keep expanded ids in `items` (they are already `disabled`), or account for the pane's rect
  (FS-02.R47, **INV §1**).

- **Worth fixing** (confirmed) The live drag preview crosses the running/stopped boundary that
  FS-02.A28 promises it will not. One `SortableContext` spans both blocks
  (`ui/src/components/grid/CardGrid.tsx:177-178`) with no `collisionDetection` or modifier limiting
  candidates to the active card's block; `onDragEnd:149` refuses the cross-block drop, but only at
  drop time, after `rectSortingStrategy` has already computed preview transforms from
  `arrayMove` over the whole shared list. FS-02.R45 and A28 state that "during that drag, only cards
  in the same running/stopped block shift to show the possible drop; cards in the other block hold
  their positions". A28 cites `CardGrid.test.tsx` as proof, but jsdom with a mocked dnd-kit cannot
  exercise transforms — the file's own header comment says so — so the clause is unverified as well
  as probably unmet. Not reproduced in a real browser. Fix: give each block its own
  `SortableContext`, or filter `over` candidates by block; verify under J5 (FS-02.R45/A28,
  **INV §10**).

- **Worth fixing** (confirmed) A diagram that parses but fails while drawing leaks a DOM node onto
  `document.body`. `renderDiagram` calls `mermaid.render(id, source)` without a containing element
  (`ui/src/components/chat/renderers/mermaid.ts:73`), so Mermaid appends its own scratch node to
  `<body>`. On a draw-stage exception Mermaid removes that node only when `suppressErrorRendering`
  is set; `mermaid.initialize` here never sets it (`:64-72`), so the library draws its own
  "Syntax error" SVG into the scratch node and rethrows without removing it. The visible transcript
  is correct — the `catch` returns `DIAGRAM_UNRENDERABLE` and the code block stands, as FS-03.R38
  requires — but a node accumulates outside React's tree once per such failure for the life of the
  page. Fix: pass `suppressErrorRendering: true` so every failure path tears the scratch node down
  (FS-03.R38, **INV §4**).

- **Worth fixing** (confirmed) No test cites any acceptance item of the expandable-pane change.
  Nothing in the repository references `FS-02.A29`–`A34`, `FS-03.A23` or `FS-12.A14`, though the
  behavior is covered by uncited cases in `CardGrid.test.tsx`, `AgentCard.test.tsx` and
  `DashboardChatPane.test.tsx`, and the neighbouring `FS-02.A28` case is cited properly
  (`CardGrid.test.tsx:12`). The specification lifecycle asks tests that pin an acceptance criterion
  to name it, and `check-specs` does not enforce the direction. This is the same shape as the open
  `FS-02.A27` finding. Fix: add the citations, or narrow the A-items to what is actually proven
  (**INV §10**).

- **Worth fixing** (confirmed) Delegated-agent and proposal-count UI shipped with no frontend test.
  No UI case renders `AttemptAgents`/`AttemptAgentCard` with a populated `delegated_agents` — every
  `RunBrowser.test.tsx` fixture sets it to `[]` — and nothing exercises `proposalKind` in
  `AgentDeckerBuilder.tsx` or the cross-surface counts in `PipelinesPage.tsx:22-23,34-39`. Only the
  Go projections are covered (`TestPipelineRunDetailProjectsOneHopAgentsAndFallbacks`). FS-14.A17,
  A18 and A23 each name a UI test as their verification, and A16's looping-timeline and "unreported
  attempt" cases are likewise undriven. The implementations read correctly on inspection, so this is
  a coverage finding, not a defect report: a regression in delegated-card rendering, live/archive
  routing, or proposal filtering would ship silently today (FS-14.A16/A17/A18/A23, **INV §10**).

- **Worth fixing** (confirmed) `RunDetail` keeps per-run state across a `runID` change.
  `PipelineRunPage` renders `<RunDetail runID={runID} …>` with no `key`
  (`ui/src/features/pipelines/PipelinesPage.tsx:88-92`), while `RunDetail` holds `continuation`
  text, a mutation `error`, and `useAppendedAttempts`' `seen` ref
  (`ui/src/features/pipelines/RunBrowser.tsx:57-86`), none scoped to the run. When the route element
  is reused for a different run rather than remounted — browser back/forward across two visited run
  pages is the plausible route today, since in-app navigation goes through the ledger — unsent
  "New input for the blocked stage" text or a stale error from the previous run stays on screen, and
  every timeline entry of the new run plays the just-appended entrance. Not reproduced. Fix:
  `key={runID}`, INV §1's ordinary remedy for a lifecycle boundary (**INV §1**).

- **Worth fixing** (confirmed) The Retry gate is hand-duplicated across the language boundary.
  `ui/src/features/tasks/TasksPage.tsx:114-116` reimplements the eligibility switch in
  `RetryTask` (`internal/state/tasks.go:1557-1569`), and `8c9fa68`'s own message says so. That
  commit exists because the two had already drifted — the UI had narrowed Retry to `interrupted`
  and silently dropped work parked by exhausted start attempts. The copies are byte-equivalent
  today and nothing prevents the next FS-16.R23/R25 change from separating them again. Fix: return
  the eligibility (or a `park_reason`) on the task JSON and let the view read it
  (FS-16.R23/R25, **INV §2**).

- **Worth fixing** (confirmed) `ui/tsconfig.app.tsbuildinfo` is a tracked TypeScript incremental
  build cache with no ignore rule; seven commits in this range carry a churn-only diff to it, and
  running the UI suite dirties the tree. Same class as the tracked embed artifact already recorded
  below. Fix: `git rm --cached` it and add it to `.gitignore` (**INV §10**).

- **Worth fixing** (confirmed) `go.mod` marks `github.com/coder/websocket` and
  `github.com/creack/pty` `// indirect` although both are imported directly, so any `go test ./...`
  rewrites the file and dirties the tree mid-session. Fix: commit the promotion `go` already makes
  (no invariant class).

- **Worth fixing** (confirmed) `ListPage`'s snapshot-decode diagnostic reports the wrong failure.
  `internal/pipeline/manager.go:283-284` emits `run_read_failed` / "run detail could not be
  decoded" when only the frozen `TemplateSnapshot` JSON failed to unmarshal — the function's own
  doc comment and TS-03.R30 state it does not read full run detail. The following
  `frozen_stage_title_unavailable` diagnostic already describes the real condition. Fix: reword or
  drop the first diagnostic (TS-03.R30, **INV §2/§8**).

- **Worth fixing** (confirmed) TS-06 does not name the stress fixture it now owns. `d3b4400` added
  `scripts/stress-fixture/` and the `stress_stream` scenario with `FAKEACP_STRESS_CHUNKS`,
  `FAKEACP_STRESS_CHUNK_BYTES` and `FAKEACP_STRESS_DELAY_MS`
  (`internal/runtime/testdata/fakeacp/main.go:181-192`), but TS-06 §6 names neither, and no `make`
  target reaches it. TS-06's header claims `scripts/`. Fix: one line in TS-06 §6 naming the fixture
  and the connection-pool exhaustion it reproduces (TS-06, **INV §10**).

Two already-recorded findings below were re-confirmed against the current tree rather than
re-filed: the shared worker still never deletes a port (`ui/src/api/sse-shared-worker.ts:8,42`),
and `742e50e`'s new demotion path is a second trigger for it; and `ui/src/features/tasks/TasksPage.tsx:147`
still renders the stray leading `·` for an agent-target task. One correction: that finding's claim
that `790c01c` force-tracked `internal/server/ui/dist/index.html` against `.gitignore` is wrong —
`.gitignore:11` has carried an explicit `!internal/server/ui/dist/index.html` negation since the
first commit. Only `assets/sse-shared-worker-DxpB4Ebi.js` is genuinely tracked outside the ignore
rule; that half stands, and its content does still match the current source.

From the 2026-08-28 bug investigation of the field report: *"An AgentDecker session blocked the
progression of a pipeline because it needed an answer from me, then when it tried to continue it got
`stale_assignment` from AgentDeck."* No AgentDeck version, environment, or log was supplied, and this
machine's `~/.agentdeck` holds no run newer than 2026-08-01, so it is not the incident home — the
diagnosis below is from the code path and a local reproduction, not from incident logs.

- **Must fix** (confirmed) `internal/pipeline/actions.go:198` refuses with
  `stale_assignment` / "caller is not the current stage attempt" for three different conditions,
  including one where the caller *is* the current stage attempt under the current generation. A
  `blocked` report pauses the run with its stage agent still live and idle (FS-14.R11) and the run
  detail offers **Open agent** beside **Continue** (`ui/src/features/pipelines/RunBrowser.tsx:132`),
  so the ordinary human move — answering the question in the chat that is showing it — leads
  straight here: the agent finishes the stage, calls `report_pipeline_stage_result`, and is told it
  is not the current attempt. Refusing is right (FS-14.R19 forbids a second accepted result), but
  the code and message are wrong: the real reason is that this attempt's report was already
  accepted and the run is paused waiting for a human Continue, and FS-17 classifies
  `stale_assignment` as retry `never`, so the agent abandons the stage on a false reason and a full
  turn of work is silently discarded. `already_reported` is the shared vocabulary's name for this
  condition and the task path already returns it
  (`internal/messaging/task_tools.go:114`), so the second meaning here is also INV §2 drift.
  Reproduced by the committed skipped test
  `internal/pipeline/blocked_chat_answer_test.go` (`TestBlockedStageAgentAnsweredInChatIsNotCalledStale`);
  un-skip it to start the fix. Fix: split the predicate — keep `stale_assignment` for a genuine
  agent/generation mismatch, return `already_reported` naming the human Continue for the run's own
  current attempt (INV §8).

- **Must fix** (confirmed) Nothing tells a stage agent that a `blocked` report ends its
  participation until a new assignment arrives. `renderAssignment`
  (`internal/pipeline/assignment.go:35`) says only "call `report_pipeline_stage_result` exactly
  once", and the accepted report returns `"awaiting":"quiescence"`
  (`internal/messaging/pipeline_tools.go:35`) — neither states that chat input received during the
  pause is out of band, that the person's answer arrives as a new assignment, or that work done
  meanwhile cannot be recorded. FS-14 and TS-09 have no requirement covering this, so it is a
  specification gap as well as the trigger for the finding above. The same dead end is reachable a
  second way and there the work is unrecoverable by design: after a restart `RecoverRuns`
  (`internal/pipeline/manager.go:320`) pauses an `await_result` run as `restart_recovery` and stops
  the agent, `Continue` rejects that state (`internal/pipeline/actions.go:35`), and an ordinary
  chat resume of that agent mints an unrelated generation
  (`internal/server/resume.go:419`), so its report is refused forever and only **Retry** — a fresh
  agent, from scratch — moves the run. FS-14.R20 puts that boundary in place deliberately, so
  closing this needs a product decision from the user, not an agent's choice: either state the
  boundary and say it in the assignment text and the refusal, or give the blocked pause a real
  in-chat route. The **Open agent** action shipped on a pause where following it is a dead end
  (INV §10).

- **Worth fixing** (confirmed) No refusal from `report_pipeline_stage_result` is logged anywhere:
  `internal/messaging/pipeline_tools.go` and `internal/pipeline/actions.go` have no logger on the
  report path, and `internal/pipeline` logs only from `proposals.go`. `~/.agentdeck/dashboard.log`
  records every HTTP request but not one control-plane refusal, so this report could not be
  corroborated from the server log and the next one will be equally undiagnosable — the only trace
  is inside the agent's own transcript. Log at Warn on refusal: run id, attempt id, caller agent
  id/generation, the attempt's agent id/generation, the run's pending action, and the code. That
  set is exactly what separates the three conditions the first finding conflates
  (no invariant class).

- **Worth fixing** (probable) `OnTurnEnd` and `OnExit` (`internal/pipeline/actions.go:281` and
  `:305`) read the run *outside* `runLock`, then compare-and-swap on that captured revision *inside*
  the lock and return `nil` when it conflicts. `Report` re-reads the run under the lock; these two
  do not (INV §5). A revision bump in that window leaves a run parked at `await_quiescence` — state
  `running`, no attention reason, no notification, no log — with no further turn boundary coming, so
  the run silently stops progressing and any later report is refused with the same
  `stale_assignment`. Not reproduced: it needs a concurrent bump in a narrow window, and no field
  log exists to confirm it happened here. Fix: re-read the run under the lock as `Report` does, and
  log the conflict.

From the 2026-08-28 review of `790c01c` (the thirteen dashboard/SSE and usability fixes):

- **Worth fixing** — FS-02.A27 names verification that does not exist, and the whole SSE suite runs
  against the transport production no longer uses. A27 claims "shared-stream transport tests and the
  production-browser six-tab regression", but no file in the repository cites `FS-02.A27`, no test
  mentions `SharedWorker` or `sse-shared-worker`, and no six-tab browser run has been recorded since
  the fix landed. Narrowed on 2026-08-28 by the transport-fallback fix: `ui/src/api/sse.test.ts` now
  stubs `SharedWorker` and covers port fan-out, the `open`/`error`/event message kinds, and all
  three fallback routes, so the shipped path is no longer untested. What remains is A27's own
  claim — no test cites `FS-02.A27`, and the "production-browser six-tab regression" it names has
  never been run against a build carrying the shared stream. Fix: run the six-tab check on a `make
  dist` build and cite it, or narrow A27's wording to what the suite proves
  (FS-02.A27, **INV §10**).

- **Worth fixing** — every tab that attaches restarts the one shared stream for every other tab, so
  the cost of opening the dashboard grows with the square of the tab count. `self.onconnect` calls
  `connect()` unconditionally, which closes and reopens the shared `EventSource`
  (`ui/src/api/sse-shared-worker.ts:26-48`), and TS-03.R7 now specifies this. Each reopen broadcasts
  `open` to *all* ports, and every tab's `onopen` starts a fresh hydration generation, refetches the
  open transcript, and invalidates the pipeline-runs, proposals, and tasks queries
  (`ui/src/api/sse.ts:30-51`). Opening six tabs therefore costs fifteen extra full hydrations and
  about sixty extra REST refetches, and any single tab reloading re-hydrates all the others — on the
  exact six-tab workload this change exists to make fast. The reopen window also drops `new_message`
  deltas for every tab, not only the newcomer (recoverable: the open chat refetches, card previews
  self-heal on the next delta). Fix: have the worker retain the latest `state_update` rows plus the
  `hydrated` boundary and replay them to the joining port alone, leaving the live stream and the
  other tabs untouched (TS-03.R7, FS-02.A27, **INV §1/§8**).

- **Worth fixing** — the shared worker never removes a port. `ports.add(port)` has no matching
  delete on any path (`ui/src/api/sse-shared-worker.ts:8,40-48`), and the client mints a *new* port
  on every reconnect: the watchdog calls `this.es.close()`, which closes only the `MessagePort`
  (`ui/src/api/sse.ts:216-218,168-171`), then `connect()` constructs another
  `SharedWorkerEventSource`. `broadcast` keeps iterating the dead ones and the worker holds a strong
  reference to each for its whole lifetime, which is as long as any dashboard tab stays open.
  `postMessage` to a closed port is a silent no-op, so the only symptom is memory that grows with
  every closed tab and every reconnect on a dashboard left open for days. This is INV §4's
  create/teardown symmetry: the port is the artifact created at attach and nothing tears it down.
  Fix: send a close message (or listen for `port.onmessage` with a `bye` kind) before
  `this.port.close()` and delete the port in the worker; drop the shared `EventSource` when the set
  empties (**INV §4/§1**).

- **Worth fixing** — the same card preview is now derived two ways that disagree about which end of
  a long message to keep. The client keeps the **last** 120 UTF-16 code units of the streamed text
  (`ui/src/store/transcriptStore.ts:148,163` — `.slice(-120)`), while the server keeps the **first**
  120 *runes* (`clipPreview`, `internal/server/reconcile.go:196-202`, whose own comment claims it
  mirrors a tail clip). `AgentCard` renders `agent.detail || lastLine`
  (`ui/src/components/grid/AgentCard.tsx:19`), so the same agent's card shows the end of its last
  message while streaming and the beginning of it after a reload or a reconcile sweep. `.slice(-120)`
  also cuts UTF-16 code units, so a preview whose 120-unit boundary lands inside an emoji renders a
  replacement character — precisely what the server-side helper documents itself as avoiding. This is
  INV §2's "two paths building the same artifact will drift", appearing in the same commit that
  created the second path. Fix: pick one end, share one helper, and clip by code point on the client
  (`Array.from(text).slice(...)`) (FS-02.R9, **INV §2/§8**).

- **Worth fixing** — the reconciliation regression does not test what its name claims, and the load
  bound the finding asked for was never added.
  `TestReconcileSessionPathTouchesOnlyChangedTranscript`
  (`internal/server/reconcile_test.go:73-86`) writes a second, unrelated transcript and then asserts
  only that `reconcileSessionPath` returns `true` for the changed one — it never asserts that the
  other file was not read, which is the entire claim in its name and the entire point of the fix. The
  closed finding asked for "a regression that streams many deltas with several retained transcripts
  while bounding scans and latency"; nothing bounds either. A future change that restores the
  whole-tree walk on every write would leave this test green. Fix: count transcript reads (or assert
  on the untouched agent's status row being unmodified) across a many-delta stream with several
  retained sessions (FS-14.R16, **INV §7/§10**).

- **Worth fixing** — two generated files under the ignored embed directory are force-tracked, and
  one of them names an asset the repository does not contain. `.gitignore:10` ignores
  `internal/server/ui/dist/*`, but `790c01c` added `internal/server/ui/dist/index.html` and
  `assets/sse-shared-worker-DxpB4Ebi.js` to the index, so gitignore no longer applies to them while
  every other emitted chunk stays ignored. The tracked `index.html` therefore points at
  `/assets/index-*.js`, which is untracked, and it goes stale on any UI change — every `make embed`
  now produces a spurious diff a committer has to decide about, and a fresh clone's embedded
  `index.html` references a file that is not there. CLAUDE.md states this whole tree is generated by
  `make embed`. Fix: `git rm --cached` both paths so the ignore rule governs the directory uniformly
  (**INV §10**).

- **Worth fixing** — a task targeting an existing agent renders its metadata line starting with a
  stray separator. `ui/src/features/tasks/TasksPage.tsx:138` emits
  `{task.target_kind === "launch" && ...}{assignedID && <> · assigned to …</>}`, so when the target
  kind is `agent` the first expression renders nothing and the line reads "· assigned to Bob ·
  created by person". Every agent-target task row in the Tasks view shows it. Fix: build the segment
  list and join it, rather than prefixing each optional segment with its own separator
  (FS-16.A8, **INV §8**).

## Design consistency notes

None.
