import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach, vi } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { CardGrid, mergeScopedOrder } from "./CardGrid";
import { useAgentStore } from "../../store/agentStore";
import type { AgentState } from "../../api/types";

// FS-02.A28 needs two things a real drag in jsdom cannot give: the id order handed to
// dnd-kit (which is what its indices and rect transforms are derived from) and a drop
// whose active/over pair we choose. Both providers are replaced by pass-throughs that
// record what CardGrid passes them; everything else in either package stays real.
const dnd = vi.hoisted(() => ({
  items: [] as string[],
  onDragEnd: undefined as ((event: { active: { id: string }; over: { id: string } | null }) => void) | undefined,
}));

vi.mock("@dnd-kit/core", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@dnd-kit/core")>()),
  DndContext: ({ children, onDragEnd }: { children: React.ReactNode; onDragEnd: typeof dnd.onDragEnd }) => {
    dnd.onDragEnd = onDragEnd;
    return <>{children}</>;
  },
}));

vi.mock("@dnd-kit/sortable", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@dnd-kit/sortable")>()),
  SortableContext: ({ children, items }: { children: React.ReactNode; items: string[] }) => {
    dnd.items = items;
    return <>{children}</>;
  },
}));

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
  dnd.items = [];
  dnd.onDragEnd = undefined;
  act(() => useAgentStore.setState({ agents: {}, order: [] }));
});
afterAll(() => server.close());

function agent(id: string, overrides: Partial<AgentState> = {}): AgentState {
  return {
    agent_id: id, name: id, role: "implementer", project: "my-app", backend: "claude", model: "s",
    interface: "chat", created_at: "2026-07-10T00:00:00Z", running: true, state: "idle", detail: "",
    context_pct: 0, updated_at: 1, ...overrides,
  };
}

/** renderedIDs reads the cards in the order the grid actually drew them. */
function renderedIDs() {
  return [...document.querySelectorAll('[data-ui="agent-card"] [data-slot="identity"]')].map((node) => node.textContent);
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
		expect(await screen.findByText("1 task needs attention")).toHaveAttribute("href", "/tasks?project=my-app");
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
  // ---- FS-02.R45 / A28, FS-12.R37 / A13: running agents lead each group section ----

  function seedGrid(
    manualOrder: string[],
    agents: Record<string, AgentState>,
    groups: Record<string, { collapsed: boolean }> = {},
  ) {
    server.use(http.get("/api/layout", () =>
      HttpResponse.json({ order: manualOrder, density: { perRow: 3, gap: 16 }, groups })));
    act(() => useAgentStore.setState({ agents, order: manualOrder }));
  }

  function capturedLayoutOrders() {
    const orders: string[][] = [];
    server.use(http.put("/api/layout", async ({ request }) => {
      const body = (await request.json()) as { order: string[] };
      orders.push(body.order);
      return HttpResponse.json(body);
    }));
    return orders;
  }

  it("renders running agents before stopped ones and keeps the manual order inside each block", async () => {
    seedGrid(["a_1", "a_2", "a_3", "a_4"], {
      a_1: agent("a_1", { running: false }),
      a_2: agent("a_2"),
      a_3: agent("a_3", { running: false }),
      a_4: agent("a_4"),
    });
    renderWithQuery(<CardGrid />);

    await waitFor(() => expect(renderedIDs()).toEqual(["a_2", "a_4", "a_1", "a_3"]));
  });

  it("moves only the flipped card across the running boundary on a state_update", async () => {
    seedGrid(["a_1", "a_2", "a_3", "a_4"], {
      a_1: agent("a_1", { running: false }),
      a_2: agent("a_2"),
      a_3: agent("a_3", { running: false }),
      a_4: agent("a_4"),
    });
    renderWithQuery(<CardGrid />);
    await waitFor(() => expect(renderedIDs()).toEqual(["a_2", "a_4", "a_1", "a_3"]));

    act(() => useAgentStore.getState().applyStateUpdate(agent("a_2", { running: false })));

    // a_2 leaves the running block for its manual slot among the stopped cards; the
    // remaining running card and the other stopped cards hold their relative order.
    await waitFor(() => expect(renderedIDs()).toEqual(["a_4", "a_1", "a_2", "a_3"]));
  });

  it("keeps a raised-salience card in place, so only running moves a card", async () => {
    seedGrid(["a_1", "a_2", "a_3"], {
      a_1: agent("a_1"),
      a_2: agent("a_2"),
      a_3: agent("a_3", { running: false }),
    });
    renderWithQuery(<CardGrid />);
    await waitFor(() => expect(renderedIDs()).toEqual(["a_1", "a_2", "a_3"]));

    // FS-12.A13: waiting_input and error raise salience without reordering.
    act(() => useAgentStore.getState().applyStateUpdate(agent("a_2", { state: "waiting_input" })));
    await waitFor(() =>
      expect(document.querySelector('[data-ui="agent-card"][data-state="waiting_input"]')).not.toBeNull());
    expect(renderedIDs()).toEqual(["a_1", "a_2", "a_3"]);

    act(() => useAgentStore.getState().applyStateUpdate(agent("a_1", { state: "error" })));
    await waitFor(() =>
      expect(document.querySelector('[data-ui="agent-card"][data-state="error"]')).not.toBeNull());
    expect(renderedIDs()).toEqual(["a_1", "a_2", "a_3"]);
  });

  it("hands dnd-kit the exact rendered id order, skipping collapsed sections", async () => {
    seedGrid(
      ["a_1", "a_2", "a_3", "a_4"],
      {
        a_1: agent("a_1", { group: "alpha", running: false }),
        a_2: agent("a_2", { group: "alpha" }),
        a_3: agent("a_3", { running: false }),
        a_4: agent("a_4"),
      },
    );
    renderWithQuery(<CardGrid />);

    await waitFor(() => expect(renderedIDs()).toEqual(["a_2", "a_1", "a_4", "a_3"]));
    expect(dnd.items).toEqual(renderedIDs());

    cleanup();
    seedGrid(
      ["a_1", "a_2", "a_3", "a_4"],
      {
        a_1: agent("a_1", { group: "alpha", running: false }),
        a_2: agent("a_2", { group: "alpha" }),
        a_3: agent("a_3", { running: false }),
        a_4: agent("a_4"),
      },
      { alpha: { collapsed: true } },
    );
    renderWithQuery(<CardGrid />);

    // A collapsed section mounts no card, so its ids must not occupy sortable indices.
    await waitFor(() => expect(renderedIDs()).toEqual(["a_4", "a_3"]));
    expect(dnd.items).toEqual(["a_4", "a_3"]);
  });

  it("keeps the Ungrouped section last", async () => {
    seedGrid(["a_1", "a_2"], {
      a_1: agent("a_1"),
      a_2: agent("a_2", { group: "alpha" }),
    });
    renderWithQuery(<CardGrid />);

    await waitFor(() => expect(renderedIDs()).toEqual(["a_2", "a_1"]));
    expect([...document.querySelectorAll('[data-ui="agent-group"] header strong')].map((n) => n.textContent))
      .toEqual(["alpha", "Ungrouped"]);
  });

  it("commits the flat manual order for a drag inside one block", async () => {
    const orders = capturedLayoutOrders();
    seedGrid(["a_1", "a_2", "a_3", "a_4"], {
      a_1: agent("a_1", { running: false }),
      a_2: agent("a_2"),
      a_3: agent("a_3", { running: false }),
      a_4: agent("a_4"),
    });
    renderWithQuery(<CardGrid />);
    await waitFor(() => expect(renderedIDs()).toEqual(["a_2", "a_4", "a_1", "a_3"]));

    // Both stopped: a same-block drop, committed against the flat manual order exactly
    // as it was before the running-first split existed.
    act(() => dnd.onDragEnd?.({ active: { id: "a_1" }, over: { id: "a_3" } }));

    await waitFor(() => expect(orders.at(-1)).toEqual(["a_2", "a_3", "a_1", "a_4"]));
    await waitFor(() => expect(renderedIDs()).toEqual(["a_2", "a_4", "a_3", "a_1"]));
  });

  it("neither reorders nor saves a layout for a drop onto the other block", async () => {
    const orders = capturedLayoutOrders();
    seedGrid(["a_1", "a_2", "a_3", "a_4"], {
      a_1: agent("a_1", { running: false }),
      a_2: agent("a_2"),
      a_3: agent("a_3", { running: false }),
      a_4: agent("a_4"),
    });
    renderWithQuery(<CardGrid />);
    await waitFor(() => expect(renderedIDs()).toEqual(["a_2", "a_4", "a_1", "a_3"]));
    await waitFor(() => expect(orders.length).toBe(1));

    // a_1 is stopped and a_2 is running: manual order cannot cross the boundary.
    act(() => dnd.onDragEnd?.({ active: { id: "a_1" }, over: { id: "a_2" } }));

    await new Promise((resolve) => setTimeout(resolve, 600));
    expect(orders).toHaveLength(1);
    expect(renderedIDs()).toEqual(["a_2", "a_4", "a_1", "a_3"]);
  });
});
