import { create } from "zustand";
import type { TranscriptEvent } from "../api/types";

interface TranscriptStoreState {
  byAgent: Record<string, TranscriptEvent[]>;
  pending: Record<string, TranscriptEvent | null>;
  appendMessage: (agentId: string, event: TranscriptEvent) => void;
  setTranscript: (agentId: string, events: TranscriptEvent[]) => void;
  resolvePermission: (agentId: string, toolCallId: string, decision: "approve" | "deny") => void;
}

// normalizeEvent flattens the raw runtime wire shape ({type, data:{...}}) into the
// render-ready shape ({kind, ...payload}). Events authored locally (Composer's
// user message, unit tests) already use `kind` with fields at the top level and
// pass through unchanged — making this idempotent.
export function normalizeEvent(event: TranscriptEvent): TranscriptEvent {
  if (event.kind && event.type === undefined) return event;
  const type = (event.type ?? event.kind) as string | undefined;
  const data = event.data;
  if (data && typeof data === "object") {
    return { kind: type, seq: event.seq, ts: event.ts, ...(data as Record<string, unknown>) };
  }
  const { type: _drop, ...rest } = event;
  return { ...rest, kind: type };
}

function kindOf(event: TranscriptEvent) {
  return event.kind ?? event.type;
}

function textOf(event: TranscriptEvent) {
  return event.text ?? event.delta ?? "";
}

// A runtime permission decision ("approve"|"deny"|"timeout"|"auto_approve") maps
// onto the two-state chip the prompt renders.
function decisionToResolved(decision: unknown): "approve" | "deny" {
  return decision === "approve" || decision === "auto_approve" ? "approve" : "deny";
}

function markResolved(
  events: TranscriptEvent[],
  toolCallId: string,
  resolved: "approve" | "deny",
) {
  return events.map((event) =>
    kindOf(event) === "permission_request" && String(event.tool_call_id ?? "") === toolCallId
      ? { ...event, resolved }
      : event,
  );
}

// foldTranscript normalizes a full event list AND folds each permission_resolved
// into its matching prior permission_request (then drops the resolution event,
// which is never rendered on its own). Live append folds incrementally; a REST
// refetch / archive reload replays the whole list, so it must fold the same way
// or a resolved request would render as still-pending.
export function foldTranscript(raw: TranscriptEvent[] | null | undefined): TranscriptEvent[] {
  const out: TranscriptEvent[] = [];
  for (const r of raw ?? []) {
    const event = normalizeEvent(r);
    if (kindOf(event) === "permission_resolved") {
      const toolCallId = String(event.tool_call_id ?? "");
      for (let i = out.length - 1; i >= 0; i--) {
        if (kindOf(out[i]) === "permission_request" && String(out[i].tool_call_id ?? "") === toolCallId) {
          out[i] = { ...out[i], resolved: decisionToResolved(event.decision) };
          break;
        }
      }
      continue;
    }
    out.push(event);
  }
  return out;
}

export const useTranscriptStore = create<TranscriptStoreState>((set) => ({
  byAgent: {},
  pending: {},
  appendMessage: (agentId, raw) =>
    set((state) => {
      const event = normalizeEvent(raw);
      const kind = kindOf(event);

      // permission_resolved is not rendered on its own; it updates the matching
      // prior permission_request (covers replay of archived/resumed sessions).
      if (kind === "permission_resolved") {
        const toolCallId = String(event.tool_call_id ?? "");
        const events = markResolved(state.byAgent[agentId] ?? [], toolCallId, decisionToResolved(event.decision));
        return {
          byAgent: { ...state.byAgent, [agentId]: events },
          pending: { ...state.pending, [agentId]: null },
        };
      }

      const events = [...(state.byAgent[agentId] ?? [])];
      const last = events[events.length - 1];
      // Streamed assistant deltas carry no message_id; merge consecutive
      // assistant_text events into a single bubble.
      if (kind === "assistant_text" && last && kindOf(last) === "assistant_text") {
        events[events.length - 1] = { ...last, kind, text: `${textOf(last)}${textOf(event)}` };
      } else {
        events.push(event);
      }
      return {
        byAgent: { ...state.byAgent, [agentId]: events },
        pending: kind === "permission_request" ? { ...state.pending, [agentId]: event } : state.pending,
      };
    }),
  setTranscript: (agentId, events) =>
    set((state) => ({ byAgent: { ...state.byAgent, [agentId]: foldTranscript(events) } })),
  resolvePermission: (agentId, toolCallId, decision) =>
    set((state) => ({
      byAgent: { ...state.byAgent, [agentId]: markResolved(state.byAgent[agentId] ?? [], toolCallId, decision) },
      pending: { ...state.pending, [agentId]: null },
    })),
}));
