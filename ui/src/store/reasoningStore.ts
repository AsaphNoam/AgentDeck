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
  /** The open root turn at admission (TS-08.R103); a repeated span id in a
   * later turn starts a new span instead of growing a finished one. */
  turn?: string;
}

// Bound per-agent retention (INV §16): old spans and oversized text are dropped.
const MAX_SPANS = 50;
const MAX_SPAN_CHARS = 64_000;
const MAX_CHOICES = 256;

interface AgentReasoning {
  generation: string;
  spans: ReasoningSpan[];
  /** `${turn}|${activityId}` scopes the person collapsed while live (FS-03.R73). */
  collapsed: string[];
}

interface ReasoningState {
  byAgent: Record<string, AgentReasoning>;
  append: (activity: RuntimeActivity, anchor: number, turn?: string) => void;
  setCollapsed: (agentId: string, scope: string, collapsed: boolean) => void;
  clearAll: () => void;
}

export function thoughtScope(turn: string, activityId?: string) {
  return `${turn}|${activityId ?? ""}`;
}

export const useReasoningStore = create<ReasoningState>((set) => ({
  byAgent: {},
  append: (activity, anchor, turn) =>
    set((state) => {
      if (activity.kind !== "reasoning_delta" || !activity.delta) return state;
      const current = state.byAgent[activity.agent_id];
      // A new runtime generation starts from nothing, live choices included (INV §1).
      const same = current && current.generation === activity.generation;
      const spans = same ? [...current.spans] : [];
      const index = spans.findIndex((span) => span.spanId === activity.span_id && span.activityId === activity.activity_id && span.turn === turn);
      if (index >= 0) {
        const span = spans[index];
        spans[index] = { ...span, text: `${span.text}${activity.delta}`.slice(0, MAX_SPAN_CHARS) };
      } else {
        spans.push({ spanId: activity.span_id, activityId: activity.activity_id, text: activity.delta.slice(0, MAX_SPAN_CHARS), anchor, turn });
      }
      return {
        byAgent: { ...state.byAgent, [activity.agent_id]: { generation: activity.generation, spans: spans.slice(-MAX_SPANS), collapsed: same ? current.collapsed : [] } },
      };
    }),
  setCollapsed: (agentId, scope, collapsed) =>
    set((state) => {
      const current = state.byAgent[agentId];
      if (!current) return state;
      const kept = current.collapsed.filter((item) => item !== scope);
      const next = collapsed ? [...kept, scope].slice(-MAX_CHOICES) : kept;
      return { byAgent: { ...state.byAgent, [agentId]: { ...current, collapsed: next } } };
    }),
  clearAll: () => set({ byAgent: {} }),
}));
