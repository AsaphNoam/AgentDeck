---
name: fix-review
description: Take the findings a review left in HANDOFF.md, validate each is actually true (drop false positives), then fix the real ones to a GREEN checkpoint with regression tests. The post-review counterpart to /review-phase. Use when the user says "/fix-review", "fix the review findings", "address the review", "validate and fix the findings", or runs this right after a review.
---

# Fix the review findings (validate, then fix)

You are running **after a review**. The review agent (`/review-phase`) wrote its findings — BLOCKING
and ADVISORY — to `## Review findings` in [`docs/phases/HANDOFF.md`](../../../docs/phases/HANDOFF.md).
Your job is to **validate each finding is actually true, then fix the real ones**. Unlike the review
itself, this **writes code, runs tests, and commits** — it's the build loop scoped to review findings.

## The protocol is canonical — read it

**Read [`docs/phases/AGENT-WORKFLOW.md`](../../../docs/phases/AGENT-WORKFLOW.md) §9 (and §2–§6 it
leans on) and [`docs/phases/HANDOFF.md`](../../../docs/phases/HANDOFF.md) now, then follow §9's
two-gate loop exactly.** That doc is the single source of truth; everything below is just reminders.

`$ARGUMENTS`, if present, scopes the run: a severity (`blocking` → only BLOCKING items) or a finding
title/keyword (just that one). Otherwise work **all** unresolved findings, **BLOCKING first**.

## Non-negotiables (don't skip these even if you skim)

1. **Validate before you fix — don't trust the review.** Read the cited `file:line`, trace the real
   path, and confirm the issue would bite under *normal use*. Where practical, reproduce it with a
   **failing test first** — that proves the finding and becomes the regression guard once fixed.
2. **False positives get no code change — and get deleted.** If a finding doesn't reproduce (or was
   already fixed since the review), make no code change: **delete its bullet** and add a one-line
   changelog entry (`dismissed <title> — false positive: <evidence>`). Your validation is the verdict;
   **call it out in your summary** — a dismissal is a judgment call — but it does not linger in the handoff.
3. **Do the work yourself.** Do **not** spawn coding subagents — they have Bash denied here and can't
   build/test, so they can't reach a checkpoint. Read-only Explore research is fine — run it on a
   cheaper model and keep the premium main thread for judgment and code (workflow §4).
4. **Fix to GREEN only.** A fix is done when its checkpoint passes (`go build ./... && go test ./...`,
   `-tags sqlite_fts5` and `cd ui && npm run build` where relevant). Never mark a finding resolved or
   commit on red. Keep/add a regression test for every real fix.
5. **Record + commit per finding.** When green, **delete the finding's bullet** and add a one-line
   changelog entry (fix + the test that covers it) — the section holds only open findings (§5). Commit
   code + `HANDOFF.md` together **directly on `main`** (trunk-based: no branches, no PRs), message
   `review fix: <short title> — green checkpoint`, with a `Co-Authored-By:` trailer naming the model
   that did the work. Don't push unless the user asked.
6. **Stop only for real STOP conditions (§3).** A real finding you can't get green, or whose fix needs
   a human decision/info, is a STOP: leave the bullet as-is, record it under `## Blocked on human`,
   and move to the next finding rather than abandoning the run.
7. **Flag every judgment call (§3).** If a fix forced *you* to make a design/implementation decision
   the finding didn't dictate, record it under `## Autonomous decisions (please review)` and say so
   explicitly in your closing summary.
8. **Feed the catalog.** Most real findings are instances of a class in
   [`docs/phases/INVARIANTS.md`](../../../docs/phases/INVARIANTS.md) — name the class in the
   changelog line. If the root cause is a genuinely new class, or the fix produced a new canonical
   helper worth reusing, append it there; that file is how the next agent avoids reintroducing the bug.

## When you finish a session (any exit)

Leave the tree at a GREEN checkpoint, `HANDOFF.md` updated + condensed, work committed. Tell the user
what you **fixed**, what you **dismissed as a false positive (and why)**, and anything that became a
blocker — and **explicitly list any autonomous decisions** you made, or say plainly there were none.
