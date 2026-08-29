import { getTranscript } from "./client";
import { QUERY_KEYS, queryClient } from "./config";
import { PIPELINE_QUERY_KEYS } from "./pipelines";
import { TASK_QUERY_KEYS } from "./tasks";
import type { Config } from "../schemas/config";
import type { AgentState, BusEvent, NotificationPayload, TranscriptEvent } from "./types";
import { pipelineUpdateSchema, type PipelineRunDetail } from "../schemas/pipeline";
import { useAgentStore } from "../store/agentStore";
import { useAnnotationStore } from "../store/annotationStore";
import { useTranscriptStore } from "../store/transcriptStore";
import { useUiStore } from "../store/uiStore";
import { discardChatDraft } from "../components/chat/drafts";

class SseClient {
  private es: EventSourceLike | null = null;
  private watchdog: number | null = null;
  private lastPing = Date.now();
  private opened = false;
  private sharedStreamUnavailable = false;
  private hydrationIds: string[] = [];
  private openAgents = new Map<string, number>();
  private transcriptRequestToken: Record<string, number> = {};
  private lastAgentSeq: Record<string, number> = {};

  connect() {
    if (this.es) return;
    // Give each fresh connection the full liveness window before the watchdog
    // can reap it; otherwise a stale lastPing from a prior stream would close
    // the new stream before its first ping arrives, looping forever.
    this.lastPing = Date.now();
    this.opened = false;
    useUiStore.getState().setConnection("connecting");
    this.es = this.createTransport();
    this.es.onopen = () => {
      this.opened = true;
      useUiStore.getState().setConnection("open");
      // Every (re)open is a hydration/liveness boundary — including the browser's
      // automatic EventSource reconnect, which fires onopen again on the SAME
      // object without going through connect(). The server re-sends a full
      // snapshot + `hydrated` on every connection, so each onopen MUST start a
      // fresh generation and reset all connection-scoped state. Resetting only
      // when `!hydrating` left two bugs on a drop mid-hydration: stale IDs from
      // the partial snapshot were unioned into the next hydrateComplete (deleted
      // agents survived forever), and a stale lastPing let the watchdog reap the
      // freshly-reopened stream before its first ping.
      this.lastPing = Date.now();
      useAgentStore.getState().hydrateBegin();
      this.hydrationIds = [];
      this.lastAgentSeq = {};
      // The open route can outlive its agent. Missing transcripts are represented
      // by ChatPanel's recovery view, not an unhandled reconnect rejection.
      for (const agentId of this.openAgents.keys()) {
        void this.refetchTranscript(agentId).catch(() => undefined);
      }
      queryClient.invalidateQueries({ queryKey: PIPELINE_QUERY_KEYS.runs });
      queryClient.invalidateQueries({ queryKey: PIPELINE_QUERY_KEYS.proposals });
      queryClient.invalidateQueries({ queryKey: TASK_QUERY_KEYS.all });
    };
    this.es.onerror = () => useUiStore.getState().setConnection("reconnecting");
    this.es.addEventListener("state_update", (event) => this.onStateUpdate(event as MessageEvent<string>));
    this.es.addEventListener("new_message", (event) => this.onNewMessage(event as MessageEvent<string>));
    this.es.addEventListener("notification", (event) => this.onNotification(event as MessageEvent<string>));
    this.es.addEventListener("pipeline_update", (event) => this.onPipelineUpdate(event as MessageEvent<string>));
    this.es.addEventListener("pipeline_proposal_update", () => queryClient.invalidateQueries({ queryKey: PIPELINE_QUERY_KEYS.proposals }));
    // A task_update carries only enough to decide whether to refetch; detail
    // comes back over REST, and a reconnect rehydrates the same way (TS-03.R28).
    this.es.addEventListener("task_update", () => {
      queryClient.invalidateQueries({ queryKey: TASK_QUERY_KEYS.all });
      queryClient.invalidateQueries({ queryKey: PIPELINE_QUERY_KEYS.runDetails });
    });
    this.es.addEventListener("config_source_update", () => this.onConfigSourceUpdate());
    this.es.addEventListener("ping", () => {
      this.lastPing = Date.now();
    });
    this.startWatchdog();
  }

  registerOpenAgent(agentId: string) {
    const count = this.openAgents.get(agentId) ?? 0;
    this.openAgents.set(agentId, count + 1);
    if (count === 0) void this.refetchTranscript(agentId).catch(() => undefined);
    let active = true;
    return () => {
      if (!active) return;
      active = false;
      const next = (this.openAgents.get(agentId) ?? 1) - 1;
      if (next > 0) this.openAgents.set(agentId, next);
      else this.openAgents.delete(agentId);
    };
  }

  private onStateUpdate(event: MessageEvent<string>) {
    const envelope = JSON.parse(event.data) as BusEvent<AgentState>;
    if (envelope.data.hydrated) {
      useAgentStore.getState().hydrateComplete(this.hydrationIds);
      this.hydrationIds = [];
      return;
    }
    if (envelope.data.removed && envelope.agent_id) {
      useAgentStore.getState().removeAgent(envelope.agent_id);
      // A deleted agent is gone as an annotation source too — the batch endpoint
      // 404s on it — so its pending tray is dropped with the rest of the state
      // derived from that agent (FS-13.R16). Nothing else ever clears it.
      useAnnotationStore.getState().discard(envelope.agent_id);
      discardChatDraft(envelope.agent_id);
      return;
    }
    useAgentStore.getState().applyStateUpdate(envelope.data);
    if (envelope.agent_id && envelope.data.last_sent_at) {
      const sentAt = envelope.data.last_sent_at;
      window.setTimeout(() => useAgentStore.getState().clearLastSentAt(envelope.agent_id!, sentAt), 2_000);
    }
    if (envelope.agent_id) this.hydrationIds.push(envelope.agent_id);
  }

  private onNewMessage(event: MessageEvent<string>) {
    const envelope = JSON.parse(event.data) as BusEvent<TranscriptEvent>;
    if (!envelope.agent_id) return;
    const agentId = envelope.agent_id;
    const seq = (envelope.data as { seq?: number }).seq ?? 0;
    if (seq > 0) {
      const last = this.lastAgentSeq[agentId] ?? 0;
      this.lastAgentSeq[agentId] = seq;
      if (last > 0 && seq > last + 1 && this.openAgents.has(agentId)) {
        void this.refetchTranscript(agentId).catch(() => undefined);
      }
    }
    useTranscriptStore.getState().updatePreview(agentId, envelope.data);
    if (this.openAgents.has(agentId)) {
      useTranscriptStore.getState().appendMessage(agentId, envelope.data);
    }
  }

  // A federation source changed on disk (or was refreshed/bound): invalidate the
  // project-scoped config-source queries so the Settings panel re-fetches the
  // effective view, health and inventory. Invalidating the prefix key covers
  // every project's query.
  private onConfigSourceUpdate() {
    queryClient.invalidateQueries({ queryKey: ["config-sources"] });
  }

  private onNotification(event: MessageEvent<string>) {
    const envelope = JSON.parse(event.data) as BusEvent<NotificationPayload>;
    const notification = envelope.data;
    const cfg = queryClient.getQueryData<Config>(QUERY_KEYS.config);
    if (cfg?.notifications?.muted?.[notification.notification_type]) return;

    const canDesktop =
      cfg?.notifications?.desktop_enabled !== false &&
      typeof document !== "undefined" &&
      document.visibilityState === "hidden" &&
      "Notification" in window &&
      Notification.permission === "granted";
    if (canDesktop) {
      new Notification(notification.title, { body: notification.body, tag: notification.agent_id });
      return;
    }
    useUiStore.getState().pushToast(notification);
  }

  private onPipelineUpdate(event: MessageEvent<string>) {
    const envelope = JSON.parse(event.data) as BusEvent<unknown>;
    const update = pipelineUpdateSchema.parse(envelope.data);
    const key = PIPELINE_QUERY_KEYS.run(update.run_id);
    const cached = queryClient.getQueryData<PipelineRunDetail>(key);
    if (!cached || cached.run.revision < update.revision) {
      queryClient.invalidateQueries({ queryKey: key });
    }
    queryClient.invalidateQueries({ queryKey: PIPELINE_QUERY_KEYS.runs });
  }

  // The shared worker is preferred, but SharedWorker EXISTING is not proof that
  // it WORKS here: a browser can expose it and still refuse a module worker, and
  // a stale or missing worker asset fails asynchronously. Both must reach the
  // direct stream that already sits here, or the tab shows no live data at all
  // with nothing to explain it (INV §6/§8/§12).
  private createTransport(): EventSourceLike {
    if (this.sharedStreamUnavailable || typeof SharedWorker === "undefined") {
      return new EventSource("/api/events");
    }
    try {
      return new SharedWorkerEventSource(() => this.useDirectStream());
    } catch {
      this.sharedStreamUnavailable = true;
      return new EventSource("/api/events");
    }
  }

  // Demote the transport for the rest of the session and reconnect directly.
  // Idempotent: the load error and the never-opened watchdog can both fire.
  private useDirectStream() {
    if (this.sharedStreamUnavailable) return;
    this.sharedStreamUnavailable = true;
    this.es?.close();
    this.es = null;
    useUiStore.getState().setConnection("reconnecting");
    this.connect();
  }

  private startWatchdog() {
    if (this.watchdog) window.clearInterval(this.watchdog);
    this.watchdog = window.setInterval(() => {
      if (Date.now() - this.lastPing <= 25_000) return;
      // A stream that never opened at all is a broken transport, not a dropped
      // connection. Reconnecting it would loop on the same dead worker forever,
      // so demote once and let the direct stream take the liveness window.
      if (!this.opened && !this.sharedStreamUnavailable) {
        this.useDirectStream();
        return;
      }
      this.es?.close();
      this.es = null;
      useUiStore.getState().setConnection("down");
      this.connect();
    }, 5_000);
  }

  private async refetchTranscript(agentId: string) {
    const token = (this.transcriptRequestToken[agentId] ?? 0) + 1;
    this.transcriptRequestToken[agentId] = token;
    const transcript = await getTranscript(agentId);
    if (this.transcriptRequestToken[agentId] !== token || !this.openAgents.has(agentId)) return;
    useTranscriptStore.getState().setTranscript(transcript.agent_id, transcript.events);
  }
}

interface EventSourceLike {
  onopen: ((event: Event) => unknown) | null;
  onerror: ((event: Event) => unknown) | null;
  addEventListener(type: string, listener: (event: MessageEvent<string>) => void): void;
  close(): void;
}

type SharedWorkerMessage =
  | { kind: "open" }
  | { kind: "error" }
  | { kind: "event"; type: string; data: string };

class SharedWorkerEventSource implements EventSourceLike {
  onopen: ((event: Event) => unknown) | null = null;
  onerror: ((event: Event) => unknown) | null = null;
  private readonly listeners = new Map<string, Set<(event: MessageEvent<string>) => void>>();
  private readonly port: MessagePort;

  constructor(onTransportFailure: () => void) {
    const worker = new SharedWorker(new URL("./sse-shared-worker.ts", import.meta.url), {
      name: "agentdeck-events",
      type: "module",
    });
    // An `error` on the SharedWorker itself means its script never loaded or
    // never evaluated. No port message can follow, so nothing else would ever
    // notice: neither onopen nor onerror on this object can fire.
    worker.onerror = () => onTransportFailure();
    this.port = worker.port;
    this.port.onmessage = (event: MessageEvent<SharedWorkerMessage>) => this.receive(event.data);
    this.port.start();
  }

  addEventListener(type: string, callback: (event: MessageEvent<string>) => void) {
    let listeners = this.listeners.get(type);
    if (!listeners) this.listeners.set(type, (listeners = new Set()));
    listeners.add(callback);
  }

  close() {
    this.port.close();
  }

  private receive(message: SharedWorkerMessage) {
    if (message.kind === "open") {
      this.onopen?.(new Event("open"));
      return;
    }
    if (message.kind === "error") {
      this.onerror?.(new Event("error"));
      return;
    }
    const event = new MessageEvent(message.type, { data: message.data });
    for (const listener of this.listeners.get(message.type) ?? []) {
      listener(event);
    }
  }
}

export const sseClient = new SseClient();
