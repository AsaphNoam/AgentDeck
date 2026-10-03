import type { Task, TaskArm } from "../../schemas/task";
import { TASK_ATTENTION_STATES } from "../../schemas/task";

/** needsAttention is the one definition the page and the dashboard count share:
 *  parked work and work whose agent went away without a result (FS-02.R44). */
export function needsAttention(task: Pick<Task, "state">): boolean {
  return (TASK_ATTENTION_STATES as readonly string[]).includes(task.state);
}

/** A task still owes runtime cleanup after its state settled (FS-16.R38). */
export function cleanupPending(task: Pick<Task, "pending_release" | "pending_yield" | "cleanup_phase">): boolean {
  return task.pending_release || task.pending_yield || task.cleanup_phase !== "";
}

/** Inbound relationship shown on a row: a prerequisite that gates it, or the
 *  delegation that created it. Delegation never implies a start condition
 *  (FS-16.R42, TS-10.R25). */
export type TaskLink =
  | { kind: "prerequisite"; sourceID: string; source?: Task; outcomes: string[]; state: TaskArm["state"] }
  | { kind: "delegation"; sourceID: string; source?: Task };

/** A waiting condition outside the project's task graph: a named signal, a
 *  pipeline run, or a task reference that is not in the loaded project. */
export type ExternalWait =
  | { kind: "signal"; name: string; state: TaskArm["state"] }
  | { kind: "run"; runID: string; outcomes: string[]; state: TaskArm["state"] };

export interface WorkRow {
  task: Task;
  links: TaskLink[];
  external: ExternalWait[];
  /** Successors recorded in this group, so a branch reads as a branch. */
  next: Task[];
  /** Bounded visual indentation from earlier rows in the same group. */
  depth: number;
}

export interface WorkGroup {
  id: string;
  rows: WorkRow[];
  unfinished: number;
  attention: number;
  /** Any member is unfinished or owes cleanup (FS-16.R43). */
  active: boolean;
}

export interface ProjectWork {
  project: string;
  active: WorkGroup[];
  history: WorkGroup[];
  unfinished: number;
  attention: number;
}

export const MAX_DEPTH = 3;

function byCreated(a: Task, b: Task): number {
  return a.created_at.localeCompare(b.created_at) || a.task_id.localeCompare(b.task_id);
}

/** topologicalOrder orders a group by its prerequisite links, breaking ties by
 *  creation. Delegation is not chronology, and links need not form a DAG, so
 *  members left in a cycle follow in creation order (TS-08.R84). */
function topologicalOrder(tasks: Task[], links: Map<string, TaskLink[]>): Task[] {
  const ids = new Set(tasks.map((task) => task.task_id));
  const pending = new Map(tasks.map((task) => [task.task_id, new Set(
    (links.get(task.task_id) ?? []).filter((link) => link.kind === "prerequisite" && ids.has(link.sourceID)).map((link) => link.sourceID),
  )]));
  const remaining = [...tasks].sort(byCreated);
  const out: Task[] = [];
  while (remaining.length > 0) {
    let index = remaining.findIndex((task) => pending.get(task.task_id)!.size === 0);
    if (index < 0) index = 0;
    const [task] = remaining.splice(index, 1);
    out.push(task);
    for (const waiting of pending.values()) waiting.delete(task.task_id);
  }
  return out;
}

/** projectWork groups one project's retained tasks by recorded relationships
 *  only — task-result prerequisites and parent lineage within the project —
 *  never by creator, signal or run identity (FS-16.R42). Snapshot-based:
 *  references outside the list stay unavailable rather than fetched (R42).
 *  `pinned` keeps a group under inspection in the active list (R43). */
export function projectWork(project: string, tasks: Task[], pinned: ReadonlySet<string> = new Set()): ProjectWork {
  const byID = new Map(tasks.map((task) => [task.task_id, task]));
  const parent = new Map(tasks.map((task) => [task.task_id, task.task_id]));
  const find = (id: string): string => {
    let root = id;
    while (parent.get(root) !== root) root = parent.get(root)!;
    parent.set(id, root);
    return root;
  };
  const union = (a: string, b: string) => {
    const left = find(a);
    const right = find(b);
    if (left !== right) parent.set(left < right ? right : left, left < right ? left : right);
  };

  const links = new Map<string, TaskLink[]>();
  const external = new Map<string, ExternalWait[]>();
  const next = new Map<string, Task[]>();
  for (const task of tasks) {
    const inbound: TaskLink[] = [];
    const outside: ExternalWait[] = [];
    const parentID = task.lineage.parent_task_id;
    if (parentID) {
      inbound.push({ kind: "delegation", sourceID: parentID, source: byID.get(parentID) });
      if (byID.has(parentID)) union(task.task_id, parentID);
    }
    for (const arm of task.arms ?? []) {
      if (arm.kind === "signal") {
        outside.push({ kind: "signal", name: arm.signal_name, state: arm.state });
      } else if (arm.source_kind === "pipeline_run") {
        outside.push({ kind: "run", runID: arm.source_id, outcomes: arm.satisfying_outcomes ?? [], state: arm.state });
      } else {
        inbound.push({ kind: "prerequisite", sourceID: arm.source_id, source: byID.get(arm.source_id), outcomes: arm.satisfying_outcomes ?? [], state: arm.state });
        if (byID.has(arm.source_id)) union(task.task_id, arm.source_id);
      }
    }
    for (const link of inbound) {
      if (link.source && !(next.get(link.sourceID) ?? []).includes(task)) next.set(link.sourceID, [...(next.get(link.sourceID) ?? []), task]);
    }
    links.set(task.task_id, inbound);
    external.set(task.task_id, outside);
  }

  const members = new Map<string, Task[]>();
  for (const task of tasks) {
    const root = find(task.task_id);
    members.set(root, [...(members.get(root) ?? []), task]);
  }

  const groups: WorkGroup[] = [...members.entries()].map(([id, groupTasks]) => {
    const ordered = topologicalOrder(groupTasks, links);
    const depth = new Map<string, number>();
    const rows = ordered.map((task) => {
      // Depth follows only links to earlier rows, so a cycle cannot recurse.
      const earlier = (links.get(task.task_id) ?? []).filter((link) => depth.has(link.sourceID));
      const level = earlier.length === 0 ? 0 : Math.min(MAX_DEPTH, Math.max(...earlier.map((link) => depth.get(link.sourceID)!)) + 1);
      depth.set(task.task_id, level);
      return { task, links: links.get(task.task_id) ?? [], external: external.get(task.task_id) ?? [], next: (next.get(task.task_id) ?? []).sort(byCreated), depth: level };
    });
    const unfinished = groupTasks.filter((task) => task.state !== "finished").length;
    return {
      id,
      rows,
      unfinished,
      attention: groupTasks.filter(needsAttention).length,
      active: unfinished > 0 || groupTasks.some(cleanupPending) || groupTasks.some((task) => pinned.has(task.task_id)),
    };
  });

  // TS-08.R84: attention groups first, then newest-created; history by latest finish.
  const latest = (group: WorkGroup, field: "created_at" | "finished_at") =>
    group.rows.reduce((value, row) => (row.task[field] ?? "") > value ? row.task[field] ?? "" : value, "");
  const activeOrder = (a: WorkGroup, b: WorkGroup) =>
    (b.attention > 0 ? 1 : 0) - (a.attention > 0 ? 1 : 0) || latest(b, "created_at").localeCompare(latest(a, "created_at")) || a.id.localeCompare(b.id);
  const historyOrder = (a: WorkGroup, b: WorkGroup) =>
    latest(b, "finished_at").localeCompare(latest(a, "finished_at")) || a.id.localeCompare(b.id);
  return {
    project,
    active: groups.filter((group) => group.active).sort(activeOrder),
    history: groups.filter((group) => !group.active).sort(historyOrder),
    unfinished: tasks.filter((task) => task.state !== "finished").length,
    attention: tasks.filter(needsAttention).length,
  };
}

export type StatusTone = "info" | "success" | "warning" | "danger" | "neutral";

/** taskStatus names the row's authoritative state and its immediate reason
 *  (FS-16.R44). Readiness is never presented as capacity exhaustion, and
 *  waiting or cleanup is never presented as an interruption. */
export function taskStatus(task: Task): { label: string; tone: StatusTone } {
  switch (task.state) {
    case "armed":
      return { label: "Waiting on prerequisites", tone: "neutral" };
    case "ready":
      return task.continuation_pending ? { label: "Ready to resume", tone: "info" } : { label: "Ready to start", tone: "info" };
    case "starting":
      return { label: "Starting", tone: "info" };
    case "running":
      return task.pending_yield ? { label: "Yielding to wait", tone: "info" } : { label: "Running", tone: "info" };
    case "waiting":
      return { label: "Waiting for work updates", tone: "neutral" };
    case "interrupted":
      return { label: "Interrupted", tone: "warning" };
    case "dependency_failed":
      return { label: "Prerequisite failed", tone: "danger" };
    case "finished": {
      if (task.pending_release) return { label: "Finishing cleanup", tone: "neutral" };
      const outcome = task.outcome || "finished";
      const tone: StatusTone = outcome === "success" ? "success" : outcome === "failure" ? "danger" : outcome === "blocked" ? "warning" : "neutral";
      return { label: outcome[0].toUpperCase() + outcome.slice(1), tone };
    }
  }
}

/** taskReason is the short wait or attention explanation that leads a row. */
export function taskReason(row: WorkRow): string {
  const { task } = row;
  if (task.attention_reason) return task.attention_reason;
  if (task.cleanup_last_error) return `Cleanup retrying: ${task.cleanup_last_error}`;
  if (task.state === "armed" || task.state === "dependency_failed") {
    const waits = [
      ...row.links.filter((link): link is Extract<TaskLink, { kind: "prerequisite" }> => link.kind === "prerequisite" && link.state === "unsatisfied")
        .map((link) => `${link.source?.display_name ?? "unavailable task"} → ${link.outcomes.join(" or ")}`),
      ...row.external.filter((wait) => wait.state === "unsatisfied")
        .map((wait) => wait.kind === "signal" ? `signal ${wait.name}` : `run ${wait.runID} → ${wait.outcomes.join(" or ")}`),
    ];
    if (waits.length > 0) return `Waits for ${waits.join("; ")}`;
  }
  if (task.state === "waiting") return "The agent yielded its slot and resumes when work it watches changes.";
  return "";
}
