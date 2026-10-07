# Think Tank workspace implementation evidence — 2026-10-07

The interrupted composer/mention work was resumed on `main`, preserving the earlier slices.
Implementation begins at `28d6c92`; its last product-code slice is `9b5e8cb`. This receipt and the
rendered harnesses belong to the same review unit. Embedded UI was regenerated through Make.

## Verification

- Final `make test` passed both Go variants (`GOMAXPROCS=4`); Think Tank state/MCP/server race
  checks passed both with and without `sqlite_fts5`.
- Full UI suite passed 676 tests. After the final presentation edits, 49 focused composer/room
  tests and the production UI/style/contract build passed. `make dist`/final embedding and binary
  build passed. Spec lint, harness syntax and diff checks passed.
- The initial tagged Go run hit two pre-existing mail completion timing assertions:
  `TestCoalescedMailProducesOnePromptAndIsNeverReplayed` and
  `TestMailActivationStartsIdleAgentWithoutMutatingMailProvenance`. Both passed three focused
  reruns without changes, and the final full closure passed under lighter load.
- The isolated compiled fake-ACP app on port 4414 passed 60 real REST/MCP/browser checks:
  concurrent openings hidden until the barrier, configured-order publication, raised live ceilings,
  shared mention snapshots and keyboard selection/send, paused discussion, actual scroll movement
  with anchored input, speaker identity, ordinary/room composer parity, participant navigation,
  End/judge, exact result body and live/reloaded/Archive retention. Room views used 1024×768 and
  1600×1000 in Core, Sky & Grove and Studio. Report/screenshots:
  `/tmp/chuck-room-final-populated/`.
- Nine follow-up checks passed: annotation tray at 1024px in each appearance, selected-agent
  annotation mail and retained room audit entry, ended state, ordinary recipient chat, room deletion,
  unchanged judge result identity/body and truthful unavailable room UI.
  Report/screenshots: `/tmp/chuck-room-followup-check/`.
- Retained/deleted-source room fixtures and the development presentation matrix rendered in all
  appearances at 1024px/1440px. Evidence: `/tmp/chuck-room-resume-final/`.

The browser/MCP harness acts as the synthetic agents' explicit room-tool client while fake ACP
holds/completes provider turns. It proves host ownership and publication, not native provider
tool use. The bounded credentialed Claude/Codex smoke remains owed, including addressed input,
approvals, private Send/Steer, native resume and the end-only judge's retained result.

## Reproduce

After `make embed`, start `go run -tags sqlite_fts5 ./scripts/stress-fixture -port 4414
-scenario hold_turn -hold-file /tmp/chuck-room-hold`. It prints its isolated temporary home.
Run `ui/scripts/room-journey.mjs` with the base URL, output directory, hold path and that home,
in that order. Its report names the ended room. Run `ui/scripts/room-followup-check.mjs` with
base URL, that room id, output directory, fixture home and hold path; it deletes only the synthetic
test room after proving mail and retention. Both use the UI's installed Playwright/Chromium.
