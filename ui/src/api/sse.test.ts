import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./client", () => ({
  getTranscript: vi.fn(async (id: string) => ({ agent_id: id, events: [] })),
}));

// A minimal fake EventSource that records instances and lets the test drive
// open/ping/close. Each construction is tracked so we can assert reconnects.
class FakeEventSource {
  static instances: FakeEventSource[] = [];
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  private listeners: Record<string, Array<(event: MessageEvent<string>) => void>> = {};
  closed = false;

  constructor(public url: string) {
    FakeEventSource.instances.push(this);
  }

  addEventListener(type: string, cb: (event: MessageEvent<string>) => void) {
    (this.listeners[type] ??= []).push(cb);
  }

  emit(type: string, data = "") {
    (this.listeners[type] ?? []).forEach((cb) => cb({ data } as MessageEvent<string>));
  }

  close() {
    this.closed = true;
  }
}

describe("SseClient watchdog reconnect", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    FakeEventSource.instances = [];
    vi.stubGlobal("EventSource", FakeEventSource as unknown as typeof EventSource);
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.resetModules();
  });

  it("does not reap a freshly reconnected stream before its first ping", async () => {
    const { sseClient } = await import("./sse");
    sseClient.connect();

    expect(FakeEventSource.instances).toHaveLength(1);
    const first = FakeEventSource.instances[0];
    first.onopen?.();

    // No pings ever arrive on the first stream → watchdog should reap it once
    // the 25s liveness window lapses (it ticks every 5s).
    vi.advanceTimersByTime(30_000);
    expect(first.closed).toBe(true);
    expect(FakeEventSource.instances).toHaveLength(2);

    const second = FakeEventSource.instances[1];
    second.onopen?.();

    // The reconnected stream's first ping legitimately arrives ~10s after open.
    // The watchdog ticks at 5s; with the liveness timestamp reset on connect,
    // the fresh stream must survive that tick instead of being killed because
    // of the stale timestamp inherited from the dead stream.
    vi.advanceTimersByTime(6_000);
    expect(second.closed).toBe(false);
    expect(FakeEventSource.instances).toHaveLength(2);

    second.emit("ping");
    vi.advanceTimersByTime(6_000);
    expect(second.closed).toBe(false);
  });

  // Regression (review fix): a seq-gap transcript refetch must only fire for the
  // OPEN agent (others aren't displayed and ChatPanel refetches on open), and the
  // gap event must not also be appended (the async setTranscript would clobber /
  // duplicate it).
  it("only refetches on a seq gap for the open agent", async () => {
    const { sseClient } = await import("./sse");
    const client = await import("./client");
    sseClient.connect();
    const es = FakeEventSource.instances[0];
    const msg = (agent: string, seq: number) =>
      JSON.stringify({ type: "new_message", seq, ts: 1, agent_id: agent, data: { kind: "assistant_text", seq, ts: "t", delta: "x" } });

    sseClient.setOpenAgent("a_open");
    // Seed lastSeq=1 for both agents (no gap yet).
    es.emit("new_message", msg("a_open", 1));
    es.emit("new_message", msg("a_bg", 1));
    (client.getTranscript as ReturnType<typeof vi.fn>).mockClear();

    // Gap for the open agent → refetch.
    es.emit("new_message", msg("a_open", 5));
    expect(client.getTranscript).toHaveBeenCalledWith("a_open");
    // Gap for a background agent → no refetch.
    es.emit("new_message", msg("a_bg", 9));
    expect(client.getTranscript).toHaveBeenCalledTimes(1);
  });

  // Regression (review fix, BLOCKING): a drop mid-hydration triggers the
  // browser's automatic EventSource reconnect, which fires onopen again on the
  // SAME object. Each onopen must reset the hydration generation; otherwise the
  // partial snapshot's stale IDs are unioned into the next hydrateComplete and a
  // server-deleted agent survives the reconnect indefinitely.
  it("resets the hydration generation on auto-reconnect so deleted agents are pruned", async () => {
    const { sseClient } = await import("./sse");
    const { useAgentStore } = await import("../store/agentStore");
    sseClient.connect();
    const es = FakeEventSource.instances[0];
    const upd = (id: string) =>
      JSON.stringify({ type: "state_update", seq: 1, ts: 1, agent_id: id, data: { agent_id: id } });
    const hydrated = JSON.stringify({ type: "state_update", seq: 2, ts: 2, agent_id: "", data: { hydrated: true } });

    // First (partial) hydration: two agents arrive, then the connection drops
    // BEFORE the `hydrated` marker (still hydrating).
    es.onopen?.();
    es.emit("state_update", upd("a_keep"));
    es.emit("state_update", upd("a_gone"));

    // Browser auto-reconnects on the same EventSource → onopen fires again. The
    // fresh full snapshot no longer contains a_gone (deleted server-side), then
    // the hydrated marker closes the generation.
    es.onopen?.();
    es.emit("state_update", upd("a_keep"));
    es.emit("state_update", hydrated);

    const agents = useAgentStore.getState().agents;
    expect(agents["a_keep"]).toBeDefined();
    expect(agents["a_gone"]).toBeUndefined();
  });

  it("invalidates config-source queries on a config_source_update event", async () => {
    const { sseClient } = await import("./sse");
    const { queryClient } = await import("./config");
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    sseClient.connect();
    FakeEventSource.instances[0].onopen?.();

    FakeEventSource.instances[0].emit(
      "config_source_update",
      JSON.stringify({ backend_id: "claude", project_id: "app", generation: 3, health: "ok", changed: ["model"], stale: false }),
    );
    expect(spy).toHaveBeenCalledWith({ queryKey: ["config-sources"] });
  });

  it("drops muted notification types", async () => {
    const { sseClient } = await import("./sse");
    const { queryClient } = await import("./config");
    const { useUiStore } = await import("../store/uiStore");
    queryClient.setQueryData(["config"], {
      notifications: { desktop_enabled: true, muted: { done: true } },
    });
    sseClient.connect();
    const first = FakeEventSource.instances[0];
    first.emit("notification", JSON.stringify({
      type: "notification",
      seq: 1,
      ts: 1,
      agent_id: "a_1",
      data: { type: "notification", notification_type: "done", agent_id: "a_1", title: "Done", ts: "2026-06-29T00:00:00Z" },
    }));
    expect(useUiStore.getState().toasts).toEqual([]);
  });

  it("uses Web Notification for hidden tabs when permission is granted", async () => {
    const calls: Array<{ title: string; body?: string; tag?: string }> = [];
    class FakeNotification {
      static permission = "granted";
      static requestPermission = vi.fn();
      constructor(title: string, opts?: NotificationOptions) {
        calls.push({ title, body: opts?.body, tag: opts?.tag });
      }
    }
    Object.defineProperty(document, "visibilityState", { value: "hidden", configurable: true });
    vi.stubGlobal("Notification", FakeNotification as unknown as typeof Notification);

    const { sseClient } = await import("./sse");
    const { queryClient } = await import("./config");
    const { useUiStore } = await import("../store/uiStore");
    queryClient.setQueryData(["config"], {
      notifications: { desktop_enabled: true, muted: { done: false } },
    });
    sseClient.connect();
    FakeEventSource.instances[0].emit("notification", JSON.stringify({
      type: "notification",
      seq: 1,
      ts: 1,
      agent_id: "a_1",
      data: { type: "notification", notification_type: "done", agent_id: "a_1", title: "Atlas finished", body: "done", ts: "2026-06-29T00:00:00Z" },
    }));
    expect(calls).toEqual([{ title: "Atlas finished", body: "done", tag: "a_1" }]);
    expect(useUiStore.getState().toasts).toEqual([]);
  });
});
