import { describe, expect, it } from "vitest";
import { taskListSchema, type Task } from "../../schemas/task";
import fixture from "./fixtures/taskLists.json";
import { childIndex, projectWork, rowVisibility, taskReason, taskStatus, type WorkRow } from "./taskWork";

// The fixture is marshalled by Go (internal/server/task_wire_fixture_test.go),
// so these assertions run against the server's real task wire shape.
const tasks = (project: "my-app" | "other") => taskListSchema.parse(fixture[project]).tasks;
const names = (rows: { task: { task_id: string } }[]) => rows.map((row) => row.task.task_id);

describe("projectWork", () => {
  const work = projectWork("my-app", tasks("my-app"));
  const groupOf = (id: string) => [...work.active, ...work.history].find((group) => group.rows.some((row) => row.task.task_id === id))!;

  it("groups a chain with its completed predecessor and keeps each task once (FS-16.A27)", () => {
    expect(names(groupOf("tk_b").rows)).toEqual(["tk_a", "tk_b", "tk_c"]);
    const all = [...work.active, ...work.history].flatMap((group) => names(group.rows));
    expect(all).toHaveLength(tasks("my-app").length);
    expect(new Set(all).size).toBe(all.length);
  });

  it("keeps a branch and join explicit without repeating the joined task", () => {
    const group = groupOf("tk_g");
    expect(names(group.rows)).toEqual(["tk_d", "tk_e", "tk_f", "tk_g"]);
    const split = group.rows.find((row) => row.task.task_id === "tk_d")!;
    expect(split.next.map((task) => task.task_id)).toEqual(["tk_e", "tk_f"]);
    const join = group.rows.find((row) => row.task.task_id === "tk_g")!;
    expect(join.links.map((link) => [link.kind, link.sourceID])).toEqual([["prerequisite", "tk_e"], ["prerequisite", "tk_f"]]);
    expect(join.depth).toBe(2);
  });

  it("labels delegation separately and never groups by shared creator, signal or run", () => {
    const delegated = groupOf("tk_q").rows.find((row) => row.task.task_id === "tk_q")!;
    expect(delegated.links).toEqual([expect.objectContaining({ kind: "delegation", sourceID: "tk_p" })]);
    expect(names(groupOf("tk_r").rows)).toEqual(["tk_r"]);
    expect(names(groupOf("tk_t").rows)).toEqual(["tk_t"]);
    expect(names(groupOf("tk_u").rows)).toEqual(["tk_u"]);
    expect(groupOf("tk_t").rows[0].external.map((wait) => wait.kind)).toEqual(["signal", "run"]);
  });

  it("keeps a missing predecessor unavailable rather than inventing one", () => {
    const row = groupOf("tk_s").rows[0];
    expect(names(groupOf("tk_s").rows)).toEqual(["tk_s"]);
    expect(row.links).toEqual([expect.objectContaining({ kind: "prerequisite", sourceID: "tk_gone", source: undefined })]);
  });

  it("moves only settled finished groups into history and keeps pending cleanup active (FS-16.A28)", () => {
    expect(work.history.map((group) => names(group.rows))).toEqual([["tk_h1", "tk_h2"]]);
    expect(work.active.some((group) => names(group.rows).includes("tk_k"))).toBe(true);
    expect(projectWork("other", tasks("other")).history.map((group) => names(group.rows))).toEqual([["tk_o1"]]);
  });

  it("keeps an inspected finished group in place until inspection ends", () => {
    const pinned = projectWork("my-app", tasks("my-app"), new Set(["tk_h2"]));
    expect(pinned.history).toEqual([]);
  });

  it("orders by prerequisites, not creation, and survives a delegation/dependency cycle", () => {
    const [a, b] = tasks("my-app").filter((task) => task.task_id === "tk_a" || task.task_id === "tk_b").sort((x, y) => x.task_id.localeCompare(y.task_id));
    // b is created first but waits for a; a was delegated by b, closing a cycle.
    const early = { ...b, created_at: "2026-01-01T00:00:00Z" };
    const child = { ...a, lineage: { ...a.lineage, parent_task_id: "tk_b" } };
    const cycle = projectWork("my-app", [early, child]);
    expect(cycle.active.map((group) => names(group.rows))).toEqual([["tk_a", "tk_b"]]);
    const looped = projectWork("my-app", [{ ...early, arms: [{ ...early.arms![0], source_id: "tk_a" }] }, { ...child, arms: [{ ...early.arms![0], arm_id: "x", task_id: "tk_a", source_id: "tk_b" }] }]);
    expect(looped.active.flatMap((group) => names(group.rows)).sort()).toEqual(["tk_a", "tk_b"]);
  });

  it("never presents a delegated child as the parent's successor (FS-16.R42)", () => {
    const parent = groupOf("tk_p").rows.find((row) => row.task.task_id === "tk_p")!;
    expect(parent.next).toEqual([]);
    expect(groupOf("tk_d").rows.find((row) => row.task.task_id === "tk_d")!.next.map((task) => task.task_id)).toEqual(["tk_e", "tk_f"]);
  });

  it("projects a long reversed chain and a wide fan-out in prerequisite order (TS-08.R84)", () => {
    // tk_c is unfinished, so every generated group stays active.
    const [c] = tasks("my-app").filter((task) => task.task_id === "tk_c");
    const arm = c.arms![0];
    const n = 5000;
    const id = (i: number) => `tk_${String(i).padStart(5, "0")}`;
    // Created newest-first so creation order fights prerequisite order.
    const chain = Array.from({ length: n }, (_, i) => ({
      ...c, task_id: id(i), created_at: new Date(Date.UTC(2026, 0, 1) + (n - i) * 1000).toISOString(),
      arms: i === 0 ? [] : [{ ...arm, arm_id: `arm${i}`, task_id: id(i), source_id: id(i - 1) }],
    }));
    const started = performance.now();
    const linear = projectWork("my-app", chain);
    expect(linear.active.flatMap((group) => names(group.rows))).toEqual(chain.map((task) => task.task_id));
    const fan = Array.from({ length: n }, (_, i) => ({
      ...c, task_id: id(i), created_at: new Date(Date.UTC(2026, 0, 1) + (n - i) * 1000).toISOString(),
      arms: i === 0 ? [] : [{ ...arm, arm_id: `arm${i}`, task_id: id(i), source_id: id(0) }],
    }));
    const wide = projectWork("my-app", fan);
    expect(wide.active).toHaveLength(1);
    const rows = names(wide.active[0].rows);
    expect(rows[0]).toBe(id(0));
    expect(rows.slice(1)).toEqual(fan.slice(1).map((task) => task.task_id).reverse());
    expect(wide.active[0].rows[0].next).toHaveLength(n - 1);
    expect(performance.now() - started).toBeLessThan(2000);
  });

  it("keeps duplicate names as distinct rows keyed by id", () => {
    const [c] = tasks("my-app").filter((task) => task.task_id === "tk_c");
    const twin = { ...c, task_id: "tk_c2", arms: [] };
    const rows = projectWork("my-app", [c, twin]).active.flatMap((group) => names(group.rows));
    expect(rows.sort()).toEqual(["tk_c", "tk_c2"]);
  });

  it("counts unfinished and attention work and puts attention groups first", () => {
    expect(work.attention).toBe(2);
    expect(work.unfinished).toBe(11);
    expect(work.active[0].attention).toBeGreaterThan(0);
  });
});

describe("rowVisibility", () => {
  const base = tasks("my-app").find((task) => task.task_id === "tk_r")!;
  const make = (id: string, parentID: string, overrides: Partial<Task> = {}): Task => ({
    ...base, task_id: id, display_name: id, arms: [], lineage: { ...base.lineage, parent_task_id: parentID }, ...overrides,
  });
  const row = (task: Task): WorkRow => ({ task, links: [], external: [], next: [], depth: 0 });

  it("indexes children by recorded parent lineage only, ignoring a missing or self-referential parent", () => {
    const coordinator = make("coord", "");
    const worker = make("worker", "coord");
    const orphan = make("orphan", "tk_missing");
    const selfRef = make("loop", "loop");
    const children = childIndex([coordinator, worker, orphan, selfRef]);
    expect(children.get("coord")!.map((task) => task.task_id)).toEqual(["worker"]);
    expect(children.has("tk_missing")).toBe(false);
    expect([...children.values()].flat().map((task) => task.task_id)).not.toContain("loop");
  });

  // FS-16.A30 — collapsing a stage leaves its own row visible and hides its
  // descendants, reporting the hidden/unfinished/attention summary.
  it("collapses a stage to hide its descendants and reports the hidden/unfinished/attention summary", () => {
    const coordinator = make("coordinator", "");
    const worker = make("worker", "coordinator", { state: "running" });
    const reviewer = make("reviewer", "worker", { state: "interrupted", attention_reason: "stuck" });
    const rows = [coordinator, worker, reviewer].map(row);
    const visible = rowVisibility(rows, new Set(["coordinator"]), new Set());
    expect(visible.hiddenBy.has("coordinator")).toBe(false);
    expect(visible.hiddenBy.get("worker")).toEqual({ ancestorID: "coordinator", path: ["coordinator"] });
    expect(visible.hiddenBy.get("reviewer")).toEqual({ ancestorID: "coordinator", path: ["worker", "coordinator"] });
    expect(visible.descendants.get("coordinator")).toEqual({ hidden: 2, unfinished: 2, attention: 1, cleanup: 0 });
  });

  // FS-16.A31 — a nested parent collapses independently of its own ancestor.
  it("collapses a nested parent independently, leaving the outer stage expanded", () => {
    const coordinator = make("coordinator", "");
    const worker = make("worker", "coordinator", { state: "running" });
    const reviewer = make("reviewer", "worker");
    const rows = [coordinator, worker, reviewer].map(row);
    const visible = rowVisibility(rows, new Set(["worker"]), new Set());
    expect(visible.hiddenBy.has("coordinator")).toBe(false);
    expect(visible.hiddenBy.has("worker")).toBe(false);
    expect(visible.hiddenBy.get("reviewer")).toEqual({ ancestorID: "worker", path: ["worker"] });
    expect(visible.descendants.get("worker")).toEqual({ hidden: 1, unfinished: 1, attention: 0, cleanup: 0 });
  });

  it("keeps a pinned task and its ancestor path visible despite collapse, and never hides work behind a missing parent", () => {
    const coordinator = make("coordinator", "");
    const worker = make("worker", "coordinator");
    const reviewer = make("reviewer", "worker", { state: "interrupted", attention_reason: "stuck" });
    const detached = make("detached", "tk_missing");
    const rows = [coordinator, worker, reviewer, detached].map(row);
    const pinned = rowVisibility(rows, new Set(["coordinator"]), new Set(["reviewer"]));
    expect(pinned.hiddenBy.has("reviewer")).toBe(false);
    expect(pinned.hiddenBy.has("worker")).toBe(false);
    expect(pinned.hiddenBy.has("detached")).toBe(false);
    expect(pinned.descendants.get("coordinator")).toEqual({ hidden: 0, unfinished: 0, attention: 0, cleanup: 0 });
  });

  it("terminates on cyclic lineage and renders every row reachable exactly once", () => {
    const a = make("cyc_a", "cyc_b");
    const b = make("cyc_b", "cyc_a");
    const rows = [a, b].map(row);
    // Invalid lineage never hides work, even with both members collapsed.
    const visible = rowVisibility(rows, new Set(["cyc_a", "cyc_b"]), new Set());
    expect([...visible.hiddenBy.keys()]).toEqual([]);
    // A descendant whose ancestor chain never reaches a root is invalid too.
    const leaf = make("cyc_leaf", "cyc_a");
    const withLeaf = rowVisibility([a, b, leaf].map(row), new Set(["cyc_a"]), new Set());
    expect([...withLeaf.hiddenBy.keys()]).toEqual([]);
  });
});

describe("taskStatus and taskReason", () => {
  const work = projectWork("my-app", tasks("my-app"));
  const row = (id: string) => [...work.active, ...work.history].flatMap((group) => group.rows).find((item) => item.task.task_id === id)!;

  it("distinguishes durable waiting, cleanup and outcomes from interruption", () => {
    expect(taskStatus(row("tk_f").task).label).toBe("Waiting for work updates");
    expect(taskStatus(row("tk_k").task).label).toBe("Finishing cleanup");
    expect(taskStatus(row("tk_q").task)).toEqual({ label: "Interrupted", tone: "warning" });
    expect(taskStatus(row("tk_h1").task).label).toBe("Failure");
    expect(taskStatus(row("tk_h2").task).label).toBe("Cancelled");
    expect(taskStatus({ ...row("tk_e").task, continuation_pending: true }).label).toBe("Ready to resume");
    expect(taskStatus(row("tk_e").task).label).toBe("Ready to start");
  });

  it("explains what armed work waits for, by name", () => {
    expect(taskReason(row("tk_c"))).toBe("Waits for Implement API → success");
    expect(taskReason(row("tk_t"))).toBe("Waits for signal ci-green; run pr_1 → success");
    expect(taskReason(row("tk_s"))).toBe("a prerequisite can no longer be satisfied");
    expect(taskReason(row("tk_k"))).toBe("Cleanup retrying: agent did not stop");
  });
});
