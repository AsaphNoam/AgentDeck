import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach, beforeEach, vi } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
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
  return render(
    <MemoryRouter>
      <CardContextMenu />
    </MemoryRouter>,
  );
}

describe("CardContextMenu error surfacing", () => {
  it("shows an error toast with the server message when switch-runtime fails", async () => {
    vi.spyOn(window, "prompt").mockReturnValue("terminal");
    server.use(
      http.post("/api/sessions/:id/switch-runtime", () =>
        HttpResponse.json({ error: { code: "no_change", message: "no runtime change requested" } }, { status: 400 }),
      ),
    );

    renderMenu();
    fireEvent.click(screen.getByRole("button", { name: /Switch runtime/i }));

    await waitFor(() =>
      expect(useUiStore.getState().toasts.some((t) => t.type === "error" && t.body === "no runtime change requested")).toBe(true),
    );
  });

  it("shows an error toast when rename fails", async () => {
    vi.spyOn(window, "prompt").mockReturnValue("beta");
    server.use(
      http.post("/api/sessions/:id/rename", () =>
        HttpResponse.json({ error: { code: "validation", message: "bad name" } }, { status: 422 }),
      ),
    );

    renderMenu();
    fireEvent.click(screen.getByRole("button", { name: /Rename/i }));

    await waitFor(() =>
      expect(useUiStore.getState().toasts.some((t) => t.type === "error" && t.title === "Rename failed")).toBe(true),
    );
  });

  it("shows an error toast when stop fails", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    server.use(
      http.post("/api/sessions/:id/stop", () =>
        HttpResponse.json({ error: { code: "internal", message: "boom" } }, { status: 500 }),
      ),
    );

    renderMenu();
    fireEvent.click(screen.getByRole("button", { name: /^Stop$/i }));

    await waitFor(() =>
      expect(useUiStore.getState().toasts.some((t) => t.type === "error" && t.title === "Stop failed")).toBe(true),
    );
  });

  it("shows an error toast when move-to-group fails", async () => {
    vi.spyOn(window, "prompt").mockReturnValue("squad");
    server.use(
      http.post("/api/sessions/:id/identity", () =>
        HttpResponse.json({ error: { code: "validation", message: "bad group" } }, { status: 422 }),
      ),
    );

    renderMenu();
    fireEvent.click(screen.getByRole("button", { name: /Move to group/i }));

    await waitFor(() =>
      expect(useUiStore.getState().toasts.some((t) => t.type === "error" && t.title === "Move to group failed")).toBe(true),
    );
  });
});
