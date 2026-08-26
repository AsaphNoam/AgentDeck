import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { CardGrid, mergeScopedOrder } from "./CardGrid";
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

  // FS-02.R44 / A26: the dashboard's only task-shaped element is how many tasks
  // in view need a person — parked work and work whose agent went away — and it
  // counts no ordinary armed, ready, starting, or running task.
  it("counts only the tasks in view that need attention", async () => {
    server.use(http.get("/api/tasks", () => HttpResponse.json({
      tasks: [
        { task_id: "tk_1", project: "my-app", display_name: "parked", instruction: "x", target_kind: "launch", state: "dependency_failed", created_by_kind: "person", revision: 1, created_at: "2026-08-24T10:00:00Z", arms: [], attachments: [] },
        { task_id: "tk_2", project: "my-app", display_name: "abandoned", instruction: "x", target_kind: "launch", state: "interrupted", created_by_kind: "person", revision: 1, created_at: "2026-08-24T10:00:00Z", arms: [], attachments: [] },
        { task_id: "tk_3", project: "my-app", display_name: "running", instruction: "x", target_kind: "launch", state: "running", created_by_kind: "person", revision: 1, created_at: "2026-08-24T10:00:00Z", arms: [], attachments: [] },
        { task_id: "tk_4", project: "my-app", display_name: "armed", instruction: "x", target_kind: "launch", state: "armed", created_by_kind: "person", revision: 1, created_at: "2026-08-24T10:00:00Z", arms: [], attachments: [] },
      ],
    })));
    act(() => useAgentStore.setState({ agents: { a_1: agent("a_1") }, order: ["a_1"] }));
    renderWithQuery(<CardGrid projectID="my-app" projectTitle="My App" />);

    const link = await screen.findByText("2 tasks need attention");
    expect(link).toHaveAttribute("href", "/tasks?project=my-app");
  });

  it("reads zero when no task needs attention", async () => {
    server.use(http.get("/api/tasks", () => HttpResponse.json({ tasks: [] })));
    act(() => useAgentStore.setState({ agents: { a_1: agent("a_1") }, order: ["a_1"] }));
    renderWithQuery(<CardGrid projectID="my-app" projectTitle="My App" />);
    expect(await screen.findByText("0 tasks need attention")).toBeInTheDocument();
  });

  it("does not claim zero attention when the task query fails", async () => {
    server.use(http.get("/api/tasks", () => HttpResponse.json({ error: "unavailable" }, { status: 500 })));
    act(() => useAgentStore.setState({ agents: { a_1: agent("a_1") }, order: ["a_1"] }));
    renderWithQuery(<CardGrid projectID="my-app" projectTitle="My App" />);
    expect(await screen.findByText("Task attention unavailable")).toHaveAttribute("href", "/tasks?project=my-app");
    expect(screen.queryByText("0 tasks need attention")).not.toBeInTheDocument();
  });

	 it("keeps task attention visible when the project has no agents", async () => {
		server.use(http.get("/api/tasks", () => HttpResponse.json({ tasks: [
			{ task_id: "tk_1", project: "my-app", display_name: "parked", instruction: "x", target_kind: "launch", state: "dependency_failed", created_by_kind: "person", revision: 1, created_at: "2026-08-24T10:00:00Z", arms: [], attachments: [] },
		] })));
		renderWithQuery(<CardGrid projectID="my-app" projectTitle="My App" />);
		expect(await screen.findByText("1 task need attention")).toHaveAttribute("href", "/tasks?project=my-app");
	 });

  // FS-02.A25: New Agent opened from a project dashboard is bound to that
  // route's project (via fixedProject), so a person cannot accidentally launch it
  // elsewhere.
  it("locks a scoped New Agent launch to the fixed project", async () => {
    let capturedBody: unknown;
    server.use(
      http.post("/api/sessions", async ({ request }) => {
        capturedBody = await request.json();
        return HttpResponse.json({ agent: { agent_id: "a_2", name: "Atlas" } }, { status: 201 });
      }),
    );

    renderWithQuery(<CardGrid projectID="my-app" fixedProject="my-app" projectTitle="My App" />);
    fireEvent.click(await screen.findByText("New Agent"));
    await screen.findByText("New agent");

    expect(screen.queryByText("Project")).toBeNull();
    fireEvent.click(screen.getByText("Launch"));

    await waitFor(() => expect(capturedBody).toBeDefined());
    expect((capturedBody as Record<string, unknown>).project).toBe("my-app");
  });

  // INV §10 / FS-02.R43: a scoped grid for a project that is NOT a current catalog
  // member (e.g. an unavailable project shown because it still has live agents)
  // passes projectID for filtering but no fixedProject, so New Agent must keep the
  // Project picker rather than hiding it and submitting a project the server
  // rejects with `unknown project`.
  it("keeps the Project picker when no fixedProject is supplied", async () => {
    act(() => useAgentStore.setState({ agents: { a_1: agent("a_1") }, order: ["a_1"] }));
    renderWithQuery(<CardGrid projectID="my-app" projectTitle="My App" />);
    // The populated grid header uses "New agent"; the empty state uses "New Agent".
    fireEvent.click(await screen.findByText("New agent"));

    // The modal opened with no fixedProject, so the Project picker is present.
    expect(await screen.findByText("Project")).toBeInTheDocument();
  });

  it("merges a scoped reorder back into the shared layout", () => {
    expect(mergeScopedOrder(
      ["a-one", "b-one", "a-two", "b-two"],
      ["a-one", "a-two"],
      ["a-two", "a-one"],
    )).toEqual(["a-two", "b-one", "a-one", "b-two"]);
  });

  // FS-02.A13: agent state keeps the durable project id, but Dashboard metadata
  // must use the project's human-readable title whenever configuration resolves it.
  it("uses the configured project title on agent cards", async () => {
    act(() => useAgentStore.setState({ agents: { a_1: agent("a_1") }, order: ["a_1"] }));
    renderWithQuery(<CardGrid />);

    expect(await screen.findByText("implementer · My App")).toBeInTheDocument();
    expect(screen.queryByText("implementer · my-app")).not.toBeInTheDocument();
  });

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
