# Phone desktop flow and agent management

**State:** In progress
**Why:** Direct operator request (2026-10-01): the phone app is too lean for remote work; it should
show projects, open a project, open its agents, and create agents. The operator then directed the
phone to follow the desktop flow (dashboard → project page → agent chat), drop the phone's task
pages, put Start pipeline beside New agent on the project page, and omit a first message from New
agent. Recorded in `docs/ideas.md` as “Work remotely from the phone”.
**Relevant requirements:** FS-20.R33–R41, FS-20.A10–A13 (and A8's revised journey), TS-13.R19–R23,
INV §2, §8, §10, §14, §16

## Outcome

From a paired phone, the operator moves through the same dashboard → project → agent chat flow as on
the desktop. They can launch chat agents with a chosen runtime, start pipelines from a project page,
rename, retune, switch, clone, and archive agents, and see the files and commands each agent
changed and ran. Approvals stay one tap away in a Needs you strip on Home.

## Included work

Phone Home as the project dashboard with a Needs you strip; project pages; New agent with the
runtime picker; Start pipeline moved from New work to the project page; agent actions; Files and
Commands with tracked-path-only file reads (`O_NOFOLLOW`); tailnet allowlist changes, including
denying every task route; narrowed home attention and push; runtime-option defaults; and one shared
project-dashboard derivation for desktop and phone.

Excluded: phone task UI, the global New work screen and Ask AgentDecker, the first-message field,
terminal agents, interface switching, worktree forks, restore and archive search, agent mail,
arbitrary file reads, and file search. When this ships, retire or narrow the superseded shipped
items listed in FS-20 §6 in the same change.

## How we will know it works

FS-20.A10–A13 cover server tests (launch and session-route field filters, tracked-file guard
including symlink and untracked refusals, route inventory, attention and push without tasks,
secret-free runtime defaults), phone UI tests, and a fakeACP browser pass at 390px. A8's manual
real-tailnet journey now launches an agent from a project page.

## Waiting on

Nothing.
