import type { TranscriptEvent } from "../../api/types";

// The runtime-activity projection (TS-08.R59): the root conversation stays the
// reading path, and each native child sits at its causal position as one
// nested node holding its own ordinary events. Live append and replay both
// render through this one function over the folded event list (INV §2).

export type ChildState = "active" | "completed" | "failed" | "stopped" | "disconnected";

export interface ChildNode {
  id: string;
  name: string;
  task?: string;
  state: ChildState;
  items: TranscriptEvent[];
}

const TERMINAL = new Set(["completed", "failed", "stopped", "disconnected"]);

function kindOf(event: TranscriptEvent) {
  return String(event.kind ?? event.type ?? "");
}

// nestActivities returns the root list with each child's events moved into a
// `{kind:"activity", node}` row at the child's announcement. A child still
// active when its root turn ended can no longer prove an outcome: it reads as
// disconnected, never failed (FS-03.R58).
export function nestActivities(events: TranscriptEvent[]): TranscriptEvent[] {
  const nodes = new Map<string, ChildNode>();
  const root: TranscriptEvent[] = [];
  for (const event of events) {
    const kind = kindOf(event);
    const id = event.activity_id;
    if (kind === "activity_started" && id) {
      if (nodes.has(id)) continue;
      const node: ChildNode = { id, name: String(event.name ?? ""), task: event.task ? String(event.task) : undefined, state: "active", items: [] };
      nodes.set(id, node);
      const parent = event.parent_activity_id ? nodes.get(event.parent_activity_id) : undefined;
      (parent ? parent.items : root).push({ kind: "activity", message_id: `activity-${id}`, node });
      continue;
    }
    if (kind === "activity_state" && id) {
      const node = nodes.get(id);
      if (node && TERMINAL.has(String(event.state))) node.state = event.state as ChildState;
      continue;
    }
    if (kind === "turn_end" && !id) {
      for (const node of nodes.values()) if (node.state === "active") node.state = "disconnected";
    }
    const owner = id ? nodes.get(id) : undefined;
    (owner ? owner.items : root).push(event);
  }
  return root;
}

// hasPendingPermission keeps a child open while it waits on the person.
export function hasPendingPermission(node: ChildNode): boolean {
  return node.items.some((event) =>
    (kindOf(event) === "permission_request" && !event.resolved) ||
    (kindOf(event) === "activity" && hasPendingPermission(event.node as ChildNode)));
}
