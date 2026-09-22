import { create } from "zustand";
import type { RuntimeActivity } from "../api/types";

// Live-only reasoning (FS-03.R57, TS-03.R44). Spans live in memory only: they
// never enter the transcript store, archive, search, annotations, context pulls
// or clone history, and reload or a reconnect discards them.

export interface ReasoningSpan {
  spanId: string;
  activityId?: string;
  text: string;
  /** Rendered transcript length when the span opened: its chronological slot. */
  anchor: number;
}

// Bound per-agent retention (INV §16): old spans and oversized text are dropped.
const MAX_SPANS = 50;
const MAX_SPAN_CHARS = 64_000;

interface ReasoningState {
  byAgent: Record<string, { generation: string; spans: ReasoningSpan[] }>;
  append: (activity: RuntimeActivity, anchor: number) => void;
  clearAll: () => void;
}

export const useReasoningStore = create<ReasoningState>((set) => ({
  byAgent: {},
  append: (activity, anchor) =>
    set((state) => {
      if (activity.kind !== "reasoning_delta" || !activity.delta) return state;
      const current = state.byAgent[activity.agent_id];
      // A new runtime generation starts from nothing (INV §1).
      const spans = current && current.generation === activity.generation ? [...current.spans] : [];
      const index = spans.findIndex((span) => span.spanId === activity.span_id && span.activityId === activity.activity_id);
      if (index >= 0) {
        const span = spans[index];
        spans[index] = { ...span, text: `${span.text}${activity.delta}`.slice(0, MAX_SPAN_CHARS) };
      } else {
        spans.push({ spanId: activity.span_id, activityId: activity.activity_id, text: activity.delta.slice(0, MAX_SPAN_CHARS), anchor });
      }
      return {
        byAgent: { ...state.byAgent, [activity.agent_id]: { generation: activity.generation, spans: spans.slice(-MAX_SPANS) } },
      };
    }),
  clearAll: () => set({ byAgent: {} }),
}));
