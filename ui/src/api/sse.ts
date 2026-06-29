import { getTranscript } from "./client";
import type { AgentState, BusEvent, TranscriptEvent } from "./types";
import { useAgentStore } from "../store/agentStore";
import { useTranscriptStore } from "../store/transcriptStore";
import { useUiStore } from "../store/uiStore";

class SseClient {
  private es: EventSource | null = null;
  private watchdog: number | null = null;
  private lastPing = Date.now();
  private hydrationIds: string[] = [];
  private hydrating = false;
  private openAgentId: string | null = null;
  private lastAgentSeq: Record<string, number> = {};

  connect() {
    if (this.es) return;
    // Give each fresh connection the full liveness window before the watchdog
    // can reap it; otherwise a stale lastPing from a prior stream would close
    // the new stream before its first ping arrives, looping forever.
    this.lastPing = Date.now();
    useUiStore.getState().setConnection("connecting");
    this.es = new EventSource("/api/events");
    this.es.onopen = () => {
      useUiStore.getState().setConnection("open");
      if (!this.hydrating) {
        useAgentStore.getState().hydrateBegin();
        this.hydrationIds = [];
        this.hydrating = true;
        this.lastAgentSeq = {};
      }
      this.refetchOpenTranscript();
    };
    this.es.onerror = () => useUiStore.getState().setConnection("reconnecting");
    this.es.addEventListener("state_update", (event) => this.onStateUpdate(event as MessageEvent<string>));
    this.es.addEventListener("new_message", (event) => this.onNewMessage(event as MessageEvent<string>));
    this.es.addEventListener("notification", () => { /* reserved for future use */ });
    this.es.addEventListener("ping", () => {
      this.lastPing = Date.now();
    });
    this.startWatchdog();
  }

  setOpenAgent(agentId: string | null) {
    this.openAgentId = agentId;
  }

  private onStateUpdate(event: MessageEvent<string>) {
    const envelope = JSON.parse(event.data) as BusEvent<AgentState>;
    if (envelope.data.hydrated) {
      useAgentStore.getState().hydrateComplete(this.hydrationIds);
      this.hydrationIds = [];
      this.hydrating = false;
      return;
    }
    if (envelope.data.removed && envelope.agent_id) {
      useAgentStore.getState().removeAgent(envelope.agent_id);
      return;
    }
    useAgentStore.getState().applyStateUpdate(envelope.data);
    if (envelope.agent_id) this.hydrationIds.push(envelope.agent_id);
  }

  private onNewMessage(event: MessageEvent<string>) {
    const envelope = JSON.parse(event.data) as BusEvent<TranscriptEvent>;
    if (!envelope.agent_id) return;
    const agentId = envelope.agent_id;
    const seq = (envelope.data as { seq?: number }).seq ?? 0;
    if (seq > 0) {
      const last = this.lastAgentSeq[agentId] ?? 0;
      if (last > 0 && seq > last + 1) {
        // Gap detected — refetch full transcript to recover missed events.
        getTranscript(agentId).then((t) =>
          useTranscriptStore.getState().setTranscript(t.agent_id, t.events)
        ).catch(() => undefined);
      }
      this.lastAgentSeq[agentId] = seq;
    }
    useTranscriptStore.getState().appendMessage(agentId, envelope.data);
  }

  private startWatchdog() {
    if (this.watchdog) window.clearInterval(this.watchdog);
    this.watchdog = window.setInterval(() => {
      if (Date.now() - this.lastPing <= 25_000) return;
      this.es?.close();
      this.es = null;
      useUiStore.getState().setConnection("down");
      this.connect();
    }, 5_000);
  }

  private async refetchOpenTranscript() {
    if (!this.openAgentId) return;
    const transcript = await getTranscript(this.openAgentId);
    useTranscriptStore.getState().setTranscript(transcript.agent_id, transcript.events);
  }
}

export const sseClient = new SseClient();
