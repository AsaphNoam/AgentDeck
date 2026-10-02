# Lean personas and shared AgentDeck operating context

**State:** In progress
**Why:** Direct `/design-feature` request, 2026-10-02, following persona-versus-skill research.
The user confirmed four shipped roles, coordination within AgentDecker, and preservation of
existing PM/Teammate configuration and references.
**Relevant requirements:** FS-04.R47/R50–R51/A30–A31, FS-18.R15–R17/A11–A13,
TS-11.R13/R15–R17, TS-04.R14/R54/R69, TS-06.R5/R9, INV §1–§3, §7, §10–§12, §17

## Outcome

Operators choose AgentDecker, Implementer, Reviewer or Researcher, each with a lean continuing
mandate that complements task skills. Researcher covers both internal feature/code investigation
and external documentation/web research. Every role, including a custom or empty-prompt role,
receives concise AgentDeck awareness with a verified operating-skill pointer. Claude chat retains
its native coding instructions while receiving AgentDeck additions.

## Included work

- Four seed prompts with the content contracts in FS-18.R16; AgentDecker owns requested
  coordination. Stop creating PM/Teammate, preserve existing files/defaults/task and pipeline
  references, and refresh exact historical prompts for retained roles only through the existing
  migration. Preserve customized prompts and frozen session personas.
- Extend the existing runtime-only knowledge overlay with shared standing guidance. Keep the
  skill directory/env/pointer conditional on installation; stable environment and authority
  guidance remains when installation fails. Detailed workflows stay in skills and references.
- Correct Claude new/load metadata to request the native preset with appended instructions.
  Preserve terminal/Codex delivery, provider-owned snapshot policy, permissions and conversation
  history. Update tests and active documentation tied to the six-role inventory or old prompt text.

No new role schema, model presets, tool permissions, coordinator role, UI redesign, direct-action
transport or product rename. Use the canonical resident-role/product spelling at implementation
time if the independent rename lands first; do not resurrect an obsolete identity or add an alias.

## How we will know it works

- FS-04.A30–A31: four-role fresh install, legacy-reference continuity, custom prompt preservation,
  exact retained-role migration and unchanged frozen sessions, including partial-failure/retry cases.
- FS-18.A11: seeded/custom/legacy role coverage across launch, wake/resume, switch and pipeline
  paths; one runtime-only context block, conditional skill pointer and no persistence contamination.
- FS-18.A12: bounded Claude/Codex role scenarios and follow-ups record evidence, limits and role
  boundaries. No automatic accuracy claim or word-count snapshot tests.
- FS-18.A13: pinned-provider new/load contract checks, unchanged terminal/Codex behavior, and
  credentialed fresh/resume receipts. Missing credentials remain an explicit manual gate.
- TS-11.R17 and TS-06.R5: focused config/CLI/server/runtime/terminal checks during implementation,
  followed by the applicable shared closure matrix after the final relevant edit.

## Design evidence

Luna research informed narrow scopes and real verification
([GitHub](https://docs.github.com/en/copilot/tutorials/cloud-agent/get-the-best-results)),
contextual reviews and evidence rather than personal-style objections
([Google](https://google.github.io/eng-practices/review/reviewer/looking-for.html)), and keeping one
owner of the combined outcome unless different instructions/tools/policy warrant a split
([OpenAI](https://developers.openai.com/api/docs/guides/agents/orchestration)). Researcher guidance
synthesizes primary-source attribution and explicit uncertainty; these texts have no claimed
benchmark advantage.

The pinned Claude ACP 0.75.1 `dist/acp-agent.js` accepts object `_meta.systemPrompt` and fixes its
type/preset while forwarding `append`; a string replaces the native preset. Agent SDK 0.3.257
`sdk.d.ts` documents the additive preset and snapshot behavior. No adapter limitation or new
dependency requires a workaround. In that pinned SDK, omitted snapshot configuration with an
append is documented to apply fresh; explicitly recorded prompts can persist until compaction or
a new session. Verify the actual bundled provider at implementation closure and do not generalize
this across versions/accounts. The [public SDK guidance](https://code.claude.com/docs/en/agent-sdk/modifying-system-prompts)
supports native preset plus additions. Source/contract evidence does not substitute for the
credentialed adoption checks in FS-18.A13.

## Waiting on

Nothing. Product scope and legacy-role preservation are confirmed. Implementation has not started;
the role scenarios and provider receipts are implementation acceptance work.
