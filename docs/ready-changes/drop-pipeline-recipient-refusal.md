# Drop the stale pipeline-stage recipient refusal

**State:** Waiting to start
**Why:** `docs/ideas.md` entry "A pipeline agent stays unmailable forever after its run ends"
(operator, 2026-09-05). Designing it on 2026-09-28 found the exclusion already removed by
FS-14.R74 in `8d8ca6e` (2026-09-13); `TestStoppedPipelineAgentIsAddressableAndWoken` and
`TestStoppedPipelineAgentRemainsAddressable` prove stage agents are mailable and task targets
again. The stale specs were reconciled in the same design commit. What remains is the leftover
refusal wording.
**Relevant requirements:** FS-06.R37, FS-06.A26, TS-04.R67, TS-04.R26, INV §2

## Outcome

An agent addressing a stopped agent that once ran a pipeline stage gets either success or the
ordinary refusal. It is never told the agent is "held out while associated with pipeline stage"
or that Resume will fix a call Resume cannot fix.

## Included work

- Remove `pipelineRecipientRefusal` (`internal/messaging/tools.go`) and its two call sites in
  `send_message` and `create_task`. Today it fires only for a stopped stage agent with no resumable
  snapshot, whose Resume cannot succeed.
- Rewrite `internal/messaging/pipeline_agent_task_target_test.go` for FS-06.A26. Drop
  `TestArchivedProjectDoesNotOfferPipelineResume` (FS-06.A19 is retired) unless it is folded into
  the ordinary-refusal case.
- Remove the `pipeline association` subcase of
  `internal/server/activation_test.go::TestIneligibleMailActivationIsDiscarded` and its comment.
  The subcase now passes only because the wake succeeds and retires the opportunity. Fix the
  stale FS-06.A16/R22/R27 wording in the `TestRunningPipelineAgentStillActivatesForMail` comment
  and the `messaging_loops.go` eligibility comment.
- Not included: any change to the addressable set, wake gates, FS-14.R74 ownership, or the
  context-recipient resolver (FS-15.R17).

## How we will know it works

FS-06.A26 in `internal/messaging/pipeline_agent_task_target_test.go`; both `make test` variants
and `make check-specs`. On shipping, flip FS-06.R37/A26 and TS-04.R67 to shipped and return FS-06
to `Current` if nothing else there is planned.

## Waiting on

Nothing.
