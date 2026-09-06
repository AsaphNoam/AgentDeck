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
  // Whether the docked tray is reduced to its strip (FS-13.R21). It rides this
  // record rather than a second browser key so it inherits the tray's expiry,
  // cap, and delete-with-agent path instead of outliving the drafts it belongs
  // to. Only the docked form exposes the control; the overlay ignores the flag.
  collapsedBySource: Record<string, boolean>;
}

interface AnnotationStoreState extends PersistedTrays {
  add: (sourceId: string, draft: AnnotationDraft) => boolean;
  updateInstruction: (sourceId: string, index: number, instruction: string) => void;
  remove: (sourceId: string, index: number) => void;
  discard: (sourceId: string) => void;
  setOverall: (sourceId: string, instruction: string) => void;
  setCollapsed: (sourceId: string, collapsed: boolean) => void;
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
  const next: PersistedTrays = { bySource: {}, overallBySource: {}, editedAt: {}, collapsedBySource: {} };
  for (const id of keep) {
    next.bySource[id] = stored[id];
    next.editedAt[id] = editedAt[id] ?? now;
    const overall = persisted.overallBySource?.[id];
    if (overall) next.overallBySource[id] = overall;
    if (persisted.collapsedBySource?.[id]) next.collapsedBySource[id] = true;
  }
  return next;
}

export const useAnnotationStore = create<AnnotationStoreState>()(
  persist(
    (set, get) => ({
      bySource: {},
      overallBySource: {},
      editedAt: {},
      collapsedBySource: {},
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
          const { [sourceId]: _collapsed, ...collapsedBySource } = state.collapsedBySource;
          return { bySource, overallBySource, editedAt, collapsedBySource };
        }),
      setOverall: (sourceId, overall) =>
        set((state) => ({ overallBySource: { ...state.overallBySource, [sourceId]: overall }, editedAt: touch(state.editedAt, sourceId) })),
      // Collapsing hides drafts; it does not edit them, so it deliberately
      // leaves editedAt alone and lets the tray expire on its real last edit.
      setCollapsed: (sourceId, collapsed) =>
        set((state) => ({ collapsedBySource: { ...state.collapsedBySource, [sourceId]: collapsed } })),
    }),
    {
      name: "agentdeck-annotation-tray",
      partialize: (state) => ({ bySource: state.bySource, overallBySource: state.overallBySource, editedAt: state.editedAt, collapsedBySource: state.collapsedBySource }),
      merge: (persisted, current) => ({ ...current, ...pruneTrays((persisted ?? {}) as Partial<PersistedTrays>) }),
    },
  ),
);

function touch(editedAt: Record<string, number>, sourceId: string) {
  return { ...editedAt, [sourceId]: Date.now() };
}
