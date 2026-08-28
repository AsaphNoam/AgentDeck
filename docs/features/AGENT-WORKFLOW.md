# AgentDeck work workflow

This is the shared way Claude Code and Codex make changes, review work, and leave the repository
easy to resume. The feature and technical specifications say what the product must do; this document
says how agents work with them.

## 1. Start with the current state

1. Read [`HANDOFF.md`](HANDOFF.md) from top to bottom, then inspect `git status` and the diff. Treat
   a dirty tree as user or interrupted work; do not discard it.
2. If a change is in progress, read its change file and the relevant feature, technical, and invariant
   requirements before reading code. A `/work` request with no active change may start the sole
   waiting ready change, or a ready change explicitly named in that request. If several ready changes
   are waiting, ask the user to choose; do not prioritize one yourself. Outside that explicit request,
   do not choose future work yourself.

Use plain status words: **waiting to start**, **in progress**, **paused**, and **finished**.
Requirement IDs such as `FS-05.A2` and `TS-03.R4` are kept because they are stable links to a
precise requirement, not process vocabulary.

## 2. Make a change

Work in small, complete pieces. For each piece:

1. If it changes what a user sees or changes an architectural rule, update the relevant specification
   first. Add or change its R/A items and mark unshipped behavior `(planned)`. A bug fix that restores
   already-specified behavior does not need a specification change.
2. Implement the work and add or keep the test that demonstrates the requirement.
3. Verify it. Product-code changes run:

   ```bash
   make test
   make build
   cd ui && npm test && npm run build  # when ui/ changed
   make dist  # when producing a distributable or refreshing embedded UI output
   ```

   Documentation-only work runs `make check-specs`, appropriate syntax or rendering checks, and `git diff --check`. Run an additional command when the documentation changes that command or makes a claim that needs executable evidence.
4. Before committing, check that the specifications describe what shipped, the active work state is
   accurate, and the diff has no unfinished or accidental changes.
5. Commit the completed work, its specification update when needed, and the handoff update together
   on `main`. Continue with the next piece until the request is complete or there is a real reason to stop.

Implement the smallest change that satisfies the requirement. Extend an existing seam, pattern, or
interface before inventing a parallel one (INV §2 and its canonical-helpers registry), and do not
add abstraction, configurability, or edge-case handling that no requirement or real report demands.
When a requirement itself seems to force disproportionate machinery, question it under §3 rather
than building around it.

Do not claim work is complete while required checks fail. Do not make tests pass by weakening useful coverage or by changing a requirement without recording that change in the relevant specification.

## 3. When to ask the user

Stop and ask only when the next action requires a product, security, privacy, compatibility, or data-retention decision; credentials or other external input; a destructive/irreversible action; or resolving a real conflict between requirements and shipped behavior. Also ask when honest attempts cannot make required checks pass.

Record the question under `## Blocked on human` in `HANDOFF.md`, with enough context for someone starting cold. Leave the repository in the last verified state when possible.

For a reversible local implementation choice, record a short note for the next independent review rather than asking the user. The reviewer either removes it after confirming it is sound or turns it into a user question when it has broader consequences.

## 4. Keep the handoff useful

`HANDOFF.md` is current working state, not history. It contains one change in progress, its next small step, unresolved user questions, open review findings, the last reviewed code commit, and a short changelog. Remove finished steps and resolved findings. Keep completed details in specifications, tests, commits, and Git history.

When delegation is available, use it for bounded independent work such as a repository search, a focused audit, or an isolated test. The main agent remains responsible for interpreting requirements, combining the work, and doing final verification.

## 5. Commit and resume safely

Commit each completed, verified piece directly to `main`; this repository does not use per-change branches or pull requests. The message should say what changed and, where useful, cite the requirement IDs. Push only when the user or environment authorizes it.

At the end of a session, either leave a verified commit or clearly describe unfinished work in the handoff. Never pretend interrupted work is complete.

## 6. Human update

Every feature-design, implementation, review, fix, and usability-review session adds one short update to [`BRIEFS.md`](BRIEFS.md) and sends that exact text as the final response.

Write for the person who asked for the work, not for the next agent. Use plain language. Explain a project abbreviation the first time it matters, or leave it out. Do not use internal process labels, requirement-ID strings, commit hashes, command inventories, or changed-file lists unless the person needs one to act. The handoff holds the internal detail.

Use this shape:

```md
### YYYY-MM-DD — <kind of work>: <plain-language scope>

<What changed or was learned, why it matters, and how it affects the product or next decision.>

**Needs attention:** <a real decision, blocker, or material risk; otherwise “None.”>

**Next:** <one concrete next action and who should take it.>
```

## 7. Review work

Review another agent's unreviewed code, not your own. Unless the user names a range, start after the last reviewed code commit and continue through `main`. Read the relevant requirements before the diff.

Check both directions:

- Does the code do what the relevant requirements say?
- Did the change introduce user-visible behavior or an architectural rule that the specifications do not describe?

Also look for normal-use bugs: missing error handling at boundaries, realistic races, unsafe writes, dead code, and incomplete wiring. Unrequired complexity is likewise a finding: a new parallel mechanism or abstraction where an existing seam could extend, or code serving a case no requirement names. Ignore style preferences, demands for speculative edge-case handling, and micro-optimizations.

Record each real finding in `## Review findings` in `HANDOFF.md` with its location, normal-use trigger, why it matters, relevant requirement ID when one exists, and a suggested test or fix. Start the bullet with either **Must fix** (a likely normal-use failure, data-loss risk, or requirement violation) or **Worth fixing** (useful but not urgent). Update the last reviewed commit only across a continuous range actually reviewed. Commit only the review-state files.

## 8. Fix review findings

Handle one finding at a time, starting with **Must fix** items.

1. Confirm the report is true by reading the code, the cited requirement, and the real path. Reproduce it with a failing test when practical. A finding that cites a committed skipped reproduction test (§12) starts by un-skipping it and confirming it fails.
2. If it is false or already fixed, remove the finding and record the short evidence in the changelog and human update; do not change code.
3. If it is real, fix it, add or keep a regression test, run the required checks, and update the relevant specification if the correct fix changes behavior or fills missing coverage.
4. When the work is verified, remove the finding, update the handoff, and commit.

If the correct fix needs a user decision or cannot pass the required checks, leave the finding open and follow §3.

## 9. Usability reviews

Usability reviews do not change product code or specifications. Exercise the real user journeys in [`USABILITY-REVIEW.md`](USABILITY-REVIEW.md) against the feature acceptance criteria. Record problems a person is likely to meet, with reproduction evidence, using the same **Must fix** / **Worth fixing** format as §7. If a browser or credentialed journey cannot run, say so plainly rather than treating it as passed.

## 10. Keep specifications accurate

Specifications describe shipped behavior and architecture. Requirement IDs are append-only: do not renumber or silently change their meaning. Mark unshipped items `(planned)`; record an explicit deviation when shipped code and a specification temporarily differ.

Tests, commits, and review findings should cite a requirement ID only when that link helps someone find the rule being checked. The specification checker verifies the mechanics of these links, but people still need to judge whether the text and code agree.

Build and fix sessions edit specifications. Review sessions report missing or incorrect specification coverage; usability reviews report observed behavior and do not edit specifications.

## 11. Design a feature

`/design-feature` turns one new or recorded idea into work that is ready to implement. It changes
specifications and planning documents, not product code.

1. Use the idea named by the user. If none is named, take the first entry under `New ideas` in
   `docs/ideas.md`; this explicit default is the only time an agent selects future work itself.
2. Move the idea to `Ideas being defined` and work with the user to understand its outcome, scope,
   exclusions, edge cases, compatibility, and acceptance criteria.
3. Draft the feature specification first. Extend an existing FS when it already owns the capability;
   create a new one only for a distinct capability. Mark every unshipped requirement `(planned)`.
4. Explain the proposed feature behavior in the conversation before asking for confirmation: state
   what changes, when it happens, where its data lives, what users and agents/API clients observe,
   lifecycle/retention effects, exclusions, and each remaining product decision. Do not ask a
   person to confirm an unspecified “behavior.” Then wait for their confirmation before designing
   the architecture.
5. Draft the matching technical specification. Read the relevant code and invariants first. For a
   non-trivial tradeoff, explain the options and recommendation and wait for the user's decision;
   do not silently choose security, privacy, persistence, compatibility, migration, or protocol
   boundaries.
6. When both specifications are complete, create a descriptive file in `docs/ready-changes/` with
   the exact requirements and acceptance evidence, remove the source idea, and run the documentation
   checks. Do not make the change active in `HANDOFF.md`; implementation has not started.

If a material decision remains unresolved, leave the idea under `Ideas being defined` and do not
call it ready. Keep partial specifications honest and record what is needed from the user.

## 12. Investigate a reported bug

`/investigate-bug` turns a field bug report into a diagnosed finding that §8 can fix. Reports come
from real use, often from a machine where live debugging is impossible, so the report and any
captured logs may be the only evidence there will ever be. The investigation changes no product code
and no specifications, with one exception: it may commit a reproduction test marked skipped.

1. **Record the report.** Capture the report verbatim before interpreting it: the symptom, what the
   person was doing, the AgentDeck version or commit when known, the environment, and the logs — or
   the explicit fact that nothing was logged. If the request carries no report, ask for one.
2. **Establish expected behavior first.** Find the FS/TS/INV items that govern the reported area
   before reading code, and classify the report:
   - a **code defect** — shipped behavior violates a requirement; keep investigating;
   - a **spec gap** — the behavior is unspecified; record a coverage finding, and keep investigating
     any defect that remains;
   - **works as specified** — an expectation mismatch, not a bug. Say so plainly, route the wish to
     `docs/ideas.md` or a user product question, and stop.
3. **Investigate with evidence.** Map each log line — and each silence — to the exact code path, and
   check history for when the behavior changed. Try to reproduce locally; a failing test is the
   strongest evidence. When one is achieved, commit it marked skipped with a comment citing the
   finding, so the repository stays green; the §8 fix session un-skips it as the regression test.
4. **State confidence honestly.** Every conclusion is **confirmed** (reproduced, or proven from the
   code path), **probable** (consistent with all evidence but not reproduced), or **undetermined**.
   Never present a plausible story as a confirmed root cause.
5. **Missing observability is a finding.** When the investigation stalls because the failure point
   logged nothing, record what diagnostic should exist, where, and what question it would have
   answered, as its own finding. An undiagnosable report must at least make the next one diagnosable.
6. **Close.** Record findings in `## Review findings` in `HANDOFF.md` using the §7 format plus the
   confidence label, so §8 consumes them unchanged. Commit only the state files and any skipped
   reproduction test, then finish with the §6 human update.

## 13. Review a design

`/review-design` reviews a waiting change in `docs/ready-changes/` — its change file and the planned
FS/TS/INV items it cites — before implementation starts. It is the §7 author/reviewer split applied
to a design: record findings, change nothing. Resolving findings is follow-up design-feature work
with the human (§11), which removes each finding as it revises.

`$ARGUMENTS` names the change; with exactly one waiting change, use it; with several, ask. Read the
change file, every planned requirement it cites, the shipped requirements around them, and the
matching invariant classes before judging. Then review through three lenses, in this order:

1. **Over-engineering.** Every planned requirement must earn its place from the confirmed user
   outcome, a real report, or a binding invariant. Abstraction layers, configuration surfaces,
   modes, and edge-case machinery that no journey or report demands are findings; a requirement
   that exists "in case" of an unobserved situation is a finding, not prudence. This repository
   stays understandable only while each feature ships at its smallest coherent size.
2. **Maintainability and extension.** Prefer a design that extends an existing FS/TS area, seam,
   route, storage shape, or presentation pattern over one that mints a parallel mechanism,
   interface, or vocabulary an existing one could stretch to cover — the design-level twin of
   INV §2. Anything genuinely new must be worth the full §6 interface-contract cost, and the
   design should say why extension was rejected.
3. **Research.** Verify the design's factual assumptions against the tree: the cited seams,
   helpers, and routes exist and behave as assumed; claimed package, CLI, or protocol capabilities
   are real for the pinned versions; named dependencies are actually available. An assumption the
   code or a pinned tool contradicts is a finding, recorded with the contradicting evidence.
   Claimed limitations get the same rigor as claimed capabilities, and an implied limitation is
   still a claim: when a design says something cannot be done the straightforward way, silently
   takes an indirect route whose only sensible motive is such a limitation, or reads as a strange
   strategy for the stated goal, validate the limitation itself. Show it is real against the
   actual code, the pinned tool's real surface, newer releases, and better alternatives — a direct
   approach, an existing seam, an official package — with the evidence in the change file. An
   unverified impossibility claim is a finding, and the workaround it motivated is re-judged under
   lens 1 once the claim falls. Your own findings are claims too and earn the same rigor: before
   asserting that something fails, check whether the tree already prevents it — a guard, an existing
   validation, an ordering the design inherits — and cite what you read. A finding whose scenario the
   shipped code forbids is worse than silence, because the revision it forces deletes a real
   protection's justification.

Also check the design's own hygiene: planned items tagged `(planned)`; no contradiction with
shipped requirements or invariant classes; failure, concurrency, and rollback paths owned;
acceptance evidence observable; and no product decision silently pre-made. Where a hygiene defect has
a real consequence it is a finding; where it exists only on the page it is a consistency note.

**Every finding names its consequence.** Write each one as who experiences what, and when: the person
or agent affected, the situation that reaches them, and what goes wrong. A finding you cannot finish
that sentence for is not a finding. "Two requirements word this differently" is not a consequence; "an
agent that reports a result never receives its tool response" is. Order findings by that consequence,
worst first, so severity is visible without reading all of them.

**Findings and consistency defects are separate outputs.** A defect that exists only on the page — an
acceptance item that drifted from the requirement it verifies, a list missing a state named elsewhere,
a sentence an earlier edit left stale — goes in `## Design consistency notes` in `HANDOFF.md` as a
plain list for the next design pass to sweep. It is not a Must fix, it does not gate implementation,
and it does not belong in `## Review findings`. Mixing the two flattens severity and makes a
proofreading pass read like a broken design. Expect most of them to come from the previous round's own
edits: report them, do not dramatise them.

**The review has an exit.** A round that produces no finding with a user-visible or unbuildable
consequence closes the design for review — say so plainly, record it in the review history, and do not
ask for another pass. Without an exit the answer to "did you find anything" is always yes, because any
document can be sharpened indefinitely. Falling severity across rounds is evidence the design is
ready, not a reason to look harder.

Record findings in `## Review findings` in `HANDOFF.md` using the §7 format. The change stays
`Waiting to start` — implementation must not begin while a design has open Must-fix findings.
Consistency notes never hold it. With no findings, say so plainly and leave the change untouched.
Commit only the state files and finish with the §6 human update.

## 14. Design UI

`/design` is the design-quality workflow for AgentDeck's interface. It runs explicitly when named
and accompanies another role automatically when that role materially depends on visual judgment:
a new screen, redesign, meaningful composition or styling change, motion, visual polish, or design
critique. It stays out of data wiring, state fixes, behavior tests, copy-only changes, and other
frontend engineering whose visual result should remain unchanged.

The workflow does not authorize work by itself. It never chooses an idea or ready change, enlarges
the active request, replaces `/design-feature`, turns review into implementation, or relaxes the
specification and role rules above. It adds design reasoning and rendered verification to the work
the human already selected. A design-only or critique request edits no product code; an explicit
implementation request follows §§1–6 and §10, including specification-first behavior changes.

### 14.1 Ground the direction

Read the governing feature requirements, FS-12, TS-08, `ui/AGENTS.md`, the affected components and
styles, and at least one representative rendered incumbent surface. Treat the shipped presentation
as authority unless the request is a redesign. A local addition inherits its surrounding visual
language; it is not a reason to invent another skin, theme, component layer, or identity.

Before implementation, capture a compact direction in the working plan or existing change document:

- **Operator and job:** who uses this surface, the repeated task, its frequency, and the one result
  the first view must make easiest.
- **Composition:** first-view thesis, reading order, density, focal point, and what stays quiet.
- **AgentDeck proof:** one choice rooted in agent lifecycle, coordination, technical work, or another
  real product mechanism; name why it belongs here rather than in any administration dashboard.
- **States and interaction:** real content ranges plus relevant empty, loading, error, permission,
  paused, active, and completed states; feedback for every action in scope.
- **Motion decision:** no motion, or one named purpose: feedback, spatial continuity, state
  indication, or bridging a jarring change. Delight is reserved for rare moments.
- **Anti-goals:** the incumbent behavior and visual commitments that must survive, plus category
  defaults the result must not fall into.

For a standalone `/design` critique, do not create a direction or change document. Use the governing
requirements and the rendered incumbent surface as the evaluation direction, report the evidence in
the conversation, and edit no repository file.

Do not create a parallel design-brief hierarchy. Keep transient direction in the plan; put durable
user-visible behavior in the governing FS and durable presentation architecture in TS-08. If a
genuinely open new screen or redesign has materially different viable structures, show the human two
or three named compositions that differ in hierarchy, density, interaction, or motion before
building. Do not run this exploration for a narrow extension or an already-settled direction.

### 14.2 Compose before decorating

Make hierarchy legible before adding polish. Structure must encode real information: sequence,
ownership, state, dependency, urgency, or scope. Avoid flattening the interface into equal cards,
nested surfaces, decorative labels, or repeated icon tiles. Use real product-shaped content while
designing so long names, dense runs, blocked work, errors, and empty states influence the result.

Spend visual boldness in one place and keep the operating surface disciplined around it. AgentDeck's
lifecycle and coordination states are strong material: transitions between working, waiting,
blocked, delegated, failed, and completed can carry hierarchy and continuity. Expression must never
obscure the task, state, familiar control, semantic status color, or technical content. Typography,
spacing, alignment, and copy carry most of the finish in a frequently used tool.

Implement through TS-08's existing seams: semantic `--ad-*` tokens, `components/ui`, feature-owned
composition, the versioned presentation hooks, Core, Sky & Grove, and the deterministic visual
matrix. Extend a seam only when the requested result requires it. Do not add a design framework,
theme provider, one-off token system, screenshot baseline, runtime dependency, or generic design
detector as part of this workflow.

### 14.3 Make motion earn its place

Frequency is a gate. High-frequency and keyboard-driven actions should normally be instant;
occasional transitions may be brief; rare lifecycle milestones have the most room for expression.
Animate only when the purpose named in the direction improves comprehension. Prefer the cheapest
existing mechanism, usually a narrow CSS transition or animation, and do not install a motion
library for an effect CSS already handles.

Motion must preserve stable reading and interaction: do not move content the person is reading for
decoration; make triggered motion interruptible where repeat input is possible; use an origin that
explains where an overlay or item came from; gate hover behavior to hover-capable pointers; and ship
a `prefers-reduced-motion` treatment with the change. Lifecycle motion plays for a newly observed
transition, not on initial render, reload, background refetch, or history replay.

### 14.4 Inspect the rendered result

Rendered review is required for material visual work. Run the working tree's actual UI in a real
browser and inspect the affected route or fixture after fonts, data, and motion settle. The visual
matrix is repeatable input, not a substitute for the product surface. Choose viewports from the
governing requirements and affected layout rather than assuming universal mobile support; always
include the supported desktop floor when layout changes. Compare Core and Sky & Grove when a shared
token, primitive, hook, integration, or feature composition changes.

Exercise the important state matrix from the direction, including interaction states that static
fixtures cannot prove. For motion, observe the transition at normal speed, repeat or interrupt it,
and check reduced motion. A valid inspection checks hierarchy, scan path, density, state salience,
copy, focus and hover feedback, contrast, alignment, clipping, overflow, stacking, and whether the
result still belongs unmistakably to AgentDeck.

Critique against the direction rather than personal taste. Name rendered evidence and consequence,
rank implementation findings as **Must fix**, **Worth fixing**, or polish, and remove unsupported
decoration before adding more. A major new or redesigned surface benefits from an independent,
fresh-context rendered critique when delegation is available; a narrow edit does not justify that
overhead. In a build/fix role, batch the in-scope findings into a repair pass and render again. If
material in-scope problems remain, repair them and re-render; otherwise stop. Do not continue
iterating on subjective polish once the direction and material findings are satisfied. A standalone
`/design` critique reports rendered evidence and findings in the conversation and edits no
repository file. A §7 code review records findings and state updates exactly as §7 directs. Source
review alone cannot close a visual claim, and open-ended polishing is not a completion strategy.
