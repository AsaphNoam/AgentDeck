import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { AnnotationDraft } from "../api/types";

const maxDrafts = 20;

interface AnnotationStoreState {
  bySource: Record<string, AnnotationDraft[]>;
  overallBySource: Record<string, string>;
  add: (sourceId: string, draft: AnnotationDraft) => boolean;
  updateInstruction: (sourceId: string, index: number, instruction: string) => void;
  remove: (sourceId: string, index: number) => void;
  discard: (sourceId: string) => void;
  setOverall: (sourceId: string, instruction: string) => void;
}

export const useAnnotationStore = create<AnnotationStoreState>()(
  persist(
    (set, get) => ({
      bySource: {},
      overallBySource: {},
      add: (sourceId, draft) => {
        const current = get().bySource[sourceId] ?? [];
        if (current.length >= maxDrafts) return false;
        set({ bySource: { ...get().bySource, [sourceId]: [...current, draft] } });
        return true;
      },
      updateInstruction: (sourceId, index, instruction) =>
        set((state) => ({
          bySource: {
            ...state.bySource,
            [sourceId]: (state.bySource[sourceId] ?? []).map((draft, i) => i === index ? { ...draft, instruction } : draft),
          },
        })),
      remove: (sourceId, index) =>
        set((state) => ({ bySource: { ...state.bySource, [sourceId]: (state.bySource[sourceId] ?? []).filter((_, i) => i !== index) } })),
      discard: (sourceId) =>
        set((state) => {
          const { [sourceId]: _drafts, ...bySource } = state.bySource;
          const { [sourceId]: _overall, ...overallBySource } = state.overallBySource;
          return { bySource, overallBySource };
        }),
      setOverall: (sourceId, overall) => set((state) => ({ overallBySource: { ...state.overallBySource, [sourceId]: overall } })),
    }),
    { name: "agentdeck-annotation-tray", partialize: (state) => ({ bySource: state.bySource, overallBySource: state.overallBySource }) },
  ),
);
