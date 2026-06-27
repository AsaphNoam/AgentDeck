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
- **Commit at every checkpoint** on a branch (not `main`); don't push unless the human asked. Use your own `Co-Authored-By` trailer.
- **Only stop for real blockers** (workflow §3: unresolvable ambiguity, a checkpoint you can't fix, missing credentials, destructive actions). Write the blocker under `## Blocked on human`, then stop.
- **Flag every judgment call.** If an ambiguity or spec gap forced *you* to make a design/implementation decision (without stopping), record it under `## Autonomous decisions (please review)` **and** call it out explicitly in your end-of-turn summary — never let the human find a self-made decision only by reading the diff.
- **Do the work yourself** — don't hand the build to sub-agents that can't run the build/test commands.

## Project orientation

- [`MAP.md`](MAP.md) — index of all planning docs.
- [`docs/agent-dashboard-prd.md`](docs/agent-dashboard-prd.md) — master PRD.
- [`docs/phases/`](docs/phases/) — phase PRDs (`phase-N-*.md`) and tech specs (`tech/phase-N-*-techspec.md`).
- Build/test: `make build`, `make test`, `make dist`, or the raw `go`/`npm` commands above. Server binds `127.0.0.1` only.
