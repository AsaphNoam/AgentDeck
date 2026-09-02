import React from "react";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { ScopedProjectDashboard } from "./ProjectDashboard";
import { useAgentStore } from "../../store/agentStore";

// FS-02.R60's second entry point: the scoped project dashboard's own header
// offers the same fork action, and a worktree project's header names its branch
// (FS-19.R6).

const server = setupServer(
  http.get("/api/layout", () => HttpResponse.json({ order: [], density: { perRow: 3, gap: 12 }, expanded: [] })),
  http.get("/api/roles", () => HttpResponse.json({})),
  http.get("/api/backends", () => HttpResponse.json({ version: 2, backends: {} })),
  http.get("/api/capabilities", () => HttpResponse.json({})),
  http.get("/api/tasks", () => HttpResponse.json({ tasks: [] })),
);

beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterEach(() => {
  cleanup();
  server.resetHandlers();
  useAgentStore.setState({ agents: {}, order: [] });
});
afterAll(() => server.close());

function renderScoped(id: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[`/project/${id}`]}>
        <Routes>
          <Route path="project/:project" element={<ScopedProjectDashboard />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("ScopedProjectDashboard worktree surface", () => {
  it("offers the fork action in the header of a repo-backed project", async () => {
    server.use(http.get("/api/projects", () => HttpResponse.json({
      app: { title: "App", color: [100, 116, 139], cwd: "/tmp/app", add_dirs: [], context_prompt: "", archived: false, repo_backed: true },
    })));
    renderScoped("app");
    expect(await screen.findByRole("button", { name: "New worktree project" })).toBeInTheDocument();
  });

  it("offers no fork action when the project is not repo-backed", async () => {
    server.use(http.get("/api/projects", () => HttpResponse.json({
      app: { title: "App", color: [100, 116, 139], cwd: "/tmp/app", add_dirs: [], context_prompt: "", archived: false },
    })));
    renderScoped("app");
    await screen.findByText("Back to projects");
    expect(screen.queryByRole("button", { name: "New worktree project" })).toBeNull();
  });

  it("names the branch in the header of a worktree project", async () => {
    server.use(http.get("/api/projects", () => HttpResponse.json({
      fork: {
        title: "Fork", color: [100, 116, 139], cwd: "/home/wt/fork", add_dirs: [], context_prompt: "",
        archived: false, repo_backed: true, worktree: { owned: true, branch: "agentdeck/fork" },
      },
    })));
    renderScoped("fork");
    expect(await screen.findByText("⑂ agentdeck/fork")).toBeInTheDocument();
  });

  it("opens the creation form from the header", async () => {
    server.use(
      http.get("/api/projects", () => HttpResponse.json({
        app: { title: "App", color: [100, 116, 139], cwd: "/tmp/app", add_dirs: [], context_prompt: "", archived: false, repo_backed: true },
      })),
      http.get("/api/projects/app/worktree", () => HttpResponse.json({
        owned: false, repo_backed: true, branch: "", base: "trunk", dirty: false, dirty_known: true,
      })),
    );
    renderScoped("app");
    fireEvent.click(await screen.findByRole("button", { name: "New worktree project" }));
    expect(await screen.findByLabelText("Title")).toHaveValue("App");
    await waitFor(() => expect(screen.getByLabelText("Base")).toHaveValue("trunk"));
  });
});
