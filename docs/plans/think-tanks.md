# Think Tanks — temporary implementation notes

Non-authoritative sequencing notes for `docs/ready-changes/think-tanks.md`; delete at closure.

## State (slice 1, done)

`internal/state/think_tanks.go` (schema v36, room/member/entry/input types, create, pause/resume/end,
settle transitions, `NextThinkTankOpportunity`, delete) and `think_tank_turns.go` (begin attempt in
the `before(turnID)` callback, stage, finalize/fail by agent+generation+turn id, input, judge
reserve/launched/retry, setup retry, startup recovery, paged reads with UTF-8 offsets and derived
read receipt). Activity rows table exists; its writer arrives with capture.

## Verified seams (2026-10-06 survey)

- Activation: `runtime.Registry.StartActivation(ctx, agentID, kind, before func(string) error)`
  (`runtime/registry.go`, `runtime/chat.go` `StartActivation`); kinds in
  `runtime/activation_kinds.go` (`activationKinds` map: Instruction/StatusDetail/LastTrace). Not idle
  → error; busy → `(false, nil)`. Server helper `runActivationTurn` and `wakeForActivation`
  (`server/task_dispatcher.go`) under `claimLifecycle/releaseLifecycle` (`server/resume.go`).
- Turn end: `ChatRuntime.finishTurn` emits `EvTurnEnd` with `TurnEndData{StopReason}`; server sink
  in `server/server.go` (`SetEventSink`) calls `dispatchTurnEnd(agentID, generation)`
  (`server/task_dispatcher.go`). No turn id on events: TS-14.R4 adds the executing turn id.
- Runtime emission: `emit/emitIn` (`runtime/chat.go`), `nextTurnIDLocked`, `settleAndReserveHeld`.
- MCP: `messaging.addTool`, `ToolNames`, `caller(req)` → `SessionIdentity{AgentID, Generation}`
  (`messaging/tools.go`); `jsonResult/errResult`; service setters like `SetTaskControl`; approvals
  auto-exempt every registered tool (`server/launch.go`).
- Launch: `launchAgent(ctx, launchRequest, launchOptions{AgentID, Generation})` (`server/launch.go`).
- REST: `routes.go` `routeTable`; `writeAPIError/apiError` (`server/apierror.go`); remote allowlist
  `server/remote_routes.go` (`remoteAllowed`, room routes stay out). SSE: `eventBus.Publish(typ,
  nil, data)`; task wire fixture `server/task_wire_fixture_test.go`.
- Files: `resolveFileReadPath(cwd, raw)`, `readLocalFileAfterClassificationWithOptions`
  (`server/fileread.go`).
- Eligibility: `wakeCandidate`, `projectArchiveGate`, `ReadStatus(...).State == "idle"`, chat only.
- Server start: `Start(ctx)` beside `startTaskDispatcher(sweepCtx)`.
