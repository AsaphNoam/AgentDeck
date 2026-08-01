import React from "react";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { fireEvent, render, screen, cleanup, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { ProjectDashboard } from "./ProjectDashboard";
import { useAgentStore } from "../../store/agentStore";
import type { AgentState } from "../../api/types";

const server = setupServer(
  http.get("/api/projects", () => HttpResponse.json({
    app: { title: "App", color: [100, 116, 139], cwd: "/tmp/app", add_dirs: [], context_prompt: "", archived: false },
  })),
);

beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterEach(() => {
  cleanup();
  server.resetHandlers();
  useAgentStore.setState({ agents: {}, order: [] });
});
afterAll(() => server.close());

function agent(id: string, state: AgentState["state"], archived = false): AgentState {
  return {
    agent_id: id, name: id, role: "implementer", project: "app", backend: "claude", model: "sonnet",
    interface: "chat", created_at: "2026-07-29T00:00:00Z", running: state === "busy", state, detail: "",
    context_pct: 0, updated_at: 1, archived,
  };
}

function renderDashboard() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter><ProjectDashboard /></MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("ProjectDashboard", () => {
  it("shows project color and live state summary, with all active-project actions", async () => {
    useAgentStore.setState({
      agents: {
        busy: agent("busy", "busy"),
        done: agent("done", "done"),
        archived: agent("archived", "done", true),
      },
      order: ["busy", "done", "archived"],
    });

    renderDashboard();
    expect(await screen.findByText("App")).toBeInTheDocument();
    expect(screen.getByText("2 agents")).toBeInTheDocument();
    expect(screen.getByText("1 busy · 1 done")).toBeInTheDocument();
    expect(screen.getByLabelText("Project color")).toHaveStyle({ background: "rgb(100, 116, 139)" });

    const card = screen.getByText("App").closest("article")!;
    fireEvent.contextMenu(card, { clientX: 40, clientY: 50 });
    expect(screen.getByRole("button", { name: "Rename" })).toBeInTheDocument();
    expect(screen.getByText("Change color")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Archive" })).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: /project color$/ })).toHaveLength(6);
    expect(screen.getByRole("button", { name: "Slate project color" })).toHaveAttribute("aria-pressed", "true");
    expect(card.querySelector(".context-menu")).toBeNull();
  });

  it("renames through a dialog and leaves the request untouched on cancel", async () => {
    let saved: Record<string, unknown> | null = null;
    server.use(http.put("/api/projects/app", async ({ request }) => {
      saved = await request.json() as Record<string, unknown>;
      return HttpResponse.json({ project: "app", ...saved });
    }));
    renderDashboard();
    const card = (await screen.findByText("App")).closest("article")!;
    fireEvent.contextMenu(card);
    fireEvent.click(screen.getByRole("button", { name: "Rename" }));
    fireEvent.change(await screen.findByLabelText("Name"), { target: { value: "Renamed app" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(saved).toMatchObject({ title: "Renamed app" }));

    expect(saved).toMatchObject({ title: "Renamed app" });
  });

  // FS-02.A22 / FS-04.A19: color selection is an immediate API update from the
  // portal menu and carries the preset's RGB triple unchanged.
  it("updates the project to the chosen preset from the portal menu", async () => {
    let saved: Record<string, unknown> | null = null;
    let color = [100, 116, 139];
    server.use(
      http.get("/api/projects", () => HttpResponse.json({
        app: { title: "App", color, cwd: "/tmp/app", add_dirs: [], context_prompt: "", archived: false },
      })),
      http.put("/api/projects/app", async ({ request }) => {
        saved = await request.json() as Record<string, unknown>;
        color = saved.color as number[];
        return HttpResponse.json({ project: "app", ...saved });
      }),
    );
    renderDashboard();
    const card = (await screen.findByText("App")).closest("article")!;
    fireEvent.contextMenu(card);
    fireEvent.click(screen.getByRole("button", { name: "Blue project color" }));
    await waitFor(() => expect(saved).toMatchObject({ color: [59, 130, 246] }));
  });

  // FS-02.R37 / FS-12.R26 / INV §8: project Rename retains the server's
  // field-level validation detail rather than reducing it to HTTP 400.
  it("shows the field-level validation error when a project title is too long", async () => {
    server.use(http.put("/api/projects/app", () => HttpResponse.json({
      errors: [{ field: "title", message: "must be at most 120 characters" }],
    }, { status: 400 })));
    renderDashboard();
    const card = (await screen.findByText("App")).closest("article")!;
    fireEvent.contextMenu(card);
    fireEvent.click(screen.getByRole("button", { name: "Rename" }));
    fireEvent.change(await screen.findByLabelText("Name"), { target: { value: "x".repeat(121) } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByText("title: must be at most 120 characters")).toBeInTheDocument();
  });

  // FS-02.A24: the header button and the background right-click both open the
  // create modal; a card right-click opens the card menu, not the create menu.
  it("opens the create modal from the header button and the background menu, not from a card", async () => {
    renderDashboard();
    const card = (await screen.findByText("App")).closest("article")!;

    fireEvent.click(screen.getByRole("button", { name: "New project" }));
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "New project" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());

    fireEvent.contextMenu(screen.getByRole("heading", { name: "Projects" }));
    const menuItem = (await screen.findAllByRole("button", { name: "New project" })).find((b) => b.getAttribute("data-slot") === "item")!;
    fireEvent.click(menuItem);
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());

    fireEvent.contextMenu(card);
    expect(await screen.findByRole("button", { name: "Rename" })).toBeInTheDocument();
    // Only the persistent header button remains; the create menu never opened.
    expect(screen.getAllByRole("button", { name: "New project" })).toHaveLength(1);
  });

  // FS-02.A24: a valid submission creates the project through POST /api/projects
  // and its card appears from the refreshed catalog with no manual reload.
  it("creates a project and shows its card without a reload", async () => {
    const projectsData: Record<string, unknown> = {
      app: { title: "App", color: [100, 116, 139], cwd: "/tmp/app", add_dirs: [], context_prompt: "", archived: false },
    };
    let created: Record<string, unknown> | null = null;
    server.use(
      http.get("/api/projects", () => HttpResponse.json(projectsData)),
      http.post("/api/projects", async ({ request }) => {
        created = await request.json() as Record<string, unknown>;
        projectsData["new-app"] = { ...created, project: undefined, archived: false };
        return HttpResponse.json({ project: "new-app", ...created });
      }),
    );
    renderDashboard();
    await screen.findByText("App");
    fireEvent.click(screen.getByRole("button", { name: "New project" }));
    fireEvent.change(await screen.findByPlaceholderText("e.g. My App"), { target: { value: "New App" } });
    fireEvent.change(screen.getByPlaceholderText("~/Projects/my-app"), { target: { value: "/tmp/new-app" } });
    fireEvent.click(screen.getByRole("button", { name: "Create" }));
    expect(await screen.findByText("New App")).toBeInTheDocument();
    expect(created).toMatchObject({ project: "", title: "New App", cwd: "/tmp/new-app" });
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  // FS-02.A24: an API failure keeps the modal open and surfaces the server message.
  it("keeps the create modal open with the server error on failure", async () => {
    server.use(http.post("/api/projects", () => HttpResponse.json({
      errors: [{ field: "title", message: "already exists" }],
    }, { status: 400 })));
    renderDashboard();
    await screen.findByText("App");
    fireEvent.click(screen.getByRole("button", { name: "New project" }));
    fireEvent.change(await screen.findByPlaceholderText("e.g. My App"), { target: { value: "App" } });
    fireEvent.change(screen.getByPlaceholderText("~/Projects/my-app"), { target: { value: "/tmp/app" } });
    fireEvent.click(screen.getByRole("button", { name: "Create" }));
    expect(await screen.findByText("title: already exists")).toBeInTheDocument();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("shows archive consequences and cancels without archiving", async () => {
    let archives = 0;
    server.use(http.post("/api/projects/app/archive", () => { archives += 1; return HttpResponse.json({}); }));
    renderDashboard();
    const card = (await screen.findByText("App")).closest("article")!;
    fireEvent.contextMenu(card);
    fireEvent.click(screen.getByRole("button", { name: "Archive" }));
    expect(await screen.findByText(/Running agents will be stopped/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(archives).toBe(0);
  });
});
