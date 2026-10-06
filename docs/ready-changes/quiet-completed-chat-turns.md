# Quiet completed chat turns and readable live thoughts

**State:** Waiting to start
**Why:** Human `/design-feature` request, 2026-10-06: long chats are noisy because intermediate
assistant text stays permanently expanded. Human confirmed one activity control per completed
turn and unchanged ephemeral thought retention.
**Relevant requirements:** FS-03.R73–R77/A54–A58; TS-08.R100–R106; preserved FS-03.R57–R59,
TS-01.R35, TS-04.R63; INV §1, §2, §8, §10, §11, §13, §16, §17.

## Outcome

Provider-exposed thoughts start open while work runs and respect manual collapse. Completed turns
leave user input, the last assistant response and important outcomes visible, with one compact
Show activity/Hide activity control for earlier messages, thoughts and tools.

## Included work

Extend the shared transcript/activity renderer with proven root turn boundaries, local bounded
collapse choices and conservative last-passage selection. Wire full chat, dashboard panes,
Archive and phone chat; retain Think Tank attempt boundaries without hiding canonical room
contributions or synthesis. Preserve pending approval visibility, live background status/Stop,
child attribution, inspection/annotation/file actions and focus/scroll behavior. Reset nested
thought/tool choices once at completion without closing older turns manually reopened afterward.

No protocol change, durable thought storage, history rewrite, new settings, global collapse button,
new provider inference, room execution change or product code belongs to this design unit.

Evidence: `internal/runtime/acpmap.go` maps `agent_message_chunk` to `assistant_text{delta}` and
`mapPromptResult` to `turn_end`; it supplies no final/intermediate channel. The shared fold merges
adjacent same-scope assistant deltas, so keep its last root passage visible without guessing from
prose. `ThinkingDisclosure` currently starts closed and has no terminal lifecycle input;
`ToolRun` collapses only uninterrupted tool runs. `reasoningStore` is bounded and memory-only.
Source inspection establishes these seams; rendered long-chat acceptance is still owed during
implementation, not claimed by this design.

Direction: the experienced operator follows live work, then reads answers or reopens details.
Keep the root exchange primary and activity as subdued text with a chevron, no extra cards,
animated layout or new visual system. Turn/child/task lifecycle supplies hierarchy; approvals,
failure and continuing work must remain obvious at narrow widths in every appearance.

## How we will know it works

FS-03.A54–A58 cover live manual choices, long interleaved turns, Steer, child activity, terminal
outcomes, incomplete history, surviving background work, older reopened activity, replay/archive,
phone parity, keyboard/focus and scroll stability. TS-08.R106 specifies shared projection/UI tests
and real-binary fake-ACP rendered journeys in Core, Sky & Grove and Studio, including narrow panes
and 390px phone width. Run the applicable TS-06 closure matrix once after the final implementation
edit, with presentation/style checks and generated embed output. No new credentialed smoke gate.

## Waiting on

None. All new requirements remain planned; the independent provider-refresh change stays active.
