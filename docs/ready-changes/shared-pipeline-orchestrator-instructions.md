# Shared pipeline orchestrator instructions

**State:** Waiting to start
**Why:** Direct human request on 2026-10-08 to define every-stage naming/grouping guidance once;
the operator approved delivery through Claude system/Codex developer instructions, not ordinary
messages or repetition in stage assignments.
**Relevant requirements:** FS-14.R87–R89/A53–A55; TS-09.R57–R60; TS-11.R20;
existing TS-04.R69; INV §1–§3, §7–§8, §10–§11, §15–§17.

## Outcome

An operator writes shared orchestration conventions once in a reusable pipeline. Its standing
owner and freshly launched dedicated coordinators receive that run's frozen guidance in their
standing instructions, including after resume, native-session recovery, switch or replacement.

## Included work

- Optional bounded `orchestrator_instructions` across template editor, config/schema, API/CLI,
  Chucky draft/proposal/digest and frozen run snapshot; existing templates default to empty.
- One provenance-based resolver at ordinary fresh task launch; compose the labelled block into
  the frozen base prompt, preserving existing role/project/native prompt and runtime overlays.
- Preserve ordinary stage assignments, existing permissions/authority and session retention;
  allow explicit stage exceptions to the shared defaults. Update progressive pipeline knowledge.

No live editing, run-start override, interpolation, saved-role mutation, ordinary-message bootstrap,
automatic worker/Think Tank inheritance, naming/grouping enforcement, migration/reset or extra
instruction store. Existing-agent targets retain their own frozen prompts.

## How we will know it works

FS-14.A53 covers editor/save/start and serialized API/CLI/proposal preservation plus limits.
A54 captures actual Claude system addition and Codex developer parameters, verifies one block and
no copies in assignment messages or unrelated agents. A55 covers template edits, frozen recovery,
native-load fallback, runtime switch, replacement and retained-agent resume after run deletion.
Apply the TS-06 closure matrix after implementation; distinguish fake delivery from bounded
packaged provider adoption evidence. All behavior remains planned until implementation.

## Verified seams and integration

`TemplateSnapshot` already freezes version-2 configuration; the current `MaxInstructionRunes`
is 16,000. `task_dispatcher.go:startLaunchedTask` uses the ordinary `launchAgent`/base composition
path. Standing stage identity, coordinator binding and task lineage live in
`internal/state/pipeline_tasks.go`; lineage by itself includes workers and is not an eligibility
test. `ReadPipelineStageTaskByTask` selects only the standing task id, so dedicated children require
their binding/provenance lookup rather than that direct read. `NewSessionMeta` persists base
`SystemPrompt`; resume/switch consume that frozen value, while process suffixes are not persisted.
Existing Claude native-preset append and Codex developer-instruction merge need no new capability.

The in-progress Think Tank pipeline change touches the same template/editor/task boundaries.
Integrate with its current shapes and preserve its room-managed task exclusion; this is a separate
ready unit and does not enlarge or replace that active change.

## Waiting on

None. Approved scope and technical design are specified; implementation has not started.
