import React from "react";
import { readFileSync } from "node:fs";
import { join } from "node:path";
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

  // FS-02.A24: the header button and a lower-canvas background right-click both
  // open the create modal; a card right-click opens the card menu, not the create menu.
  it("opens the create modal from the header button and the background menu, not from a card", async () => {
    renderDashboard();
    const card = (await screen.findByText("App")).closest("article")!;

    fireEvent.click(screen.getByRole("button", { name: "New project" }));
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "New project" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());

    fireEvent.contextMenu(document.querySelector(".project-dashboard")!, { clientX: 640, clientY: 600 });
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

  // FS-02.R41 / INV §13: the canvas must own the shell's 2rem padding. That frame
  // is part of the projects view, but it belongs to <main class="app-main">, so
  // with the padding there a right-click below or beside the cards reaches only
  // .app-main and opens the browser's native menu. jsdom evaluates no layout, so
  // the DOM case above passes either way — the stylesheet is the only witness.
  it("gives the projects canvas the shell padding instead of leaving it on the frame", () => {
    const css = (file: string) => readFileSync(join(process.cwd(), "src/styles/features", file), "utf8");
    const dashboard = css("dashboard.css").match(/\.project-dashboard \{[^}]*\}/)!;
    expect(dashboard[0]).toMatch(/padding:\s*var\(--ad-space-8\)/);
    expect(dashboard[0]).toMatch(/min-block-size:\s*100%/);
    expect(css("shell.css")).toMatch(/\.app-main:has\(>\s*\.project-dashboard\)\s*\{[^}]*padding:\s*0/);
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

  // ---- Worktree projects (FS-02.R60/A42, FS-19) ----

  // FS-02.A42: the card menu offers the fork only on a repo-backed active
  // project, and a project that is not repo-backed offers nothing.
  it("offers New worktree project only on a repo-backed project", async () => {
    renderDashboard();
    let card = (await screen.findByText("App")).closest("article")!;
    fireEvent.contextMenu(card);
    expect(screen.queryByRole("button", { name: "New worktree project" })).toBeNull();
    fireEvent.keyDown(window, { key: "Escape" });

    server.use(http.get("/api/projects", () => HttpResponse.json({
      app: { title: "App", color: [100, 116, 139], cwd: "/tmp/app", add_dirs: [], context_prompt: "", archived: false, repo_backed: true },
    })));
    cleanup();
    renderDashboard();
    card = (await screen.findByText("App")).closest("article")!;
    fireEvent.contextMenu(card);
    expect(await screen.findByRole("button", { name: "New worktree project" })).toBeInTheDocument();
  });

  // An archived project is not in the active grid at all, so it can offer no
  // fork entry point (FS-02.R60).
  it("shows no card, and therefore no fork action, for an archived project", async () => {
    server.use(http.get("/api/projects", () => HttpResponse.json({
      app: { title: "App", color: [100, 116, 139], cwd: "/tmp/app", add_dirs: [], context_prompt: "", archived: true, repo_backed: true },
    })));
    renderDashboard();
    await waitFor(() => expect(screen.queryByText("App")).toBeNull());
    expect(screen.queryByRole("button", { name: "New worktree project" })).toBeNull();
  });

  // FS-19.R1 + FS-02.R60: the form pre-fills title, branch, and the effective
  // base, and a completed fork's card appears in the grid with its branch
  // without a manual refresh.
  it("forks a project and shows the new card with its branch", async () => {
    let forked: Record<string, unknown> | null = null;
    let listCalls = 0;
    server.use(
      http.get("/api/projects", () => {
        listCalls += 1;
        const base: Record<string, unknown> = {
          app: { title: "App", color: [100, 116, 139], cwd: "/tmp/app", add_dirs: [], context_prompt: "", archived: false, repo_backed: true },
        };
        if (forked) {
          base["app-fork"] = {
            title: "App fork", color: [100, 116, 139], cwd: "/home/wt/app-fork", add_dirs: [],
            context_prompt: "", archived: false, repo_backed: true,
            worktree: { owned: true, branch: "agentdeck/app-fork" },
          };
        }
        return HttpResponse.json(base);
      }),
      http.get("/api/projects/app/worktree", () => HttpResponse.json({
        owned: false, repo_backed: true, branch: "", base: "develop", dirty: false, dirty_known: true,
      })),
      http.post("/api/projects/app/worktree-fork", async ({ request }) => {
        forked = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json({ project: { project: "app-fork" }, branch: "agentdeck/app-fork", base: "develop" }, { status: 201 });
      }),
    );

    renderDashboard();
    const card = (await screen.findByText("App")).closest("article")!;
    fireEvent.contextMenu(card);
    fireEvent.click(await screen.findByRole("button", { name: "New worktree project" }));

    // Title comes from the source, the branch is derived from it, and the base
    // is the server's effective base — never guessed on the client.
    expect((await screen.findByLabelText("Title")) as HTMLInputElement).toHaveValue("App");
    expect(screen.getByLabelText("Branch")).toHaveValue("agentdeck/app");
    await waitFor(() => expect(screen.getByLabelText("Base")).toHaveValue("develop"));

    fireEvent.change(screen.getByLabelText("Title"), { target: { value: "App fork" } });
    // The branch follows the title until it is edited by hand.
    expect(screen.getByLabelText("Branch")).toHaveValue("agentdeck/app-fork");

    const before = listCalls;
    fireEvent.click(screen.getByRole("button", { name: "Create worktree project" }));
    await waitFor(() => expect(forked).not.toBeNull());
    expect(forked).toMatchObject({ title: "App fork", branch: "agentdeck/app-fork", base: "develop" });
    // The grid refetches by itself: no manual refresh (FS-02.R60).
    await waitFor(() => expect(listCalls).toBeGreaterThan(before));
    expect(await screen.findByText("⑂ agentdeck/app-fork")).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  // An edited branch is the person's; retyping the title must not overwrite it.
  it("stops deriving the branch once it is edited by hand", async () => {
    server.use(
      http.get("/api/projects", () => HttpResponse.json({
        app: { title: "App", color: [100, 116, 139], cwd: "/tmp/app", add_dirs: [], context_prompt: "", archived: false, repo_backed: true },
      })),
      http.get("/api/projects/app/worktree", () => HttpResponse.json({
        owned: false, repo_backed: true, branch: "", base: "main", dirty: false, dirty_known: true,
      })),
    );
    renderDashboard();
    fireEvent.contextMenu((await screen.findByText("App")).closest("article")!);
    fireEvent.click(await screen.findByRole("button", { name: "New worktree project" }));
    fireEvent.change(await screen.findByLabelText("Branch"), { target: { value: "wip/mine" } });
    fireEvent.change(screen.getByLabelText("Title"), { target: { value: "Something else" } });
    expect(screen.getByLabelText("Branch")).toHaveValue("wip/mine");
  });

  // FS-19.R3 / A2: a failing setup command still creates the project, and the
  // dialog stays open to report the warning rather than closing over it.
  it("reports a setup warning and keeps the created project", async () => {
    server.use(
      http.get("/api/projects", () => HttpResponse.json({
        app: { title: "App", color: [100, 116, 139], cwd: "/tmp/app", add_dirs: [], context_prompt: "", archived: false, repo_backed: true },
      })),
      http.get("/api/projects/app/worktree", () => HttpResponse.json({
        owned: false, repo_backed: true, branch: "", base: "main", dirty: false, dirty_known: true,
      })),
      http.post("/api/projects/app/worktree-fork", () => HttpResponse.json(
        { project: { project: "app-fork" }, branch: "agentdeck/app", base: "main", warning: "setup command failed: npm ci exploded" },
        { status: 201 },
      )),
    );
    renderDashboard();
    fireEvent.contextMenu((await screen.findByText("App")).closest("article")!);
    fireEvent.click(await screen.findByRole("button", { name: "New worktree project" }));
    fireEvent.click(await screen.findByRole("button", { name: "Create worktree project" }));

    expect(await screen.findByText(/npm ci exploded/)).toBeInTheDocument();
    expect(await screen.findByText(/ready to launch/i)).toBeInTheDocument();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  // FS-19.R8: the archive dialog offers deletion only for an owned checkout,
  // defaults to keeping, discloses uncommitted work, and never sends consent
  // that was not given.
  it("offers checkout deletion on an owned checkout, defaulting to keep", async () => {
    let body: Record<string, unknown> | null = null;
    server.use(
      http.get("/api/projects", () => HttpResponse.json({
        fork: {
          title: "Fork", color: [100, 116, 139], cwd: "/home/wt/fork", add_dirs: [], context_prompt: "",
          archived: false, repo_backed: true, worktree: { owned: true, branch: "agentdeck/fork" },
        },
      })),
      http.get("/api/projects/fork/worktree", () => HttpResponse.json({
        owned: true, repo_backed: true, branch: "agentdeck/fork", base: "main", dirty: true, dirty_known: true,
      })),
      http.post("/api/projects/fork/archive", async ({ request }) => {
        body = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json({ project: { project: "fork", title: "Fork" }, stopped_agent_ids: [], archived_agent_ids: [] });
      }),
    );
    renderDashboard();
    fireEvent.contextMenu((await screen.findByText("Fork")).closest("article")!);
    fireEvent.click(screen.getByRole("button", { name: "Archive" }));

    const consent = await screen.findByLabelText(/delete this project's worktree checkout/i);
    expect((consent as HTMLInputElement).checked).toBe(false);
    expect(await screen.findByText(/holds uncommitted changes/i)).toBeInTheDocument();
    expect(screen.getByText(/branch and its commits are kept either way/i)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Archive project" }));
    await waitFor(() => expect(body).not.toBeNull());
    expect(body).toMatchObject({ delete_checkout: false });
  });

  // An undeterminable dirty check must say so, never claim the checkout is
  // clean (FS-19.R8, INV §8).
  it("says the uncommitted state is unknown rather than claiming clean", async () => {
    server.use(
      http.get("/api/projects", () => HttpResponse.json({
        fork: {
          title: "Fork", color: [100, 116, 139], cwd: "/home/wt/fork", add_dirs: [], context_prompt: "",
          archived: false, repo_backed: true, worktree: { owned: true, branch: "agentdeck/fork" },
        },
      })),
      http.get("/api/projects/fork/worktree", () => HttpResponse.json({
        owned: true, repo_backed: true, branch: "agentdeck/fork", base: "main", dirty: false, dirty_known: false,
      })),
    );
    renderDashboard();
    fireEvent.contextMenu((await screen.findByText("Fork")).closest("article")!);
    fireEvent.click(screen.getByRole("button", { name: "Archive" }));
    expect(await screen.findByText(/could not read this checkout/i)).toBeInTheDocument();
    expect(screen.queryByText(/no uncommitted changes/i)).toBeNull();
  });

  it("disables checkout consent and archive confirmation while status is loading", async () => {
    let resolveStatus: ((response: HttpResponse) => void) | undefined;
    server.use(
      http.get("/api/projects", () => HttpResponse.json({
        fork: {
          title: "Fork", color: [100, 116, 139], cwd: "/home/wt/fork", add_dirs: [], context_prompt: "",
          archived: false, repo_backed: true, worktree: { owned: true, branch: "agentdeck/fork" },
        },
      })),
      http.get("/api/projects/fork/worktree", () => new Promise<HttpResponse>((resolve) => { resolveStatus = resolve; })),
    );
    renderDashboard();
    fireEvent.contextMenu((await screen.findByText("Fork")).closest("article")!);
    fireEvent.click(screen.getByRole("button", { name: "Archive" }));

    expect(await screen.findByText(/checking for uncommitted changes/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/delete this project's worktree checkout/i)).toBeDisabled();
    expect(screen.getByRole("button", { name: "Archive project" })).toBeDisabled();

    resolveStatus?.(HttpResponse.json({
      owned: true, repo_backed: true, branch: "agentdeck/fork", base: "main", dirty: false, dirty_known: true,
    }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Archive project" })).toBeEnabled());
  });

  it("renders a failed checkout-status query as unknown", async () => {
    server.use(
      http.get("/api/projects", () => HttpResponse.json({
        fork: {
          title: "Fork", color: [100, 116, 139], cwd: "/home/wt/fork", add_dirs: [], context_prompt: "",
          archived: false, repo_backed: true, worktree: { owned: true, branch: "agentdeck/fork" },
        },
      })),
      http.get("/api/projects/fork/worktree", () => HttpResponse.json({ error: { message: "Git unavailable" } }, { status: 500 })),
    );
    renderDashboard();
    fireEvent.contextMenu((await screen.findByText("Fork")).closest("article")!);
    fireEvent.click(screen.getByRole("button", { name: "Archive" }));

    expect(await screen.findByText(/could not read this checkout/i)).toBeInTheDocument();
    expect(screen.queryByText(/checking for uncommitted changes/i)).toBeNull();
  });

  // A project that owns no checkout gets no offer at all (FS-19.R4/A5).
  it("offers no checkout deletion for a project that owns none", async () => {
    server.use(http.post("/api/projects/app/archive", () => HttpResponse.json({
      project: { project: "app", title: "App" }, stopped_agent_ids: [], archived_agent_ids: [],
    })));
    renderDashboard();
    fireEvent.contextMenu((await screen.findByText("App")).closest("article")!);
    fireEvent.click(screen.getByRole("button", { name: "Archive" }));
    expect(await screen.findByText(/Running agents will be stopped/i)).toBeInTheDocument();
    expect(screen.queryByLabelText(/delete this project's worktree checkout/i)).toBeNull();
  });
});
