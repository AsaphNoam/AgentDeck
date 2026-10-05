# Refresh the provider bundle and adopt new adapter features

**State:** Waiting to start
**Why:** Direct request on 2026-10-05 to check the bundled Claude/Codex adapters, CLIs and ACP
against upstream, pick up useful features, and recheck the paused internal-actions-without-MCP gate.
**Relevant requirements:** FS-03.R67–R68/A48–A49; FS-09.R79/A48; FS-17 §6 transport recheck;
TS-04.R79–R83 and §5 transport watch; TS-06.R32; TS-08.R86; INV §2, §7, §8, §10

## Outcome

The bundle ships current adapters: Claude ACP 0.85.1 over Claude Agent SDK 0.3.286 or later, and
Codex ACP 2.1.1 with Codex 0.159.3. The internal MCP server runs on go-sdk 1.8.0. The integration
work behind that applies to both Installed and Bundle backends. People see Claude's advisory notices
as compact transcript rows. A Steer that moves a running tool to the background shows it continuing
as a background task. A model change refused by provider policy shows the provider's reason and
never pretends the requested model is active.

## Included work

- Version pins, lockfile, assembly, `install.sh`, manifests and fixtures (TS-06.R32). Codex 0.160.0
  is excluded because it is outside the adapter's `^0.159.1` range.
- Regenerate the Codex `promptRequired` steering patch for 2.1.1 (it is still required). Re-derive
  the AIR tool-call decoder and fake from Codex ACP 2.0's changed contract, withholding any AIR
  feature that cannot be mapped losslessly (TS-04.R79).
- Opt-in session notices end to end (TS-04.R80, TS-08.R86), the steer-backgrounded tool mapping
  (TS-04.R81), and truthful model-policy refusals (TS-04.R82).
- go-sdk 1.8.0 with the MCP protocol unchanged (TS-04.R83).

Not included: unstable ACP state/subagent updates, MCP-over-ACP, Codex's read-only preset and
`mcpStartupAwaitTimeoutMs`, either adapter's ACP v2 surface, MCP `2026-07-28`, and `outputSchema`.
The paused `migrate-internal-actions-from-mcp.md` stays paused: its gate was rechecked and remains
closed.

## How we will know it works

FS-03.A48–A49 and FS-09.A48 pass against the fake runtime and in component tests. TS-06.R32's
assembly proofs pass with the regenerated patch. The shared specification, Go (both variants),
UI and distributable checks pass. Credentialed Claude and Codex two-point smokes per TS-06.R31/R32
are recorded or explicitly owed.

## Waiting on

Nothing.

## Evidence (2026-10-05)

Upstream versions came from `npm view` and the Go module list. Findings came from diffing the
published adapter, SDK and schema files:

- Claude advertises MCP `{http, sse}`. Codex advertises `{acp:false, http:true, sse:false}`.
- Codex 2.1.1 still starts a detached turn on idle steer, and its `parseSessionSteerParams` still
  drops `_meta`.
- The 2.1.1 `package.json` declares `"@openai/codex": "^0.159.1"`.
- Claude 0.85.1 gates notices on `clientCapabilities.session.notices`. It backgrounds a foreground
  tool on steer (CLI 2.1.286) and marks it with `_meta.ai.async_tasks.backgrounded`. It enforces
  model allow/deny lists and honours a `PreModelSwitch` hook veto.
- ACP SDK 1.7.0 reshaped MCP-over-ACP to stateless `mcp/message`, which is still unstable.
- go-sdk 1.8.0 fixes session leaks and a close deadlock, and removes flags Chuck does not use.
