# Collapse, reject, and delete pending pipeline proposals

**State:** Waiting to start
**Why:** Direct request on 2026-08-31 after using the Pipelines Templates page: one 32-stage draft
renders its whole canonical payload and pushes the template library off the screen, and an offer the
person never wants can only age out under the retention bound.
**Relevant requirements:** FS-14.R49–R51, R57, A27–A28; TS-02.R29; TS-03.R36; TS-09.R26/R32;
INV §5, §8, §10, §11

## Outcome

Every pending proposal on the Pipelines approval surface is collapsed to a summary a person can scan,
expands to the exact payload an approval acts on, and carries an explicit **Reject** beside its review
action. A rejected offer moves to a **Declined** list that states when it was declined and offers
**Delete**. Declining refuses one offer rather than blocking its content: an agent that proposes the
same content again returns exactly one pending offer.

## Included work

The durable record gains `declined_at` and its three states with conditional claims (TS-02.R29); the
API gains decline and delete routes and a two-collection list with typed refusals (TS-03.R36); the
Pipelines surface gains the collapse, the two lists, and the actions (TS-09.R23/R32).

The Reject-versus-approval ordering is decided and must not be re-opened during implementation: the
durable mutation always wins, and a record an approval consumes leaves both lists as consumed even if
a Reject landed first (FS-14.R57). Do not route a proposal id through the template or start API, do
not add a pre-check against a decline to either approval path, and do not add a cross-store
transaction: the approval paths keep the shape TS-09.R26 gives them.

Not included: any agent-facing effect. Declining and deleting send no message, change no
already-returned tool result, and add no agent-facing surface. No per-content refusal list is kept,
and no confirmation dialog is added.

## How we will know it works

FS-14.A27 and A28 pass, including both orderings of the Reject-versus-approval race for both proposal
kinds against the durable store, concurrent Reject/Delete losers reporting the real state, the
leftover-offer failure mode, and the pending/declined × Save/Start collapse matrix at maximum record
size. Shared specification, build, and test checks pass, and journey J14 covers the surface.

## Waiting on

Nothing.
