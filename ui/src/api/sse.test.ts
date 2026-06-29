import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

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
