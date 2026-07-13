# AgentDeck — Spec-Driven Session Workflow

**Canonical protocol for autonomous, quota-limited, spec-driven work.** Both Claude Code
and Codex follow this exact loop (the human runs one at a time). The goal is **fire-and-forget**:
you are handed a change, you drive it spec-first to GREEN checkpoints until it is done or your
quota runs out, and you keep [`HANDOFF.md`](HANDOFF.md) so accurate that the next agent
(possibly a different CLI) can resume cold without you explaining anything.

> Claude Code reaches this loop via the `/work-phase` skill. Codex reaches it via the
> repo-root [`AGENTS.md`](../../AGENTS.md). Both land here. This file is the single source
> of truth for *process* — if a skill and this doc ever disagree, this doc wins. The
> **specs** ([`docs/specs/`](../specs/README.md)) are the single source of truth for the
> *product*: what it does and how it is built.

---

## 0. The map (read once, then trust the handoff)

- [`../specs/README.md`](../specs/README.md) — **the spec system.** Feature specs (`FS-nn`,
  product behavior) and technical specs (`TS-nn`, architecture & constraints) are the source of
  truth; the constitution defines IDs, statuses, templates, and the spec lifecycle. Read the
  governing specs *before* the code.
- [`HANDOFF.md`](HANDOFF.md) — **agent-facing live state.** Where we are, what's next, open findings,
  decisions, and blockers. Read first, every session.
- [`../product-backlog.md`](../product-backlog.md) — **human idea intake.** Inbox ideas,
  human-authorized discovery, ready-to-build work, candidates, and known gaps. It is not a spec and
  is never an autonomous work queue.
- [`BRIEFS.md`](BRIEFS.md) — **human-facing session log.** The newest entry is the only thing the
  human should need for a quick return. Agents do not read old briefs to resume.
- [`INVARIANTS.md`](INVARIANTS.md) — the paid-for bug-class catalog, part of the technical spec
  set (cited `INV §n`). Its intro lists the hot-spot areas and how each loop role uses it: build
  reads the matching class first, review (§8) sweeps the diff against every class, fix (§9) names
  the class it closes and appends new ones.
- Plans: the active work's checklist lives in `HANDOFF.md`'s **Active work detail**; a large
  change may carry a `docs/plans/<change>.md`. Plans are disposable sequencing, never truth.
- [`../../MAP.md`](../../MAP.md) — top-level index. [`../archive/`](../archive/) — superseded
  planning history (phase PRDs/techspecs, master PRD); never build from it.

A **change** is split into checkpoint-sized steps. Each step ends at a **GREEN checkpoint** so
work is never left half-done.

### 0.1 Human idea intake and work selection

The human’s wording authorizes a different first move:

| Human request | Agent action | May product code change? |
|---|---|---|
| “Consider” / “add this idea” | Capture an `I<n>` Inbox item faithfully. | No. |
| “Design” / “spec this” | Promote the named item to active `Discovery` in `HANDOFF.md`; draft an FS/TS proposal and surface product decisions. | No. |
| “Build” / “implement this” | Select the named item as active `Implementation`; draft/confirm its FS/TS delta before code. | Yes, after the delta is adequate. |

`HANDOFF.md` is the single execution selector. An active item must state **Source**, **Stage**,
governing IDs, and **Done when**. If there is no active `Implementation` item, `/work-phase` must
not choose a candidate, gap, or Partial-spec `(planned)` item by itself. It may only capture a new
human-supplied idea or report that selection is needed.

---

## 1. The loop

```
1. ORIENT  → read HANDOFF.md; confirm an active Implementation change + next incomplete step;
             open the governing FS/TS sections named there.
2. SPEC    → if the work alters user-visible behavior or an architectural contract,
             draft the spec delta first (§11): add/update R/A items, tag unshipped ones
             `(planned)`. No governing spec? Create or extend one.
3. BUILD   → implement the step (coding subagents OK — see §4).
4. VERIFY  → run the GREEN checkpoint (§2). Not green → fix. Can't fix → STOP (§3).
5. RECORD  → flip the delta's `(planned)` tags for what shipped, update HANDOFF.md,
             condense (§5), commit at the checkpoint (§6).
6. REPEAT  → next step. Change done? Stop at the GREEN checkpoint unless the handoff already names
             the next human-queued implementation change.
7. EXIT    → on stop/quota/blocker: leave HANDOFF green and accurate; write the human brief (§7).
```

**Keep going.** Do not stop just because one step finished — a finished step at a GREEN
checkpoint is the *ideal* place to be cut off, not a reason to quit. Continue until the change is
complete, you hit a STOP condition (§3), or your quota is exhausted.

**Bug fixes that restore specified behavior skip step 2** — the spec already says what the code
should do; cite the R/A item you are restoring instead.

---

## 2. GREEN checkpoint (the definition of "safe to stop")

A checkpoint is GREEN when **all** of these pass:

```bash
make test                      # spec lint + untagged and sqlite_fts5 Go suites
make build                     # shipped, sqlite_fts5-tagged binary
cd ui && npm test && npm run build # ONLY for steps that touch ui/
```

`TS-06` owns these targets and their release tags; `make dist` additionally rebuilds and embeds the
UI. The
active plan's **"Done when"** line may add specific tests that must pass — treat those as part of
green. And green has a documentation clause: **the governing specs reflect what shipped** — no
R/A item still tagged `(planned)` for behavior that landed, no shipped behavior the spec
contradicts. A stale governing spec is a red checkpoint. Never record a step as done, and never
commit, on a red checkpoint. Never buy GREEN by skipping tests, weakening assertions, or removing
regression coverage unless the spec and a focused review establish that the test itself is obsolete.

For a **docs-only spec/workflow change**, GREEN is proportional: run `make check-specs`, syntax or
render checks for changed tooling/docs, and `git diff --check`, plus enough targeted inspection to
validate factual claims. Do not rerun unrelated product/UI suites merely for ceremony. If the docs
change build/test commands or assert behavior needing executable confirmation, run that command.
Any product-code change still runs the full checkpoint above.

---

## 3. STOP conditions — when to surface to the human instead of pushing on

Fire-and-forget means: **only stop for things genuinely outside your authority or ability.**
When you hit one, append it to **`## Blocked on human`** in `HANDOFF.md` with enough context
to answer cold, leave the checkpoint green, and make the blocker the leading attention item in the brief.

Stop when:

- **A spec gap that is a product call.** The governing FS/TS genuinely doesn't say, writing the
  delta yourself would decide user-visible scope, security posture, or compatibility, and guessing
  would be expensive to undo. (First *try* to resolve it from the specs — most "ambiguities" are
  answered by an existing R-item or `INV` class. A gap you can fill with a reversible, local
  choice is a PEER-decision spec delta, not a STOP.)
- **A checkpoint won't go green** after a reasonable, honest effort, and the fix needs a decision or info you don't have.
- **Missing credentials / external input** (e.g. a real CLI login for a credential-gated acceptance item).
- **A destructive or irreversible action** would be required (force-push, deleting user data, rewriting history, anything outward-facing).
- **Spec conflict** — two specs contradict each other, or a spec contradicts already-shipped code
  in a way that needs a human call about which one is wrong.

Do **not** stop for: a step finishing, a normal failing test you can fix, a design choice the
specs already make for you, or routine multi-step work.

### Decisions the specs do not make

Record only choices caused by genuine under-specification. Do not record choices dictated by a
spec, established repository convention, or ordinary implementation mechanics. Put unresolved
choices under **`## Decisions awaiting review`** in `HANDOFF.md` with one of these tags:

- **HUMAN** — the choice changes user-visible behavior or accepted scope; affects security, privacy,
  permissions, durable data, or protocol/interoperability compatibility; deviates from a spec;
  accepts material risk; or is costly to reverse. If no safe reversible default exists, STOP.
  Otherwise proceed provisionally, record the choice, reason, consequence, and reversal, and repeat
  it by the same short descriptive title in every new brief until the human explicitly acknowledges it.
  Closely related titles may share one brief sentence, but none may disappear. **Silence is not consent.**
- **PEER** — a reversible, local implementation choice with none of the effects above. Record it for
  the next independent review, not in the human brief. The originating session cannot clear it.

The review action (§8) accepts, rejects, or escalates PEER items. It never clears HUMAN items.
Shipped behavior is already described in its governing spec; the HUMAN item carries only the
pending reversal/confirmation question. When the human resolves it, update/promote/retire the spec
contract and drop the live question (§11). Accepted routine PEER choices disappear after a terse
changelog entry; a binding accepted PEER choice becomes a spec-gap finding for fix-review.
If the human acknowledges only part of a compound HUMAN item, split it and retain the unacknowledged
part. If the required attention titles alone would overload a focused brief, STOP accumulating new
HUMAN decisions and use the brief to ask the human to triage them.

---

## 4. Delegate bounded work when available

When the environment supports delegation, use it for bounded independent work: repository sweeps,
diff audits, isolated implementations, fixtures, and tests. State file ownership before parallel
edits and require structured evidence. The main agent owns spec meaning, integration, self-review,
and the final GREEN checkpoint; subagents may draft spec text but do not establish authority by
themselves. In environments without delegation, run the same steps sequentially.

---

## 5. Keeping `HANDOFF.md` lean — condensation rules

The handoff must always reflect *current* truth and nothing stale. Condense as you go:

- **A step finishes** → tick it. The active change is the **only** place granular steps live.
- **A step reaches GREEN and is fully done** → delete its per-step list, mark it done in the
  change line, and expand the next step's checklist in the "Active work detail" block.
- **A whole change is done** (all steps green, its FS acceptance criteria pass, specs Current) →
  collapse it to one changelog line and **delete its breakdown entirely**; delete its
  `docs/plans/<change>.md` if one existed (git keeps the history). Durable knowledge produced by
  the change belongs in the specs, not the handoff.
- **Decisions / blockers** that still matter → keep them, tersely, in their sections. Drop ones that no longer apply.
- **Decisions awaiting review** → HUMAN items stay until explicit human resolution, linked to the
  shipped contract already in the governing spec. PEER items stay only until review accepts,
  rejects, or escalates them. Do not grow a decision graveyard.
- **Review findings** → the section holds **only open findings.** The moment the fix agent (§9) resolves a
  finding (fixed + green + committed) **or** dismisses it as a validated false positive, **delete the bullet**
  and drop a one-line entry in the changelog. The commit/changelog + the session brief are the record —
  do not keep a `✅ RESOLVED` / `❌ DISMISSED` graveyard here.
- **Changelog** → keep only the last ~10 entries; older history lives in git.
- **Last GREEN checkpoint** → record the commit subject plus verification before committing; do not
  create another commit merely to embed a commit's own SHA.

What survives in live state: the one-line-per-change status, the *active* work detail, unresolved
decisions, open blockers/findings, and a short recent changelog. Everything else is junk — remove it.

---

## 6. Commit at every GREEN checkpoint

Commits are the recovery anchor across spaced sessions, so the work survives a hard quota cut-off.

- **Commit directly to `main`.** This repo is trunk-based: no per-change branches, no PRs. Each GREEN
  checkpoint is a commit on `main` — that's the recovery anchor. Never commit a red checkpoint.
- At each GREEN checkpoint, commit the code, the spec delta, **and** the updated `HANDOFF.md` together.
- Message: `work: <step title> — green checkpoint`, naming the governing spec IDs in the subject
  or a `Spec: FS-nn.Rk, TS-nn` trailer. A commit that only lands a spec delta is
  `spec: <what changed and why>`.
- Add a `Co-Authored-By:` trailer naming the model that actually did the work.
- Push only when the human request or execution environment authorizes publishing. A local GREEN
  commit is a valid recovery anchor; never force-push unless the human explicitly asks.

---

## 7. Start-of-session checklist

1. Read `HANDOFF.md` top to bottom.
2. Inspect `git status` and the diff before editing. A dirty tree is presumed interrupted or
   user-owned work: reconcile it with the active detail and last GREEN checkpoint; never discard it.
3. Trust a recent recorded GREEN checkpoint unless the tree/environment/marker changed or the next
   step touches a high-risk seam. Otherwise confirm the committed baseline before new work. If a
   dirty tree prevents that run, preserve and document it first; red on arrival is the first problem.
4. Open the governing FS/TS sections named in the active work detail, then the active plan/step.
   Spec first, then build. Verify. Record. Repeat.
5. Before a checkpoint commit, self-review the complete diff against the step's **Done when**
   requirements and the governing spec sections: look for missing deliverables, unchecked error
   paths, boundary validation gaps, leftover debug/TODO code, unintended scope, and spec text
   left stale by the diff. Fix any issue found before recording GREEN.

## End-of-session checklist (every exit, including quota cut-off mid-work)

1. Tree at a GREEN checkpoint (or, if cut off mid-step, handoff clearly says what's half-done and how to finish it).
2. `HANDOFF.md` updated + condensed; `Last GREEN checkpoint` and changelog current; governing
   specs not left contradicting shipped code.
3. Committed (if green). The session's exact human brief is stored and returned as defined below.

Reserve enough quota for this closeout. If the process is terminated too abruptly to record state,
the next session treats the dirty tree as interrupted work, reconstructs it before editing, and writes
one brief covering the recovery; do not invent a blocker or pretend the interrupted step was GREEN.

### Human brief — every session, one bounded report

Every implementation, review, fix-review, and usability-review session produces exactly one brief.
Optimize for **a focused, intuitive explanation that rebuilds the reader's mental model**, not for a
word count: long enough to make the current state and what changed genuinely click, short enough that
every sentence earns its place — a few compact paragraphs, never padded to fill space or truncated to
hit a number. Prepend it to [`BRIEFS.md`](BRIEFS.md), commit it with
the session's final state, then send that exact Markdown verbatim as the final chat response with
nothing before or after it. Do not add a second report.

Write for someone returning cold who may have forgotten project vocabulary. In priority order:

1. State the current project position and session outcome.
2. Explain what materially changed, why it matters, and just enough of how that part works to rebuild
   the reader's mental model.
3. Include every open HUMAN decision by descriptive title/category, every actual blocker and BLOCKING
   finding, and any spec deviation or material risk. Mention a nonblocking acceptance gate only when
   it affects the next action or a compatibility/release claim.
4. State the next concrete action and whether it belongs to the human or an agent.
5. Add the optional learning item below only when it earns its place without crowding out the required
   state and attention above.

Use this minimal shape. `Needs attention` is always present as a safety check; only the learning heading
is optional. Avoid changed-file lists, raw symbols, validation-command inventories, routine decisions,
and “no decisions” boilerplate. Expand project-specific acronyms when first relevant. For a blocker,
give one sentence of current state, then put `Needs attention` immediately next.
Within `Needs attention`, lead with **New/changed this session**, then label unchanged items **Carried**;
use the same short HUMAN titles as `HANDOFF.md`. The brief is the orientation dashboard, not the full
decision packet—the human can ask the agent to explain a titled choice without reading the handoff.

```md
### YYYY-MM-DD — <role>: <scope>

<State/outcome, why it matters, and how this part now works.>

**Needs attention:** <all open human-tier items/blockers/blocking risks, or “None.”>

**Next:** <one concrete action.>

**What this teaches:** <optional only>
```

Include **What this teaches** only when the session introduced or materially clarified an AI protocol,
framework, or agent concept; a consequential architecture trade-off or limitation; surprising agent
behavior; or a reusable failure/debugging lesson that will improve future decisions. Explain it through
this session's work, expand the first relevant acronym, and do not reteach it unless the new work adds a
new consequence. It remains lower priority than required state and attention items; omit it if it would
make those vague. Omit it for routine work, however technically complex. If the fact is needed by future
agents, also update the governing spec or invariant document—never create a learning log.

---

## 8. Review action (separate from the build loop)

Triggered independently (Claude Code: `/review-phase`; Codex: `"Review the unreviewed work per AGENTS.md"`).
Reviews the other agent's work — not your own. This is **product-code-read-only**: it must update
workflow state, but it does not change product code, specs, plans, or the invariant catalog.

### What to review

Default target: every non-state commit after `Last code review` through current `main` (trunk-based—
no PRs). The human may name a specific commit or range; review an unlabeled commit rather than assuming
it is GREEN.
Workflow-state-only commits are not review targets; classify them by their changed paths/content, not
the subject alone (this also excludes state-only dismissal/brief commits). When done, set
`Last code review` to the newest contiguous code/content commit actually reviewed. A scoped or
noncontiguous review does not advance it past an unreviewed gap.

Read the governing FS/TS sections **before**
opening the diff, and note the expected deliverables. A diff can show an incorrect implementation,
but it cannot reveal work that was never attempted. Then inspect changed code in its caller,
error-path and concurrency context.

Cross-reference against — **traceability runs in both directions**:

- **Code → spec:** does the diff do what the governing R/A items say? Any required deliverable
  missing or wrong? Cite the violated ID in the finding.
- **Spec → code:** did user-visible behavior or an architectural contract ship that **no spec
  covers**, or that the spec still tags `(planned)`, or that contradicts an R-item without a
  recorded deviation? That is a **spec-gap finding** — the fix is a spec delta (or a code revert),
  decided by §9.
- **Dead code** — exported symbols never referenced, unreachable paths, leftover stubs or TODOs that should be done.
- **Bad practices** — error swallowing, obvious data races, magic strings, hardcoded paths, missing input validation at system boundaries.
- **Flagrant bugs** — nil dereference risks, wrong status codes, logic inversions, off-by-ones in critical paths, missing error checks on writes.
- **Pending PEER decisions** — accept them, reject them into a finding, or escalate them to HUMAN using §3.

### What NOT to chase

- Style nits, naming preferences, formatting.
- Micro-optimizations ("this could be a map instead of a slice").
- One-in-a-million edge cases that won't arise in regular personal use.
- Theoretical issues with no realistic trigger path.

The bar is: **would this cause a real problem during normal usage?** (For spec-gap findings the
bar is: would the next agent, building from the spec alone, produce the wrong thing?)

### Output

Categorize every finding as one of:

- **BLOCKING** — must fix before the next change starts (spec violation, data-loss risk, crash under normal use).
- **ADVISORY** — worth fixing but not blocking; next agent should address it when convenient.

Write **every** finding — BLOCKING *and*
ADVISORY — to `## Review findings` in `HANDOFF.md`, each tagged with its severity. This is the
hand-off contract: the fix agent (§9) and the next build agent can only act on findings that land
in the handoff, so an advisory spoken only in chat is lost. Use the entry shape in §9. If there are
no findings, write nothing to the findings section. Group a review's open bullets under
`### Review through <sha> — <date>`; delete the heading when its last bullet is resolved. Update
`Last code review`, disposition every PEER item, write the brief, and commit only workflow-state files
as
`review: through <sha> — state recorded`. A suspected new invariant remains a finding until fix-review
validates it; review does not edit product specs or the invariant catalog. A PEER choice that would
establish a binding contract becomes a spec-gap finding for fix-review; do not hide it in descriptive
ADR or architecture-flow documents.

The brief lists each BLOCKING finding and its normal-use impact. Summarize advisories by count/impact
unless one is itself a material risk. It repeats every unresolved HUMAN item.

### Review-findings entry shape (written by §8, consumed by §9)

Each finding lives as one bullet under `## Review findings`, opening with its severity tag so the
fix agent can triage at a glance. Include `file:line` + what's wrong + why it matters + a fix hint,
and cite the governing spec ID (`FS-nn.Rk` / `TS-nn.Rk` / `INV §n`) when one exists:

```
- **BLOCKING — <one-line title>.** <where: file:line> <what's wrong, citing FS/TS/INV ids> <why it matters under normal use> <fix hint + what test would prove it>
- **ADVISORY — <one-line title>.** <same shape>
```

Every finding must also state the concrete normal-use trigger or evidence that proves it. If the
reviewer cannot describe that path after re-reading the cited code, it is not a finding.

The §9 fix agent **deletes** a bullet once it has resolved the finding (fixed + green) or dismissed it
as a validated false positive, recording the outcome in the changelog + its brief (§5, §7) — the section
holds only open findings.

---

## 9. Fix action — validate, then fix the review findings

Triggered independently **after a review** (Claude Code: `/fix-review`; Codex: `"Fix the review
findings per AGENTS.md §9"`). Unlike §8 this is **not** read-only — it writes code, runs tests, and
commits, exactly like the build loop. Its input is the `## Review findings` section §8 just populated.

`$ARGUMENTS`, if present, scopes the run: a severity (`blocking` → only BLOCKING items), a review batch
(`through:<sha>`), or a finding title/keyword. Default with no args: **all** unresolved findings, BLOCKING first.

### The two-gate loop, per finding

Work findings one at a time, BLOCKING before ADVISORY. For each:

```
1. VALIDATE → confirm the finding is ACTUALLY TRUE before touching anything.
2. FIX      → only if validated real: implement to a GREEN checkpoint (§2) with a regression test.
3. RECORD   → delete the resolved bullet, condense (§5), commit at the checkpoint (§6).
```

**Gate 1 — VALIDATE (don't trust the review).** Reviews produce false positives. Read the cited
code at `file:line`, check the cited spec ID actually says what the finding claims, trace the actual
path, and convince yourself the problem is real and would bite under **normal use** (the §8 bar).
Where practical, reproduce it first with a **failing test** — that
both proves the finding and becomes the regression guard once you fix it.
  - **Real** → proceed to fix.
  - **False positive / not reproducible / already fixed since the review** → make **no code change**.
    Your validation is the verdict — **delete the bullet** and record a one-line changelog entry
    (`dismissed <title> — false positive: <one-line evidence>`). Call it out in your end-of-turn
    brief too, with concise evidence. A dismissal is not a HUMAN decision unless it depends on a
    HUMAN-tier assumption, and it does not linger in the findings section. Commit that verdict before
    starting the next finding as `review fix: dismiss <title> — state recorded`; a hard cutoff must not
    resurrect or lose the validated dismissal.

**Gate 2 — FIX.** Same rules as the build loop: implement the fix, add/keep the regression
test, and reach a **GREEN checkpoint** (§2). Two spec-aware cases:
  - **Spec-gap finding, behavior is right** → the fix *is* a spec delta: bring the governing spec
    up to date (or create the missing section) in the checkpoint commit.
  - **The right fix changes specified behavior** (not just restores it) → update the governing
    R/A items in the same commit as the code. If choosing the new behavior is a product call,
    that's a **STOP** (§3) / HUMAN decision, not a silent spec edit.
A fix is only done when green; never mark one resolved or commit on red. If a finding is real but
you can't get it green, or the right fix needs a human decision, that's a **STOP** (§3) — leave the
bullet as-is, record it under `## Blocked on human`, and stop after preserving the current safe state.

**RECORD.** When green, **delete the finding's bullet** and add a one-line changelog entry
(`review fix: <title> — <fix + the test that covers it>`). Commit code + spec delta + `HANDOFF.md`
together on `main` with message `review fix: <short title> — green checkpoint`. Then take the next finding.

### Carry over the same discipline

- **STOP conditions and decision routing (§3)** apply unchanged.
- **Condensation (§5):** delete each finding's bullet as you resolve or dismiss it (changelog +
  brief carry the record); the findings section trends to empty, never a graveyard of stale tags.
- **End-of-session (§7 checklist):** tree green, handoff updated, work committed, and the exact brief
  stored and returned. State what was fixed, what was dismissed (and why), and any blocker. If all
  per-finding checkpoint commits were already made before the final brief existed, make one small
  `review fix: session brief — state recorded` commit rather than leaving the brief uncommitted.

---

## 10. Usability review action (separate, product-code-read-only)

Triggered independently after a user-facing slice is runnable (Claude Code: `/usability-review`;
Codex: `"Run a usability review per AGENTS.md"`). Exercise the normal user journey against the
governing feature spec's **acceptance criteria** (`FS-nn.Ak`) and the journey matrix in
[`USABILITY-REVIEW.md`](USABILITY-REVIEW.md). Flag only friction or broken behavior a person is
likely to encounter; do not redesign from taste or chase polish with no normal-use impact. A
mismatch between observed behavior and an A-item is a finding citing that ID; observed behavior no
A-item covers is a candidate spec-gap finding (§8 shape).

Record every BLOCKING and ADVISORY usability finding in the same `## Review findings` contract as §8,
tagging the title `Usability`. Do not fix product code in this action. Update workflow state, write the
brief under §7, and commit it as `usability review: <journey> — state recorded`. If a browser or
credentialed journey cannot be exercised, record a HUMAN acceptance gate rather than pretending it passed.

---

## 11. Spec maintenance — keeping the source of truth true

The full rules live in the constitution ([`../specs/README.md`](../specs/README.md)); these are
the process hooks that keep the specs and the repo in lockstep:

- **Delta-first.** Draft behavior/architecture changes in the governing spec before implementation.
  Tag unshipped items `(planned)` and normally commit the spec with the code at GREEN; a separately
  committed spec-only checkpoint is also valid. A pure restore-the-spec bug fix needs no delta.
- **IDs are append-only.** Never renumber R/A items; retire them in place with a dated one-liner.
- **Deviations are explicit.** The only legitimate spec-vs-code mismatch is a `(planned)` tag or
  an entry in the spec's "Deviations & open decisions". Anything else is a finding.
- **Keep shipped truth in specs and questions in HANDOFF.** A provisional shipped boundary is
  documented in FS/TS immediately. HANDOFF carries only the pending question, linked to that item.
  When resolved, update/promote/retire the spec contract and drop the question.
- **Traceability marks.** Regression tests that pin an acceptance criterion cite it
  (`// FS-05.A2`); commits name the spec IDs they implement (§6); findings cite the IDs they
  violate (§8). Don't annotate beyond that — traceability that nobody reads is bureaucracy.
- **Lint.** `scripts/check-specs.sh` (in `make test`) keeps IDs, the index, and intra-spec links
  consistent. It cannot judge truth — reviews do that (§8).
- **Who edits specs:** build (§1) and fix (§9) sessions. Review sessions record spec-gap findings
  but do not edit specs; usability sessions never edit them.
