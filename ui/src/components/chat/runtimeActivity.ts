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
    // Task lifecycle renders in the tail list, never inline (FS-03.R59).
    if (kind === "background_task_state") continue;
    if (kind === "turn_end" && !id) {
      for (const node of nodes.values()) if (node.state === "active") node.state = "disconnected";
    }
    const owner = id ? nodes.get(id) : undefined;
    (owner ? owner.items : root).push(event);
  }
  return root;
}

export type TaskState = "running" | "completed" | "failed" | "stopped";

export interface BackgroundTask {
  taskId: string;
  activityId?: string;
  /** The native child that owns it, when one does. */
  owner?: string;
  toolTitle?: string;
  name: string;
  state: TaskState;
  canStop: boolean;
  /** A later resume started a new runtime generation that cannot control it. */
  fenced: boolean;
}

const TASK_STATES = new Set(["running", "completed", "failed", "stopped"]);

// collectTasks folds durable task lifecycle into one row per task, in
// announcement order, naming its related tool call. Output stays with that tool
// call. A task still running when a later session resume began belonged to the
// previous runtime generation, whose process is gone: it reads as stopped and
// offers no control (INV §1).
export function collectTasks(events: TranscriptEvent[]): BackgroundTask[] {
  const tasks = new Map<string, BackgroundTask>();
  const titles = new Map<string, string>();
  const owners = new Map<string, string>();
  for (const event of events) {
    const kind = kindOf(event);
    if (kind === "activity_started" && event.activity_id) owners.set(event.activity_id, String(event.name || "Subagent"));
    if (kind === "tool_call" && event.tool_call_id) titles.set(String(event.tool_call_id), String(event.title ?? event.name ?? ""));
    if (kind === "session_meta" && event.resumed_at) {
      for (const task of tasks.values()) if (task.state === "running") Object.assign(task, { state: "stopped", fenced: true });
    }
    if (kind !== "background_task_state" || !event.task_id || !TASK_STATES.has(String(event.state))) continue;
    const id = String(event.task_id);
    const prior = tasks.get(id);
    tasks.set(id, {
      taskId: id,
      activityId: event.activity_id,
      owner: event.activity_id ? owners.get(event.activity_id) : undefined,
      toolTitle: titles.get(String(event.tool_call_id ?? "")) || prior?.toolTitle,
      name: String(event.name || prior?.name || ""),
      state: event.state as TaskState,
      canStop: Boolean(event.can_stop ?? prior?.canStop),
      fenced: false,
    });
  }
  return [...tasks.values()];
}

// hasPendingPermission keeps a child open while it waits on the person.
export function hasPendingPermission(node: ChildNode): boolean {
  return node.items.some((event) =>
    (kindOf(event) === "permission_request" && !event.resolved) ||
    (kindOf(event) === "activity" && hasPendingPermission(event.node as ChildNode)));
}
