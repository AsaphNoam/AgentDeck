import { create } from "zustand";
import type { TranscriptEvent } from "../api/types";

interface TranscriptStoreState {
  byAgent: Record<string, TranscriptEvent[]>;
  pending: Record<string, TranscriptEvent | null>;
  appendMessage: (agentId: string, event: TranscriptEvent) => void;
  setTranscript: (agentId: string, events: TranscriptEvent[]) => void;
  resolvePermission: (agentId: string, toolCallId: string) => void;
}

function kindOf(event: TranscriptEvent) {
  return event.kind ?? event.type;
}

function messageID(event: TranscriptEvent) {
  return event.message_id ?? (event.data as { message_id?: string } | undefined)?.message_id;
}

function textOf(event: TranscriptEvent) {
  return event.text ?? event.delta ?? (event.data as { text?: string; delta?: string } | undefined)?.text ?? "";
}

export const useTranscriptStore = create<TranscriptStoreState>((set) => ({
  byAgent: {},
  pending: {},
  appendMessage: (agentId, event) =>
    set((state) => {
      const events = [...(state.byAgent[agentId] ?? [])];
      const kind = kindOf(event);
      const id = messageID(event);
      const last = events[events.length - 1];
      if (kind === "assistant_text" && id && last && kindOf(last) === "assistant_text" && messageID(last) === id) {
        events[events.length - 1] = { ...last, text: `${textOf(last)}${textOf(event)}`, message_id: id, kind };
      } else {
        events.push(event);
      }
      return {
        byAgent: { ...state.byAgent, [agentId]: events },
        pending: kind === "permission_request" ? { ...state.pending, [agentId]: event } : state.pending,
      };
    }),
  setTranscript: (agentId, events) => set((state) => ({ byAgent: { ...state.byAgent, [agentId]: events } })),
  resolvePermission: (agentId) => set((state) => ({ pending: { ...state.pending, [agentId]: null } })),
}));
