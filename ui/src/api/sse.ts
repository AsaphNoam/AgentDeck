import { getTranscript } from "./client";
import { QUERY_KEYS, queryClient } from "./config";
import type { Config } from "../schemas/config";
import type { AgentState, BusEvent, NotificationPayload, TranscriptEvent } from "./types";
import { useAgentStore } from "../store/agentStore";
import { useTranscriptStore } from "../store/transcriptStore";
import { useUiStore } from "../store/uiStore";

class SseClient {
  private es: EventSource | null = null;
  private watchdog: number | null = null;
  private lastPing = Date.now();
  private hydrationIds: string[] = [];
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
      this.refetchOpenTranscript();
    };
    this.es.onerror = () => useUiStore.getState().setConnection("reconnecting");
    this.es.addEventListener("state_update", (event) => this.onStateUpdate(event as MessageEvent<string>));
    this.es.addEventListener("new_message", (event) => this.onNewMessage(event as MessageEvent<string>));
    this.es.addEventListener("notification", (event) => this.onNotification(event as MessageEvent<string>));
    this.es.addEventListener("config_source_update", () => this.onConfigSourceUpdate());
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
      // Only the open agent's transcript is displayed; a gap refetch for any
      // other agent is wasted work (ChatPanel refetches on open anyway). On a
      // gap we refetch the authoritative transcript — which already includes
      // this event — and return WITHOUT also appending it, so the async
      // setTranscript can't clobber (or duplicate, since appendMessage doesn't
      // dedupe) a concurrent optimistic append.
      if (last > 0 && seq > last + 1 && agentId === this.openAgentId) {
        getTranscript(agentId)
          .then((t) => useTranscriptStore.getState().setTranscript(t.agent_id, t.events))
          .catch(() => undefined);
        return;
      }
    }
    useTranscriptStore.getState().appendMessage(agentId, envelope.data);
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
