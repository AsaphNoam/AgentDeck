# Rename the product to Deckhand

**State:** Waiting to start
**Why:** Direct request, 2026-09-12: rename AgentDeck to Deckhand.
**Relevant requirements:** FS-00.R16, FS-04.R48/A28, FS-10.R15–R19/A7–A9, FS-13.R24/A15,
FS-18.R14/A10, TS-02.R32–R33, TS-04.R52, TS-06.R24, TS-08.R58, TS-11.R14, INV §2, §5, §7, §10, §15

## Outcome

The product is Deckhand everywhere a person, an agent, or a shell can observe it: the application
and its documentation, the `deckhand` command, `$DECKHAND_HOME`, the release artifacts and install
tree, the MCP server identity and its token header, the agent-facing operating skill and seeded
prompts, and every on-screen string. The seeded resident-operator role becomes **FirstMate**. An
existing AgentDeck install moves over without losing agents, sessions, transcripts, configuration,
project resources, or worktrees.

## Included work

Included: the Go module path and `cmd/` directory; binary, shim, manifest, archive and install-tree
names; the `DECKHAND_*` environment; a one-time state-directory migration on first start; the
`agentdecker` → `firstmate` role rename; MCP server name, token header and hook scripts; the
`operating-deckhand` skill and retirement of the previously published `operating-agentdeck`
directory; seeded prompts, tool descriptions, the backend-switch primer and rendered-context error
strings; UI branding, browser storage keys and the SharedWorker name; README and the non-archived
documentation set.

Excluded: any behavior, API shape, schema, or permission change; an `agentdeck` alias or dual-name
runtime; an in-place `agentdeck update` path onto Deckhand; rewriting historical records under
`docs/archive/` and `LearningArtifacts/`. Renaming the GitHub repository is an operational step
outside the codebase.

Two compatibility paths are kept deliberately and are not cleanup debt: annotation blocks are
written as `[Deckhand annotations]` while both spellings are recognized forever, because transcripts
are append-only (FS-13.R24); and tmux session discovery accepts both prefixes so a detached
pre-rename session is adopted rather than orphaned (TS-04.R52).

## How we will know it works

- FS-10.A7 — fresh install yields `deckhand --version`, nothing named `agentdeck` on PATH or in the
  install tree, and a launch environment with no `AGENTDECK_` prefix.
- FS-10.A8 — a populated `~/.agentdeck` migrates to `~/.deckhand` with everything readable
  afterward; both-homes-present, running-dashboard and unreadable-source each refuse and leave both
  directories untouched.
- FS-10.A9 — release documentation states the one-time installer run, the untouched previous install
  tree, and the command to remove it.
- FS-04.A28 — a user-edited `agentdecker` role arrives as `firstmate` byte-for-byte, the
  pipeline-proposal gate accepts the new id and rejects the old, and an agent launched under the old
  role stays addressable.
- FS-13.A15 — a transcript holding the old annotation prefix still renders as an annotation card;
  no code path emits the old line.
- FS-18.A10 — `operating-deckhand` is published, the previously published `operating-agentdeck`
  directory is gone, and no agent-readable string contains `AgentDeck` or `AgentDecker`.
- TS-06.R24 — a CI assertion that no build, release, or packaging artifact still spells the old name.

## Waiting on

Nothing. The GitHub repository rename is an operational step to perform alongside the first Deckhand
release; the design deliberately does not depend on GitHub redirecting REST API calls or
release-asset downloads, since GitHub documents redirects only for web links and git
clone/fetch/push.
