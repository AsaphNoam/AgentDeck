import React from "react";
import fs from "node:fs";
import path from "node:path";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { PhoneApp } from "./PhoneApp";
import { disconnect, useConnection } from "./connection";
import type { HomeLists } from "./api";

class FakeEventSource {
  static last: FakeEventSource | null = null;
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  constructor(public url: string) {
    FakeEventSource.last = this;
  }
  private listeners = new Map<string, Array<(event: Event) => void>>();
  addEventListener(type: string, listener: (event: Event) => void) {
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), listener]);
  }
  removeEventListener() {}
  close() {}
  emit(type: string, body: unknown) {
    const event = { data: JSON.stringify(body) } as MessageEvent<string>;
    for (const listener of this.listeners.get(type) ?? []) listener(event);
  }
}

let paired = true;
let claims = 0;
let waits = 0;
const home: HomeLists = {
  needs_you: [
    { kind: "agent", id: "a1", title: "implementer@my-app", project: "my-app", state: "waiting_input", reason: "needs permission", agent_id: "a1", since: new Date().toISOString() },
  ],
  active_runs: [
    { kind: "run", id: "r1", title: "Ship it", project: "my-app", state: "running", reason: "running", stage_number: 2, stage_count: 4, since: new Date().toISOString() },
  ],
};

const server = setupServer(
  http.get("/api/health", () =>
    paired ? HttpResponse.json({ status: "ok" }) : HttpResponse.json({ error: { code: "remote_unpaired", message: "pair" } }, { status: 401 }),
  ),
  http.get("/api/remote/home", () => HttpResponse.json(home)),
  http.get("/api/projects", () => HttpResponse.json({ "my-app": { title: "My app", color: [1, 2, 3] } })),
  http.get("/api/config", () => HttpResponse.json({ appearance_skin: "" })),
  http.post("/api/remote/pair", async ({ request }) => {
    claims++;
    const body = (await request.json()) as { code: string };
    if (body.code !== "ABCD2345") return HttpResponse.json({ error: { code: "remote_pairing_invalid", message: "x" } }, { status: 400 });
    return HttpResponse.json({ pending_id: "w1" }, { status: 202 });
  }),
  http.get("/api/remote/pair/:id", () => {
    waits++;
    if (waits < 2) return HttpResponse.json({ status: "waiting" });
    paired = true;
    return HttpResponse.json({ status: "allowed" });
  }),
);

beforeAll(() => {
  (globalThis as unknown as { EventSource: unknown }).EventSource = FakeEventSource;
  server.listen({ onUnhandledRequest: "error" });
});
beforeEach(() => {
  paired = true;
  claims = 0;
  waits = 0;
  localStorage.clear();
  window.history.replaceState(null, "", "/");
  useConnection.setState({ link: "checking", unreachableSince: null, revision: 0, agents: {}, transcriptRev: {} });
});
afterEach(() => {
  cleanup();
  disconnect();
  server.resetHandlers();
  document.documentElement.removeAttribute("data-skin");
});
afterAll(() => server.close());

function renderApp() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <PhoneApp />
    </QueryClientProvider>,
  );
  return client;
}

describe("PhoneApp", () => {
  it("refreshes all configured appearances without remounting Home and falls back on failure", async () => {
    let appearance = "studio";
    let failed = false;
    server.use(http.get("/api/config", () => failed ? HttpResponse.json({ error: "unavailable" }, { status: 503 }) : HttpResponse.json({ appearance_skin: appearance })));
    const client = renderApp();
    const heading = await screen.findByRole("heading", { name: "Away, not out of the loop." });
    for (appearance of ["studio", "sky-grove", "", "unknown"]) {
      await act(async () => { await client.invalidateQueries({ queryKey: ["phone-appearance"] }); });
      await waitFor(() => expect(document.documentElement.dataset.skin).toBe(["studio", "sky-grove"].includes(appearance) ? appearance : undefined));
      expect(screen.getByRole("heading", { name: "Away, not out of the loop." })).toBe(heading);
    }
    appearance = "studio";
    await act(async () => { await client.invalidateQueries({ queryKey: ["phone-appearance"] }); });
    await waitFor(() => expect(document.documentElement.dataset.skin).toBe("studio"));
    failed = true;
    await act(async () => { await client.invalidateQueries({ queryKey: ["phone-appearance"] }); });
    await waitFor(() => expect(document.documentElement.hasAttribute("data-skin")).toBe(false));
  });

  it("opens the companion navigation and dismisses it without changing the page", async () => {
    renderApp();
    fireEvent.click(await screen.findByRole("button", { name: "Open navigation" }));
    expect(screen.getByRole("navigation", { name: "Mobile navigation" })).toHaveTextContent("This phone");
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(window.location.pathname).toBe("/");
  });
  it("opens Home on what needs the person", async () => {
    renderApp();
    expect(await screen.findByText("implementer@my-app")).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Needs you" })).toHaveTextContent("needs permission");
    expect(await screen.findByRole("region", { name: "Projects" })).toHaveTextContent("My app");
    expect(screen.queryByRole("region", { name: "Moving" })).toBeNull();
    expect(screen.queryByRole("region", { name: "Since you last looked" })).toBeNull();
  });

  // FS-20.R40 — tasks left the phone; an old task link lands on Home.
  it("opens Home for a /task/<id> link", async () => {
    window.history.replaceState(null, "", "/task/t1");
    renderApp();
    expect(await screen.findByRole("region", { name: "Needs you" })).toHaveTextContent("needs permission");
    expect(screen.queryByText(/task/i)).toBeNull();
  });

  it("pairs from the QR link's code once the Mac allows it", async () => {
    paired = false;
    window.history.replaceState(null, "", "/pair#abcd2345");
    renderApp();
    const code = await screen.findByLabelText("Pairing code");
    expect(code).toHaveValue("ABCD2345");
    expect(window.location.hash).toBe("");
    fireEvent.click(screen.getByRole("button", { name: "Pair" }));
    expect(await screen.findByText("implementer@my-app")).toBeInTheDocument();
    expect(claims).toBe(1);
    expect(waits).toBe(2);
    expect(localStorage.getItem("chuck.paired")).toBe("1");
  });

  it("tells the phone to start again when the code is refused", async () => {
    paired = false;
    renderApp();
    fireEvent.change(await screen.findByLabelText("Pairing code"), { target: { value: "WRONG234" } });
    fireEvent.click(screen.getByRole("button", { name: "Pair" }));
    expect(await screen.findByText(/Show a new code in Chuck on your Mac/)).toBeInTheDocument();
  });

  it("says a previously paired phone was unpaired", async () => {
    paired = false;
    localStorage.setItem("chuck.paired", "1");
    renderApp();
    expect(await screen.findByText("This phone was unpaired")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Pair again" }));
    expect(await screen.findByLabelText("Pairing code")).toBeInTheDocument();
  });

  it("marks the Mac unreachable and the content stale", async () => {
    renderApp();
    await screen.findByText("implementer@my-app");
    act(() => useConnection.getState().setLink("unreachable"));
    expect(screen.getByRole("alert")).toHaveTextContent(/Mac unreachable since/);
    expect(document.querySelector("main")?.getAttribute("data-stale")).toBe("true");
    act(() => useConnection.getState().setLink("connected"));
    await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
  });

  it("replaces stale agents only after reconnect hydration completes", async () => {
    renderApp();
    await screen.findByText("implementer@my-app");
    const stream = FakeEventSource.last!;
    act(() => {
      useConnection.setState({
        agents: { stale: { agent_id: "stale", role: "chucky", project: "old", state: "busy" } as never },
        transcriptRev: { stale: 3 },
      });
      stream.onopen?.();
      stream.emit("state_update", { data: { agent_id: "fresh", role: "chucky", project: "new", state: "idle" } });
    });
    expect(useConnection.getState().link).toBe("reconnecting");
    expect(useConnection.getState().agents.stale).toBeDefined();
    act(() => stream.emit("state_update", { agent_id: "__hydrated__", data: { hydrated: true } }));
    expect(useConnection.getState().link).toBe("connected");
    expect(Object.keys(useConnection.getState().agents)).toEqual(["fresh"]);
    expect(useConnection.getState().transcriptRev).toEqual({});
  });
});

// The desktop bundle never imports the phone app (TS-08.R73).
it("keeps ui/src/remote out of the desktop bundle", () => {
  const src = path.resolve(__dirname, "..");
  const offenders: string[] = [];
  const walk = (dir: string) => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        if (full !== __dirname) walk(full);
      } else if (/\.(ts|tsx)$/.test(entry.name) && /from\s+["'][./]*(?:\.\.\/)*remote\//.test(fs.readFileSync(full, "utf8"))) {
        offenders.push(path.relative(src, full));
      }
    }
  };
  walk(src);
  expect(offenders).toEqual([]);
});

// Live reasoning reaches an open phone conversation through its own stream,
// and completed turns read the same quiet way as on the desktop (FS-03.A54, A57; TS-08.R106).
describe("phone conversation turns", () => {
  const phoneAgent = { agent_id: "a1", name: "", role: "implementer", project: "my-app", backend: "claude", model: "m", interface: "chat", running: true, state: "busy", detail: "" };
  const wire = (seq: number, type: string, data: Record<string, unknown>) => ({ agent_id: "a1", seq, type, ts: "", data });
  let window_: Record<string, unknown>[] = [];
  beforeEach(() => {
    window_ = [
      wire(1, "user_text", { text: "Fix it" }),
      wire(2, "assistant_text", { delta: "Looking first." }),
      wire(3, "tool_call", { tool_call_id: "t1", name: "Read" }),
      wire(4, "assistant_text", { delta: "Fixed." }),
      wire(5, "turn_end", { stop_reason: "end_turn" }),
      wire(6, "user_text", { text: "Now test" }),
    ];
    server.use(
      http.get("/api/sessions/a1/transcript", () => HttpResponse.json({ agent_id: "a1", events: window_, has_more: false })),
      http.get("/api/sessions/a1/prompt", () => HttpResponse.json({ error: { code: "not_found", message: "none" } }, { status: 404 })),
    );
  });

  it("collapses a completed turn, streams open live thoughts and drops them on reconnect", async () => {
    window.history.replaceState(null, "", "/agent/a1");
    renderApp();
    const stream = await waitFor(() => FakeEventSource.last!);
    act(() => {
      stream.onopen?.();
      stream.emit("state_update", { data: phoneAgent });
      stream.emit("state_update", { agent_id: "__hydrated__", data: { hydrated: true } });
    });
    await screen.findByText("Fixed.");
    expect(screen.queryByText("Looking first.")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: /Show activity/ }));
    expect(screen.getByText("Looking first.")).toBeInTheDocument();

    act(() => stream.emit("runtime_activity", { agent_id: "a1", data: { agent_id: "a1", generation: "g1", span_id: "r1", kind: "reasoning_delta", delta: "Planning tests" } }));
    const thought = await screen.findByRole("button", { name: /Thinking/ });
    expect(thought).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("Planning tests")).toBeInTheDocument();
    // It belongs to the open turn, after the latest prompt.
    expect(screen.getByText("Now test").compareDocumentPosition(thought) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

    act(() => stream.onopen?.());
    await waitFor(() => expect(screen.queryByText("Planning tests")).toBeNull());
  });

  const thinking = (span: string, delta: string) =>
    ({ agent_id: "a1", data: { agent_id: "a1", generation: "g1", span_id: span, kind: "reasoning_delta", delta } });

  // TS-08.R103 (QT-04) — a disconnect that misses a turn's end must not let the
  // next turn's thought join the stale window's turn.
  it("admits no thought after a reconnect until the window has re-read", async () => {
    window_ = [wire(1, "user_text", { text: "Old ask" })];
    window.history.replaceState(null, "", "/agent/a1");
    renderApp();
    const stream = await waitFor(() => FakeEventSource.last!);
    const hydrate = () => act(() => {
      stream.onopen?.();
      stream.emit("state_update", { data: phoneAgent });
      stream.emit("state_update", { agent_id: "__hydrated__", data: { hydrated: true } });
    });
    hydrate();
    await screen.findByText("Old ask");

    // Missed while disconnected: the answer, the root end and the next prompt.
    window_ = [...window_, wire(2, "assistant_text", { delta: "Old answer" }), wire(3, "turn_end", { stop_reason: "end_turn" }), wire(4, "user_text", { text: "New ask" })];
    hydrate();
    act(() => stream.emit("runtime_activity", thinking("r1", "Stale thought")));
    await screen.findByText("New ask");
    act(() => stream.emit("runtime_activity", thinking("r2", "Fresh thought")));
    const fresh = await screen.findByText("Fresh thought");
    expect(screen.queryByText("Stale thought")).toBeNull();
    expect(screen.getByText("New ask").compareDocumentPosition(fresh) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    // The completed first turn holds no hideable activity, so no thought was tucked into it.
    expect(screen.queryByRole("button", { name: /Show activity/ })).toBeNull();
  });

  // TS-08.R102 (QT-05) — when a long turn pushes its opening boundary out of the
  // 150-event window, its thoughts and their collapse choice keep their turn.
  it("keeps a live turn's identity as the window slides past its boundary", async () => {
    window_ = [wire(9, "assistant_text", { delta: "Earlier answer" }), wire(10, "turn_end", { stop_reason: "end_turn" }), wire(11, "user_text", { text: "Long job" })];
    let hasMore = true;
    server.use(http.get("/api/sessions/a1/transcript", () => HttpResponse.json({ agent_id: "a1", events: window_, has_more: hasMore })));
    window.history.replaceState(null, "", "/agent/a1");
    renderApp();
    const stream = await waitFor(() => FakeEventSource.last!);
    act(() => {
      stream.onopen?.();
      stream.emit("state_update", { data: phoneAgent });
      stream.emit("state_update", { agent_id: "__hydrated__", data: { hydrated: true } });
    });
    await screen.findByText("Long job");
    await waitFor(() => expect(screen.getByText("Long job")).toBeInTheDocument());
    act(() => stream.emit("runtime_activity", thinking("r1", "First idea")));
    const toggle = await screen.findByRole("button", { name: /Thinking/ });
    fireEvent.click(toggle);
    expect(screen.queryByText("First idea")).toBeNull();

    // 150 rows from the prompt on: the root end at seq 10 has left the window.
    window_ = [wire(11, "user_text", { text: "Long job" }), ...Array.from({ length: 149 }, (_, i) => wire(12 + i, "tool_call", { tool_call_id: `t${i}`, name: "Read" }))];
    hasMore = true;
    act(() => stream.emit("new_message", { agent_id: "a1", data: { seq: 160 } }));
    await screen.findByRole("button", { name: /Ran 149 tools/ });
    act(() => stream.emit("runtime_activity", thinking("r2", "Second idea")));
    await waitFor(() => expect(screen.getAllByRole("button", { name: /Thinking/ })).toHaveLength(2));
    for (const button of screen.getAllByRole("button", { name: /Thinking/ })) expect(button).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("Second idea")).toBeNull();
  });
});
