# Agent groups on desktop and mobile

**State:** Waiting to start
**Why:** Direct `/design-feature` request on 2026-10-10, expanded to bidirectional updates,
cross-group drag and bulk Stop/Archive; current-project scope and desktop drag/mobile picker
confirmed. The user explicitly waived backwards compatibility and requested replacing global release.
**Relevant requirements:** FS-02.R72–R76/A54–A57; FS-20.R45–R46/A17–A18;
TS-03.R56–R58; TS-08.R118; TS-13.R24; existing TS-01.R13/R16;
INV §§1/2/4/5/8/10/11/13/14/16/17.

## Outcome

Choose or create a group during ordinary agent creation and reassignment, move desktop agents
between group sections by drag, and see/change project groups from a phone. Stop or archive a
named group's members in the current project, with clear confirmations and per-member results.
Changes from either device appear live on the other.

## Included work

Visible searchable picker with global name suggestions; desktop/phone creation and reassignment;
collapsible phone sections; desktop cross-group drop including Ungrouped and collapsed headers;
project-scoped Stop/Archive using existing lifecycle/archive services and retained conversations.
Replace `/api/groups/{group}/release` with `POST /api/projects/{project}/groups/stop` and
`/groups/archive`, each with `{group}`. Migrate callers, route inventory and documented operator
examples; remove the old global route with no alias. Paired-phone access admits only the group
launch field, group-only identity updates and the two scoped actions.

No new group database, retention policy, mobile drag, Ungrouped bulk actions, whole-card drag,
project archival or group Restore. Existing room/annotation assignment flows retain their grouping.
The independent whole-card drag-usability idea stays separate.

## How we will know it works

FS-02.A54–A57 and FS-20.A17–A18 cover create/move, keyboard access, drop/refusal, two-device
updates, cross-project isolation, conflict with zero changes, partial failures and individual
restoration. Server tests prove real serialized bodies/results, names containing `/`, narrow
tailnet fields/authentication and absence of the legacy route. Include Stop/Resume/Archive/move
races and persistence failure after stop. Browser journeys cover all three appearances at the
1024px desktop floor and wider, 390px phone, and 360px/430px long-name/overflow cases. Apply
the shared TS-06/workflow closure matrix when implemented; design checks are documentation-only.

Verified seams: `CardContextMenu` currently uses input/datalist with all loaded labels;
`NewAgentModal` lacks group while server launch and phone client already support its shape;
tailnet filters currently reject group launch and identity/release. `CardGrid` drag only persists
order. `groups.go` matches release labels globally; `releaseAgents` already claims members before
stopping with a four-worker cap. `archive_actions.go` and `archive_gate.go` supply archive services
and exclusion; existing identity/archive columns and full `state_update` need no migration.
Caller audit found only `api/client.ts` → `CardGrid` for the old HTTP route, with server route,
remote classification and lifecycle tests to migrate. Refresh README's endpoint table and the
FS-01/FS-07/FS-14/TS-01 group references when shipping; their planned replacements are recorded
in the feature deviations. No CLI, MCP or embedded operator-package caller was found; software
`chuck release` is unrelated. Archived historical references remain history.

## Waiting on

None. Product and API/access scope are settled; implementation has not started.
