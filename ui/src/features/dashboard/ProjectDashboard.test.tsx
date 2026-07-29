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
    app: { title: "App", color: [10, 20, 30], cwd: "/tmp/app", add_dirs: [], context_prompt: "", archived: false },
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
    expect(screen.getByLabelText("Project color")).toHaveStyle({ background: "rgb(10, 20, 30)" });

    fireEvent.contextMenu(screen.getByText("App").closest("article")!);
    expect(screen.getByRole("button", { name: "Rename" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Change color" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Archive" })).toBeInTheDocument();
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

    fireEvent.contextMenu(card);
    fireEvent.click(screen.getByRole("button", { name: "Change color" }));
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(saved).toMatchObject({ title: "Renamed app" });
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
