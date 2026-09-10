import { create } from "zustand";

// The queued follow-up (FS-03.R48) is live state on both sides. The runtime holds
// at most one per agent and it dies with the process, so this mirror is memory
// only: a reload correctly starts empty rather than restoring a pending badge for
// a message the server may already have sent.
//
// It is deliberately not merged into the transcript event list (TS-08.R56): the
// server sends no event for a message it has not delivered, so keeping it beside
// the list is what makes it structurally impossible for a live render and a
// reload to disagree about it.
interface HeldStoreState {
  byAgent: Record<string, string>;
  hold: (agentId: string, text: string) => void;
  release: (agentId: string) => void;
}

export const useHeldStore = create<HeldStoreState>((set) => ({
  byAgent: {},
  hold: (agentId, text) =>
    set((state) => ({ byAgent: { ...state.byAgent, [agentId]: text } })),
  release: (agentId) =>
    set((state) => {
      if (!(agentId in state.byAgent)) return state;
      const byAgent = { ...state.byAgent };
      delete byAgent[agentId];
      return { byAgent };
    }),
}));
