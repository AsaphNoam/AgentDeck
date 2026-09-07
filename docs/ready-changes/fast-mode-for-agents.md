# Offer provider fast mode alongside model and effort

**State:** Waiting to start
**Why:** Direct request on 2026-09-07 — "Add fast mode for agents/tasks alongside model and effort."
Designed the same day; the two sibling requests from that prompt (streaming agent thinking, and
steering a running turn) stay under `Ideas being defined` in [`../ideas.md`](../ideas.md) and are
not part of this change.
**Relevant requirements:** FS-01.R35, FS-03.R45, FS-03.R46, FS-09.R50–R56, FS-14.R59, FS-16.R29,
TS-01.R28, TS-02.R30, TS-03.R37, TS-04.R45, TS-04.R46, TS-08.R55, TS-09.R34, TS-10.R24,
INV §1, §2, §3, §8, §11, §12, §14

## Outcome

A person can run an agent in the provider's fast mode: choose it at launch beside model and effort,
and turn it on or off on a running chat agent from the chat header, where it takes effect on the
next turn without restarting the process or rebuilding the conversation. Tasks and pipeline stages
can request it for work that launches later. Every surface that reports an agent's runtime reports
the fast mode it actually ran at, so a request that could not be honored is never displayed as if it
had been.

## Included work

Included: the per-model `fast` catalog capability and its validation; Codex `autosync_models`
filling it from the model cache's `additional_speed_tiers`; the launch field on the New Agent modal,
`POST /api/sessions`, and a CLI `--fast` flag; the fast-mode field on task launch specifications and
pipeline stage assignments with their creation-time capability checks; the adapter-declared
post-session delivery for `claude-acp` and `codex-acp` chat; decoding the session's advertised
configuration options so the setting is sent only when the live session offers it; persisting the
requested value on tasks and pipeline attempts and the applied value on agents and sessions;
`POST /api/sessions/{id}/fast-mode`; and the chat-header toggle with its unavailable and
not-running states.

Not included, each for a stated reason. **Cooldown honesty** — only the pinned Claude adapter
reports fast-mode state back mid-session, and consuming that `config_option_update` needs an ACP
update kind AgentDeck maps nowhere plus a third display state; until then the toggle shows the
person's intent and a rate-limit suspension goes unreported, which FS-03.R46 records as a known
limitation and `docs/ideas.md` keeps as a follow-on. **Claude terminal fast mode** — the interactive
executable exposes no fast-mode flag (verified against CLI 2.1.238, which has `--effort` and no
counterpart), so terminal agents have no capability and a terminal launch requesting it is rejected.
**Fast mode in switch runtime** — deliberately excluded; that path stops and restarts the CLI, which
is disproportionate for a session setting the providers accept live. **A boolean config-option
client capability** — AgentDeck advertises none, so the string `on`/`off` form both pinned adapters
fall back to is the one form to send.

## How we will know it works

FS-09.A23 (catalog validation, backend rejection, API shape, New Agent gating, Codex autosync from
the speed tier), FS-09.A24 (exact outbound calls against `fakeacp` scenarios that do and do not
advertise the option, and that no case fails the launch), FS-09.A25 (terminal rejection, switch
leaves it unchanged), FS-01.A19 (modal/API/CLI launch parity, resume and clone carry it, headers
report it), FS-03.A28 (toggle applies without Switch, persists, same process and conversation,
failure returns to actual state), FS-03.A29 (no toggle without capability; unavailable reason shown;
static text when not running), FS-16.A19 (creation and admission rejection, and that an
unadvertised option still launches rather than failing the attempt), FS-14.A34 (per-stage gating,
all-or-nothing start validation, supervision reports applied not assigned).

Two checks need `fakeacp` scenarios that do not exist yet: a session advertising the fast-mode
option and one withholding it. They carry FS-09.A24, FS-16.A19, and FS-14.A34, so they are part of
the work rather than a follow-up.

## Waiting on

Nothing. All product and technical decisions are recorded in the requirements above.

## Verified provider evidence

Recorded so implementation does not re-derive it, and so a later adapter bump can re-check it:

- `claude-agent-acp` 0.59.0 exposes fast mode as session config option id `fast`; `codex-acp` 1.1.2
  uses `fast-mode`. Both surface it only when the session's current model supports it — Codex gates
  on `additional_speed_tiers` containing `fast`, Claude on a per-model support flag.
- AgentDeck sends `clientCapabilities: {}` (`internal/runtime/chat.go:291`, `:604`), so neither
  adapter advertises the boolean option type and both accept the `on`/`off` select fallback.
- Both return `configOptions` from `session/new` and `session/load`; AgentDeck discards the result
  today (`internal/runtime/chat.go:304`).
- The adapters disagree on an unadvertised id: Claude throws `Unknown config option`, Codex accepts
  the call and silently ignores it. Gating on the advertisement removes the disagreement.
- `~/.codex/models_cache.json` already carries `additional_speed_tiers: ["fast"]`, so FS-09.R38's
  add-only autosync path covers Codex with no new file reading.
- The interactive `claude` CLI 2.1.238 has `--effort` and no fast-mode flag.
