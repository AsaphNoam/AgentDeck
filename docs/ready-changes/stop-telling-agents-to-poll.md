# Stop telling agents to poll for work

**State:** Waiting to start
**Why:** Direct request on 2026-09-10 to "remove the AgentDeck requirements to provide updates every
60 seconds", believed to be a remnant of the pre-durable-task polling era. There is no 60-second
update requirement: the only one AgentDeck ever had was `FS-06.R10`'s stuck nudge in-flight marker,
already deleted in `648a9fc` (2026-08-21) when durable mail activation replaced the unread-polling
nudger, and the surviving 60-second constant (`FS-09.R19`, `FS-04.R24`) is the unrelated credential-
probe cache. The operator confirmed on 2026-09-10 to take the real remnant instead. Recorded in
`docs/ideas.md` under `Ideas being defined`, removed from there by this change.
**Relevant requirements:** FS-18.R12, FS-18.R13, FS-18.A9, FS-04.R47, FS-04.A27, TS-11.R13,
INV §2, INV §8, INV §10, INV §17

## Outcome

Four seeded role prompts stop instructing agents to look for work on their own, and existing
installs get the correction rather than only fresh ones. Today `internal/config/seed.go` ships
`teammate` with a work loop that opens *"Start each turn by checking current AgentDeck coordination"*
— per-turn unread polling that `FS-06.R24` and `FS-16.R6` removed from the product — and ships
`implementer`, `reviewer`, and `researcher` each ending with *"If you are woken with no new
instruction, check your AgentDeck mail (`check_messages`)."* That case cannot occur: every host-owned
turn carries its own instruction naming the tool it needs
(`internal/runtime/activation_kinds.go:27`). The text costs tokens on every turn and teaches a habit
the product no longer wants.

## Included work

- **The prompts.** `teammate` keeps its assignment-queue stance without the per-turn coordination
  check; `implementer`, `reviewer`, and `researcher` drop their trailing mail-check bullet and keep
  every other bullet. `agentdecker` (FS-18.R2) and `pm` are already clean and do not change.
  (FS-18.R12)
- **The migration.** `Store.MigrateLegacyAgentDecker` becomes one role-agnostic pass over a
  code-owned table mapping each seeded role id to digests of prompts AgentDeck previously shipped for
  that id, with replacement text read from `seedRoles()` so the current prompt keeps one authority
  (INV §2, INV §10). Per-role read/decode/write failure warns and continues rather than aborting the
  pass or startup (INV §8); `cli.prepareAgentKnowledge` keeps its single warn-and-continue call site.
  A digest list per role covers installs several releases behind. (FS-04.R47, TS-11.R13, FS-18.R13)
- **Not included.** No tool, argument, result, authorization, or lifecycle change — agents still call
  `check_messages` and `get_assigned_task` because the activation tells them to. No managed-role
  architecture, no recurring seed synchronization, no rewriting of a prompt the user edited by even
  one byte, and no validation or warning about polling text in user-authored role prompts
  (FS-18 §6). No UI and no REST change.

## How we will know it works

- FS-18.A9 — seed-content assertions that the four corrected prompts carry none of the three banned
  instructions and are byte-equal to the shipped constants, plus per-role migration fixtures: exact
  previous prompt migrates with all other fields preserved byte-for-byte; one-byte edit, empty or
  custom prompt, non-seeded role, missing role, undecodable file, and read/write I/O error each leave
  that role unchanged; one role's failure still migrates the rest and does not fail startup;
  unavailable package leaves all four unchanged and a later verified start migrates once; re-running
  is idempotent.
- FS-04.A27 — the replacement runs for every seeded role, a digest belonging to a different role
  never matches, and absent-only seeding (FS-04.R14, A7) still never clobbers a populated home.
- TS-11.R13's drift guard — a test recomputes every table digest from the shipped constants and fails
  if an entry names a role absent from `seedRoles()` or matches that role's current prompt
  (INV §10, INV §17).
- Both Go test variants per TS-06; no `-race` hot spot is added by this change.

## Waiting on

Nothing.
