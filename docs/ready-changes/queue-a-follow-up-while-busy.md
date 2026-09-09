# Queue a follow-up, and steer the running turn

**State:** Paused
**Why:** Direct request on 2026-09-07 — "Add steer in the chat like you can do from the CLIs/apps for
codex and Claude at least." Designed 2026-09-09. The sibling entry from that prompt, streaming the
agent's thinking, stays under `Ideas being defined` in [`../ideas.md`](../ideas.md).
**Relevant requirements:** FS-03.R48, FS-03.R49, FS-03.R50, TS-01.R29, TS-03.R38, TS-04.R48, TS-04.R49, TS-08.R56,
INV §1, §2, §4, §5, §8, §12, §16

## Outcome

Two deliberate actions on a busy chat agent, matching the Codex app. **Send** queues: the message is
held, shown pending at the end of the transcript, and delivered as the next turn when the current one
ends; it can be replaced or withdrawn before it goes. **Steer** delivers immediately into the running
turn, so an agent heading the wrong way can be redirected without discarding its work as Cancel
would.

## Included work

One held message per agent as live runtime state, a separate runtime entry point that only the
person's chat prompt handler calls, release on turn end and on cancel, release-to-composer on stop or
crash, the prompt route returning `202` instead of `409` with a sent-or-held indicator, `DELETE` on
the same path to withdraw, and the pending tail affordance in the transcript.

Steer is offered only where the runtime advertises the capability and is absent — not disabled —
elsewhere; Send is always present. With the composer empty and a message held, Steer delivers the
held one immediately.

**Blocked on the adapter bump.** Steering is the `_session/steering` ACP extension, and the versions
AgentDeck pins predate it. See `bump-pinned-acp-adapters.md`. Because the design gates on the
advertised capability rather than a version, the queue half is buildable today and steering appears
when the pins move — but the unit is paused rather than split, so one control pair ships with one
coherent explanation.

Not included: **agent-initiated prompts still fail closed.** Mail wakes, task assignments, and
pipeline stage instructions keep receiving `ErrTurnInFlight`, because those callers use that refusal
to arbitrate — a queue there would let a run continue past the gate that paused it (TS-01.R29). Also
excluded: durability across a dashboard restart (the hold is live state; the release-to-composer path
covers the ordinary loss case), more than one queued message, and any use of a provider's native
queue.

## Design note: why AgentDeck holds it, not the adapter

Verified against the pinned binaries on 2026-09-07:

- `claude-agent-acp` 0.59.0 has a real FIFO `turnQueue` and would work.
- `codex-acp` 1.1.2 does not queue — it supersedes. A second `session/prompt` overwrites the single
  per-session active prompt and the displaced turn is interrupted. Pushing to Codex would kill the
  turn the person is waiting on.
- A pushed prompt cannot be withdrawn; the Claude adapter's orphan/zombie result accounting exists
  because a cancelled queued turn's message has already reached the SDK.
- Holding above the adapter gives OpenCode and OpenHands the same behavior without first verifying
  how their adapters treat a concurrent prompt — the posture BR-1 exists because AgentDeck did not
  take.

Those facts are about the **pinned** versions and are why the queue half is AgentDeck's own. They do
not apply to Steer: current `claude-agent-acp` 0.75.1 and `codex-acp` 1.10.0 both implement the
`_session/steering` extension, advertised at handshake as `initialize._meta.steering.supported`, and
the adapter itself injects into the live turn or starts a new turn when none is steerable. Steer uses
that and adds no AgentDeck-side fallback (TS-04.R49).

## How we will know it works

FS-03.A31 (accepted while busy, exactly one held, delivered once as the next turn, replaced rather
than stacked, absent from transcript and index until sent, and agent-initiated callers still refused)
FS-03.A32 (cancel sends it; stop with an empty composer returns the text; stop with typed text
discards the held message and leaves the typed text alone; restart clears it), and FS-03.A33 (Steer
injects without ending the turn; no Steer control where unadvertised; Steer promotes a held message;
a turn ending mid-call reports the new-turn outcome; a refusal keeps the composer text).

## Waiting on

Nothing.

## Open assumption to confirm during implementation

The operator specified stop behavior explicitly. Cancel behavior — that cancelling a turn *sends* the
held message rather than discarding it, on the reading that cancel means "stop that, do this instead"
— was carried over from the proposal without separate confirmation. If it should discard instead,
that is a one-line change to FS-03.R49 and half of FS-03.A32.
