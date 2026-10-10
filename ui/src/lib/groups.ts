import type { AgentState } from "../api/types";

/** Existing identity labels, projected consistently on desktop and phone. */
export function groupAgents(items: AgentState[]) {
  const map = new Map<string, AgentState[]>();
  for (const agent of items) {
    if (agent.archived) continue;
    const key = agent.group?.trim() || "_ungrouped";
    const members = map.get(key) ?? [];
    members.push(agent);
    map.set(key, members);
  }
  return [...map.entries()]
    .sort(([a], [b]) => a === b ? 0 : a === "_ungrouped" ? 1 : b === "_ungrouped" ? -1 : a.localeCompare(b))
    .map(([key, agents]) => ({
      key,
      label: key === "_ungrouped" ? "Ungrouped" : key,
      agents: [...agents.filter((agent) => agent.running), ...agents.filter((agent) => !agent.running)],
    }));
}
