# AgentDeck — Spaced-Session Implementation Workflow

**Canonical protocol for autonomous, quota-limited phase work.** Both Claude Code
and Codex follow this exact loop (the human runs one at a time). The goal is **fire-and-forget**:
you are handed a phase, you build subphase by subphase until the phase is done or your
quota runs out, and you keep [`HANDOFF.md`](HANDOFF.md) so accurate that the next agent
(possibly a different CLI) can resume cold without you explaining anything.

> Claude Code reaches this loop via the `/work-phase` skill. Codex reaches it via the
> repo-root [`AGENTS.md`](../../AGENTS.md). Both land here. This file is the single source
> of truth — if the skill and this doc ever disagree, this doc wins.

---

## 0. The map (read once, then trust the handoff)

- [`HANDOFF.md`](HANDOFF.md) — **agent-facing live state.** Where we are, what's next, open findings,
  decisions, and blockers. Read first, every session.
- [`BRIEFS.md`](BRIEFS.md) — **human-facing session log.** The newest entry is the only thing the
  human should need for a quick return. Agents do not read old briefs to resume.
- [`README.md`](README.md) — phase plan, dependency graph, build order.
- `phase-N-*.md` — phase PRD (the *what* + acceptance criteria).
- `tech/phase-N-*-techspec.md` — tech spec (the *how*). Each ends in a **`## Subphase plan`** section — this is your task list.
- [`INVARIANTS.md`](INVARIANTS.md) — the paid-for bug-class catalog. Its intro lists the hot-spot
  areas and how each loop role uses it: build reads the matching class first, review (§8) sweeps
  the diff against every class, fix (§9) names the class it closes and appends new ones.
- [`../../MAP.md`](../../MAP.md) — top-level index. [`../agent-dashboard-prd.md`](../agent-dashboard-prd.md) — master PRD.

A **phase** is split into **subphases** (e.g. `5.1`, `5.2`). Each subphase is a single
quota-sized step that ends at a **GREEN checkpoint** so work is never left half-done.

---

## 1. The loop

```
1. ORIENT  → read HANDOFF.md; find the active phase + next incomplete subphase; open its spec section.
2. BUILD   → implement the subphase (coding subagents OK — see §4).
3. VERIFY  → run the GREEN checkpoint (§2). Not green → fix. Can't fix → STOP (§3).
4. RECORD  → update HANDOFF.md, condense (§5), commit at the checkpoint (§6).
5. REPEAT  → next subphase. Phase done? Roll to the next phase per the build order.
6. EXIT    → on stop/quota/blocker: leave HANDOFF green and accurate; write the human brief (§7).
```

**Keep going.** Do not stop just because one subphase finished — a finished subphase at a
GREEN checkpoint is the *ideal* place to be cut off, not a reason to quit. Continue until
the phase is complete, you hit a STOP condition (§3), or your quota is exhausted.

---

## 2. GREEN checkpoint (the definition of "safe to stop")

A checkpoint is GREEN when **all** of these pass:

```bash
go build ./...                 # whole module compiles
go test ./...                  # all existing + new tests pass
go test -tags sqlite_fts5 ./... # shipped FTS5 path also passes
cd ui && npm run build         # ONLY for subphases that touch ui/
```

`make build` / `make test` / `make dist` wrap these. Each subphase's **"Done when
(checkpoint)"** line in the tech spec may add specific tests that must pass — treat those
as part of green. Never record a subphase as done, and never commit, on a red checkpoint.

---

## 3. STOP conditions — when to surface to the human instead of pushing on

Fire-and-forget means: **only stop for things genuinely outside your authority or ability.**
When you hit one, append it to **`## Blocked on human`** in `HANDOFF.md` with enough context
to answer cold, leave the checkpoint green, and make the blocker the leading attention item in the brief.

Stop when:

- **Ambiguity the specs don't resolve.** The PRD/techspec genuinely doesn't say, and guessing
  would be expensive to undo. (First *try* to resolve it from the docs — most "ambiguities" are answered in the tech spec.)
- **A checkpoint won't go green** after a reasonable, honest effort, and the fix needs a decision or info you don't have.
- **Missing credentials / external input** (e.g. a real CLI login for a credential-gated acceptance subphase).
- **A destructive or irreversible action** would be required (force-push, deleting user data, rewriting history, anything outward-facing).
- **Scope conflict** — the spec contradicts itself or contradicts already-shipped code in a way that needs a human call.

Do **not** stop for: a subphase finishing, a normal failing test you can fix, a design choice the
tech spec already makes for you, or routine multi-step work.

### Decisions the specs do not make

Record only choices caused by genuine under-specification. Do not record choices dictated by the
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
Accepted durable rationale belongs in the existing architecture/spec/invariant documentation;
accepted routine choices disappear from live state after a terse changelog entry.
If the human acknowledges only part of a compound HUMAN item, split it and retain the unacknowledged
part. If the required attention titles alone no longer fit the 250-word brief, STOP accumulating new
HUMAN decisions and use the brief to ask the human to triage them.

---

## 4. Delegate freely — tier the quota

Subagents in this environment have full tool access (Bash, Edit, Write) and **can** run
`go build`, `go test`, and `npm run build`. You may delegate coding work to them — especially
self-contained pieces (a new file, a test, a migration) where the subagent can build and
verify in isolation. The constraint: **the main thread owns the GREEN checkpoint.** Before
committing, *you* must run the full checkpoint (§2) yourself to confirm integration.

**Tier the quota.** Sessions run on a premium, quota-limited model (Opus-class or above);
its turns and context are the scarce resource. Farm discovery and self-contained coding out
to subagents on a cheaper model (Sonnet, or Opus if the work requires deeper reasoning):
repo/doc sweeps, diff audits, isolated implementations, test writing. Have them return
structured results so the main thread spends its quota and context on design, judgment,
and integration.

---

## 5. Keeping `HANDOFF.md` lean — condensation rules

The handoff must always reflect *current* truth and nothing stale. Condense as you go:

- **A step finishes** → tick it. The active subphase is the **only** place granular steps live.
- **A subphase reaches GREEN and is fully done** → delete its per-step list, mark it done in the
  phase line (`5.2 ✅`), and expand the next subphase's steps in the "Active subphase detail" block.
- **A whole phase is done** (all subphases green, acceptance criteria met) → collapse it to a single
  line in "Phase status" (`[x] Phase 5 — Coordination ✅`) and **delete its subphase breakdown entirely.**
- **Decisions / blockers** that still matter → keep them, tersely, in their sections. Drop ones that no longer apply.
- **Decisions awaiting review** → HUMAN items stay until explicit human acknowledgement. PEER items
  stay only until the next independent review accepts, rejects, or escalates them. Promote durable
  accepted rationale to an existing architecture/spec/invariant document; do not grow a decision graveyard.
- **Review findings** → the section holds **only open findings.** The moment the fix agent (§9) resolves a
  finding (fixed + green + committed) **or** dismisses it as a validated false positive, **delete the bullet**
  and drop a one-line entry in the changelog. The commit/changelog + the session brief are the record —
  do not keep a `✅ RESOLVED` / `❌ DISMISSED` graveyard here.
- **Changelog** → keep only the last ~10 entries; older history lives in git.
- **Last GREEN checkpoint** → record the commit subject plus verification before committing; do not
  create another commit merely to embed a commit's own SHA.

What survives in live state: the one-line-per-phase status, the *active* subphase detail, unresolved
decisions, open blockers/findings, and a short recent changelog. Everything else is junk — remove it.

---

## 6. Commit at every GREEN checkpoint

Commits are the recovery anchor across spaced sessions, so the work survives a hard quota cut-off.

- **Commit directly to `main`.** This repo is trunk-based: no per-phase branches, no PRs. Each GREEN
  checkpoint is a commit on `main` — that's the recovery anchor. Never commit a red checkpoint.
- At each GREEN checkpoint, commit the code **and** the updated `HANDOFF.md` together.
- Message: `phase N.M: <subphase title> — green checkpoint`.
- Add a `Co-Authored-By:` trailer naming the model that actually did the work.
- **Pushing** is a STOP-style action — don't push unless the human has asked for it.

---

## 7. Start-of-session checklist

1. Read `HANDOFF.md` top to bottom.
2. Inspect `git status` and the diff before editing. A dirty tree is presumed interrupted or
   user-owned work: reconcile it with the active detail and last GREEN checkpoint; never discard it.
3. Confirm the committed baseline is green before new work. If the dirty tree prevents a clean
   baseline run, preserve and document it first. Red on arrival is the first problem to resolve.
4. Open the active subphase's tech-spec section. Build. Verify. Record. Repeat.

## End-of-session checklist (every exit, including quota cut-off mid-work)

1. Tree at a GREEN checkpoint (or, if cut off mid-step, handoff clearly says what's half-done and how to finish it).
2. `HANDOFF.md` updated + condensed; `Last GREEN checkpoint` and changelog current.
3. Committed (if green). The session's exact human brief is stored and returned as defined below.

Reserve enough quota for this closeout. If the process is terminated too abruptly to record state,
the next session treats the dirty tree as interrupted work, reconstructs it before editing, and writes
one brief covering the recovery; do not invent a blocker or pretend the interrupted step was GREEN.

### Human brief — every session, one bounded report

Every implementation, review, fix-review, and usability-review session produces exactly one brief,
at most **250 words including its heading**. Prepend it to [`BRIEFS.md`](BRIEFS.md), commit it with
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
5. Add the optional learning item below only when it earns space inside the same 250-word budget.

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
agents, also update an existing architecture/spec/invariant document—never create a learning log.

---

## 8. Review action (separate from the build loop)

Triggered independently (Claude Code: `/review-phase`; Codex: `"Review the last commit per AGENTS.md"`).
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

Cross-reference against:
- **Phase spec adherence** — does the code match the phase PRD and tech spec? Any required deliverable missing or wrong?
- **Dead code** — exported symbols never referenced, unreachable paths, leftover stubs or TODOs that should be done.
- **Bad practices** — error swallowing, obvious data races, magic strings, hardcoded paths, missing input validation at system boundaries.
- **Flagrant bugs** — nil dereference risks, wrong status codes, logic inversions, off-by-ones in critical paths, missing error checks on writes.
- **Pending PEER decisions** — accept them, reject them into a finding, or escalate them to HUMAN using §3.

### What NOT to chase

- Style nits, naming preferences, formatting.
- Micro-optimizations ("this could be a map instead of a slice").
- One-in-a-million edge cases that won't arise in regular personal use.
- Theoretical issues with no realistic trigger path.

The bar is: **would this cause a real problem during normal usage?**

### Output

Categorize every finding as one of:

- **BLOCKING** — must fix before the next phase starts (spec violation, data-loss risk, crash under normal use).
- **ADVISORY** — worth fixing but not blocking; next agent should address it when convenient.

Write **every** finding — BLOCKING *and*
ADVISORY — to `## Review findings` in `HANDOFF.md`, each tagged with its severity. This is the
hand-off contract: the fix agent (§9) and the next build agent can only act on findings that land
in the handoff, so an advisory spoken only in chat is lost. Use the entry shape in §9. If there are
no findings, write nothing to the findings section. Group a review's open bullets under
`### Review through <sha> — <date>`; delete the heading when its last bullet is resolved. Update
`Last code review`, disposition every PEER item, write the brief, and commit only workflow-state files
(plus an allowed architecture-rationale update) as
`review: through <sha> — state recorded`. A suspected new invariant remains a finding until fix-review
validates it; review does not edit product specs or the invariant catalog. It may record accepted durable
PEER rationale in the existing architecture decision/flow documents and include them in the state commit.

The brief lists each BLOCKING finding and its normal-use impact. Summarize advisories by count/impact
unless one is itself a material risk. It repeats every unresolved HUMAN item.

### Review-findings entry shape (written by §8, consumed by §9)

Each finding lives as one bullet under `## Review findings`, opening with its severity tag so the
fix agent can triage at a glance. Include `file:line` + what's wrong + why it matters + a fix hint:

```
- **BLOCKING — <one-line title>.** <where: file:line> <what's wrong> <why it matters under normal use> <fix hint + what test would prove it>
- **ADVISORY — <one-line title>.** <same shape>
```

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
code at `file:line`, trace the actual path, and convince yourself the problem is real and would bite
under **normal use** (the §8 bar). Where practical, reproduce it first with a **failing test** — that
both proves the finding and becomes the regression guard once you fix it.
  - **Real** → proceed to fix.
  - **False positive / not reproducible / already fixed since the review** → make **no code change**.
    Your validation is the verdict — **delete the bullet** and record a one-line changelog entry
    (`dismissed <title> — false positive: <one-line evidence>`). Call it out in your end-of-turn
    brief too, with concise evidence. A dismissal is not a HUMAN decision unless it depends on a
    HUMAN-tier assumption, and it does not linger in the findings section. Commit that verdict before
    starting the next finding as `review fix: dismiss <title> — state recorded`; a hard cutoff must not
    resurrect or lose the validated dismissal.

**Gate 2 — FIX.** Same rules as building a subphase: implement the fix, add/keep the regression
test, and reach a **GREEN checkpoint** (§2). A fix is only
done when green; never mark one resolved or commit on red. If a finding is real but you can't get it
green, or the right fix needs a human decision, that's a **STOP** (§3) — leave the bullet as-is,
record it under `## Blocked on human`, and stop after preserving the current safe state.

**RECORD.** When green, **delete the finding's bullet** and add a one-line changelog entry
(`review fix: <title> — <fix + the test that covers it>`). Commit code + `HANDOFF.md` together on
`main` with message `review fix: <short title> — green checkpoint`. Then take the next finding.

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
relevant acceptance criteria. Flag only friction or broken behavior a person is likely to encounter;
do not redesign from taste or chase polish with no normal-use impact.

Record every BLOCKING and ADVISORY usability finding in the same `## Review findings` contract as §8,
tagging the title `Usability`. Do not fix product code in this action. Update workflow state, write the
brief under §7, and commit it as `usability review: <journey> — state recorded`. If a browser or
credentialed journey cannot be exercised, record a HUMAN acceptance gate rather than pretending it passed.
