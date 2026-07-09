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

- [`HANDOFF.md`](HANDOFF.md) — **live state, agent-facing.** Where we are, what's next, decisions, blockers. Read first, every session.
- [`BRIEFS.md`](BRIEFS.md) — **the human's only channel** (§10). Every session ends by writing one short plain-language brief there.
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
6. EXIT    → on stop/quota/blocker: leave HANDOFF green and accurate, write the human brief (§10).
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
cd ui && npm run build         # ONLY for subphases that touch ui/
```

`make build` / `make test` / `make dist` wrap these. Each subphase's **"Done when
(checkpoint)"** line in the tech spec may add specific tests that must pass — treat those
as part of green. Never record a subphase as done, and never commit, on a red checkpoint.

---

## 3. STOP conditions — when to surface to the human instead of pushing on

Fire-and-forget means: **only stop for things genuinely outside your authority or ability.**
When you hit one, append it to **`## Blocked on human`** in `HANDOFF.md` with enough context
to answer cold, leave the checkpoint green, and end your turn with a one-line summary of what's blocking.

Stop when:

- **Ambiguity the specs don't resolve.** The PRD/techspec genuinely doesn't say, and guessing
  would be expensive to undo. (First *try* to resolve it from the docs — most "ambiguities" are answered in the tech spec.)
- **A checkpoint won't go green** after a reasonable, honest effort, and the fix needs a decision or info you don't have.
- **Missing credentials / external input** (e.g. a real CLI login for a credential-gated acceptance subphase).
- **A destructive or irreversible action** would be required (force-push, deleting user data, rewriting history, anything outward-facing).
- **Scope conflict** — the spec contradicts itself or contradicts already-shipped code in a way that needs a human call.

Do **not** stop for: a subphase finishing, a normal failing test you can fix, a design choice the
tech spec already makes for you, or routine multi-step work.

### Judgment calls you made anyway — record all, surface few

Sometimes an issue doesn't rise to a STOP (it wasn't blocking, or stopping would waste a whole
session) but it still forced **you** to make a design or implementation decision the specs didn't
dictate — you resolved an ambiguity, picked between reasonable options, worked around a spec gap or
a contradiction, or named/structured something the spec left open. **Never let those pass silently —
but the human is not the audience for most of them** (§10).

For each such call:

- Record it under **`## Autonomous decisions (please review)`** in `HANDOFF.md`: what was ambiguous/
  missing, the options, **what you chose and why**, and how to reverse it. This is the agent-facing
  record; the §8 review agent — not the human — is its first reviewer (§10).
- Promote to the human brief (§10) **only** the calls that clear the brief bar: a user-visible
  behavior change, a call that is costly to reverse, or a deviation from spec. Translate it to plain
  language and state the default you applied. Do **not** dump every call on the human — a 15-bullet
  decision list is a wall they will not read, which is worse than silence.

When in doubt whether a call clears the brief bar, one plain-language line in the brief costs little.
The full technical detail always goes to `HANDOFF.md`, never the brief.

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
- **Autonomous decisions** → keep each until the §8 review agent has peer-reviewed it (`— peer-reviewed <date>`
  tag, §8) *and*, for brief-worthy calls, it has appeared in a brief (§10); then fold the durable contracts into
  "Decisions & notes" and delete the rest. Do **not** hold entries waiting for human acknowledgment — the human
  reviews via briefs and objects by replying to one; silence is consent (§10).
- **Review findings** → the section holds **only open findings.** The moment the fix agent (§9) resolves a
  finding (fixed + green + committed) **or** dismisses it as a validated false positive, **delete the bullet**
  and drop a one-line entry in the changelog. The commit/changelog + the brief (§10) are the record —
  do not keep a `✅ RESOLVED` / `❌ DISMISSED` graveyard here.
- **Changelog** → keep only the last ~10 entries; older history lives in git.
- **Briefs** (`BRIEFS.md`, §10) → keep the last 4; delete older ones when adding a new one.

What survives long-term: the one-line-per-phase status, the *active* subphase detail, durable
decisions, open blockers, a short recent changelog. Everything else is junk — remove it.

---

## 6. Commit at every GREEN checkpoint

Commits are the recovery anchor across spaced sessions, so the work survives a hard quota cut-off.

- **Commit directly to `main`.** This repo is trunk-based: no per-phase branches, no PRs. Each GREEN
  checkpoint is a commit on `main` — that's the recovery anchor. (Only commit on a red checkpoint never.)
- At each GREEN checkpoint, commit the code **and** the updated `HANDOFF.md` together.
- Message: `phase N.M: <subphase title> — green checkpoint`.
- **Push to origin `main` when the task completes** — at session exit (§7), after the final green
  commit; no human approval needed (intermediate pushes are fine too). Only **force-pushes** remain
  a STOP-style destructive action (§3).

---

## 7. Start-of-session checklist

1. Read `HANDOFF.md` top to bottom.
2. Confirm the tree is green *before* you touch anything: `go build ./... && go test ./...`.
   Red on arrival → that's the first thing to fix (or a STOP if you can't).
3. Open the active subphase's tech-spec section. Build. Verify. Record. Repeat.

## End-of-session checklist (every exit, including quota cut-off mid-work)

1. Tree at a GREEN checkpoint (or, if cut off mid-step, handoff clearly says what's half-done and how to finish it).
2. `HANDOFF.md` updated + condensed; `Last GREEN checkpoint` and changelog current.
3. Committed (if green) and **pushed to origin** (§6). **Write the human brief** (§10): prepend it to
   `BRIEFS.md` and paste the same brief as your end-of-turn message. The brief *is* the summary — do
   not follow it with a longer technical report.

---

## 8. Review action (separate from the build loop)

Triggered independently (Claude Code: `/review-phase`; Codex: `"Review the last commit per AGENTS.md"`).
Reviews the other agent's work — not your own. This is **read-only toward code**: no code changes.
Its doc writes (findings, peer-review tags, `INVARIANTS.md`, the brief) are committed together on
`main` with a `docs: review <date> — <scope>` message and pushed at session exit (§6) — never left
as a dirty tree.

### What to review

Default target: the last GREEN-checkpoint commit(s) on `main` since the previous review (trunk-based —
no PRs). The human may also name a specific commit or range. Get the diff with `git log` / `git show` / `git diff`.

Cross-reference against:
- **Phase spec adherence** — does the code match the phase PRD and tech spec? Any required deliverable missing or wrong?
- **Dead code** — exported symbols never referenced, unreachable paths, leftover stubs or TODOs that should be done.
- **Bad practices** — error swallowing, obvious data races, magic strings, hardcoded paths, missing input validation at system boundaries.
- **Flagrant bugs** — nil dereference risks, wrong status codes, logic inversions, off-by-ones in critical paths, missing error checks on writes.

### What NOT to chase

- Style nits, naming preferences, formatting.
- Micro-optimizations ("this could be a map instead of a slice").
- One-in-a-million edge cases that won't arise in regular personal use.
- Theoretical issues with no realistic trigger path.

The bar is: **would this cause a real problem during normal usage?**

### Audit the pending autonomous decisions — you are the reviewer of record

The human is zoomed out and no longer reviews judgment calls (§10); this review is where they get
checked instead. Sweep every un-endorsed entry under `## Autonomous decisions` in `HANDOFF.md`
against the specs and shipped code:

- **Sound** → append `— peer-reviewed <date>` to the bullet (this is the one handoff write a review
  may make besides findings; it lets §5 condensation fold the entry).
- **Unsound / spec-violating** → write a normal finding (BLOCKING or ADVISORY) that names the
  decision entry, so the §9 fix agent reverses it.

### Output

Categorize every finding as one of:

- **BLOCKING** — must fix before the next phase starts (spec violation, data-loss risk, crash under normal use).
- **ADVISORY** — worth fixing but not blocking; next agent should address it when convenient.

Write **every** finding — BLOCKING *and* ADVISORY — to `## Review findings` in `HANDOFF.md`, each
tagged with its severity. This is the hand-off contract: the fix agent (§9) and the next build agent
can only act on findings that land in the handoff, so an advisory recorded nowhere else is lost.
Use the entry shape below.

Report to the human **via the brief** (§10), at digest granularity: severity counts, each BLOCKING
finding as one plain-language sentence (what breaks, for whom), and advisories summarized as a count
plus dominant theme — never an enumerated list. The full technical detail lives only in the handoff.
If there are no findings, the brief says so plainly and nothing is written to the handoff. Do not pad.

### Review-findings entry shape (written by §8, consumed by §9)

Each finding lives as one bullet under `## Review findings`, opening with its severity tag so the
fix agent can triage at a glance. Keep the same `file:line` + what's wrong + why it matters + a
fix hint as the spoken report:

```
- **BLOCKING — <one-line title>.** <where: file:line> <what's wrong> <why it matters under normal use> <fix hint + what test would prove it>
- **ADVISORY — <one-line title>.** <same shape>
```

The §9 fix agent **deletes** a bullet once it has resolved the finding (fixed + green) or dismissed it
as a validated false positive, recording the outcome in the changelog + its summary (§5) — the section
holds only open findings.

### Companion action: the usability review

This §8 review is a **static diff review** — it structurally cannot see what only running the app
reveals (fresh-install crashes, unstyled surfaces, real-CLI variance; all three escaped every diff
review once). The separate **behavior-driven usability review** — build the real binary, drive
every user journey on fresh/seeded/lived-in fixtures — has its own spec:
[`USABILITY-REVIEW.md`](USABILITY-REVIEW.md) (Claude Code: `/usability-review`). Its findings land
in the same `## Review findings` shape above, so §9 consumes them unchanged.

---

## 9. Fix action — validate, then fix the review findings

Triggered independently **after a review** (Claude Code: `/fix-review`; Codex: `"Fix the review
findings per AGENTS.md §9"`). Unlike §8 this is **not** read-only — it writes code, runs tests, and
commits, exactly like the build loop. Its input is the `## Review findings` section §8 just populated.

`$ARGUMENTS`, if present, scopes the run: a severity (`blocking` → only BLOCKING items) or a finding
title/keyword (just that one). Default with no args: **all** unresolved findings, BLOCKING first.

### The two-gate loop, per finding

Work findings one at a time, BLOCKING before ADVISORY. For each:

```
1. VALIDATE → confirm the finding is ACTUALLY TRUE before touching anything.
2. FIX      → only if validated real: implement to a GREEN checkpoint (§2) with a regression test.
3. RECORD   → rewrite the bullet's tag, condense (§5), commit at the checkpoint (§6).
```

**Gate 1 — VALIDATE (don't trust the review).** Reviews produce false positives. Read the cited
code at `file:line`, trace the actual path, and convince yourself the problem is real and would bite
under **normal use** (the §8 bar). Where practical, reproduce it first with a **failing test** — that
both proves the finding and becomes the regression guard once you fix it.
  - **Real** → proceed to fix.
  - **False positive / not reproducible / already fixed since the review** → make **no code change**.
    Your validation is the verdict — **delete the bullet** and record a one-line changelog entry
    (`dismissed <title> — false positive: <one-line evidence>`). Dismissals are judgment calls (§3):
    the brief (§10) carries the dismissal count, plus one plain-language line for any dismissal of a
    BLOCKING finding; they do not linger in the findings section.

**Gate 2 — FIX.** Same rules as building a subphase: implement the fix yourself (**no coding
§4**), add/keep the regression test, and reach a **GREEN checkpoint** (§2). A fix is only
done when green; never mark one resolved or commit on red. If a finding is real but you can't get it
green, or the right fix needs a human decision, that's a **STOP** (§3) — leave the bullet as-is,
record it under `## Blocked on human`, and move to the next finding.

**RECORD.** When green, **delete the finding's bullet** and add a one-line changelog entry
(`review fix: <title> — <fix + the test that covers it>`). Commit code + `HANDOFF.md` together on
`main` with message `review fix: <short title> — green checkpoint`. Then take the next finding.

### Carry over the same discipline

- **STOP conditions (§3)** and **autonomous-decision flagging (§3)** apply unchanged — if a fix forces
  a design call the finding didn't dictate, record it under `## Autonomous decisions` and say so.
- **Condensation (§5):** delete each finding's bullet as you resolve or dismiss it (changelog +
  summary carry the record); the findings section trends to empty, never a graveyard of stale tags.
- **End-of-session (§7 checklist):** tree green, handoff updated, work committed, and the human brief
  (§10) written: what was fixed in plain language, the dismissal count (with a line for any dismissed
  BLOCKING finding), and anything that became a blocker.

---

## 10. The human brief — how the human stays in the loop

The human running this project is **zoomed out**: they do not read `HANDOFF.md`, the diffs, the
changelog, or the specs, and they will not "go over the handoff" to catch up. The only channel they
reliably read is [`BRIEFS.md`](BRIEFS.md) — short, self-contained digests they can take in small
chunks. `HANDOFF.md` stays the agent-to-agent record; `BRIEFS.md` is the agent-to-human interface.

**Every session exit** — build (§1), review (§8), fix (§9), usability review — prepends one brief to
`BRIEFS.md` and pastes that same brief as the end-of-turn message. One text, two places; never write
a second, longer report in chat.

### Writing rules (this is the hard part — respect them)

- **Assume a cold reader.** The human forgot the architecture and the vocabulary since the last
  chunk. Re-anchor every brief: name the component you touched and remind them in one clause what
  it does ("the *nudger* — the background loop that pokes an idle agent when it has unread mail").
- **No walls of text.** Hard cap ~25 lines / ~250 words. Short sentences, plain language. No file
  paths, symbol names, test names, or flag lists without a gloss — those belong in the handoff and
  changelog for the next agent, not in the brief.
- **Decide, then report.** Never hand the human an open decision without a default. Every
  "needs your input" item states what happens if they never answer.

### Entry shape

```
## <date> — <build|review|fix|usability> — <one-line title>
**TL;DR:** ≤3 sentences — what changed / what was found, and whether the tree is green.
**Where this fits:** 2–4 sentences of architecture re-orientation (see writing rules).
**Decisions made for you:** 0–3 bullets, ≤2 sentences each — only calls that change user-visible
  behavior, are costly to reverse, or deviate from spec; what was chosen and why, in plain language.
  (Every other call is recorded in HANDOFF only.)
**Needs your input:** 0–2 bullets — a direct question, a recommended answer, and the default applied
  if you stay silent. Restate every open `Blocked on human` item here as one line until resolved.
**Next up:** 1 sentence.
```

Omit any section with nothing to say (except the TL;DR). Keep the last 4 briefs; delete older ones
when adding a new one — git keeps history.

### The zoomed-out decision contract

- The human no longer reviews every autonomous decision. **The §8 review agent is the reviewer of
  record**: it audits `## Autonomous decisions` entries against the specs, endorsing each
  (`— peer-reviewed <date>`) or converting it into a finding.
- The brief carries only above-the-bar decisions (shape above). **Silence is consent:** once an
  entry is peer-reviewed — and, if brief-worthy, has appeared in a brief — the next session folds
  or deletes it per §5. The human objects by replying to a brief; that objection is a new
  instruction to act on, not a standing review queue.
