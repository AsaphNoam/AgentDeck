# Bump the pinned ACP adapters

**State:** Waiting to start
**Why:** Prerequisite for steering (`queue-a-follow-up-while-busy.md`), which needs the
`_session/steering` extension that both current adapters implement and both pinned versions predate.
Kept separate on 2026-09-09 at the operator's direction, because BR-1 and BR-2 both came from
adapter-version assumptions and a two-adapter version jump deserves its own verification rather than
riding along with a feature.
**Relevant requirements:** TS-04.R18, TS-04.R45–R49, TS-06 (release runtime), FS-09.R52, FS-09.R55,
FS-09.R57, FS-09.R58, INV §12, INV §17

## Outcome

`scripts/release/package.json` moves `@agentclientprotocol/claude-agent-acp` 0.59.0 → 0.75.1 and
`@agentclientprotocol/codex-acp` 1.1.2 → 1.10.0, with evidence that everything AgentDeck depends on
across those ranges still behaves as its requirements say. Two things follow: steering becomes
reachable, and Codex CLI resolution stops being contradictory.

## Why this also closes BR-2's structural half

The release runtime already pins `@openai/codex` directly at 0.153.4, but `codex-acp` 1.1.2 declares
`@openai/codex: ^0.144.0` — a range 0.153.4 cannot satisfy. The adapter therefore resolves its own
nested 0.144.x and runs that unless `CODEX_PATH` overrides it, which is exactly BR-2: a model
imported from a newer personal cache is selectable while the process handling the prompt predates it.
`codex-acp` 1.10.0 declares `^0.153.3`, so the direct pin and the adapter's own range agree and
dedupe instead of contradicting. Confirm during implementation that the assembled tree contains one
Codex, at the pinned version, and that the `CODEX_PATH` workaround is no longer load-bearing.

## Included work

The two version changes, a refreshed lockfile, and verification of the dependent surfaces below.
Adopting any newly available capability is **out of scope** — steering is its own unit, and
`session/fork`, provider management, async-task control, and goals are not designed. This change must
not alter AgentDeck behavior except where a fixed requirement demands it.

## Compatibility findings so far

Checked statically against the current packages on 2026-09-09. All the load-bearing surfaces the
shipped session-configuration work depends on are unchanged:

- **Protocol version 1** on both. No handshake change.
- **Config option ids stable.** Claude 0.75.1 keeps `model`, `effort`, `fast`, `mode`, `agent`
  (`effort` moved into `dist/session-config-ids.js` but the value is identical). Codex 1.10.0 keeps
  `model`, `reasoning_effort`, `fast-mode`, `mode`. FS-09.R57's ordered apply and R58's Codex
  delivery hold unchanged.
- **Claude still reads `_meta.claudeCode.options`**, so its model delivery channel is intact.
- **Codex still reads no model from the session request** — zero occurrences — so BR-1's post-session
  delivery remains correct rather than becoming redundant.
- **Steering** is present on both as `_session/steering`, advertised at
  `initialize._meta.steering.supported` (TS-04.R49).

## Still to verify before this is done

Static reading is not the oracle here; BR-1 exists because it was treated as one.

- `session/update` kinds AgentDeck maps, and any new kind it should ignore rather than mis-map.
- Permission request option kinds (`allow_once`/`allow_always`/`reject_once`/`reject_always`) and the
  Codex approval surfaces behind them.
- MCP HTTP server registration through the session parameter, on both.
- Codex `CODEX_HOME` isolation and the `CODEX_CONFIG` `developer_instructions` prompt overlay
  (FS-09.R43/R44, TS-04.R14) against the newer bundled Codex.
- Usage/context reporting (TS-04.R25) and available-commands replacement (TS-04.R24).
- Claude terminal flags, which are a different executable and unaffected in principle — confirm.
- A credentialed run of both adapters, not a `fakeacp` pass. The existing acceptance gates cover
  this; this change is the reason to run the Claude/Codex portions rather than defer them again.

## How we will know it works

The full Go and UI suites, `make dist`, and an assembled-tree check that the runtime contains one
Codex at the pinned version. Then the credentialed Claude and Codex chat, resume, MCP, model, effort,
and fast-mode checks from the open acceptance gates — the same surfaces FS-09.A24, A26, and A27
specify, run against the real adapters rather than `fakeacp`.

## Waiting on

Nothing to decide. Note that a credentialed provider run needs human authorization, and the operator
has previously chosen not to let that block roles; here it is the point of the change, so a bump
landed without it should say so plainly rather than claim verification.
