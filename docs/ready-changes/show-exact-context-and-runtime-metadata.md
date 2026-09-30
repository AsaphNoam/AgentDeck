# Show exact context and expanded-card runtime metadata

**State:** Waiting to start
**Why:** Direct operator request to place the actual used/total token counts beside context usage and
show agent model and reasoning effort on project-page cards; the existing collapsed card already
showed backend/model/effort, so the confirmed scope restores that metadata when the card expands.
**Relevant requirements:** FS-02.R62–R63, FS-03.R66, TS-02.R39, TS-03.R50, TS-04.R68, TS-08.R81,
INV §1, §2, §8–§11, §13, §16–§17

## Outcome

Operators see exact current and total context tokens beside the percentage whenever the runtime
reports both values, and retain model/effort visibility while supervising an expanded agent card.

## Included work

Preserve the runtime-reported context pair through durable state and the existing AgentState stream;
render it through the shared full and compact context meter; keep percentage-only producers truthful;
and reuse the existing backend/model/effort metadata in expanded scoped-project cards. Collapsed-card
context, aggregate project cards, token alerts, runtime controls, and inferred raw counts are excluded.

## How we will know it works

FS-02.A44–A45 and FS-03.A47 cover protocol mapping, persistence/lifecycle restoration, live and
fallback labels, component behavior, the three-appearance visual matrix, and focused browser checks.

## Waiting on

Nothing.
