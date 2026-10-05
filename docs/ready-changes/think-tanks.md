# Think Tank discussions between independent agents

**State:** Waiting to start
**Why:** Human `/design-feature` request, resumed 2026-10-05 from the **Think tanks** idea recorded
2026-10-04; feature scope and SQLite/explicit-tool boundary confirmed 2026-10-06.
**Relevant requirements:** FS-21.R1–R7/R9–R40, FS-02.R65, FS-03.R69–R70, FS-05.R39,
FS-13.R26–R27, FS-17.R21, FS-18.R19; TS-14.R1–R18, TS-01.R37, TS-02.R42, TS-03.R55,
TS-04.R84, TS-05.R25, TS-06.R33, TS-08.R87, TS-11.R19; INV §1–§11/§13–§17.

## Outcome

An operator starts a room beside New agent in a non-archived project, mixes new and existing
normal agents across eligible projects, and follows one durable attributed group discussion toward
a goal. Each agent keeps its own provider session and normal card for private follow-up. Incremental
room reads, per-agent ceilings, optional independent openings and departures bound deliberation
without requiring consensus; optional fresh judge synthesis runs only at the end.

## Included work

Implement the approved [FS-21](../specs/features/FS-21-think-tanks.md) behavior over
[TS-14](../specs/tech/TS-14-think-tank-control-plane.md): SQLite room authority, explicit MCP read/
submission, host-committed contributions and read checkpoints, ordinary guarded activations,
boundary input, pause/resume/End, source-owned retained activity, recovery, project/Archive discovery
and guarded deletion. Compose existing chat, annotations, files, commands and approval controls;
keep private activity scoped to the ordinary agent view. Update shared catalogs, fixtures and
progressive operator knowledge. No product code has been written by this design.

The operator repeatedly explores architecture/research and follows up with an individual agent.
Use goal/phase first, a readable chronological room and compact participants with normal card links.
Show queued input, current speaker/wait reason, remaining ceilings and the next applicable action.
Source labels distinguish workspaces and file/diff origins. Preserve existing style and motion;
rendered room validation is owed during implementation.

Verified seams supporting this design:

- `runtime.StartActivation` provides a guarded `before(turnID)` callback; `server.launchAgent`
  accepts reserved agent/generation identities. Existing held/Steer successor allocation precedes
  old terminal emission, so source attribution must capture the executing turn id, not read the
  mutable counter. No plain prompt dispatch workaround or ACP identity extension is needed.
- `messaging.ToolNames` owns approval exemptions; transcript folding/tool grouping, annotation
  helpers and the local process-readable UTF-8 file reader already exist. Extend these scoped
  seams, preserving the current same-machine trust boundary and remote allowlist.

Excluded: live membership/goal editing, autonomous room creation, background judging, export,
full-file snapshots, new phone room UI, provider bumps and the paused direct-action migration.

## How we will know it works

[FS-21.A1–A30](../specs/features/FS-21-think-tanks.md) cover independent histories, one floor,
committed delta reads, blind openings, allowance/departure/closing accounting, boundary inputs,
private Send/Steer, approval/failure/restart/lifecycle recovery, rich room features and retention.
Adjacent acceptance is FS-02.A47, FS-03.A50–A51, FS-05.A22, FS-13.A17–A18, FS-17.A12 and FS-18.A15.
TS-14 §4 and TS-06.R33 require independent provider-frame/wire/durable-row evidence, focused races
and normal final Go/UI closure. Render creation → discussion → annotation/private follow-up →
End/judge → retained Archive at 1024px and wider in every appearance. Record the bounded
credentialed Claude/Codex room-tool/native-resume smoke separately; fake ACP is not that receipt.

## Waiting on

None. Implementation and its rendered/credentialed acceptance have not started.
