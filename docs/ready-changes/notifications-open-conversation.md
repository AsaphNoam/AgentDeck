# Notifications open the agent's conversation

**State:** Waiting to start
**Why:** `docs/ideas.md` "Approval notifications link to the conversation"; widened by the user on
2026-10-02 to every agent notification type.
**Relevant requirements:** FS-02.R64, FS-02.A46, TS-03.R51, INV §2

## Outcome
Clicking any agent notification, whether toast or desktop, opens that agent's full conversation, so
the person reaches a pending permission or finished work without hunting for the agent.

## Included work
Toast body opens `/agent/<agent_id>` and dismisses; a close control only dismisses. Desktop Web
Notification click focuses the tab, opens the conversation, and closes the notification. Error
toasts, mutes, dedupe, cap, timers, the server payload, and phone push (FS-20.R21) are unchanged. No
in-notification actions, dashboard-pane destination, or service-worker desktop notifications.

## How we will know it works
FS-02.A46: `NotificationCenter.test.tsx` and `sse.test.ts`, a real-browser toast check, and a manual
macOS desktop-notification click.

## Waiting on
Nothing.
