/// <reference lib="webworker" />

type WorkerMessage =
  | { kind: "open" }
  | { kind: "error" }
  | { kind: "event"; type: string; data: string };

const ports = new Set<MessagePort>();
let source: EventSource | null = null;

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

function connect() {
  if (source) source.close();
  source = new EventSource("/api/events");
  source.onopen = () => broadcast({ kind: "open" });
  source.onerror = () => broadcast({ kind: "error" });
  for (const type of eventTypes) {
    source.addEventListener(type, (event) => {
      broadcast({ kind: "event", type, data: (event as MessageEvent<string>).data });
    });
  }
}

declare const self: SharedWorkerGlobalScope;

self.onconnect = (event) => {
  const port = event.ports[0];
  ports.add(port);
  port.start();
  // A newcomer missed the existing stream's snapshot. Reopening the one shared
  // stream gives every tab a fresh hydration generation without adding another
  // long-lived same-origin HTTP connection.
  connect();
};

export {};
