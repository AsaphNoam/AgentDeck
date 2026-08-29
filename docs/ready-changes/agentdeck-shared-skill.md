# Thin AgentDecker and shared AgentDeck skill

**State:** Waiting to start
**Why:** The user asked to keep AgentDecker as AgentDeck's resident operator while moving reusable,
release-matched product expertise into a skill any launched role can use.
**Relevant requirements:** FS-18.R1–R8/R10–R11, FS-18.A1–A5/A7–A8, FS-04.R44/A24,
TS-11.R1–R2/R4–R8/R10,
INV §1, INV §2, INV §4, INV §6, INV §8, INV §10, INV §15

## Outcome

AgentDecker has a thin purpose-and-stance prompt, and every AgentDeck-launched agent can discover or
directly read one concise `operating-agentdeck` skill with job-oriented references and current
cross-tool operating guidance.

## Included work

Embed and securely install the canonical skill package; deliver it through one launch/resume/switch
composition seam for chat, terminal, wake, and pipeline work; migrate only the exact historical
AgentDecker prompt; keep individual tool mechanics in their definitions; and degrade with a clear
warning, no migration, and no advertised skill overlay when secure installation cannot complete.
Keep overlay directories and prompt text runtime-only, and clean duplicated coordination mechanics
and the numeric mail budget out of the fresh PM and teammate seed prompts.

Excluded: UI or external API changes, new MCP tools or schemas, managed roles, personal/provider-home
skill writes, a product-knowledge service, and all development or `/release` workflow knowledge.

## How we will know it works

FS-18.A1–A5/A7–A8 and FS-04.A24: package/install and conditional lifecycle-composition regressions,
exact migration fixtures, unchanged tool contracts, three-reference progressive-disclosure checks,
warning-only install failure with no advertised overlay or migration, transient-overlay persistence
checks, fresh PM/teammate prompt cleanup, pinned Claude/Codex discovery, direct-path fallback, and an
AgentDecker requested-orchestration journey. Run the shared TS-06 verification suite.

## Waiting on

Nothing.
