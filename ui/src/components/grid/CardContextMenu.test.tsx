import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach, beforeEach, vi } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { CardContextMenu } from "./CardContextMenu";
import { useAgentStore } from "../../store/agentStore";
import { useUiStore } from "../../store/uiStore";
import type { AgentState } from "../../api/types";

const agent: AgentState = {
  agent_id: "a_1",
  name: "alpha",
  role: "implementer",
  project: "my-app",
  backend: "claude",
  model: "sonnet",
  interface: "chat",
  created_at: "2026-06-30T00:00:00Z",
  running: true,
  state: "idle",
  detail: "",
  context_pct: 0.1,
  updated_at: 1,
};

const server = setupServer();

beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterAll(() => server.close());

beforeEach(() => {
  useAgentStore.setState({ agents: { a_1: agent }, order: ["a_1"], hydrating: false });
  useUiStore.setState({ contextMenu: { agentId: "a_1", x: 0, y: 0 }, toasts: [] });
});

afterEach(() => {
  cleanup();
  server.resetHandlers();
  vi.restoreAllMocks();
});

function renderMenu() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <CardContextMenu />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CardContextMenu error surfacing", () => {
  it("resumes a stopped agent from its card", async () => {
    useAgentStore.setState({ agents: { a_1: { ...agent, running: false } }, order: ["a_1"], hydrating: false });
    let calls = 0;
    server.use(
      http.post("/api/sessions/:id/resume", () => {
        calls += 1;
        return HttpResponse.json({ resumed: true });
      }),
    );

    renderMenu();
    fireEvent.click(screen.getByRole("button", { name: /^Resume$/i }));

    await waitFor(() => expect(calls).toBe(1));
  });

  it("omits Resume for a running agent", () => {
    renderMenu();
    expect(screen.queryByRole("button", { name: /^Resume$/i })).not.toBeInTheDocument();
  });

  it("shows an error toast when resume fails", async () => {
    useAgentStore.setState({ agents: { a_1: { ...agent, running: false } }, order: ["a_1"], hydrating: false });
    server.use(
      http.post("/api/sessions/:id/resume", () =>
        HttpResponse.json({ error: { code: "resume_failed", message: "session is unavailable" } }, { status: 422 }),
      ),
    );

    renderMenu();
    fireEvent.click(screen.getByRole("button", { name: /^Resume$/i }));
    await waitFor(() =>
      expect(useUiStore.getState().toasts.some((toast) => toast.type === "error" && toast.title === "Resume failed" && toast.body === "session is unavailable")).toBe(true),
    );
  });

  it("shows an error toast with the server message when switch-runtime fails", async () => {
    server.use(
      http.get("/api/backends", () => HttpResponse.json({ version: 2, backends: { claude: { name: "Claude", type: "claude-acp", default_model: "sonnet", models: { sonnet: { name: "Sonnet", model: "sonnet" } } } } })),
      http.get("/api/capabilities", () => HttpResponse.json({ terminal: { available: true } })),
      http.post("/api/sessions/:id/switch-runtime", () =>
        HttpResponse.json({ error: { code: "no_change", message: "no runtime change requested" } }, { status: 400 }),
      ),
    );

    renderMenu();
    fireEvent.click(screen.getByRole("button", { name: /Switch runtime/i }));
    const interfaceSelect = await screen.findByLabelText("Interface");
    fireEvent.change(interfaceSelect, { target: { value: "terminal" } });
    fireEvent.click(screen.getByRole("button", { name: /^Switch runtime$/i }));

    await waitFor(() =>
      expect(useUiStore.getState().toasts.some((t) => t.type === "error" && t.body === "no runtime change requested")).toBe(true),
    );
  });

  it("shows an error toast when rename fails", async () => {
    server.use(
      http.post("/api/sessions/:id/rename", () =>
        HttpResponse.json({ error: { code: "validation", message: "bad name" } }, { status: 422 }),
      ),
    );

    renderMenu();
    fireEvent.click(screen.getByRole("button", { name: /Rename/i }));
    fireEvent.change(await screen.findByLabelText("Name"), { target: { value: "beta" } });
    fireEvent.click(screen.getByRole("button", { name: /^Rename$/i }));

    await waitFor(() =>
      expect(useUiStore.getState().toasts.some((t) => t.type === "error" && t.title === "Rename failed")).toBe(true),
    );
  });

  it("shows an error toast when stop fails", async () => {
    server.use(
      http.post("/api/sessions/:id/stop", () =>
        HttpResponse.json({ error: { code: "internal", message: "boom" } }, { status: 500 }),
      ),
    );

    renderMenu();
    fireEvent.click(screen.getByRole("button", { name: /^Stop$/i }));
    fireEvent.click(await screen.findByRole("button", { name: "Stop agent" }));

    await waitFor(() =>
      expect(useUiStore.getState().toasts.some((t) => t.type === "error" && t.title === "Stop failed")).toBe(true),
    );
  });

  it("clones an agent by launching a new session with the same config", async () => {
    let received: Record<string, unknown> | null = null;
    server.use(
      http.post("/api/sessions", async ({ request }) => {
        received = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json({ agent: { agent_id: "a_2", name: "beta" } }, { status: 201 });
      }),
    );

    renderMenu();
    fireEvent.click(screen.getByRole("button", { name: /^Clone$/i }));

    await waitFor(() => expect(received).not.toBeNull());
    expect(received).toMatchObject({
      role: "implementer",
      project: "my-app",
      backend: "claude",
      model: "sonnet",
      interface: "chat",
    });
    // No name is sent so the server auto-suggests a fresh one.
    expect(received).not.toHaveProperty("name");
  });

  it("shows an error toast when clone fails", async () => {
    server.use(
      http.post("/api/sessions", () =>
        HttpResponse.json({ error: { code: "internal", message: "project cwd does not exist" } }, { status: 502 }),
      ),
    );

    renderMenu();
    fireEvent.click(screen.getByRole("button", { name: /^Clone$/i }));

    await waitFor(() =>
      expect(useUiStore.getState().toasts.some((t) => t.type === "error" && t.title === "Clone failed")).toBe(true),
    );
  });

  it("shows an error toast when move-to-group fails", async () => {
    server.use(
      http.post("/api/sessions/:id/identity", () =>
        HttpResponse.json({ error: { code: "validation", message: "bad group" } }, { status: 422 }),
      ),
    );

    renderMenu();
    fireEvent.click(screen.getByRole("button", { name: /Move to group/i }));
    fireEvent.change(await screen.findByLabelText("Group"), { target: { value: "squad" } });
    fireEvent.click(screen.getByRole("button", { name: /^Move$/i }));

    await waitFor(() =>
      expect(useUiStore.getState().toasts.some((t) => t.type === "error" && t.title === "Move to group failed")).toBe(true),
    );
  });

  it("cancels rename without sending a request", async () => {
    let calls = 0;
    server.use(http.post("/api/sessions/:id/rename", () => { calls += 1; return HttpResponse.json({}); }));

    renderMenu();
    fireEvent.click(screen.getByRole("button", { name: /Rename/i }));
    fireEvent.click(await screen.findByRole("button", { name: "Cancel" }));

    expect(calls).toBe(0);
  });
});
