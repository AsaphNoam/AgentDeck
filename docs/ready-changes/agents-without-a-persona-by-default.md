# Agents without a persona by default

**State:** Waiting to start
**Why:** Direct `/design-feature` request on 2026-10-10: create agents without a persona by default.
The operator suggested an empty Default persona and confirmed that Implementer must stop being
the default, including on upgrade; the remaining launch scope was approved.
**Relevant requirements:** FS-01.R46/A29, FS-04.R54–R55/A34–A35, FS-18.R15/R17,
TS-02.R3–R5/R43, TS-11.R15–R16/R21, INV §2, §3, §7, §10, §17.

## Outcome

Ordinary agent creation starts with an empty Default persona while retaining shared Chuck,
project and native provider guidance. Specialized personas remain available by explicit choice.

## Included work

- Seed ordinary `default` with empty text/inherited permissions alongside the four existing roles.
- Choose Default on fresh installs and upgrade saved Implementer defaults once. Preserve other
  preferences and all existing role/session/explicit automation data. Later deliberate default
  changes survive restart. Default remains ordinarily editable and is reseeded if deleted.
- Wire saved/default/explicit selection precedence across desktop, phone and onboarding; prevent
  async loading/refetch from selecting an unintended persona or overwriting an operator's choice.
- Reuse explicit CLI/API launch syntax (`default@project`, `role:"default"`) and existing prompt
  composition, frozen snapshots and runtime context delivery.

Exclude product code during design, a persona-management redesign, immutable roles, nullable role
identity, role-omitting CLI/API syntax, live persona switching, agent-history rewrites, automation
assignment remapping, prompt-content rewrites and provider changes.

## Verified seams and local design choices

`seedRoles`/`SeedIfAbsent` are the existing role authority and preserve occupied files;
`DefaultConfig` currently names Implementer. `resolvePreparedConfig` prepares startup before serving.
Role prompt refresh is a package-gated exact-digest pass and cannot record a completed preference
upgrade: selecting Implementer later produces the same value as a legacy home. Use TS-02.R43's one
internal completion file and existing atomic JSON writer, without a migration registry or public
schema field. Never register Default in the historical prompt digest table.

`NewAgentModal` and phone `ProjectScreen` already read the saved default but can select a fallback
before config arrives. Onboarding `LaunchStep` hardcodes Implementer. TS-11.R21 covers consistent
query-ready selection and explicit-choice preservation. `composeLaunch` already skips empty persona
text; `applyKnowledgeOverlay` adds shared guidance independently. No provider limitation motivates
a workaround and no new native-provider seam is needed.

Local choices for independent review: preserve an occupied customized Default definition and
report a startup conflict before switching to it, rather than overwrite text or change permission
policy. Use normal seeding/config write-error semantics; publish upgrade completion last. Default
is initially empty, with deliberate later edits retaining ordinary role semantics. No new visual
composition or motion is needed; reuse existing selection controls and friendly labels.

## How we will know it works

- FS-01.A29: desktop global/project, phone and onboarding Default/custom selection; composed
  context and permissions; normal stop/resume/clone/switch retain persona choice.
- FS-04.A34–A35: independent five-role seed enumeration; once-only upgrade, preserved custom
  defaults and data, later Implementer preference, occupied role, actual read/write failure
  injection, interruption/retry and deletion/reseeding.
- Retain existing pinned native-prompt preservation checks; a fake peer is not model-behavior proof.
- Run focused checks during implementation, then the applicable TS-06 closure matrix once and
  the desktop/phone launch journey. Design verification is documentation-only.

## Waiting on

No product decision. Documentation checks have unrelated pre-existing failures recorded in
HANDOFF; focused checks for this feature pass. Implementation has not started.
