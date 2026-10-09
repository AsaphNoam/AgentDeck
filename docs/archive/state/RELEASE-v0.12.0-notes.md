# Chuck v0.12.0

v0.12.0 improves the Think Tank and pipeline workspace, shared orchestration, chat navigation,
and task recovery while keeping the existing provider and workspace model intact.

## Think Tank pipeline stages

- Run a pipeline stage in a fresh same-project room with fresh participants and a judge.
- Publish the accepted judge synthesis as the stage output and carry it into the next stage.
- Preserve stage lineage, pending acceptance intent, and retry state through manager restart.
- Recover Stop, room close/cancel, late context, and failed launch/resume paths safely, including
  bounded cleanup and retained holds.
- Keep shared-workspace consent and run-hero guidance readable, and collapse tasks by parent lineage.

## Shared orchestrator instructions

- Add optional standing instructions for the pipeline owner or dedicated coordinator.
- Freeze the instruction text into the run snapshot and compose one labelled block into the launched
  session prompt, including after recovery or replacement.
- Validate frozen template structure before reading optional guidance so malformed snapshots fail
  clearly while valid older snapshots remain compatible.

## Chat links, tables, and Commands removal

- Open web links in new tabs and provide Open in new tab and Copy link actions from the context menu.
- Render Markdown tables with dividers, roomier cells, and local scrolling across chat surfaces.
- Remove the Commands tab from desktop and phone agent views; `?tab=commands` opens Transcript while
  command tracking, API, and history remain available.

## Task collapse

- Collapse tasks by parent lineage across the workspace, with readable collapsed rows and run-hero
  text in each supported appearance.

## Clone source Copy names

- Name a clone `<source name> Copy`, shortening the source name to fit the 256-character limit.
- Keep the curated suggestion for an unnamed source.

## Lifecycle and recovery fixes

- Make stop settlement, room cancellation, generation-scoped launch teardown, and restart sweeps
  durable across injected failures and manager restarts.
- Refuse oversized later-room context before immutable success and allow same-turn judge correction.
- Bound provider prompt-failure diagnostics to agent, backend, operation, RPC code, recognized reason,
  and sorted data-key names without leaking prompt or raw provider data.

## Installation

Apple-silicon macOS releases are published in `AsaphNoam/AgentDeck` while the repository rename
is pending. Install with `CHUCK_REPO=AsaphNoam/AgentDeck`; update with
`chuck update --repo AsaphNoam/AgentDeck`.

Credentialed provider, packaged Think Tank, device, browser, notification, and cutover receipts
remain manual gates recorded in the live handoff.
