# Project shared resources

**State:** Waiting to start
**Why:** The user asked for project-scoped shared resources that agents can create and read without
putting them in a repository or accidentally committing them.
**Relevant requirements:** FS-11.R1–R10, FS-11.A1–A5, TS-02.R3/R13, TS-03.R11/R12, TS-05.R5–R7/R11/R13, INV §2, INV §10, INV §11

## Outcome

Every project will have an AgentDeck-owned, owner-only shared-resource directory outside its
repository. New agents will receive it as an accessible directory, environment variable, and clear
instruction; Settings will show its path without making it configurable.

## Included work

Add the shared config helper and owner-only resource layout; create it during project creation and
lifecycle composition; inject it consistently into launch/resume/switch; expose its read-only path
through project responses and Settings; preserve it when a project definition is deleted; and add
the specified filesystem, lifecycle, server, and UI coverage.

## How we will know it works

FS-11.A1–A5: test creation/lazy creation and modes, fake-agent launch/resume/switch composition,
stable retention across project edits/deletion, invalid/unusable-path refusal with no launch, API/SSE
content non-exposure, and the Settings copyable read-only path.

## Waiting on

None.
