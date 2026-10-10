# No persona by default and optional persona awareness

**State:** Waiting to start
**Why:** Direct `/design-feature` request on 2026-10-10: create agents without a persona by default.
Implementer must stop being the default, including on upgrade. The follow-up hides Default behind
**No persona**, moves persona selection to **Advanced**, and adds an on-by-default **Chuck aware
agent** checkbox only in persona configuration. The user explicitly rejected a per-agent creation
override as unnecessary promotion of an edge case.
**Relevant requirements:** FS-01.R46–R47/A29–A30, FS-04.R54–R57/A34–A36, FS-18.R15/R17/R20/A16,
TS-02.R3–R5/R43–R44, TS-03.R59, TS-08.R69/R120, TS-11.R15–R16/R21–R22,
INV §1, §2, §3, §7, §9, §10, §11, §17.

## Outcome

Ordinary creation shows **No persona** and launches without persona text. Persona choice lives in
Advanced; the internal Default is absent from ordinary persona management. Agents normally receive
Chuck operating guidance. A persona can deliberately opt out in its configuration; tools and
project/provider/task-specific instructions retain their contracts.

## Included work

- Seed internal `default` with empty text/inherited permissions and awareness on alongside the
  four existing roles. Hide its canonical empty definition; represent it as No persona in the UI.
- Choose it on fresh installs and upgrade saved Implementer defaults once. Preserve other
  preferences and existing role/session/explicit automation data. Later deliberate default changes
  survive restart. Preserve direct configuration ownership and shipped-role reseeding; customized
  same-id definitions are never silently hidden as No persona.
- Wire saved/default/explicit selection across desktop, phone and onboarding; query loading and
  refetch cannot overwrite choices. Move persona selection to Advanced with a truthful summary.
- Add the checkbox/help sentence to persona create/edit only. Preserve explicit false and default
  legacy omissions to on. No persona stays aware; creation has no awareness control.
- Freeze the resolved choice in the existing session snapshot/metadata path, preserve it through
  resume/wake/clone/switch and reindex, and gate only shared operating-context/skill-pointer prompts.
- Reuse explicit CLI/API role syntax and existing provider/prompt composition boundaries.

Exclude a persona-management redesign, immutable-role architecture, nullable role identity,
role-omitting CLI/API syntax, per-agent awareness overrides, live persona/awareness switching,
history purges, assignment remapping, capability/permission removal, skill-installation changes,
user-prompt rewriting and provider changes. This design task changes no product code.

## Verified seams and local design choices

`seedRoles`/`SeedIfAbsent` are the role authority and preserve occupied files; `DefaultConfig`
currently names Implementer. `resolvePreparedConfig` prepares startup before serving. Role prompt
refresh is a package-gated exact-digest pass, which cannot record a completed preference upgrade:
selecting Implementer later has the same value as a legacy home. Use TS-02.R43's one internal
completion file and existing atomic writer, without a migration registry/public schema field.

`NewAgentModal` and phone `ProjectScreen` already read the saved default but can choose a fallback
before config arrives; onboarding `LaunchStep` hardcodes Implementer. `RoleForm` has existing
prompt/permission fields but no awareness choice. Reuse these forms and typed role API schemas.
`composeLaunch` already skips empty prompt parts; `applyKnowledgeOverlay` independently adds
the shared operating context and verified skill prompt. Gate both under TS-11.R22, retaining
knowledge dirs/env and tools. No provider limitation motivates a workaround.

Session snapshots are typed SQLite data; transcript-only state cannot preserve resume behavior.
Use TS-02.R44's one session boolean plus additive metadata, not federation JSON or a public launch
flag. Clone already holds the source snapshot under its lifecycle claim; pass its choice through
internal launch options. An occupied customized Default stays preserved and triggers the existing
upgrade conflict/repair path before automatic selection; no text or permissions are overwritten.
Publish preference-upgrade completion last. These local choices remain subject to independent review.

## Design and UX direction

Experienced operators repeatedly launch work in a project. Lead with project and compact runtime/
persona summary; No persona requires no extra decision. One Advanced disclosure owns persona
selection and incumbent optional controls, preserving keyboard flow, selected values and repair.
The persona editor's checkbox is quiet beneath its prompt field, with **Adds Chuck operating
guidance to the agent's system instructions.** immediately underneath and accessibly associated.
Do not promote it with badges, onboarding explanations or a launch toggle. No persona denotes
empty persona text, not missing native/project instructions or reduced tool access.

Reuse existing form typography, checkbox/disclosure construction and tokens in all three
appearances. No motion or decorative treatment is needed. Loading, custom defaults, a customized
occupied default id, duplicate names and rejected saves/launches retain honest summaries and
entered values. Implementation inspects incumbent rendered forms, then records the focused real
desktop/phone keyboard journey; design-time source reasoning is not rendered acceptance.

## How we will know it works

- FS-01.A29–A30: desktop global/project, phone and onboarding No persona/custom selection; Advanced
  open/closed, honest summaries, no visible internal Default, retained drafts and lifecycle behavior.
- FS-04.A34–A36: independent five-role seed enumeration; once-only upgrade, preserved custom data,
  later Implementer preference, occupied role, actual failure/retry and deletion/reseeding;
  persona-only checkbox, accessible help, true/false/omitted API/form values and no launch override.
- FS-18.A16: both on/off prompt additions, legacy snapshot/metadata defaults and reindex; retained
  tools/permissions/knowledge/provider/project/task guidance and no live rewrite/history purge.
- Retain pinned native-prompt preservation checks; a fake peer is not model-behavior proof.
- Run focused implementation checks, then the applicable TS-06 closure matrix once, presentation
  checks/development matrix and actual launch/edit journeys in all appearances at desktop and
  phone widths. Design verification is documentation-only.

## Waiting on

No product decision. Final scope is persona-configuration-only awareness, confirmed on 2026-10-10.
Implementation has not started. Documentation checks have unrelated pre-existing failures recorded
in HANDOFF; focused checks for this change pass.
