# AgentDeck ready changes

This directory holds changes that are fully described, approved to start, and waiting for an agent
to begin. The linked feature and technical specifications remain the source of truth for what to
build.

Each change has one short Markdown file with a descriptive name, such as
`improve-archive-search.md`. Create it only after the needed specifications and acceptance checks
are clear and implementation is wanted.

A change is either:

- **Waiting to start** — it is in this directory and is not in the handoff.
- **In progress** — an agent has started it. `HANDOFF.md` names the change and records the next step.
- **Paused** — it remains here with the decision or blocker needed to continue.
- **Finished or cancelled** — remove the file; the specifications and Git history record the result.

## Change-file template

```md
# <Plain-language change title>

**State:** Waiting to start | In progress | Paused
**Why:** <direct human request or link to the related idea>
**Relevant requirements:** FS-nn.Rk, TS-nn.Rk, INV §n

## Outcome
<What someone will be able to do or what problem will be solved.>

## Included work
<What is included and what is intentionally not included.>

## How we will know it works
<Linked acceptance criteria, tests, user journeys, or manual checks.>

## Waiting on
<Only a decision or dependency that prevents starting.>
```

Keep this file short. It points to the specifications; it does not repeat them or become a detailed
implementation plan. A large change may have a temporary plan in `docs/plans/` for sequencing.

## Changes waiting to start

- [Project shared resources](project-shared-resources.md) — waiting to start.
