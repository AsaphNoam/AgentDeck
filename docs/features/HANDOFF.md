# Chuck — Implementation handoff

**Live agent state.** Settled state through 2026-10-10 is archived in
[`HANDOFF-through-2026-10-10`](../archive/state/HANDOFF-through-2026-10-10.md).
Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md).

## Current position

- **Release preparation:** `v0.13.0` approved for publication on 2026-10-10 after ROOM-WIDTH-01
  was fixed. User's “go ahead and release” waives pending reviews for this release and authorizes
  pushing all unpushed main commits and the tag. Independent reviews remain available below;
  waiver is not completed review. Both Go variants, go vet, FTS5 build proof, shell syntax and
  `make dist VERSION=0.13.0` passed. Operator package and component pins already match the range.
  README install/update examples now use the renamed repository. 758 UI tests and 41
  style/presentation checks passed.
- **Repository:** `AsaphNoam/Chuck`; branch `main`.
- **Review pending — desktop/phone group controls:** FS-02.R72–R76/A54–A57,
  FS-20.R45–R46/A17–A18, TS-03.R56–R58, TS-08.R118, TS-13.R24.
  Range `4a7899b` through `1ec40f4`. Implementation/browser evidence is in the archive.
- **Review pending — button redesign:** `0a4bfb9`, FS-12.R68/A38, TS-08.R119.
  Independent comparison owed; shared resources `button-design/PLAN.md`.
- **Review pending — Think Tank room page:** `815a57e`, FS-12.R64/A36, TS-08.R115.
  Independent comparison owed, including paused/held failure/pipeline, Files viewer and focus.
  Evidence: shared resources `think-tank-design/runs/r1/`.
- **Review pending — mobile details/composer:** `58b63d1`, FS-20.R32/R43, TS-08.R116.
  Inline selected-file preview, compact Manage and unboxed composer footer. Shared Send touch
  target preserved. Three-appearance evidence: `/tmp/chuck-release-phone-*.png`.
- **Design ready — No persona and optional persona awareness:**
  [`agents-without-a-persona-by-default.md`](../ready-changes/agents-without-a-persona-by-default.md)
  is Waiting to start. Final scope: hide canonical Default, show No persona, persona selection
  under Advanced; Chuck aware agent only in persona configuration, on by default with help text.
  User rejected per-agent launch overrides. No persona stays aware; existing agents freeze their
  resolved choice. References: FS-01.R46–R47/A29–A30, FS-04.R54–R57/A34–A36, FS-18.R20/A16,
  TS-02.R43–R44, TS-03.R59, TS-08.R120, TS-11.R21–R22. Only the two generic shared prompt
  additions are gated; tools/permissions/knowledge/project/provider/task guidance remain.
  No product code changed; this feature has not begun implementation.
- **Design ready — quota continuation:**
  [`continue-chats-after-quota-reset.md`](../ready-changes/continue-chats-after-quota-reset.md)
  is Waiting to start: FS-01.R40–R45/FS-04.R53, TS-02.R45/TS-03.R60,
  TS-04.R86–R88/TS-10.R39–R44. Global on by default, every chat agent, exact attributed notice,
  one-record-per-conversation retention and API replacement approved. Current Cancel clients move
  to an explicit target; no backwards API compatibility is required. Codex reset forwarding is
  absent and stays an ACP wishlist dependency per the 2026-10-10 instruction; unknown reset means
  indication/manual recovery, never a guessed schedule. No product code or active unit selected.
- **Design prepared/TBD — subscription quota views:** FS-09.R80–R81/A49–A50,
  TS-04.R89/TS-03.R61 prepare Claude and Codex 5-hour/Weekly views. Chat-header disclosure confirmed.
  Full passive paired snapshots/reads remain TBD for both providers: Claude only has partial live
  metadata; full `/usage` and Codex `/status` occupy the provider turn queue and return text.
  Verified gaps and source evidence are in [`ideas.md`](../ideas.md)'s design entry/ACP Wait-list.
  Live Claude percentage/timestamp units also need a verified producer/provider receipt; bare
  numeric declarations are not unit evidence. The full view is not a ready implementation unit.
  Do not implement a hidden prompt, command parser, login side client or new adapter patch.
- **Paused work:** `migrate-internal-actions-from-mcp.md` remains paused on its transport blocker.
- **Known flaky check:** `internal/server` `TestOrdinaryStageAgentStopPausesPipelineRun`
  intermittently returns 409 “a resume is already in progress” (pre-existing at `74c8e84`);
  synchronization remains separate work.

## Active change

None.

## Acceptance gates still owed

- **FS-14.A50 / FS-21.A31:** bounded packaged Claude/Codex Think Tank stage probe (start → fresh
  participants → judge → accepted output → next stage). Fake-provider E2E and the 318-check render
  pass are not live receipts. **FS-14.A49** restart/race matrix and **FS-21.A32** live Stop-cancel
  race remain planned.
- **FS-21 / TS-06.R33:** credentialed Claude/Codex Think Tank smoke covering room read/submit,
  addressed input, approval/denial, private Send/Steer, native resume, end-only judge, and retained
  exact result. Fake-provider journey and automated/race closure passed; live receipt is owed.
- **FS-10.R25/A14 / TS-02.R41:** supervised disposable-real-home cutover rehearsal with receipt;
  GitHub rename to `AsaphNoam/Chuck` remains owed.
- **FS-02.A46 / A27:** real-browser toast and macOS notification clicks; six-tab shared-stream,
  J14 real-browser, and Sky & Grove with Codex capabilities.
- **FS-14.A54:** packaged Claude/Codex probe that a standing owner/dedicated coordinator adopts
  shared orchestrator instructions; fake delivery capture is not a live receipt.
- **FS-18.A12/A13:** six manual role scenarios and role-free follow-ups on pinned Claude/Codex;
  credentialed fresh/resumed Claude native-preset adoption (pinned adapter 0.85.1 only proves
  the `_meta.systemPrompt` request, not live adoption).
- **FS-20.A1/A5/A6/A8:** real tailnet, Android, iPhone, and `pmset -g assertions`; iPhone Home
  Screen PNG touch icon is absent. The combined 390px dashboard/project/agent-management/Files/
  Commands-retired journey remains owed; fast-mode picker and Continue on approval pause remain
  outside the fake-provider coverage.
- **TS-06.R21:** credentialed Claude/Codex login/chat.
- **TS-06.R31 / FS-09.A40/A42/A46/A47/R78 / FS-10.A10–A12:** finite four-combination provider
  smoke (each provider with current Bundle and one current Installed CLI), plus Installed update →
  Refresh → model and missing Installed → Bundle → retry → Installed-with-overrides journeys.
  Claude Installed fresh launch passed 2026-10-05; the broader credentialed matrix is owed.
- **TS-06.R26/R32:** credentialed Codex 2.1.1 receipt for FS-03.A41/A42 and FS-01.A20.
- **TS-06.R32:** two-point Claude/Codex smoke on ACP 0.85.1/2.1.1 and Codex 0.159.3, one notice,
  Steer during a running command, and refused model switch where policy allows. Automated assembly,
  fake-runtime, release-CI assembly, and native-provider proofs passed; live receipts are owed.

## Decisions needing input

- Whether to standardize TS-03.R3–R4 mixed legacy error envelopes or TS-04.R3 provider model-ID
  ownership (compatibility change).
- Whether a pipeline stage paused after failed launch/resume continues withholding **Open agent**,
  matching restart recovery, or permits chat under a wider continuation contract.

## Design consistency notes

- Next presentation review: decide whether crisp asymmetric technical-surface radii are deliberate
  under FS-12.R52.
- When direct-action work resumes, align TS-01.R25/TS-03.R32 with TS-04.R40 and scope FS-17
  section 6's opening sentence to the planned boundary.
- Injected-steer lifetime edge case needs `/investigate-bug` before `/fix`; FilesTab still copies
  silently through bare `writeText`.

## Review findings

None.

## Blocked on human

The published `v0.11.0` release body remains empty; its notes are in
[`RELEASE-v0.11.0-notes.md`](../archive/state/RELEASE-v0.11.0-notes.md).
Editing that older release is separate publication work.

## Release record

See [`RELEASE-v0.13.0-notes.md`](../archive/state/RELEASE-v0.13.0-notes.md).
Publication verification is pending.
