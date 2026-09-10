import { create } from "zustand";

// The queued follow-up (FS-03.R48) is live state on both sides. The runtime holds
// at most one per agent and it dies with the process, so this mirror is memory
// only: a browser reload rehydrates it from the runtime's live snapshot, while a
// dashboard restart correctly clears it with the runtime process.
//
// It is deliberately not merged into the transcript event list (TS-08.R56): the
// server sends no event for a message it has not delivered, so keeping it beside
// the list is what makes it structurally impossible for a live render and a
// reload to disagree about it.
interface HeldStoreState {
  byAgent: Record<string, string>;
  afterSeqByAgent: Record<string, number>;
  hold: (agentId: string, text: string, afterSeq?: number) => void;
  release: (agentId: string) => void;
}

export const useHeldStore = create<HeldStoreState>((set) => ({
  byAgent: {},
  afterSeqByAgent: {},
  hold: (agentId, text, afterSeq = 0) =>
    set((state) => ({
      byAgent: { ...state.byAgent, [agentId]: text },
      afterSeqByAgent: { ...state.afterSeqByAgent, [agentId]: afterSeq },
    })),
  release: (agentId) =>
    set((state) => {
      if (!(agentId in state.byAgent)) return state;
      const byAgent = { ...state.byAgent };
      const afterSeqByAgent = { ...state.afterSeqByAgent };
      delete byAgent[agentId];
      delete afterSeqByAgent[agentId];
      return { byAgent, afterSeqByAgent };
    }),
}));
