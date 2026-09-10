# Keep steering inside AgentDeck's turn lifecycle

**State:** Waiting to start
**Why:** Follow-up to the queue-and-steer change. The idle race is now understood: when the active
turn settles before the adapter handles Steer, the adapter must return a no-consumption
`promptRequired` result so AgentDeck can own the replacement turn.
**Relevant requirements:** FS-03.R50/R56/A33/A38, TS-01.R29/R30, TS-03.R39/R41,
TS-04.R49/R51, INV §2, §5, §11, §12, §17

## Outcome

Steer never leaves a provider turn outside AgentDeck's lifecycle. An idle fallback is resubmitted as
one ordinary host-owned prompt, with correct busy state, Send arbitration, cancellation, transcript
ownership, and exactly one terminal outcome.

## Included work

Implement the `promptRequired` fallback for the Claude adapter's opt-in contract, obtain or maintain
the equivalent Codex adapter contract, and route the unchanged text through the normal prompt seam.
Extend the fake ACP scenarios and focused runtime/UI tests to cover the completion race, active
injection, Send holding, Cancel, refusal, and exactly-once transcript delivery. Do not retry
`startedNewTurn`, and do not change the separate behavior of an active `injected` steer.

## How we will know it works

FS-03.A38 passes against a fake peer whose original prompt settles before Steer is handled; the full
steering acceptance coverage still proves FS-03.A33; `make check-specs`, the focused Go/UI tests, and
the final closure matrix pass. Real Claude and Codex steering remains an acceptance gate until
credentialed runs exercise both adapter contracts.

## Waiting on

No product decision. The Codex adapter must expose the same no-consumption idle fallback through a
compatible release or an explicitly maintained patch before the cross-backend implementation is
complete.
