import { create } from "zustand";
import type { AgentState } from "../api/types";

interface AgentStoreState {
  agents: Record<string, AgentState>;
  order: string[];
  hydrating: boolean;
  applyStateUpdate: (agent: AgentState) => void;
  hydrateBegin: () => void;
  hydrateComplete: (seenIds: string[]) => void;
  removeAgent: (id: string) => void;
  setOrder: (order: string[]) => void;
}

export const useAgentStore = create<AgentStoreState>((set) => ({
  agents: {},
  order: [],
  hydrating: false,
  applyStateUpdate: (agent) =>
    set((state) => ({
      agents: { ...state.agents, [agent.agent_id]: agent },
      order: state.order.includes(agent.agent_id) ? state.order : [...state.order, agent.agent_id],
    })),
  hydrateBegin: () => set({ hydrating: true }),
  hydrateComplete: (seenIds) =>
    set((state) => {
      const seen = new Set(seenIds);
      const agents = Object.fromEntries(Object.entries(state.agents).filter(([id]) => seen.has(id)));
      return { agents, order: state.order.filter((id) => seen.has(id)), hydrating: false };
    }),
  removeAgent: (id) =>
    set((state) => {
      const { [id]: _removed, ...agents } = state.agents;
      return { agents, order: state.order.filter((item) => item !== id) };
    }),
  setOrder: (order) => set({ order }),
}));
