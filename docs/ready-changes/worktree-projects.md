# Worktree projects

**State:** In progress
**Why:** Direct request on 2026-09-01 — make isolated parallel coding work essentially effortless
(the "First-class Git worktree workspaces" idea, defined and removed from `docs/ideas.md` in this
change).
**Relevant requirements:** FS-19.R1–R11/A1–A7, FS-04.R45/A25, FS-02.R60/A42, TS-12.R1–R10,
TS-01.R26, TS-02.R27, TS-03.R33, INV §2, INV §4, INV §5, INV §7, INV §12, INV §14, INV §15

## Outcome

Forking a repo-backed project creates, as one action, a new branch off the project's base branch, a
fresh AgentDeck-owned Git worktree, and a new project whose cwd is that checkout — bootstrapped by
the project's optional setup command. Isolation is between projects; sharing is membership in one.
Checkouts are disposable (recreated from the durable branch when missing) and deleted only by
explicit consent at project archive/delete, with uncommitted work disclosed first; external
checkouts are never deleted.

## Included work

The FS-19 behavior set: fork action and form, base-branch and setup-command project fields
(FS-04.R45), dashboard entry points and branch display (FS-02.R60), ownership record, recreation on
every start path through one shared helper (TS-01.R26, TS-12.R4), consented deletion inside the
existing archive claim, and the TS-12 §3 API surface with its migration (TS-02.R27, TS-03.R33).
Excluded: worktree creation from the unscoped New-project modal, `add_dirs` repo isolation,
merge/PR automation, cross-project diff UI, automatic project creation for tasks.

## How we will know it works

FS-19.A1–A7 (integration tests, the fork journey, and the manual archive-dialog gate), FS-04.A25,
FS-02.A42.

## Waiting on

Nothing.
