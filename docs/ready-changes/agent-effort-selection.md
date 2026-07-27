# Choose an agent's effort level at launch

**State:** Waiting to start
**Why:** the human asked for model effort support "like in claude/codex"; promoted from the
`General agent effort selection` idea in `docs/ideas.md`
**Relevant requirements:** FS-09.R35–R42/A14–A16, FS-01.R30/A14, FS-08.R31/A8, FS-14.R31/A11,
TS-01.R12, TS-02.R18, TS-03.R19, TS-04.R18–R19, TS-07.R14, TS-09.R24, INV §1, §2, §4, §8, §11, §12

## Outcome

A person picks an effort level next to backend and model — in New Agent, on the `agentdeck` CLI, on
a running agent through switch runtime, and per stage when starting a pipeline run — and the agent
actually runs at that level. Today effort exists only as a federation override that nothing reads,
so choosing it changes nothing about how an agent runs.

## Included work

A model entry in `backends.json` gains optional `efforts` and `default_effort`. A model that
declares no levels has no effort capability and shows no control anywhere, which keeps OpenCode,
OpenHands, and every un-migrated catalog exactly as they are today. `autosync_models` fills the
levels for Codex from the model cache it already reads, add-only; Claude levels are hand-declared.

One resolver applies the precedence explicit → bound-source override → `default_effort` → omit, and
one `internal/config` check decides whether a level is valid for a model. Three adapter-declared
delivery mechanisms carry it: `model[effort]` in the ACP model id for Codex chat, a post-session
configuration option for Claude chat, and an argv flag for Claude terminal. The resolved level is
frozen in a new `sessions.effort` column and re-applied on resume and switch.

Intentionally excluded: any AgentDeck-invented effort vocabulary or cross-provider normalization;
per-turn effort switching mid-session; effort for OpenCode/OpenHands; template-level effort (it
stays a run-time assignment like backend and model); and any change to credential probing.

## How we will know it works

FS-09.A14 covers catalog validation, the API/UI surfacing, and autosync. FS-09.A15 covers the three
delivery mechanisms, the Claude post-session teardown under injected failure, the save-time rejection
for backends with no mechanism, and precedence. FS-01.A14 covers modal/API/CLI launch parity, resume,
clone, and switch. FS-08.A8 covers the federation override finally reaching launch. FS-14.A11 covers
per-stage assignment and all-or-nothing run-start validation. FS-09.A16 is the credentialed gate: only
real Claude and Codex CLIs prove a level is *honored*, not merely delivered.

Load-bearing invariants for the build: **INV §2** — the Codex model suffix must be composed by one
accessor used by both `sessionNewParams` and `sessionLoadParams`, the pair that has already drifted
twice on `model`. **INV §4** — the Claude post-session step runs before registration and returns an
error, so the existing generation-scoped `teardownAgentRegistration` is the only cleanup path.
**INV §1** — resume must re-apply effort or a resumed Claude agent silently reverts to its settings
default. **INV §12** — a provider-rejected level fails closed and is deliberately *not* retried bare,
departing from that class's usual pattern because retrying would substitute a level nobody chose.

## Waiting on

Nothing. All product and technical decisions are resolved.
