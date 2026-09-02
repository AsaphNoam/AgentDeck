# Let a pipeline run without a person acting as its control plane

**State:** Waiting to start
**Why:** Direct request on 2026-09-02 — pipeline stage agents demand too many MCP action approvals,
which "defeats the purpose of automating work" — extended the same day with the four foldable
findings from the `806bcb7` bug investigation of a 23h24m run that completed `success` only because
a person supervised it throughout. Both describe one problem: the run made the human be its
orchestrator.
**Relevant requirements:** FS-03.R40–R44/A24–A27; FS-04.R46/A26; FS-06.R28–R29/A18–A19;
FS-14.R52–R56/A29–A32; TS-01.R27; TS-02.R28; TS-03.R34–R35; TS-04.R41–R44; TS-05.R20;
TS-08.R50–R52; TS-09.R29–R31; INV §1, §2, §5, §8, §9, §10, §11, §13, §14

## Outcome

A pipeline run advances on its own. AgentDeck's own fifteen actions stop asking for approval, an
approval that is genuinely needed waits for a person instead of failing after three minutes and says
on the run that it is waiting, a stage agent that is refused a retryable report is told to send it
again rather than to stop, a task aimed at a stopped pipeline agent names the real reason, and a
run's declared output is readable where the attempt that produced it is read.

## Included work

**Approvals.** AgentDeck's own actions execute without a human decision for every agent, whatever
`skip_permissions` was frozen at launch, recorded with the existing auto-approved transcript shape.
Identification is an exact match on a composed `mcp__<server>__<tool>` identity derived from the
registered MCP server, carried as a runtime overlay parameter rather than frozen launch config, and
it fails closed. The exemption answers with the adapter's single-use allow option, never its
always-allow option. Every permission outcome that withheld a tool is logged with agent id, tool
name, and decision.

**Waiting.** No approval deadline by default; a gated request holds until decided, cancelled, or the
agent stops, with an explicit deadline still available. A run whose stage agent holds an unanswered
approval carries an attention reason and joins FS-14.R29's needs-attention notification category —
derived on the existing server→manager fan-out, edge-triggered once, persisting nothing.

**Reporting.** The assignment and every refusal state the boundary as the result AgentDeck
*accepted*, not the call the agent made; a retryable refusal says the attempt still owes a result
and names what to change. One vocabulary produces the refusal wording and its FS-17 retry class.

**Coordination.** The per-turn messaging budget becomes a `config.json` value defaulting to 50. A
task or message refused because R22 holds a stopped pipeline agent out of the addressable set names
that condition and the resume that resolves it.

**Run page.** Pause actions state their consequence wherever both are offered and the disabled
Continue names its missing input; each timeline attempt renders its declared outputs; a finished
run's named values open by default.

**Not included.** No per-template, per-run, per-stage, or per-role autonomy setting. No change to
`skip_permissions` for any non-AgentDeck tool, and no provider-side permission rule written. No
Settings control for the budget. No general detector of a stage agent gone quiet for reasons other
than a held approval, and no change to how long a stopped agent stays unaddressable — both are open
decisions in the handoff. No work-unit, checkpoint, partial-success carry across Retry, workspace
isolation, durable stage handoff, or change to how a run's outcome is decided. No migration, no new
route, no new SSE event type, and no agent-facing tool, argument, or result change.

## How we will know it works

FS-03.A24, A25, A27; FS-04.A26; FS-06.A18, A19; FS-14.A29–A32 pass. A29 and A19 are verified by the
two committed skipped reproductions — `internal/pipeline/refused_report_retry_test.go` and
`internal/messaging/pipeline_agent_task_target_test.go` — which the implementation unskips rather
than rewrites. FS-03.A26 is a real-browser gate (journey J14): a stage reports its result with no
prompt shown, a file edit from the same agent still prompts, and leaving that prompt undecided for
longer than three minutes and then approving it continues the stage. The per-backend shape of the
adapter-supplied tool name is confirmed under the existing credentialed gate (FS-09.A7) before a
backend is claimed; an unconfirmed backend prompts. Shared specification, build, and test checks
pass, with focused `-race` coverage on the new permission→pipeline fan-out.

## Waiting on

Nothing. Two related product decisions remain open in `HANDOFF.md` and are deliberately outside this
change: what qualifies as a stage agent that can no longer advance, and how long a stopped agent
stays unaddressable after its stage.
