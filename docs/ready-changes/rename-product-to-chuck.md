# Rename the product to Chuck

**State:** Waiting to start
**Why:** Direct request, 2026-10-02: rename AgentDeck to Chuck. Scope revised 2026-10-03–04:
the sole operator accepts a supervised cutover and restarted sessions instead of general backward
compatibility; preserve valuable data rather than silently starting empty.
**Relevant requirements:** FS-00.R19, FS-04.R52/A32, FS-10.R15/R25–R26/A7/A13–A14,
FS-13.R24/A15, FS-18.R18/A14, TS-02.R40–R41, TS-04.R78, TS-06.R24, TS-08.R58,
TS-11.R18, INV §2, §7, §10, §15

## Outcome

The product is Chuck everywhere a person, an agent, or a shell can observe it: the application
and its documentation, the `chuck` command, `$CHUCK_HOME`, the release artifacts and install
tree, the MCP server identity and its token header, the agent-facing operating skill and seeded
prompts, and every on-screen string. The seeded resident-operator role becomes **FirstMate**.
Chuck installs normally; the operator's existing data moves through a supervised offline cutover,
with history/custom configuration preserved and new processes started afterward. No automatic
home migration or old-process adoption ships.

## Included work

Included: the Go module path and `cmd/` directory; binary, shim, manifest, archive and install-tree
names; the `CHUCK_*` environment and ordinary Chuck home resolution; new `firstmate` role identity
and proposal authorization; MCP server name, token header and hook scripts; the `operating-chuck`
skill published through the existing whole-root replacement; seeded prompts, tool descriptions,
the backend-switch primer and rendered-context error
strings; UI branding, browser storage keys and the SharedWorker name; README and the non-archived
documentation set. Mobile remote control shipped first (2026-09-28), so this also renames its
surfaces: the tailnet node hostname (`remoteHostname`), the phone app's title, manifest, icon and
`agentdeck.*` localStorage keys, the **Ask AgentDecker** label and `agentdecker` lookup in
`ui/src/remote/NewWorkScreen.tsx`, the VAPID `sub` URL, and the Remote settings copy (FS-00.R19,
FS-18.R18). A renamed node hostname gives the phone a new address, so paired phones re-pair; state
that in the release notes.

Included cutover preparation: document and rehearse stopping dashboards/children/tmux sessions,
pausing automatic work, preserving drafts and a recoverable source snapshot, preparing the Chuck
home, adapting concrete role/path references, checking history/resources/Git worktrees, and
starting new sessions. Failures leave the source available for repair/recovery. The actual live
transfer and any data deletion are separate operational actions, not authorized by this design.

Excluded: automatic legacy-home detection/move/merge/refusal machinery; legacy-role conversion or
rename-specific prompt-digest migrations; old-prefix tmux discovery/adoption; special skill-retirement
retries; aliases, dual-name runtime and an in-place updater bridge. No unrelated API/schema/permission
change, history rewrite or general compatibility framework. Native-provider resume continuity is
not promised. GitHub repository rename remains an operational step alongside release.

Two small preservation rules remain: emit `[Chuck annotations]` while recognizing historical
`[AgentDeck annotations]` (FS-13.R24), and retain browser-local draft/annotation copy-forward on the
same origin (TS-08.R58). Neither implies a migration framework. Custom user text/history may retain
old names; branding assertions apply to current product-authored surfaces.

Existing seams checked: `internal/config/paths.go` already centralizes home resolution;
`internal/config/seed.go` owns ordinary seed behavior; `internal/runtime/terminal/tmux.go` owns the
session prefix. `internal/agentknowledge/package.go` already stages both provider views, verifies
them, and replaces the entire managed root, so separate old-skill cleanup would duplicate it.

## How we will know it works

- FS-10.A7 — fresh install yields `chuck --version`, nothing named `agentdeck` on PATH or in the
  install tree, and a launch environment with no `AGENTDECK_` prefix.
- FS-10.A13 — Chuck ignores legacy home inputs and leaves the source untouched.
- FS-10.A14 — one disposable-copy cutover rehearsal and documentation review prove retained data,
  references/worktrees, new launches and recoverability; no general migration failure matrix.
- FS-04.A32 — FirstMate seeds normally, authorization accepts the new id and rejects the old,
  and a prepared customized role survives startup.
- FS-13.A15 — a transcript holding the old annotation prefix still renders as an annotation card;
  no code path emits the old line.
- FS-18.A14 — ordinary publication exposes only the verified current skill; product-authored
  agent-facing constants say Chuck, without altering customized or historical text.
- TS-06.R24 — a CI assertion that no build, release, or packaging artifact still spells the old name.
- TS-04.R78 / TS-08.R58 — focused new-protocol/prefix and retained browser-copy checks.

## Waiting on

Nothing. The GitHub repository rename is an operational step to perform alongside the first Chuck
release; the design deliberately does not depend on GitHub redirecting REST API calls or
release-asset downloads, since GitHub documents redirects only for web links and git
clone/fetch/push.
