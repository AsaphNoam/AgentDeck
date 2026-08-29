import { create } from "zustand";
import type { PermissionResolution, TranscriptEvent } from "../api/types";

interface TranscriptStoreState {
  byAgent: Record<string, TranscriptEvent[]>;
  rawByAgent: Record<string, TranscriptEvent[]>;
  pending: Record<string, TranscriptEvent | null>;
  previewByAgent: Record<string, string>;
  previewKindByAgent: Record<string, string | undefined>;
  appendMessage: (agentId: string, event: TranscriptEvent) => void;
  updatePreview: (agentId: string, event: TranscriptEvent) => void;
  beginReconciliation: (agentId: string) => void;
  endReconciliation: (agentId: string) => void;
  discardAgent: (agentId: string) => void;
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

// Preserve every user-visible outcome while grouping automatic approval with
// explicit approval. Unknown legacy values retain the conservative Denied state.
function decisionToResolved(decision: unknown): PermissionResolution {
  if (decision === "approve" || decision === "auto_approve") return "approve";
  if (decision === "cancelled") return "cancelled";
  if (decision === "timeout") return "timeout";
  return "deny";
}

function markResolved(
  events: TranscriptEvent[],
  toolCallId: string,
  resolved: PermissionResolution,
) {
  const next = [...events];
  for (let i = next.length - 1; i >= 0; i--) {
    const event = next[i];
    if (kindOf(event) === "permission_request" && String(event.tool_call_id ?? "") === toolCallId) {
      next[i] = { ...event, resolved };
      break;
    }
  }
  return next;
}

function appendRenderedEvent(events: TranscriptEvent[], event: TranscriptEvent) {
  const last = events[events.length - 1];
  if (kindOf(event) === "assistant_text" && last && kindOf(last) === "assistant_text") {
    events[events.length - 1] = {
      ...last,
      kind: "assistant_text",
      text: `${textOf(last)}${textOf(event)}`,
    };
    return;
  }
  events.push(event);
}

// foldTranscript normalizes a full event list, coalesces consecutive assistant
// deltas, and folds each permission_resolved into its matching prior request.
// Live append folds incrementally; REST refetch/archive replay must preserve the
// same rendered message boundaries and resolved permission state.
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
    appendRenderedEvent(out, event);
  }
  return out;
}

export const useTranscriptStore = create<TranscriptStoreState>((set) => ({
  byAgent: {},
  rawByAgent: {},
  pending: {},
  previewByAgent: {},
  previewKindByAgent: {},
  appendMessage: (agentId, raw) =>
    set((state) => {
      const event = normalizeEvent(raw);
      const kind = kindOf(event);
      // This is a short-lived reconciliation tail, not a second transcript.
      // Mutate its private array in place so a burst received during a fetch is
      // linear rather than copying the growing tail for every delta.
      const rawEvents = state.rawByAgent[agentId];

      // permission_resolved is not rendered on its own; it updates the matching
      // prior permission_request (covers replay of archived/resumed sessions).
      if (kind === "permission_resolved") {
        rawEvents?.push(event);
        const toolCallId = String(event.tool_call_id ?? "");
        const events = markResolved(state.byAgent[agentId] ?? [], toolCallId, decisionToResolved(event.decision));
        return {
          byAgent: { ...state.byAgent, [agentId]: events },
          pending: { ...state.pending, [agentId]: null },
        };
      }

      const events = [...(state.byAgent[agentId] ?? [])];
	      // The composer displays a local user bubble immediately. Replace that
	      // exact unsequenced bubble when its durable runtime event arrives so a
	      // live SSE delivery does not render the prompt twice.
      if (kind === "user_text") {
        for (let i = events.length - 1; i >= 0; i--) {
          if (kindOf(events[i]) === "user_text" && events[i].seq == null && textOf(events[i]) === textOf(event)) {
            events[i] = event;
            const rawIndex = rawEvents?.findIndex((item) => kindOf(item) === "user_text" && item.seq == null && textOf(item) === textOf(event)) ?? -1;
            if (rawEvents && rawIndex >= 0) rawEvents[rawIndex] = event;
            else rawEvents?.push(event);
            return { byAgent: { ...state.byAgent, [agentId]: events }, pending: state.pending };
          }
        }
      }
      // Streamed assistant deltas carry no message_id; merge consecutive
      // assistant_text events into a single bubble on the shared replay/live path.
      appendRenderedEvent(events, event);
      rawEvents?.push(event);
      return {
        byAgent: { ...state.byAgent, [agentId]: events },
        pending: kind === "permission_request" ? { ...state.pending, [agentId]: event } : state.pending,
      };
    }),
  updatePreview: (agentId, raw) =>
    set((state) => {
      const event = normalizeEvent(raw);
      const kind = kindOf(event);
      const nextKinds = { ...state.previewKindByAgent, [agentId]: kind };
      if (kind !== "assistant_text") return { previewKindByAgent: nextKinds };
      const delta = String(textOf(event));
      const prior = state.previewKindByAgent[agentId] === "assistant_text" ? state.previewByAgent[agentId] ?? "" : "";
      const preview = `${prior}${delta}`.trim().slice(-120);
      return {
        previewByAgent: { ...state.previewByAgent, [agentId]: preview },
        previewKindByAgent: nextKinds,
      };
    }),
  beginReconciliation: (agentId) =>
    set((state) => {
      if (state.rawByAgent[agentId]) return state;
      const optimistic = (state.byAgent[agentId] ?? []).filter((event) => event.seq == null);
      return { rawByAgent: { ...state.rawByAgent, [agentId]: optimistic } };
    }),
  endReconciliation: (agentId) =>
    set((state) => {
      if (!state.rawByAgent[agentId]) return state;
      const rawByAgent = { ...state.rawByAgent };
      delete rawByAgent[agentId];
      return { rawByAgent };
    }),
  discardAgent: (agentId) =>
    set((state) => {
      const byAgent = { ...state.byAgent };
      const rawByAgent = { ...state.rawByAgent };
      const pending = { ...state.pending };
      delete byAgent[agentId];
      delete rawByAgent[agentId];
      delete pending[agentId];
      return { byAgent, rawByAgent, pending };
    }),
  setTranscript: (agentId, events) =>
    set((state) => {
      const responseMaxSeq = events.reduce((max, event) => Math.max(max, Number(event.seq ?? 0)), 0);
      const newerDelivered = (state.rawByAgent[agentId] ?? []).filter(
        (event) => event.seq == null || Number(event.seq) > responseMaxSeq,
      );
      const reconciledRaw = [...events.map(normalizeEvent), ...newerDelivered];
      let folded = foldTranscript(reconciledRaw);
      for (const current of state.byAgent[agentId] ?? []) {
        if (kindOf(current) !== "permission_request" || !current.resolved) continue;
        folded = markResolved(folded, String(current.tool_call_id ?? ""), current.resolved as PermissionResolution);
      }
      let preview = "";
      for (let i = folded.length - 1; i >= 0; i--) {
        if (kindOf(folded[i]) === "assistant_text") {
          preview = String(textOf(folded[i])).trim().slice(-120);
          break;
        }
      }
      const rawByAgent = { ...state.rawByAgent };
      delete rawByAgent[agentId];
      return {
        byAgent: { ...state.byAgent, [agentId]: folded },
        rawByAgent,
        previewByAgent: { ...state.previewByAgent, [agentId]: preview },
        previewKindByAgent: { ...state.previewKindByAgent, [agentId]: kindOf(folded[folded.length - 1] ?? {}) },
      };
    }),
  resolvePermission: (agentId, toolCallId, decision) =>
    set((state) => {
      const raw = state.rawByAgent[agentId];
      return {
        byAgent: { ...state.byAgent, [agentId]: markResolved(state.byAgent[agentId] ?? [], toolCallId, decision) },
        rawByAgent: raw ? { ...state.rawByAgent, [agentId]: markResolved(raw, toolCallId, decision) } : state.rawByAgent,
        pending: { ...state.pending, [agentId]: null },
      };
    }),
}));
