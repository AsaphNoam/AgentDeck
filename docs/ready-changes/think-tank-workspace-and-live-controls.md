# Think Tank workspace and live controls

**State:** Finished
**Why:** Direct human request, 2026-10-06, confirmed with explicit normal Chuck composer parity.
**Relevant requirements:** FS-21.R43–R51/A33–A37; FS-02.R71/A53; FS-03.R71–R72/A52–A53;
TS-14.R22–R28; TS-08.R96–R99; INV §1–§2, §5, §7–§11, §13–§17.

## Outcome

Operators can find named Think Tanks, scan who is contributing and how much allowance remains,
navigate familiar participant chats, address shared input to specific agents and extend useful
discussion. Independent openings overlap, judge synthesis is readable in its agent chat, and new
room deployments are grouped under the title in their own projects.

## Included work

- Separate short title and legacy fallback; one wide room card per row before project agents,
  complete roster, current speaker(s), allowances, judge and attention/completion state.
- Compact participant/judge headers and Think Tank tab with room/other-chat links, including idle,
  ended, multiple and removed-source membership states. Private Send/Steer keep their meanings.
- Anchored room composer matching standard Chuck chat input, including shared textarea/action
  styling and Enter/Shift+Enter/picker behavior; selected `@agent` mentions remain shared and
  non-interrupting, explicitly addressed on the next scheduled room turn. Keep file/command access.
- Stable participant speech-bubble tints with matching roster cues and readable technical content.
- Concurrent isolated openings, guarded partial publication/retry/Pause/End/restart, and finite
  live per-participant ceiling increases while openings/discussion remain open.
- Exact synthesis as an immutable agent-owned result read projection, committed with the room
  entry, readable live/reloaded/archived and retained through room deletion. No fake provider turn.
- Title group assigned through ordinary new-agent/judge launch; preserve existing participants'
  groups, manual regrouping and agents/groups after room completion/deletion.

Excluded: live goal/title/membership editing, reopening closing/ended/judging rooms, decreasing or
unlimited budgets, mention-driven private sends or speaker reordering, retrospective regrouping,
new phone room UI, provider/runtime changes, a new appearance or decorative motion.

## How we will know it works

FS-21.A33–A37 cover navigation/composition, shared mentions, real opening overlap/isolation and
recovery, ceiling/end races, grouping and exact retained judge results. FS-02.A53 and FS-03.A52–A53
link the adjacent surfaces. Use TS-14.R28's focused state/server/runtime race and wire/UI checks,
then the applicable TS-06 closure once after final edits. Render the built product with isolated
fake providers at 1024px/wide in Core, Sky & Grove and Studio; compare ordinary and room composer
inputs side by side. Existing bounded credentialed room-tool/approval/resume probes remain separate
acceptance gates, not proof supplied by fake providers or design-time source inspection.

## Design direction

Experienced operators repeatedly scan/resume deliberation and occasionally add directed input or
more allowance. Make title, active speaker(s), attention and remaining ceilings legible on arrival.
Keep room cards wide enough for the full roster; preserve agent-grid density independently. Reading
order is compact identity/state, conversation, anchored familiar input; goals, membership navigation
and secondary inspection stay available without dominating the first message. Tints communicate
speaker identity; labels carry state and phase. Use Chuck tokens/geometry and no new motion. Cover
empty/loading/errors, opening/private waits, approval, pause/failure, End/judge and retained results.

## Existing design work and verified seams

- The shared auto-growing textarea and Send icon have shipped (FS-02.R66–R67, TS-08.R88–R89):
  `AutoGrowTextarea` in `ui/src/components/ui` and `SendIcon` in `ui/src/components/ui/icons.tsx`.
  The room composer already uses `AutoGrowTextarea` with `maxHeight="40vh"`; reuse both seams.
- Shared layout is already shipped (FS-12.R52–R59, TS-08.R74–R79); inherit normal Composer
  construction, bounded reading measure, semantic tokens/hooks and all-three-appearance behavior.
- `pipeline-think-tank-stages-and-task-collapse.md` is an independent waiting unit. These controls
  apply to its rooms through TS-14.R20's stage/run guards; neither unit selects/implements the other.
  Coordinate shared specification edits and keep both closures truthful, regardless of work order.
- The separate **Quiet completed chat turns and readable live thoughts** idea is still being
  defined. This unit introduces no transcript-collapse policy; when that design settles, keep the
  submitted judge result readable as a result rather than treating it as intermediate tool activity.
- Current `RoomList`/`RoomTurnNotice`/`ThinkTankPage` are the UI entry seams; the room composer
  already follows discussion in markup but lacks anchored standard-input construction/keyboard parity.
- `idx_think_tank_one_running`, `BeginThinkTankAttempt` and singular detail settlement currently
  serialize openings. Extend those guards/barrier and the bounded dispatcher; irreversible phase
  and fixed membership eliminate the need for an opening epoch or second execution mechanism.
- Successful judge finalization already owns the exact submitted text in one SQLite transaction.
  An agent-owned snapshot there avoids a second provider-transcript writer/cross-store outbox.
  `work_results` and pipeline report rows have different ownership/body contracts and do not fit.
  Match the existing `/api/sessions/{id}` history route family; source completion identity comes
  from `finishThinkTankTurn`, not current agent counters. Room deletion must not cascade the snapshot.

## Waiting on

Implementation and automated/fake-provider closure finished 2026-10-07. Evidence:
[`implementation-think-tank-workspace-2026-10-07.md`](../archive/reviews/implementation-think-tank-workspace-2026-10-07.md).
Independent review is available in the handoff. Credentialed provider smokes remain separate owed
gates; they were not run during this quota recovery.
