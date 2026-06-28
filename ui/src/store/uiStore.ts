import { create } from "zustand";

interface UiStoreState {
  density: { perRow: number; gap: number };
  connection: "connecting" | "open" | "reconnecting" | "down";
  contextMenu: { agentId: string; x: number; y: number } | null;
  setDensity: (density: { perRow: number; gap: number }) => void;
  setConnection: (connection: UiStoreState["connection"]) => void;
  openContextMenu: (agentId: string, x: number, y: number) => void;
  closeContextMenu: () => void;
}

export const useUiStore = create<UiStoreState>((set) => ({
  density: { perRow: 3, gap: 16 },
  connection: "connecting",
  contextMenu: null,
  setDensity: (density) => set({ density }),
  setConnection: (connection) => set({ connection }),
  openContextMenu: (agentId, x, y) => set({ contextMenu: { agentId, x, y } }),
  closeContextMenu: () => set({ contextMenu: null }),
}));
