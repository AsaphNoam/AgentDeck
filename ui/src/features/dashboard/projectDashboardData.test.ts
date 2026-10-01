import { describe, expect, it } from "vitest";
import { deriveDashboardProjects } from "./projectDashboardData";

const agent = (id: string, project: string, running: boolean, state = "idle") => ({
  agent_id: id, project, running, state, archived: false,
} as never);

describe("deriveDashboardProjects", () => {
  it("keeps active empty projects, derives unavailable projects, and puts running agents first", () => {
    const projects = { alpha: { title: "Alpha", color: [1, 2, 3] as [number, number, number] }, archived: { title: "Archived", color: [3, 2, 1] as [number, number, number], archived: true } };
    const entries = deriveDashboardProjects(projects, { stopped: agent("stopped", "alpha", false, "done"), live: agent("live", "alpha", true, "busy"), missing: agent("missing", "gone", true) });
    expect(entries.map((entry) => entry.id)).toEqual(["alpha", "gone"]);
    expect(entries[0].agents.map((entry) => entry.agent_id)).toEqual(["live", "stopped"]);
    expect(entries[0].stateSummary).toBe("1 busy · 1 done");
    expect(entries[1]).toMatchObject({ unavailable: true, title: "gone" });
  });
});
