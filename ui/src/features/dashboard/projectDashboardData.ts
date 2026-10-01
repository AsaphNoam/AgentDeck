import type { AgentState } from "../../api/types";
type DashboardProjectConfig = { title: string; color: [number, number, number]; archived?: boolean };

export type DashboardProject = {
  id: string;
  title: string;
  color?: [number, number, number];
  unavailable: boolean;
  agents: AgentState[];
  stateSummary: string;
};

/** The dashboard and phone deliberately share this projection: a project card
 * is configuration plus its live, non-archived agents, never a second query. */
export function deriveDashboardProjects(
  projects: Record<string, DashboardProjectConfig> | undefined,
  agents: Record<string, AgentState>,
): DashboardProject[] {
  const active = Object.entries(projects ?? {})
    .filter(([, project]) => !project.archived)
    .map(([id, project]) => ({ id, title: project.title, color: project.color, unavailable: false }));
  const missing = [...new Set(Object.values(agents)
    .filter((agent) => !agent.archived && !projects?.[agent.project])
    .map((agent) => agent.project))]
    .map((id) => ({ id, title: id, unavailable: true }));
  return [...active, ...missing].map((project) => {
    const projectAgents = orderProjectAgents(Object.values(agents).filter((agent) => agent.project === project.id && !agent.archived));
    const counts = projectAgents.reduce<Record<string, number>>((current, agent) => ({ ...current, [agent.state]: (current[agent.state] ?? 0) + 1 }), {});
    return { ...project, agents: projectAgents, stateSummary: Object.entries(counts).map(([state, count]) => `${count} ${state}`).join(" · ") || "No agents" };
  });
}

/** FS-02.R45: running first; preserving snapshot order within each block. */
export function orderProjectAgents(agents: AgentState[]): AgentState[] {
  return [...agents].sort((a, b) => Number(b.running) - Number(a.running));
}
