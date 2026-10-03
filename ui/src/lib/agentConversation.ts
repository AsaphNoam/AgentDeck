import type { NotificationPayload } from "../api/types";

// One conversation path for every notification surface (TS-03.R51, INV §2):
// toasts and desktop Web Notifications both open it through the app router.
export function agentConversationPath(agentId: string) {
  return `/agent/${encodeURIComponent(agentId)}`;
}

const AGENT_NOTIFICATION_TYPES = new Set(["permission_required", "waiting_input", "done", "budget_exceeded"]);

// The agent a notification opens, or undefined when it names none (FS-02.R64).
export function notificationAgentId(notification: NotificationPayload) {
  return AGENT_NOTIFICATION_TYPES.has(notification.notification_type) && notification.agent_id
    ? notification.agent_id
    : undefined;
}

// The router's navigate, registered by the mounted shell. Module code outside
// the router tree (the SSE client) cannot import the router without a cycle.
let navigateTo: ((path: string) => void) | null = null;

export function registerConversationNavigator(navigate: ((path: string) => void) | null) {
  navigateTo = navigate;
}

export function openAgentConversation(agentId: string) {
  navigateTo?.(agentConversationPath(agentId));
}
