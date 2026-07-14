# AgentDeck work workflow

This is the shared way Claude Code and Codex make changes, review work, and leave the repository
easy to resume. The feature and technical specifications say what the product must do; this document
says how agents work with them.

## 1. Start with the current state

1. Read [`HANDOFF.md`](HANDOFF.md) from top to bottom, then inspect `git status` and the diff. Treat
   a dirty tree as user or interrupted work; do not discard it.
2. If a change is in progress, read its change file and the relevant feature, technical, and invariant
   requirements before reading code. If no change is in progress, do not choose future work yourself.

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

Every implementation, review, fix, and usability-review session adds one short update to [`BRIEFS.md`](BRIEFS.md) and sends that exact text as the final response.

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

Also look for normal-use bugs: missing error handling at boundaries, realistic races, unsafe writes, dead code, and incomplete wiring. Ignore style preferences, speculative edge cases, and micro-optimizations.

Record each real finding in `## Review findings` in `HANDOFF.md` with its location, normal-use trigger, why it matters, relevant requirement ID when one exists, and a suggested test or fix. Start the bullet with either **Must fix** (a likely normal-use failure, data-loss risk, or requirement violation) or **Worth fixing** (useful but not urgent). Update the last reviewed commit only across a continuous range actually reviewed. Commit only the review-state files.

## 8. Fix review findings

Handle one finding at a time, starting with **Must fix** items.

1. Confirm the report is true by reading the code, the cited requirement, and the real path. Reproduce it with a failing test when practical.
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
