import type { TranscriptEvent } from "../../api/types";
import { slotTurnKeys } from "./turnActivity";
import type { ReasoningSpan } from "../../store/reasoningStore";

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
    if (kind === "background_task_state" || kind === "file_report") continue;
    if (kind === "turn_end" && !id) {
      for (const node of nodes.values()) if (node.state === "active") node.state = "disconnected";
    }
    const owner = id ? nodes.get(id) : undefined;
    (owner ? owner.items : root).push(event);
  }
  return root;
}

// withReasoning places each live reasoning span at its chronological slot as a
// render-only row. It has no seq, so it is never annotated, and it never enters
// the transcript store (FS-03.R57). Desktop and phone both render through it.
// A span whose slot now falls in another turn than the one that owned it at
// admission is not shown, so it can never be backfilled into history (TS-08.R103).
export function withReasoning(events: TranscriptEvent[], all: ReasoningSpan[] | undefined): TranscriptEvent[] {
  if (!all?.length) return events;
  const keys = slotTurnKeys(events);
  const spans = all.filter((span) => span.turn === undefined || keys[Math.min(span.anchor, events.length)] === span.turn);
  const row = (span: ReasoningSpan): TranscriptEvent => ({
    kind: "reasoning",
    activity_id: span.activityId,
    message_id: `reasoning-${span.turn ?? ""}-${span.activityId ?? ""}-${span.spanId}`,
    text: span.text,
  });
  const out: TranscriptEvent[] = [];
  events.forEach((event, index) => {
    for (const span of spans) if (span.anchor === index) out.push(row(span));
    out.push(event);
  });
  for (const span of spans) if (span.anchor >= events.length) out.push(row(span));
  return out;
}

export type TaskState = "running" | "completed" | "failed" | "stopped";

export interface BackgroundTask {
  taskId: string;
  activityId?: string;
  toolCallId?: string;
  /** The native child that owns it, when one does. */
  owner?: string;
  toolTitle?: string;
  name: string;
  state: TaskState;
  canStop: boolean;
  /** A later resume or clone boundary left it with a runtime that cannot
   * control it: the previous session, or the source a clone was forked from. */
  fenced?: "resume" | "fork";
}

const TASK_STATES = new Set(["running", "completed", "failed", "stopped"]);

// collectTasks folds durable task lifecycle into one row per task, in
// announcement order, naming its related tool call. Output stays with that tool
// call. A task still running when a later session resume began belonged to the
// previous runtime generation, whose process is gone; one copied into a clone
// stays owned by its source (FS-01.R36). Either reads as stopped and offers no
// control here (INV §1).
export function collectTasks(events: TranscriptEvent[]): BackgroundTask[] {
  const tasks = new Map<string, BackgroundTask>();
  const titles = new Map<string, string>();
  const owners = new Map<string, string>();
  for (const event of events) {
    const kind = kindOf(event);
    if (kind === "activity_started" && event.activity_id) owners.set(event.activity_id, String(event.name || "Subagent"));
    if (kind === "tool_call" && event.tool_call_id) titles.set(String(event.tool_call_id), String(event.title ?? event.name ?? ""));
    const fence = kind === "session_meta" && event.resumed_at ? "resume" : kind === "fork_boundary" ? "fork" : undefined;
    if (fence) {
      for (const task of tasks.values()) if (task.state === "running") Object.assign(task, { state: "stopped", fenced: fence });
    }
    if (kind !== "background_task_state" || !event.task_id || !TASK_STATES.has(String(event.state))) continue;
    const id = String(event.task_id);
    const prior = tasks.get(id);
    tasks.set(id, {
      taskId: id,
      activityId: event.activity_id,
      toolCallId: event.tool_call_id ? String(event.tool_call_id) : prior?.toolCallId,
      owner: event.activity_id ? owners.get(event.activity_id) : undefined,
      toolTitle: titles.get(String(event.tool_call_id ?? "")) || prior?.toolTitle,
      name: String(event.name || prior?.name || ""),
      state: event.state as TaskState,
      canStop: Boolean(event.can_stop ?? prior?.canStop),
    });
  }
  return [...tasks.values()];
}

// markBackgrounded gives each tool call the latest state of the background
// task its runtime linked to it, so a command that moved to the background
// (including on Steer) reads as continuing there rather than finished. Desktop
// and phone both render through it (FS-03.R67, TS-08.R86, INV §2). It reads
// collectTasks, so resume and clone fences apply to the label too (FS-01.R36).
export function markBackgrounded(events: TranscriptEvent[]): TranscriptEvent[] {
  const states = new Map<string, string>();
  for (const task of collectTasks(events)) if (task.toolCallId) states.set(task.toolCallId, task.state);
  if (states.size === 0) return events;
  return events.map((event) => {
    const state = kindOf(event) === "tool_call" ? states.get(String(event.tool_call_id ?? "")) : undefined;
    return state ? { ...event, background_state: state } : event;
  });
}

// hasPendingPermission keeps a child open while it waits on the person.
export function hasPendingPermission(node: ChildNode): boolean {
  return node.items.some((event) =>
    (kindOf(event) === "permission_request" && !event.resolved) ||
    (kindOf(event) === "activity" && hasPendingPermission(event.node as ChildNode)));
}
