# Chat session configuration: model, effort, and fast mode

**State:** Waiting to start
**Why:** Direct request on 2026-09-07 — "Add fast mode for agents/tasks alongside model and effort",
then "sounds like effort could also be simplified like this". Designing the second half uncovered a
defect in Codex model and effort delivery, and the fix is the same seam, so the three land together.
The two sibling requests from the original prompt (streaming agent thinking, and steering a running
turn) stay under `Ideas being defined` in [`../ideas.md`](../ideas.md).
**Relevant requirements:** FS-01.R35, FS-03.R45, FS-03.R46, FS-03.R47, FS-09.R50–R58, FS-14.R59,
FS-16.R29, TS-01.R28, TS-02.R30, TS-03.R37, TS-04.R45, TS-04.R46, TS-04.R47, TS-08.R55, TS-09.R34,
TS-10.R24, INV §1, §2, §3, §8, §10, §11, §12, §14

## Outcome

Three things a person can observe:

1. **Fast mode exists.** Chosen at launch beside model and effort — New Agent, `POST /api/sessions`,
   a `--fast` CLI flag, task launch specifications, pipeline stage assignments — and toggled on a
   running chat agent from the chat header, taking effect on the next turn with no process restart.
2. **Changing effort no longer needs approval.** The chat header's effort select applies on
   selection instead of staging a change behind **Switch**. Backend and model still stage, because
   changing those genuinely does restart the provider process.
3. **Codex agents run the model you picked.** Today they do not — see below.

## Included work

One ordered post-session configuration step — **model, then effort, then fast mode** — applied after
session creation and resume, shared by launch, resume, switch, and the running-agent change. Around
it: the per-model `fast` catalog capability and its validation, Codex autosync filling it from the
model cache, the launch and task/pipeline fields, `POST /api/sessions/{id}/session-config`,
persisting the requested value separately from the applied one, and the header's two control groups
with their unavailable and not-running states.

The ordering is a correctness constraint, not tidiness: applying a model resets the session's effort
to that model's own level and can remove fast-mode capability outright, so anything applied before
the model is silently discarded. Confirmed live, not inferred.

Not included, each for a stated reason. **Cooldown honesty** — only the pinned Claude adapter
reports fast-mode state back mid-session; consuming that update needs an ACP update kind AgentDeck
maps nowhere plus a third display state. FS-03.R46 records it as a known limitation and
`docs/ideas.md` keeps it as a follow-on. **Claude terminal fast mode** — the interactive executable
has `--effort` and no fast-mode counterpart (CLI 2.1.238), so terminal agents have no capability.
**Narrowing switch-runtime** — it keeps accepting `effort` unchanged; removing a shipped request
field would be a compatibility break for no gain, and the header no longer uses that path.
**OpenCode and OpenHands model delivery** — left exactly as-is, because whether their pinned adapters
read the session-creation `model` parameter has not been checked and an unverified change is not a
fix.

## Defect being fixed

**Codex chat agents have been ignoring the selected model and effort.** The pinned ACP
`NewSessionRequest` schema declares only `cwd`, `additionalDirectories`, `mcpServers`, and `_meta` —
there is no `model` field — and `codex-acp` 1.1.2 reads none, taking the session's model and
reasoning effort from its own thread-start response instead. AgentDeck's `model[effort]` parameter
went nowhere, so every Codex chat agent ran the user's configured Codex default while the New Agent
picker, the backends validator, and the recorded session identity all reported the chosen values.
`claude-acp` was unaffected: it receives its model through `_meta`, the extensibility channel the
protocol does define.

FS-09.A15 passed throughout because it asserts the outbound parameter against `fakeacp`, not that a
real Codex honors it (INV §17). Fixing this changes which model existing Codex agents run, from
their next launch or resume onward.

**Verified live on 2026-09-07**, not only by code reading. Driving the pinned `codex-acp` over
stdio with `session/new` carrying `model: "gpt-5.4-mini[xhigh]"` returned a session reporting
`currentModelId: "gpt-5.6-luna[high]"` — the local default. Applying `model` then `reasoning_effort`
as configuration options afterwards produced `gpt-5.4-mini[xhigh]` correctly. The same run confirmed
the ordering constraint (effort stayed at the old level until set after the model) and the fast-mode
capability gate (the `fast-mode` option was advertised for the first model and disappeared after
switching to one without the speed tier).

## How we will know it works

FS-09.A23 (catalog validation, backend rejection, API shape, New Agent gating, Codex autosync),
FS-09.A24 (fast-mode application gated on advertisement, never failing the launch), FS-09.A25
(terminal rejection, switch leaves fast unchanged), FS-09.A26 (the ordered call sequence, asserted as
a sequence rather than a set), FS-09.A27 (Codex delivers model and effort post-session and the
recorded identity matches the selection; Claude byte-identical to today), FS-01.A19 (launch parity
across modal/API/CLI), FS-03.A28/A29 (fast toggle and its states), FS-03.A30 (effort applies without
Switch; backend and model still stage), FS-16.A19 (task creation and admission checks), FS-14.A34
(per-stage gating and applied-not-assigned reporting).

New `fakeacp` scenarios carry several of these and are part of the work: a session advertising the
fast-mode option, one withholding it, and a Codex-shaped one reporting a different default model
than the one requested — the last fails against today's delivery and passes after it.

## Waiting on

Nothing. All product and technical decisions are recorded in the requirements above.

## Verified provider evidence

Recorded so implementation does not re-derive it, and so a later adapter bump can re-check it:

- Option ids differ per adapter: `claude-agent-acp` 0.59.0 uses `model` / `effort` / `fast`;
  `codex-acp` 1.1.2 uses `model` / `reasoning_effort` / `fast-mode`.
- Both surface fast mode only when the current model supports it — Codex gates on
  `additional_speed_tiers` containing `fast`, Claude on a per-model support flag.
- AgentDeck sends `clientCapabilities: {}` (`internal/runtime/chat.go:291`, `:604`), so neither
  adapter advertises the boolean option type and both accept the `on`/`off` select fallback.
- Both return `configOptions` from `session/new` and `session/load`; AgentDeck discards it today
  (`internal/runtime/chat.go:304`).
- On an unadvertised option id, Claude throws `Unknown config option` and Codex silently ignores the
  call. Gating on the advertisement removes the disagreement.
- Effort applies live on both: Claude calls `query.applyFlagSettings({effortLevel})`, Codex mutates
  the session's model id, which its next turn reads. Both reject an unsupported level cleanly.
- `~/.codex/models_cache.json` already carries `additional_speed_tiers: ["fast"]`, so FS-09.R38's
  add-only autosync path covers Codex with no new file reading.
- The interactive `claude` CLI 2.1.238 has `--effort` and no fast-mode flag.
