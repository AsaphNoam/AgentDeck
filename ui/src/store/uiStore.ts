import { create } from "zustand";
import type { NotificationPayload } from "../api/types";

export interface ToastItem {
  id: string;
  title: string;
  body?: string;
  type: NotificationPayload["notification_type"] | "error";
}

interface UiStoreState {
  density: { perRow: number; gap: number };
  groupLayout: Record<string, { collapsed: boolean }>;
  connection: "connecting" | "open" | "reconnecting" | "down";
  contextMenu: { agentId: string; x: number; y: number } | null;
  toasts: ToastItem[];
  setDensity: (density: { perRow: number; gap: number }) => void;
  setGroupLayout: (groups: Record<string, { collapsed: boolean }>) => void;
  toggleGroupCollapsed: (group: string) => void;
  setConnection: (connection: UiStoreState["connection"]) => void;
  openContextMenu: (agentId: string, x: number, y: number) => void;
  closeContextMenu: () => void;
  pushToast: (notification: NotificationPayload) => void;
  pushError: (title: string, body?: string) => void;
  dismissToast: (id: string) => void;
}

export const useUiStore = create<UiStoreState>((set) => ({
  density: { perRow: 3, gap: 16 },
  groupLayout: {},
  connection: "connecting",
  contextMenu: null,
  toasts: [],
  setDensity: (density) => set({ density }),
  setGroupLayout: (groupLayout) => set({ groupLayout }),
  toggleGroupCollapsed: (group) =>
    set((state) => ({
      groupLayout: {
        ...state.groupLayout,
        [group]: { collapsed: !state.groupLayout[group]?.collapsed },
      },
    })),
  setConnection: (connection) => set({ connection }),
  openContextMenu: (agentId, x, y) => set({ contextMenu: { agentId, x, y } }),
  closeContextMenu: () => set({ contextMenu: null }),
  pushToast: (notification) =>
    set((state) => {
      const id = `${notification.agent_id}-${notification.notification_type}-${notification.ts}`;
      return {
        toasts: [
          ...state.toasts.filter((toast) => toast.id !== id),
          { id, title: notification.title, body: notification.body, type: notification.notification_type },
        ].slice(-4),
      };
    }),
  pushError: (title, body) =>
    set((state) => {
      const toast: ToastItem = {
        id: `error-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
        title,
        body,
        type: "error",
      };
      return { toasts: [...state.toasts, toast].slice(-4) };
    }),
  dismissToast: (id) => set((state) => ({ toasts: state.toasts.filter((toast) => toast.id !== id) })),
}));
