import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { CardGrid } from "./CardGrid";
import { useAgentStore } from "../../store/agentStore";
import type { AgentState } from "../../api/types";

const server = setupServer(
  http.get("/api/layout", () => HttpResponse.json({ order: [], density: { perRow: 3, gap: 16 }, groups: {} })),
  http.get("/api/roles", () => HttpResponse.json({ implementer: { title: "Implementer", system_prompt: "" } })),
  http.get("/api/projects", () =>
    HttpResponse.json({ "my-app": { title: "My App", color: [1, 2, 3], cwd: "/tmp/my-app", add_dirs: [], context_prompt: "" } }),
  ),
  http.get("/api/backends", () =>
    HttpResponse.json({
      version: 2,
      backends: { claude: { name: "Claude", type: "claude-acp", default: true, default_model: "s", models: { s: { name: "S", model: "s" } } } },
    }),
  ),
  http.get("/api/config", () => HttpResponse.json({})),
  http.get("/api/capabilities", () =>
    HttpResponse.json({ terminal: { available: true, default_driver: "xterm", drivers: { xterm: true } } }),
  ),
);

beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterEach(() => {
  cleanup();
  server.resetHandlers();
  act(() => useAgentStore.setState({ agents: {}, order: [] }));
});
afterAll(() => server.close());

function agent(id: string): AgentState {
  return {
    agent_id: id, name: id, role: "implementer", project: "my-app", backend: "claude", model: "s",
    interface: "chat", created_at: "2026-07-10T00:00:00Z", running: true, state: "idle", detail: "",
    context_pct: 0, updated_at: 1,
  };
}

function renderWithQuery(ui: React.ReactElement) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CardGrid", () => {
  // J3b regression: the New-Agent modal must not remount when the first agent
  // appears (0→1). If it lived inside the empty/populated branches it would
  // unmount mid-launch and its onSuccess→onClose would never fire, leaving the
  // overlay stuck. Guard by asserting the exact modal DOM node survives the flip.
  it("keeps the open New-Agent modal mounted across the 0→1 transition", async () => {
    act(() => useAgentStore.setState({ agents: {}, order: [] }));
    renderWithQuery(<CardGrid />);

    // Empty state renders once the layout query resolves.
    const openButton = await screen.findByText("New Agent");
    fireEvent.click(openButton);

    // The modal is open — capture its DOM node.
    const title = await screen.findByText("New agent");
    const modalNode = title.closest(".dialog-content");
    expect(modalNode).toBeTruthy();

    // First agent arrives: the grid replaces the empty state (0→1).
    act(() => useAgentStore.getState().applyStateUpdate(agent("a_1")));
    await waitFor(() => expect(screen.getByText("Agents")).toBeInTheDocument());

    // The SAME modal node is still connected (not remounted) and still open.
    expect(modalNode && document.body.contains(modalNode)).toBe(true);
    expect(modalNode?.textContent).toContain("New agent");
  });
});
