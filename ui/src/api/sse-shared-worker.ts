/// <reference lib="webworker" />

type WorkerMessage =
  | { kind: "open" }
  | { kind: "error" }
  | { kind: "event"; type: string; data: string };

type ClientMessage = { kind: "bye" };

const ports = new Set<MessagePort>();
let source: EventSource | null = null;
let opened = false;
// The hydration burst, retained so a joining tab can be caught up on its own
// port. One entry per agent id (rows overwrite, removals delete), so this stays
// the size of the live agent set rather than growing with stream traffic.
const snapshot = new Map<string, string>();
let hydratedRow: string | null = null;

const eventTypes = [
  "state_update",
  "new_message",
  "notification",
  "pipeline_update",
  "pipeline_proposal_update",
  "task_update",
  "config_source_update",
  "ping",
];

function broadcast(message: WorkerMessage) {
  for (const port of ports) port.postMessage(message);
}

function retain(data: string) {
  let envelope: { agent_id?: string | null; data?: { hydrated?: boolean; removed?: boolean } };
  try {
    envelope = JSON.parse(data);
  } catch {
    return;
  }
  if (envelope.data?.hydrated) {
    hydratedRow = data;
    return;
  }
  if (!envelope.agent_id) return;
  if (envelope.data?.removed) snapshot.delete(envelope.agent_id);
  else snapshot.set(envelope.agent_id, data);
}

function connect() {
  if (source) source.close();
  opened = false;
  snapshot.clear();
  hydratedRow = null;
  source = new EventSource("/api/events");
  source.onopen = () => {
    // Every open is a new hydration generation on the wire; the retained
    // snapshot from the previous one is derived state and must not survive it.
    opened = true;
    snapshot.clear();
    hydratedRow = null;
    broadcast({ kind: "open" });
  };
  source.onerror = () => {
    opened = false;
    broadcast({ kind: "error" });
  };
  for (const type of eventTypes) {
    source!.addEventListener(type, (event) => {
      const data = (event as MessageEvent<string>).data;
      if (type === "state_update") retain(data);
      broadcast({ kind: "event", type, data });
    });
  }
}

// Every port added at attach is removed here, on the client's explicit close and
// on unload, and the shared stream it feeds is torn down with the last one
// (INV §4).
function release(port: MessagePort) {
  if (!ports.delete(port)) return;
  port.close();
  if (ports.size > 0) return;
  source?.close();
  source = null;
  opened = false;
  snapshot.clear();
  hydratedRow = null;
}

declare const self: SharedWorkerGlobalScope;

self.onconnect = (event) => {
  const port = event.ports[0];
  ports.add(port);
  port.onmessage = (message: MessageEvent<ClientMessage>) => {
    if (message.data?.kind === "bye") release(port);
  };
  port.start();
  // A newcomer missed the hydration burst the live stream already delivered.
  // Replaying it on this port alone catches the newcomer up without restarting
  // the shared stream, which would cost every other tab a full re-hydration and
  // drop deltas for all of them (TS-03.R7). A stream that is gone — or that the
  // browser has stopped retrying — is reopened instead.
  if (!source || source.readyState === 2 /* CLOSED */) {
    connect();
    return;
  }
  if (!opened) return;
  port.postMessage({ kind: "open" });
  for (const row of snapshot.values()) port.postMessage({ kind: "event", type: "state_update", data: row });
  if (hydratedRow) port.postMessage({ kind: "event", type: "state_update", data: hydratedRow });
};

export {};
