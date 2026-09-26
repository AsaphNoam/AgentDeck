# Simplify agent and automation setup

**State:** In progress
**Why:** User approval, 2026-09-25, of discovery suggestions 7–11 as one effort. The audience already
knows Claude Code/Codex; reduce AgentDeck-specific configuration overhead rather than teach agent basics.
**Relevant requirements:** FS-01.R37/A21, FS-04.R49/A29, FS-08.R35–R36/A12–A13,
FS-12.R50/A24, FS-14.R80/A47, FS-16.R39–R40/A25–A26, TS-08.R69–R72,
INV §2, §3, §8, §10, §13.

## Outcome

Experienced operators can launch agents and automation with configured defaults, inspect or change
runtime choices in one disclosure, and select work dependencies by name. Advanced options remain
available without dominating every form. This is one bounded frontend effort, not a new onboarding
system or a runtime/configuration-policy change.

## Included work

1. New Agent: role/project first, compact selected-runtime summary, one Options disclosure for the
   existing name and runtime controls, friendly labels with id-based disambiguation when needed.
2. Remove both unavailable detached-copy/import controls everywhere ConfigSourcePanel is used.
3. Optional Config: compact provider-specific connect/status surface with Details; skip the step for
   unsupported providers, including correct progress and resumed-wizard backend selection.
4. Tasks Create/Re-arm: named task/run selection, readable source-specific outcomes, explicit
   replacement semantics, honest paginated choices; raw ids, signals and context attachments remain
   Advanced. Keep the existing one-work-prerequisite-plus-signal authoring capacity.
5. Pipeline start: Setup → Review, compact runtime summary plus Customize runtimes, existing
   resolved assignments and exact proposal behavior, and visible validation/fast-mode selections.

Exclude beginner tutorials, navigation changes, product rename, worktree changes, runtime-switch
or history behavior, native model-inheritance changes, a context-reference browser, a general
dependency editor, new APIs, saved UI modes, new skins, and unrelated Settings redesign.

## Implementation seams and verified boundaries

- `NewAgentModal.tsx` already owns launch drafts and uses `lib/runtimeSelection.ts`; retain its
  existing request values/default precedence. Linked-native model inheritance is a separate idea.
- `SourceStep.tsx` embeds `ConfigSourcePanel.tsx`; reuse its existing connection implementation and
  mutation claims, including consent, rather than building another connector. Current wizard
  indexes/backend fallback need adjustment only to support the approved conditional step.
- Tasks already have `api/tasks.ts: useTasks(project)` and `api/pipelines.ts: usePipelineRuns()`.
  The latter pages global history in batches of 50; filter loaded pages by project, allow Load more,
  and never imply a partially loaded list is exhaustive. Context references have no browser listing
  route in `internal/server/routes.go`; their discovery belongs to agent-scoped MCP. Keep manual
  references instead of adding a workaround transport or discovery surface.
- `RunStartForm.tsx` already resolves owner/coordinator defaults and hydrates exact proposals.
  Disclosure must not regenerate proposals, replace choices on refetch, or bypass conflict review.
- Use existing presentation hooks/tokens and feature-owned state. Read the relevant FS/TS and
  invariants before implementation; no new architecture or provider-capability assumption is needed.

## Design direction

Frequent job: choose where/how work runs and start it. Put role/workspace or instruction first,
summarize selected runtime/waits next, and keep optional detail quiet but one interaction away.
The summary must reflect the actual launch/automation request, including explicit overrides and
fast mode; hiding controls must never hide a consequential choice or an error. Preserve dense,
keyboard-operable expert workflows. Use existing form/disclosure treatments in all three skins;
no motion or new visual identity. Implementation inspects incumbent rendered forms before changing
their composition and records the affected-state pass after the change.

## How we will know it works

- FS-01.A21: default/custom and fixed/global New Agent requests, capability gates and error recovery.
- FS-04.A29 / FS-08.A12–A13: compact native link/Details/retry, no unavailable actions, no unsupported
  Config step, correct resumed provider, and unchanged completion/consent semantics.
- FS-16.A25–A26: named/manual prerequisites, correct outcome vocabulary, duplicate names, page-two
  runs, loading/error states, full-set replacement copy and intact rejected drafts.
- FS-14.A47: defaults/custom runtimes and exact proposals through Setup → Review; fast mode, hidden
  field diagnostics and shared-workspace confirmation are represented accurately.
- FS-12.A24: affected real-product journeys at 1024px and wider in Core, Sky & Grove and Studio.
  Browser validation is owed by implementation; source inspection is not rendered acceptance.
- Run focused component checks while iterating, then workflow §2's applicable closure matrix once
  after the final relevant edit, including style checks and generated embedded UI as applicable.

## Waiting on

Nothing. User approved the five changes together.

## Implementation plan

1. Compact New Agent and provider-aware onboarding/config linking; verify request preservation,
   provider step selection, connection states and focused component coverage.
2. Add shared named/manual dependency selection to Tasks Create and Re-arm; verify pagination,
   outcome vocabulary, replacement semantics and rejected-draft retention.
3. Reshape pipeline start to Setup → Review with a shared runtime summary/disclosure; verify defaults,
   proposal hydration, diagnostics, fast mode and conflict/shared-workspace recovery.
4. Run the affected rendered journey in Core, Sky & Grove and Studio at the supported desktop floor
   and a wider viewport, then run the applicable UI and repository closure matrix.

**Design/UX direction:** Experienced operators see role/workspace or work instruction first, then a
truthful summary of the runtime or dependency request that will be submitted. Advanced controls stay
one labelled interaction away and retain their values. No motion or new visual system. Loading,
partial history, duplicate names, unsupported providers, errors, proposals, overrides and fast mode
remain visible and actionable; collapsed structure never hides a consequential warning or repair.
