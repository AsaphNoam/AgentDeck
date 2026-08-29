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

// A minimal fake SharedWorker + MessagePort pair. The real transport reaches
// the client only through port messages, so the fake needs nothing more than a
// port to post on and the `error` hook the worker fires when its script dies.
class FakeMessagePort {
  onmessage: ((event: MessageEvent<unknown>) => void) | null = null;
  started = false;
  closed = false;
  // Messages the client sent INTO the worker. Deliberately recorded even after
  // close(), so a test can prove the goodbye precedes the close.
  sent: unknown[] = [];
  start() {
    this.started = true;
  }
  close() {
    this.closed = true;
  }
  postMessage(message: unknown) {
    this.sent.push(message);
  }
  deliver(message: unknown) {
    this.onmessage?.({ data: message } as MessageEvent<unknown>);
  }
}

class FakeSharedWorker {
  static instances: FakeSharedWorker[] = [];
  onerror: (() => void) | null = null;
  port = new FakeMessagePort();
  constructor() {
    FakeSharedWorker.instances.push(this);
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
    localStorage.clear();
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

  // FS-03.A23 — a sequence gap for one pane refetches only that pane's transcript.
  it("only refetches on a seq gap for a registered agent", async () => {
    const { sseClient } = await import("./sse");
    const client = await import("./client");
    sseClient.connect();
    const es = FakeEventSource.instances[0];
    const msg = (agent: string, seq: number) =>
      JSON.stringify({ type: "new_message", seq, ts: 1, agent_id: agent, data: { kind: "assistant_text", seq, ts: "t", delta: "x" } });

    sseClient.registerOpenAgent("a_open");
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

  it("contains a missing open transcript during reconnect hydration", async () => {
    const { sseClient } = await import("./sse");
    const client = await import("./client");
    (client.getTranscript as ReturnType<typeof vi.fn>).mockRejectedValueOnce(new Error("no such agent"));
    sseClient.registerOpenAgent("a_gone");
    sseClient.connect();

    FakeEventSource.instances[0].onopen?.();
    await vi.waitFor(() => expect(client.getTranscript).toHaveBeenCalledWith("a_gone"));
  });

  // FS-03.A23 — with two panes open, each agent's deltas append to that agent's
  // transcript and to no other.
  it("delivers each registered agent to its own transcript and teardown stops delivery", async () => {
    const { sseClient } = await import("./sse");
    const { useTranscriptStore } = await import("../store/transcriptStore");
    sseClient.connect();
    const es = FakeEventSource.instances[0];
    const closeA = sseClient.registerOpenAgent("a_one");
    sseClient.registerOpenAgent("a_two");
    const msg = (agent: string, seq: number, delta: string) => JSON.stringify({
      type: "new_message", seq, ts: 1, agent_id: agent,
      data: { kind: "assistant_text", seq, ts: "t", delta },
    });

    es.emit("new_message", msg("a_one", 1, "one"));
    es.emit("new_message", msg("a_two", 1, "two"));
    expect(useTranscriptStore.getState().byAgent.a_one[0].text ?? useTranscriptStore.getState().byAgent.a_one[0].delta).toBe("one");
    expect(useTranscriptStore.getState().byAgent.a_two[0].text ?? useTranscriptStore.getState().byAgent.a_two[0].delta).toBe("two");

    closeA();
    closeA();
    es.emit("new_message", msg("a_one", 2, " ignored"));
    expect(useTranscriptStore.getState().byAgent.a_one).toBeUndefined();
  });

  // FS-03.A23 — a stale response resolving after a newer one leaves the displayed
  // transcript carrying every delivered event.
  it("ignores a stale transcript response after a newer per-agent request", async () => {
    const { sseClient } = await import("./sse");
    const client = await import("./client");
    const { useTranscriptStore } = await import("../store/transcriptStore");
    let resolveFirst!: (value: unknown) => void;
    let resolveSecond!: (value: unknown) => void;
    (client.getTranscript as ReturnType<typeof vi.fn>)
      .mockImplementationOnce(() => new Promise((resolve) => { resolveFirst = resolve; }))
      .mockImplementationOnce(() => new Promise((resolve) => { resolveSecond = resolve; }));

    sseClient.registerOpenAgent("a_ordered");
    sseClient.connect();
    FakeEventSource.instances[0].onopen?.();
    resolveSecond({ agent_id: "a_ordered", events: [{ seq: 2, kind: "assistant_text", text: "new" }] });
    await vi.waitFor(() => expect(useTranscriptStore.getState().byAgent.a_ordered?.[0]?.text).toBe("new"));
    resolveFirst({ agent_id: "a_ordered", events: [{ seq: 1, kind: "assistant_text", text: "old" }] });
    await Promise.resolve();
    expect(useTranscriptStore.getState().byAgent.a_ordered[0].text).toBe("new");
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

  // FS-13.R16 / FS-03.A19: deletion drops browser-local state keyed to the
  // agent and leaves other agents' state alone.
  it("drops a deleted agent's annotation tray and chat draft", async () => {
    const { sseClient } = await import("./sse");
    const { useAnnotationStore } = await import("../store/annotationStore");
    const { useTranscriptStore } = await import("../store/transcriptStore");
    const { getChatDraft, setChatDraft } = await import("../components/chat/drafts");
    useAnnotationStore.getState().add("a_gone", { seq: 3, excerpt: "line", instruction: "look" });
    useAnnotationStore.getState().add("a_keep", { seq: 4, excerpt: "line", instruction: "look" });
    setChatDraft("a_gone", "gone draft");
    setChatDraft("a_keep", "keep draft");
    useTranscriptStore.getState().appendMessage("a_gone", { seq: 1, kind: "assistant_text", text: "gone" });
    useTranscriptStore.getState().appendMessage("a_keep", { seq: 1, kind: "assistant_text", text: "keep" });
    sseClient.connect();
    const es = FakeEventSource.instances[0];
    es.onopen?.();

    es.emit("state_update", JSON.stringify({ type: "state_update", seq: 1, ts: 1, agent_id: "a_gone", data: { agent_id: "a_gone", removed: true } }));

    expect(useAnnotationStore.getState().bySource["a_gone"]).toBeUndefined();
    expect(useAnnotationStore.getState().bySource["a_keep"]).toHaveLength(1);
    expect(useTranscriptStore.getState().byAgent.a_gone).toBeUndefined();
    expect(useTranscriptStore.getState().byAgent.a_keep).toHaveLength(1);
    expect(getChatDraft("a_gone")).toBe("");
    expect(getChatDraft("a_keep")).toBe("keep draft");
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

  it("uses pipeline revisions to invalidate stale run detail", async () => {
    const { sseClient } = await import("./sse");
    const { queryClient } = await import("./config");
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    queryClient.setQueryData(["pipelines", "run-detail", "pr_1"], { run: { revision: 4 } });
    sseClient.connect();
    FakeEventSource.instances[0].onopen?.();
    spy.mockClear();
    const update = (revision: number) => JSON.stringify({
      type: "pipeline_update", seq: revision, ts: revision, agent_id: null,
      data: { run_id: "pr_1", display_name: "Run one", revision, state: "running", current_stage_id: "work", current_agent_id: "a_1", attention_reason: "", final_outcome: "" },
    });

    FakeEventSource.instances[0].emit("pipeline_update", update(4));
    expect(spy).not.toHaveBeenCalledWith({ queryKey: ["pipelines", "run-detail", "pr_1"] });

    FakeEventSource.instances[0].emit("pipeline_update", update(5));
    expect(spy).toHaveBeenCalledWith({ queryKey: ["pipelines", "run-detail", "pr_1"] });
    expect(spy).toHaveBeenCalledWith({ queryKey: ["pipelines", "run-list"] });
  });

  it("refetches open pipeline details but keeps run-list pagination on task updates", async () => {
    const { sseClient } = await import("./sse");
    const { queryClient } = await import("./config");
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    sseClient.connect();
    FakeEventSource.instances[0].onopen?.();
    spy.mockClear();

    FakeEventSource.instances[0].emit("task_update", JSON.stringify({ type: "task_update", data: { task_id: "task_1" } }));

    expect(spy).toHaveBeenCalledWith({ queryKey: ["pipelines", "run-detail"] });
    expect(spy).not.toHaveBeenCalledWith({ queryKey: ["pipelines", "run-list"] });
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

  // TS-03.R7 / FS-02.A27: the shared stream is the preferred transport, and the
  // events a tab renders must arrive over it unchanged.
  it("delivers shared-worker port messages as stream events", async () => {
    FakeSharedWorker.instances = [];
    vi.stubGlobal("SharedWorker", FakeSharedWorker as unknown as typeof SharedWorker);
    const { sseClient } = await import("./sse");
    const { useUiStore } = await import("../store/uiStore");
    sseClient.connect();

    expect(FakeSharedWorker.instances).toHaveLength(1);
    expect(FakeEventSource.instances).toHaveLength(0);
    const port = FakeSharedWorker.instances[0].port;
    expect(port.started).toBe(true);

    port.deliver({ kind: "open" });
    expect(useUiStore.getState().connection).toBe("open");

    // A ping arriving over the port must satisfy the same liveness window a
    // direct stream's ping does, or the watchdog reaps a healthy shared stream.
    vi.advanceTimersByTime(20_000);
    port.deliver({ kind: "event", type: "ping", data: "" });
    vi.advanceTimersByTime(20_000);
    expect(FakeEventSource.instances).toHaveLength(0);
  });

  // Regression (review fix, INV §4 / FS-02.A27): the watchdog closes the
  // transport and connect() mints a NEW port, so without a goodbye the worker
  // accumulates a dead port per reconnect for the life of the tab.
  it("says goodbye on the port before closing it", async () => {
    FakeSharedWorker.instances = [];
    vi.stubGlobal("SharedWorker", FakeSharedWorker as unknown as typeof SharedWorker);
    const { sseClient } = await import("./sse");
    sseClient.connect();
    const first = FakeSharedWorker.instances[0].port;
    first.deliver({ kind: "open" });

    // No pings → the watchdog reaps and reconnects onto a fresh port.
    vi.advanceTimersByTime(30_000);
    expect(first.sent).toEqual([{ kind: "bye" }]);
    expect(first.closed).toBe(true);
    expect(FakeSharedWorker.instances).toHaveLength(2);
  });

  // Regression (review fix, Must fix): SharedWorker EXISTING is not proof it
  // works. A browser that exposes it but refuses a module worker threw out of
  // connect(), so `es` was never assigned and the watchdog never even started:
  // the dashboard sat on "connecting" with no live data, no error, and no retry.
  it("falls back to the direct stream when the shared worker cannot be constructed", async () => {
    vi.stubGlobal("SharedWorker", function BrokenSharedWorker() {
      throw new Error("module workers are not supported");
    } as unknown as typeof SharedWorker);
    const { sseClient } = await import("./sse");
    const { useUiStore } = await import("../store/uiStore");

    expect(() => sseClient.connect()).not.toThrow();
    expect(FakeEventSource.instances).toHaveLength(1);

    // The direct stream is fully wired, watchdog included.
    FakeEventSource.instances[0].onopen?.();
    expect(useUiStore.getState().connection).toBe("open");
    vi.advanceTimersByTime(30_000);
    expect(FakeEventSource.instances).toHaveLength(2);
  });

  // Regression (review fix, Must fix): a worker script that 404s or fails to
  // evaluate fires `error` on the SharedWorker object, where nothing listened.
  // No port message could follow, so the tab never opened and never recovered.
  it("falls back to the direct stream when the shared worker script fails to load", async () => {
    FakeSharedWorker.instances = [];
    vi.stubGlobal("SharedWorker", FakeSharedWorker as unknown as typeof SharedWorker);
    const { sseClient } = await import("./sse");
    sseClient.connect();
    expect(FakeEventSource.instances).toHaveLength(0);

    FakeSharedWorker.instances[0].onerror?.();

    expect(FakeSharedWorker.instances[0].port.closed).toBe(true);
    expect(FakeEventSource.instances).toHaveLength(1);
    FakeEventSource.instances[0].onopen?.();
    expect(FakeEventSource.instances).toHaveLength(1);
  });

  // Regression (review fix, Must fix): a worker that constructs and loads but
  // never opens gave the watchdog nothing to distinguish from a dropped
  // connection, so it reconnected into the same dead worker every 25s forever.
  it("demotes a shared worker that never opens instead of reconnecting into it", async () => {
    FakeSharedWorker.instances = [];
    vi.stubGlobal("SharedWorker", FakeSharedWorker as unknown as typeof SharedWorker);
    const { sseClient } = await import("./sse");
    sseClient.connect();

    vi.advanceTimersByTime(30_000);
    expect(FakeSharedWorker.instances).toHaveLength(1);
    expect(FakeEventSource.instances).toHaveLength(1);

    // Once demoted the session stays on the direct stream: a later reap must
    // reconnect directly rather than retrying the worker.
    FakeEventSource.instances[0].onopen?.();
    vi.advanceTimersByTime(30_000);
    expect(FakeSharedWorker.instances).toHaveLength(1);
    expect(FakeEventSource.instances).toHaveLength(2);
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

// The shared worker is the thing that makes six tabs cost one connection, so
// its port bookkeeping is tested directly: the client only ever sees one port.
class FakeWorkerPort {
  onmessage: ((event: MessageEvent<unknown>) => void) | null = null;
  started = false;
  closed = false;
  // Broadcasts are NOT dropped when closed: a test asserting "no longer
  // receives broadcasts" must fail if the worker still holds the port.
  received: unknown[] = [];
  postMessage(message: unknown) {
    this.received.push(message);
  }
  start() {
    this.started = true;
  }
  close() {
    this.closed = true;
  }
  say(message: unknown) {
    this.onmessage?.({ data: message } as MessageEvent<unknown>);
  }
}

// TS-03.R7 / FS-02.A27: one shared stream, one hydration generation per tab,
// and no tab's arrival or departure disturbing the tabs already attached.
describe("SSE shared worker", () => {
  type ConnectEvent = { ports: FakeWorkerPort[] };
  let attach: (event: ConnectEvent) => void;

  const row = (id: string, extra: Record<string, unknown> = {}) =>
    JSON.stringify({ type: "state_update", seq: 1, ts: 1, agent_id: id, data: { agent_id: id, ...extra } });
  const hydrated = JSON.stringify({ type: "state_update", seq: 9, ts: 9, agent_id: "", data: { hydrated: true } });

  beforeEach(async () => {
    FakeEventSource.instances = [];
    vi.stubGlobal("EventSource", FakeEventSource as unknown as typeof EventSource);
    vi.resetModules();
    await import("./sse-shared-worker");
    attach = (self as unknown as { onconnect: (event: ConnectEvent) => void }).onconnect;
  });

  afterEach(() => {
    delete (self as unknown as { onconnect?: unknown }).onconnect;
    vi.unstubAllGlobals();
    vi.resetModules();
  });

  // Attach a port and drive the shared stream through a full hydration burst.
  function attachHydrated() {
    const port = new FakeWorkerPort();
    attach({ ports: [port] });
    const source = FakeEventSource.instances[0];
    source.onopen?.();
    source.emit("state_update", row("a_one"));
    source.emit("state_update", row("a_two"));
    source.emit("state_update", hydrated);
    return { port, source };
  }

  it("stops broadcasting to a port that says goodbye", () => {
    const first = new FakeWorkerPort();
    const second = new FakeWorkerPort();
    attach({ ports: [first] });
    attach({ ports: [second] });
    const source = FakeEventSource.instances[0];

    first.say({ kind: "bye" });
    expect(first.closed).toBe(true);
    first.received = [];
    second.received = [];
    source.emit("new_message", "{}");

    expect(first.received).toEqual([]);
    expect(second.received).toEqual([{ kind: "event", type: "new_message", data: "{}" }]);
  });

  it("drops the shared stream when the last port leaves and reopens for the next one", () => {
    const port = new FakeWorkerPort();
    attach({ ports: [port] });
    expect(FakeEventSource.instances).toHaveLength(1);

    port.say({ kind: "bye" });
    expect(FakeEventSource.instances[0].closed).toBe(true);

    attach({ ports: [new FakeWorkerPort()] });
    expect(FakeEventSource.instances).toHaveLength(2);
    expect(FakeEventSource.instances[1].closed).toBe(false);
  });

  it("leaves the live stream and the attached tabs untouched when a tab joins", () => {
    const { port: first, source } = attachHydrated();
    first.received = [];

    attach({ ports: [new FakeWorkerPort()] });

    // One stream, still open, and the sitting tab gets no second `open` — which
    // would restart its hydration and refetch every open transcript.
    expect(FakeEventSource.instances).toHaveLength(1);
    expect(source.closed).toBe(false);
    expect(first.received).toEqual([]);

    // Live deltas keep flowing to the tab that was already attached.
    source.emit("new_message", "{}");
    expect(first.received).toEqual([{ kind: "event", type: "new_message", data: "{}" }]);
  });

  it("replays the retained snapshot and hydrated boundary to the joining port alone", () => {
    const { source } = attachHydrated();
    // A later live update replaces the retained row for that agent rather than
    // appending: the snapshot is one row per agent, not a growing log.
    source.emit("state_update", row("a_one", { title: "renamed" }));
    source.emit("state_update", row("a_gone"));
    source.emit("state_update", row("a_gone", { removed: true }));

    const joining = new FakeWorkerPort();
    attach({ ports: [joining] });

    expect(joining.started).toBe(true);
    expect(joining.received).toEqual([
      { kind: "open" },
      { kind: "event", type: "state_update", data: row("a_one", { title: "renamed" }) },
      { kind: "event", type: "state_update", data: row("a_two") },
      { kind: "event", type: "state_update", data: hydrated },
    ]);
  });

  it("re-broadcasts a fresh generation and discards the stale snapshot when the stream reopens", () => {
    const { port: first, source } = attachHydrated();
    first.received = [];

    // The browser's automatic EventSource reconnect: every tab is told, and the
    // retained snapshot from the dead generation must not be replayed after it.
    source.onerror?.();
    source.onopen?.();
    expect(first.received).toEqual([{ kind: "error" }, { kind: "open" }]);

    const joining = new FakeWorkerPort();
    attach({ ports: [joining] });
    expect(joining.received).toEqual([{ kind: "open" }]);
  });
});
