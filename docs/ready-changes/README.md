# AgentDeck ready changes

This directory holds changes that are fully described, approved to start, and waiting for an agent
to begin. The linked feature and technical specifications remain the source of truth for what to
build.

Each change has one short Markdown file with a descriptive name, such as
`improve-archive-search.md`. Create it only after the needed specifications and acceptance checks
are clear and implementation is wanted.

A change is either:

- **Waiting to start** — it is in this directory and is not in the handoff.
- **In progress** — an agent has started it. `HANDOFF.md` names the unit and records its next step;
  other units may be in design, work, review, or fix at the same time.
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

- [`use-installed-provider-clis.md`](use-installed-provider-clis.md) — use installed Claude/Codex
  by default with a backend-level bundle choice, scoped runtime feedback, local model refresh and
  explicitly capped compatibility work.
- [`show-exact-context-and-runtime-metadata.md`](show-exact-context-and-runtime-metadata.md) — show
  exact used/total context tokens beside the percentage and keep model/effort visible when a scoped
  project agent card expands.
- [`open-and-annotate-any-local-file.md`](open-and-annotate-any-local-file.md) — let conversation
  links open any process-readable local text file and feed file selections into the existing
  annotation tray and assignment flow.
- [`share-creative-workspace-layout.md`](share-creative-workspace-layout.md) — finish Figma-directed
  composition, project tabs and status badges, then share the layout across all three palettes.

- [`simplify-agent-and-automation-setup.md`](simplify-agent-and-automation-setup.md) — reduce setup
  overhead for experienced operators across New Agent, native linking, Tasks and pipeline start.
- [`rename-product-to-chuck.md`](rename-product-to-chuck.md) — rename AgentDeck to Chuck
  across identity, install, agent-facing surfaces and UI, with one supervised data-preserving
  cutover and restarted sessions instead of a general migration mechanism.
- [`phone-desktop-flow-and-agent-management.md`](phone-desktop-flow-and-agent-management.md) —
  give the phone the desktop's dashboard → project → chat flow, New agent with runtime choice,
  project-page pipeline start, agent management, and tracked-file views; remove phone task UI.

## Paused changes

- [`migrate-internal-actions-from-mcp.md`](migrate-internal-actions-from-mcp.md) — replace only
  AgentDeck's internal MCP action delivery with the packaged direct-action command, preserving
  provider/user MCP support; waiting for a safe direct transport supported by packaged Codex/ACP.
