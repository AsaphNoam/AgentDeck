import { create } from "zustand";
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
  setLink: (link: PhoneLink) => void;
  bump: () => void;
}

export const useConnection = create<ConnectionState>((set) => ({
  link: "checking",
  unreachableSince: null,
  revision: 0,
  setLink: (link) =>
    set((state) => ({
      link,
      unreachableSince:
        link === "unreachable" ? (state.unreachableSince ?? Date.now()) : link === "connected" ? null : state.unreachableSince,
    })),
  bump: () => set((state) => ({ revision: state.revision + 1 })),
}));

const PAIRED_KEY = "agentdeck.paired";
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
  source = new EventSource("/api/events", { withCredentials: true });
  source.onopen = () => {
    window.clearTimeout(unreachableTimer);
    unreachableTimer = undefined;
    setLink("connected");
    bump(); // catch up after reconnecting (FS-20.R17)
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
