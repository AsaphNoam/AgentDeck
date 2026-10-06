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
});
afterAll(() => server.close());

function renderApp() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <PhoneApp />
    </QueryClientProvider>,
  );
}

describe("PhoneApp", () => {
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
});
