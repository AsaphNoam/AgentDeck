import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { AnnotationDraft } from "../api/types";

const maxDrafts = 20;
// Retention for stored trays (FS-13.R16). Nothing on the server owns this state,
// so without a bound an abandoned tray lives forever and the trays of every
// agent a browser ever opened keep consuming the shared localStorage quota —
// once it is full, zustand's persist throws on every setState.
const maxSources = 20;
const maxTrayAgeMs = 30 * 24 * 60 * 60 * 1000;

interface PersistedTrays {
  bySource: Record<string, AnnotationDraft[]>;
  overallBySource: Record<string, string>;
  editedAt: Record<string, number>;
}

interface AnnotationStoreState extends PersistedTrays {
  add: (sourceId: string, draft: AnnotationDraft) => boolean;
  updateInstruction: (sourceId: string, index: number, instruction: string) => void;
  remove: (sourceId: string, index: number) => void;
  discard: (sourceId: string) => void;
  setOverall: (sourceId: string, instruction: string) => void;
}

// pruneTrays applies FS-13.R16 to what a reload restored: drop empty trays, drop
// trays untouched for maxTrayAgeMs, and keep only the most recently edited
// maxSources. A tray stored by an older build carries no timestamp; treat it as
// touched now so an upgrade never discards a draft the user is still working on.
export function pruneTrays(persisted: Partial<PersistedTrays>, now = Date.now()): PersistedTrays {
  const stored = persisted.bySource ?? {};
  const editedAt = persisted.editedAt ?? {};
  const keep = Object.keys(stored)
    .filter((id) => (stored[id] ?? []).length > 0 && now - (editedAt[id] ?? now) <= maxTrayAgeMs)
    .sort((a, b) => (editedAt[b] ?? now) - (editedAt[a] ?? now))
    .slice(0, maxSources);
  const next: PersistedTrays = { bySource: {}, overallBySource: {}, editedAt: {} };
  for (const id of keep) {
    next.bySource[id] = stored[id];
    next.editedAt[id] = editedAt[id] ?? now;
    const overall = persisted.overallBySource?.[id];
    if (overall) next.overallBySource[id] = overall;
  }
  return next;
}

export const useAnnotationStore = create<AnnotationStoreState>()(
  persist(
    (set, get) => ({
      bySource: {},
      overallBySource: {},
      editedAt: {},
      add: (sourceId, draft) => {
        const current = get().bySource[sourceId] ?? [];
        if (current.length >= maxDrafts) return false;
        set({ bySource: { ...get().bySource, [sourceId]: [...current, draft] }, editedAt: touch(get().editedAt, sourceId) });
        return true;
      },
      updateInstruction: (sourceId, index, instruction) =>
        set((state) => ({
          bySource: {
            ...state.bySource,
            [sourceId]: (state.bySource[sourceId] ?? []).map((draft, i) => i === index ? { ...draft, instruction } : draft),
          },
          editedAt: touch(state.editedAt, sourceId),
        })),
      remove: (sourceId, index) =>
        set((state) => ({
          bySource: { ...state.bySource, [sourceId]: (state.bySource[sourceId] ?? []).filter((_, i) => i !== index) },
          editedAt: touch(state.editedAt, sourceId),
        })),
      discard: (sourceId) =>
        set((state) => {
          const { [sourceId]: _drafts, ...bySource } = state.bySource;
          const { [sourceId]: _overall, ...overallBySource } = state.overallBySource;
          const { [sourceId]: _edited, ...editedAt } = state.editedAt;
          return { bySource, overallBySource, editedAt };
        }),
      setOverall: (sourceId, overall) =>
        set((state) => ({ overallBySource: { ...state.overallBySource, [sourceId]: overall }, editedAt: touch(state.editedAt, sourceId) })),
    }),
    {
      name: "agentdeck-annotation-tray",
      partialize: (state) => ({ bySource: state.bySource, overallBySource: state.overallBySource, editedAt: state.editedAt }),
      merge: (persisted, current) => ({ ...current, ...pruneTrays((persisted ?? {}) as Partial<PersistedTrays>) }),
    },
  ),
);

function touch(editedAt: Record<string, number>, sourceId: string) {
  return { ...editedAt, [sourceId]: Date.now() };
}
