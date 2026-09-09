# Queue a follow-up while the agent is working

**State:** Waiting to start
**Why:** Direct request on 2026-09-07 — "Add steer in the chat like you can do from the CLIs/apps for
codex and Claude at least." Designed 2026-09-09. The sibling entry from that prompt, streaming the
agent's thinking, stays under `Ideas being defined` in [`../ideas.md`](../ideas.md).
**Relevant requirements:** FS-03.R48, FS-03.R49, TS-01.R29, TS-03.R38, TS-04.R48, TS-08.R56,
INV §1, §2, §4, §5, §8, §12, §16

## Outcome

A person can type and send to a chat agent that is busy, instead of the composer refusing them. The
message is held, shown as pending at the end of the transcript, and delivered as the next turn when
the current one ends. They can replace or withdraw it before it goes.

## Included work

One held message per agent as live runtime state, a separate runtime entry point that only the
person's chat prompt handler calls, release on turn end and on cancel, release-to-composer on stop or
crash, the prompt route returning `202` instead of `409` with a sent-or-held indicator, `DELETE` on
the same path to withdraw, and the pending tail affordance in the transcript.

**What it deliberately does not promise.** The queued message runs *after* the current turn. It does
not redirect work in progress, cancel a tool call, or reach the model sooner. Both pinned adapters
process prompts as strictly sequential turns and neither exposes mid-turn injection, so a stronger
promise would be a lie. This matches what the Claude and Codex CLIs do with a message typed while
they work. Cancel remains the way to stop what is happening.

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

The Codex app-server does have `turn/steer` and `thread/queue`; `codex-acp` 1.1.2 wires up neither.
Adopting a real steering primitive later is a separate capability-gated requirement and a stronger
promise, not a drop-in replacement (TS-04.R48).

## How we will know it works

FS-03.A31 (accepted while busy, exactly one held, delivered once as the next turn, replaced rather
than stacked, absent from transcript and index until sent, and agent-initiated callers still refused)
and FS-03.A32 (cancel sends it; stop with an empty composer returns the text; stop with typed text
discards the held message and leaves the typed text alone; restart clears it).

## Waiting on

Nothing.

## Open assumption to confirm during implementation

The operator specified stop behavior explicitly. Cancel behavior — that cancelling a turn *sends* the
held message rather than discarding it, on the reading that cancel means "stop that, do this instead"
— was carried over from the proposal without separate confirmation. If it should discard instead,
that is a one-line change to FS-03.R49 and half of FS-03.A32.
