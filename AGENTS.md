# AGENTS.md — AgentDeck

Guidance for any coding agent (Codex, Claude Code, etc.) working in this repo.

## If you're here to implement a phase

This project is built in **spaced, quota-limited sessions** (the human runs one agent at a time,
Claude and Codex taking turns). Do **not** improvise a process — follow the shared loop:

1. Read **[`docs/phases/AGENT-WORKFLOW.md`](docs/phases/AGENT-WORKFLOW.md)** — the canonical protocol.
2. Read **[`docs/phases/HANDOFF.md`](docs/phases/HANDOFF.md)** — the live state (where we are, what's next).
3. Execute the workflow's loop: build the next subphase → reach a GREEN checkpoint → update + condense
   the handoff → commit → repeat until the phase is done, you're blocked on the human, or your quota runs out.

### The non-negotiables (full detail in the workflow doc)

- **GREEN checkpoint = `go build ./...` + `go test ./...` pass** (`+ cd ui && npm run build` if you touched `ui/`). Never record a subphase done or commit on red.
- **Keep `HANDOFF.md` lean and current** — update after every change; collapse finished subphases/phases (workflow §5). It's the only thing the next agent has.
- **Commit at every checkpoint directly on `main`** (trunk-based: no per-phase branches, no PRs); don't push unless the human asked.
- **Only stop for real blockers** (workflow §3: unresolvable ambiguity, a checkpoint you can't fix, missing credentials, destructive actions). Write the blocker under `## Blocked on human`, then stop.
- **Record every judgment call; surface only the few that matter.** If an ambiguity or spec gap forced *you* to make a design/implementation decision (without stopping), record it in full under `## Autonomous decisions (please review)` — the review step peer-reviews these. Promote to the human brief only calls that change user-visible behavior, are costly to reverse, or deviate from spec (workflow §10).
- **End every session with a human brief.** The human is zoomed out and reads **only** `docs/phases/BRIEFS.md` — not the handoff, not the diff. Prepend a ≤250-word plain-language brief per workflow §10 (TL;DR, architecture re-orientation, decisions with applied defaults, what needs input, next up) and paste the same brief as your end-of-turn message. No walls of text, no unglossed jargon.
- **Delegate freely, but own the checkpoint** — subagents have full tool access and can build/test, so delegate self-contained coding work to them. The main thread must still run the full GREEN checkpoint before committing. Tier the quota: farm discovery and isolated coding to cheaper models, keep the premium thread for design, judgment, and integration (workflow §4).

## If you're here to review the last commit

Prompt: `"Review the last commit per AGENTS.md."`

Follow **[`docs/phases/AGENT-WORKFLOW.md`](docs/phases/AGENT-WORKFLOW.md) §8** exactly. In short:
find the diff (last GREEN-checkpoint commit(s) on `main` since the previous review — trunk-based, no
PRs), cross-reference the relevant phase PRD + tech spec, flag **BLOCKING** and **ADVISORY** issues
only (spec violations, dead code, bad practices, flagrant bugs that affect normal usage — not style
nits, not micro-optimizations). Also **peer-review the pending `## Autonomous decisions`** — you are
the reviewer of record now that the human is zoomed out: endorse each sound entry
(`— peer-reviewed <date>`) or convert it into a finding. Write **every** finding (both severities,
tagged) to `## Review findings` in `HANDOFF.md` — that's the contract the fix step reads. Report to
the human at digest granularity via the brief (workflow §10). No code changes, no commits.

## If you're here to fix the review findings

Prompt: `"Fix the review findings per AGENTS.md."`

Follow **[`docs/phases/AGENT-WORKFLOW.md`](docs/phases/AGENT-WORKFLOW.md) §9** exactly. In short: take
the findings in `## Review findings`, BLOCKING first, and for each **validate it's actually true**
(trace the cited `file:line`, reproduce with a failing test where practical). If real, **fix it** to a
GREEN checkpoint with a regression test; if it's a false positive, make no code change. Either way,
**delete the finding's bullet** and record the outcome in the changelog — the section keeps only OPEN
findings, no `RESOLVED`/`DISMISSED` graveyard (workflow §5). Commit code + handoff together on `main`
(`review fix: <title> — green checkpoint`). Close with the human brief (workflow §10): what was fixed
in plain language, the dismissal count, and any new blocker. This step **does** write code and commit.
(Claude Code: `/fix-review`.)

## Project orientation

- [`docs/phases/INVARIANTS.md`](docs/phases/INVARIANTS.md) — **the paid-for bug-class catalog.** Read the matching sections before touching a hot spot; its intro lists the hot spots and how the build/review/fix roles each use it.
- [`docs/phases/BRIEFS.md`](docs/phases/BRIEFS.md) — the human-facing digest (workflow §10); every session ends by prepending a brief here.
- [`MAP.md`](MAP.md) — index of all planning docs.
- [`docs/agent-dashboard-prd.md`](docs/agent-dashboard-prd.md) — master PRD.
- [`docs/phases/`](docs/phases/) — phase PRDs (`phase-N-*.md`) and tech specs (`tech/phase-N-*-techspec.md`).
- Build/test: `make build`, `make test`, `make dist`, or the raw `go`/`npm` commands above. Server binds `127.0.0.1` only.
