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
  private openAgentId: string | null = null;

  connect() {
    if (this.es) return;
    useUiStore.getState().setConnection("connecting");
    this.es = new EventSource("/api/events");
    this.es.onopen = () => {
      useUiStore.getState().setConnection("open");
      useAgentStore.getState().hydrateBegin();
      this.hydrationIds = [];
      this.refetchOpenTranscript();
    };
    this.es.onerror = () => useUiStore.getState().setConnection("reconnecting");
    this.es.addEventListener("state_update", (event) => this.onStateUpdate(event as MessageEvent<string>));
    this.es.addEventListener("new_message", (event) => this.onNewMessage(event as MessageEvent<string>));
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
    useTranscriptStore.getState().appendMessage(envelope.agent_id, envelope.data);
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
