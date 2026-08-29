# Thin AgentDecker and shared AgentDeck skill

**State:** Waiting to start
**Why:** The user asked to keep AgentDecker as AgentDeck's resident operator while moving reusable,
release-matched product expertise into a skill any launched role can use.
**Relevant requirements:** FS-18.R1–R10, FS-18.A1–A7, FS-04.R44/A24, TS-11.R1–R9,
INV §1, INV §2, INV §4, INV §6, INV §8, INV §10, INV §11, INV §15

## Outcome

AgentDecker has a thin purpose-and-stance prompt, and every AgentDeck-launched agent can discover or
directly read one concise `operating-agentdeck` skill with job-oriented references and current
cross-tool operating guidance.

## Included work

Embed and securely install the canonical skill package; deliver it through one launch/resume/switch
composition seam for chat, terminal, wake, and pipeline work; migrate only the exact historical
AgentDecker prompt; keep individual tool mechanics in their definitions; and add the maintenance
classification a future `/release` workflow can consume.

Excluded: UI or external API changes, new MCP tools or schemas, managed roles, personal/provider-home
skill writes, a product-knowledge service, and the `/release` workflow itself.

## How we will know it works

FS-18.A1–A7 and FS-04.A24: package/install and lifecycle-composition regressions, exact migration
fixtures, unchanged tool contracts, progressive-disclosure checks, pinned Claude/Codex discovery,
direct-path fallback, and an AgentDecker requested-orchestration journey. Run the shared TS-06
verification suite.

## Waiting on

Nothing.
