import { create } from "zustand";
import type { AgentState, RuntimeActivity, TranscriptEvent } from "../api/types";
import { openTurnKey } from "../components/chat/turnActivity";
import { useAnnotationStore } from "../store/annotationStore";
import { useReasoningStore } from "../store/reasoningStore";
import { checkPaired, MacUnreachableError, PhoneAPIError } from "./api";

// The phone's view of its link to the Mac (FS-20 §3). A plain EventSource with
// the device cookie; the desktop SharedWorker is not used (TS-13.R14).

export type PhoneLink = "checking" | "connected" | "reconnecting" | "unreachable" | "unpaired";

interface ConnectionState {
  link: PhoneLink;
  /** When the Mac stopped answering; set while unreachable (FS-20.R23). */
  unreachableSince: number | null;
  /** Bumped on every event that can change Home, for debounced refetches. */
  revision: number;
  /** Every agent, hydrated from the stream's state_update snapshot. */
  agents: Record<string, AgentState>;
  /** Per-agent counter bumped when its transcript grows. */
  transcriptRev: Record<string, number>;
  setLink: (link: PhoneLink) => void;
  bump: () => void;
}

export const useConnection = create<ConnectionState>((set) => ({
  link: "checking",
  unreachableSince: null,
  revision: 0,
  agents: {},
  transcriptRev: {},
  setLink: (link) =>
    set((state) => ({
      link,
      unreachableSince:
        link === "unreachable" ? (state.unreachableSince ?? Date.now()) : link === "connected" ? null : state.unreachableSince,
    })),
  bump: () => set((state) => ({ revision: state.revision + 1 })),
}));

const PAIRED_KEY = "chuck.paired";
export const rememberPaired = (paired: boolean) =>
  paired ? localStorage.setItem(PAIRED_KEY, "1") : localStorage.removeItem(PAIRED_KEY);
export const wasPaired = () => localStorage.getItem(PAIRED_KEY) === "1";

/** Events that can change what Home shows. */
const homeEvents = ["state_update", "task_update", "pipeline_update", "notification"];
const unreachableAfterMs = 5000;

let source: EventSource | null = null;
let unreachableTimer: number | undefined;

/** classify asks the Mac whether this phone is still paired. */
export async function classify(): Promise<PhoneLink> {
  const { setLink } = useConnection.getState();
  try {
    await checkPaired();
    setLink("connected");
    return "connected";
  } catch (error) {
    if (error instanceof PhoneAPIError && error.status === 401) {
      disconnect();
      setLink("unpaired");
      return "unpaired";
    }
    if (error instanceof MacUnreachableError) {
      setLink("unreachable");
      return "unreachable";
    }
    throw error;
  }
}

export function connect() {
  if (source) return;
  const { setLink, bump } = useConnection.getState();
  let hydratingAgents: Record<string, AgentState> | null = {};
  source = new EventSource("/api/events", { withCredentials: true });
  source.onopen = () => {
    window.clearTimeout(unreachableTimer);
    unreachableTimer = undefined;
    hydratingAgents = {};
    // Live-only reasoning and its live choices have no seq to recover (TS-08.R103).
    // No thought is admitted again until hydration ends and each open window
    // has re-read past what the disconnect may have missed.
    useReasoningStore.getState().clearAll();
    admitting = false;
    setLink("reconnecting");
  };
  source.onerror = () => {
    if (useConnection.getState().link === "connected") setLink("reconnecting");
    if (unreachableTimer === undefined) {
      unreachableTimer = window.setTimeout(() => {
        unreachableTimer = undefined;
        if (useConnection.getState().link !== "connected") void classify().catch(() => setLink("unreachable"));
      }, unreachableAfterMs);
    }
  };
  for (const type of homeEvents) source.addEventListener(type, () => bump());
  source.addEventListener("state_update", (event) => {
    const envelope = parse(event);
    // The server marks hydration on the envelope: {agent_id:"__hydrated__", data:{hydrated:true}}.
    if (envelope?.agent_id === "__hydrated__") {
      const agents = hydratingAgents ?? {};
      hydratingAgents = null;
      useConnection.setState((state) => {
        const transcriptRev = Object.fromEntries(Object.entries(state.transcriptRev).filter(([id]) => agents[id]));
        for (const id of openTranscripts.keys()) {
          transcriptRev[id] = (transcriptRev[id] ?? 0) + 1;
          fenceRev.set(id, transcriptRev[id]);
        }
        return { agents, transcriptRev };
      });
      admitting = true;
      setLink("connected");
      bump(); // catch up only after the authoritative snapshot is complete.
      return;
    }
    const agent = envelope?.data as AgentState | undefined;
    if (!agent?.agent_id) return;
    if (hydratingAgents) {
      if (agent.removed) delete hydratingAgents[agent.agent_id];
      else hydratingAgents[agent.agent_id] = agent;
      return;
    }
    // A deleted agent can no longer be an annotation source (FS-13.R16).
    if (agent.removed) useAnnotationStore.getState().discard(agent.agent_id);
    useConnection.setState((state) => {
      const agents = { ...state.agents };
      if (agent.removed) delete agents[agent.agent_id];
      else agents[agent.agent_id] = agent;
      return { agents };
    });
  });
  source.addEventListener("runtime_activity", admitReasoning);
  source.addEventListener("new_message", (event) => {
    if (hydratingAgents) return;
    const id = parse(event)?.agent_id;
    if (!id) return;
    useConnection.setState((state) => ({ transcriptRev: { ...state.transcriptRev, [id]: (state.transcriptRev[id] ?? 0) + 1 } }));
  });
}

// Open conversations admit live reasoning, as the desktop's open agents do
// (TS-08.R104). Each reads its current folded window on arrival.
// A read returns null unless it holds an exact (not placeholder) window read,
// with the revision that read answered. Leaving the conversation drops its
// thoughts and live choices with it (TS-08.R102).
export interface OpenTranscript {
  events: TranscriptEvent[];
  /** The leading turn's carried key (carryLeadKey). */
  lead: string;
  rev: number;
}

const openTranscripts = new Map<string, () => OpenTranscript | null>();
// The revision each open window must reach after the latest hydration.
const fenceRev = new Map<string, number>();
let admitting = false;

export function watchReasoning(agentId: string, read: () => OpenTranscript | null): () => void {
  openTranscripts.set(agentId, read);
  return () => {
    if (openTranscripts.get(agentId) !== read) return;
    openTranscripts.delete(agentId);
    fenceRev.delete(agentId);
    useReasoningStore.getState().discard(agentId);
  };
}

// The phone's window slides as the conversation grows, so a span anchors after
// the last seq the screen had seen rather than at a list position.
function admitReasoning(event: Event) {
  const activity = parse(event)?.data as RuntimeActivity | undefined;
  const read = activity?.agent_id ? openTranscripts.get(activity.agent_id) : undefined;
  const view = admitting ? read?.() : null;
  if (!activity || !view || view.rev < (fenceRev.get(activity.agent_id) ?? 0)) return;
  const last = [...view.events].reverse().find((item) => typeof item.seq === "number")?.seq ?? 0;
  useReasoningStore.getState().append(activity, Number(last), openTurnKey(view.events, view.lead));
}

function parse(event: Event): { agent_id?: string; data?: unknown } | null {
  try {
    return JSON.parse((event as MessageEvent<string>).data) as { agent_id?: string; data?: unknown };
  } catch {
    return null;
  }
}

export function disconnect() {
  window.clearTimeout(unreachableTimer);
  unreachableTimer = undefined;
  source?.close();
  source = null;
}

/** onEvent lets a view refresh on specific stream events (for example its transcript). */
export function onEvent(type: string, handler: (event: MessageEvent<string>) => void): () => void {
  const current = source;
  if (!current) return () => undefined;
  const listener = (event: Event) => handler(event as MessageEvent<string>);
  current.addEventListener(type, listener);
  return () => current.removeEventListener(type, listener);
}
