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

/** Minimal binary min-heap in creation order, for the ready queue below. */
function heapPush(heap: Task[], task: Task) {
  let i = heap.push(task) - 1;
  while (i > 0) {
    const up = (i - 1) >> 1;
    if (byCreated(heap[up], heap[i]) <= 0) break;
    [heap[up], heap[i]] = [heap[i], heap[up]];
    i = up;
  }
}

function heapPop(heap: Task[]): Task {
  const top = heap[0];
  const last = heap.pop()!;
  if (heap.length > 0) {
    heap[0] = last;
    for (let i = 0; ;) {
      const left = 2 * i + 1;
      const right = left + 1;
      let min = i;
      if (left < heap.length && byCreated(heap[left], heap[min]) < 0) min = left;
      if (right < heap.length && byCreated(heap[right], heap[min]) < 0) min = right;
      if (min === i) break;
      [heap[min], heap[i]] = [heap[i], heap[min]];
      i = min;
    }
  }
  return top;
}

/** topologicalOrder orders a group by its prerequisite successors, breaking
 *  ties by creation. Delegation is not chronology, and links need not form a
 *  DAG, so when nothing is ready the earliest-created remaining member is
 *  taken next (TS-08.R84). O((tasks + links) log tasks). */
function topologicalOrder(tasks: Task[], next: Map<string, Task[]>, indegree: Map<string, number>): Task[] {
  const byCreation = [...tasks].sort(byCreated);
  const remaining = new Map(byCreation.map((task) => [task.task_id, indegree.get(task.task_id) ?? 0]));
  const emitted = new Set<string>();
  const ready: Task[] = [];
  for (const task of byCreation) if (remaining.get(task.task_id) === 0) heapPush(ready, task);
  const out: Task[] = [];
  let fallback = 0;
  while (out.length < tasks.length) {
    let task: Task;
    if (ready.length > 0) {
      task = heapPop(ready);
    } else {
      while (emitted.has(byCreation[fallback].task_id)) fallback++;
      task = byCreation[fallback];
    }
    emitted.add(task.task_id);
    out.push(task);
    for (const successor of next.get(task.task_id) ?? []) {
      const left = remaining.get(successor.task_id)! - 1;
      remaining.set(successor.task_id, left);
      if (left === 0 && !emitted.has(successor.task_id)) heapPush(ready, successor);
    }
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
    for (let node = id; node !== root;) {
      const up = parent.get(node)!;
      parent.set(node, root);
      node = up;
    }
    return root;
  };
  const union = (a: string, b: string) => {
    const left = find(a);
    const right = find(b);
    if (left !== right) parent.set(left < right ? right : left, left < right ? left : right);
  };

  const links = new Map<string, TaskLink[]>();
  const external = new Map<string, ExternalWait[]>();
  // Prerequisite successors only: delegation is parentage, never "leads to".
  const next = new Map<string, Task[]>();
  const indegree = new Map<string, number>();
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
    const sources = new Set<string>();
    for (const link of inbound) {
      if (link.kind !== "prerequisite" || !link.source || sources.has(link.sourceID)) continue;
      sources.add(link.sourceID);
      const successors = next.get(link.sourceID);
      if (successors) successors.push(task);
      else next.set(link.sourceID, [task]);
    }
    indegree.set(task.task_id, sources.size);
    links.set(task.task_id, inbound);
    external.set(task.task_id, outside);
  }

  const members = new Map<string, Task[]>();
  for (const task of tasks) {
    const root = find(task.task_id);
    const group = members.get(root);
    if (group) group.push(task);
    else members.set(root, [task]);
  }

  const groups: WorkGroup[] = [...members.entries()].map(([id, groupTasks]) => {
    const ordered = topologicalOrder(groupTasks, next, indegree);
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

/** Rolled-up state of the descendants a collapsed parent would hide, by
 *  recorded parent lineage only (FS-16.R46, TS-08.R92). */
export interface DescendantSummary {
  hidden: number;
  unfinished: number;
  attention: number;
  cleanup: number;
}

function emptySummary(): DescendantSummary {
  return { hidden: 0, unfinished: 0, attention: 0, cleanup: 0 };
}

/** Why a row is omitted from the visible set: the nearest collapsed ancestor
 *  and the chain a reveal must expand, nearest first (FS-16.R47). */
export interface HiddenBy {
  ancestorID: string;
  path: string[];
}

export interface RowVisibility {
  /** task_id -> descendant summary, present only for rows with retained
   *  children in this group (whether or not they are currently collapsed). */
  descendants: Map<string, DescendantSummary>;
  /** task_id -> why it is hidden; absent ids are visible. */
  hiddenBy: Map<string, HiddenBy>;
  /** task_id -> its direct children by recorded lineage, for a toggle's
   *  aria-controls identity (TS-08.R92). */
  directChildren: Map<string, Task[]>;
}

/** childIndex groups a task list by recorded parent lineage only — never by
 *  dependency edges, indentation or shared creator/run identity (FS-16.R47).
 *  A parent outside the given list, or a self-reference, is not indexed: a
 *  missing or invalid parent never manufactures a collapsible relationship. */
export function childIndex(tasks: Task[]): Map<string, Task[]> {
  const ids = new Set(tasks.map((task) => task.task_id));
  const children = new Map<string, Task[]>();
  for (const task of tasks) {
    const parentID = task.lineage.parent_task_id;
    if (!parentID || parentID === task.task_id || !ids.has(parentID)) continue;
    const list = children.get(parentID);
    if (list) list.push(task);
    else children.set(parentID, [task]);
  }
  return children;
}

/** ancestorChain walks recorded parent lineage from a task, nearest first,
 *  guarded by a visited set so cyclic lineage terminates rather than looping
 *  (FS-16.R47, TS-08.R92). */
function ancestorChain(taskID: string, parentOf: Map<string, string>): string[] {
  const chain: string[] = [];
  const visited = new Set([taskID]);
  let current = parentOf.get(taskID);
  while (current && !visited.has(current)) {
    chain.push(current);
    visited.add(current);
    current = parentOf.get(current);
  }
  return chain;
}

/** cyclicLineage reports a chain that loops back instead of reaching a root:
 *  invalid lineage never hides work (FS-16.R47). */
function cyclicLineage(taskID: string, parentOf: Map<string, string>): boolean {
  const chain = ancestorChain(taskID, parentOf);
  const last = chain.length > 0 ? chain[chain.length - 1] : taskID;
  return parentOf.has(last) && (parentOf.get(last) === taskID || chain.includes(parentOf.get(last)!));
}

/** rowVisibility is the pure projection TS-08.R92 asks for: which rows a
 *  collapsed ancestor hides, and the descendant summary each parent shows.
 *  A task pinned by active inspection or mutation, or sitting on a pinned
 *  task's ancestor path, stays visible regardless of collapse state (FS-16.R47).
 *  Bounded iterative traversal with visited guards; no path enumeration and
 *  no fetch of hidden tasks — everything comes from the already-loaded rows. */
export function rowVisibility(rows: WorkRow[], collapsed: ReadonlySet<string>, pinned: ReadonlySet<string>): RowVisibility {
  const tasks = rows.map((row) => row.task);
  const ids = new Set(tasks.map((task) => task.task_id));
  const parentOf = new Map<string, string>();
  for (const task of tasks) {
    const parentID = task.lineage.parent_task_id;
    if (parentID && parentID !== task.task_id && ids.has(parentID)) parentOf.set(task.task_id, parentID);
  }
  const children = childIndex(tasks);

  const extendedPinned = new Set(pinned);
  for (const id of pinned) for (const ancestor of ancestorChain(id, parentOf)) extendedPinned.add(ancestor);

  const hiddenBy = new Map<string, HiddenBy>();
  for (const task of tasks) {
    if (extendedPinned.has(task.task_id) || cyclicLineage(task.task_id, parentOf)) continue;
    const chain = ancestorChain(task.task_id, parentOf);
    const ancestorID = chain.find((id) => collapsed.has(id));
    if (ancestorID) hiddenBy.set(task.task_id, { ancestorID, path: chain.slice(0, chain.indexOf(ancestorID) + 1) });
  }

  const descendants = new Map<string, DescendantSummary>();
  for (const task of tasks) {
    const directChildren = children.get(task.task_id);
    if (!directChildren) continue;
    const summary = emptySummary();
    const visited = new Set([task.task_id]);
    const queue = [...directChildren];
    while (queue.length > 0) {
      const current = queue.shift()!;
      if (visited.has(current.task_id)) continue; // cyclic lineage guard
      visited.add(current.task_id);
      if (!extendedPinned.has(current.task_id)) {
        summary.hidden += 1;
        if (current.state !== "finished") summary.unfinished += 1;
        if (needsAttention(current)) summary.attention += 1;
        if (cleanupPending(current)) summary.cleanup += 1;
      }
      for (const child of children.get(current.task_id) ?? []) if (!visited.has(child.task_id)) queue.push(child);
    }
    descendants.set(task.task_id, summary);
  }

  return { descendants, hiddenBy, directChildren: children };
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
