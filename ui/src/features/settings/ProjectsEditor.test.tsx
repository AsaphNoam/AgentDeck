import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach, vi } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { ProjectsEditor } from "./ProjectsEditor";

// The server always sends the read-only resource_dir on every project payload
// (TS-03.R12); the mock mirrors that so tests exercise the real shape (INV §11).
const RESOURCE_DIR = "/home/u/.agentdeck/project-resources/my-app";

const server = setupServer(
  http.get("/api/projects", () =>
    HttpResponse.json({
      "my-app": {
        title: "My App",
        color: [100, 180, 255],
        cwd: "/tmp/my-app",
        add_dirs: [],
        context_prompt: "",
        resource_dir: RESOURCE_DIR,
      },
    }),
  ),
  http.post("/api/projects", async ({ request }) => {
    const body = (await request.json()) as Record<string, unknown>;
    return HttpResponse.json({ project: body.project, ...body }, { status: 201 });
  }),
  http.delete("/api/projects/:id", () => new HttpResponse(null, { status: 204 })),
);

beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterEach(() => {
  cleanup();
  server.resetHandlers();
});
afterAll(() => server.close());

function renderWithQuery(ui: React.ReactElement) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

describe("ProjectsEditor", () => {
  it("renders existing projects from GET /api/projects", async () => {
    renderWithQuery(<ProjectsEditor />);
    expect(await screen.findByText("My App")).toBeInTheDocument();
  });

  it("create project invalidates query so new project appears", async () => {
    let calls = 0;
    server.use(
      http.get("/api/projects", () => {
        calls++;
        const projects: Record<string, unknown> =
          calls === 1
            ? { "my-app": { title: "My App", color: [128, 128, 128], cwd: "/tmp", add_dirs: [], context_prompt: "" } }
            : {
                "my-app": { title: "My App", color: [128, 128, 128], cwd: "/tmp", add_dirs: [], context_prompt: "" },
                billing: { title: "Billing", color: [200, 100, 50], cwd: "/tmp/billing", add_dirs: [], context_prompt: "" },
              };
        return HttpResponse.json(projects);
      }),
    );

    renderWithQuery(<ProjectsEditor />);
    expect(await screen.findByText("My App")).toBeInTheDocument();

    fireEvent.click(screen.getByText("New project"));

    // Project id is server-derived (R31): the form no longer has a slug field.
    const titleInput = screen.getByPlaceholderText("e.g. My App");
    const cwdInput = screen.getByPlaceholderText("~/Projects/my-app");
    fireEvent.change(titleInput, { target: { value: "Billing" } });
    fireEvent.change(cwdInput, { target: { value: "/tmp/billing" } });

    fireEvent.click(screen.getByText("Create"));

    await waitFor(() => expect(calls).toBeGreaterThan(1));
    expect(await screen.findByText("Billing")).toBeInTheDocument();
  });

  it("shows the read-only shared-resources path when editing (FS-11.R4)", async () => {
    renderWithQuery(<ProjectsEditor />);
    await screen.findByText("My App");
    fireEvent.click(screen.getByText("Edit"));

    const field = (await screen.findByDisplayValue(RESOURCE_DIR)) as HTMLInputElement;
    expect(field.readOnly).toBe(true);
    expect(screen.getByText(/outside the repository/i)).toBeInTheDocument();
  });

  it("delete confirmation states the resources directory is retained (FS-11.R5)", async () => {
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(false);
    try {
      renderWithQuery(<ProjectsEditor />);
      await screen.findByText("My App");
      fireEvent.click(screen.getByText("Delete"));
      expect(confirmSpy).toHaveBeenCalledWith(expect.stringContaining(RESOURCE_DIR));
    } finally {
      confirmSpy.mockRestore();
    }
  });

  it("closes dialog on success even when cwd_not_found warnings are present", async () => {
    server.use(
      http.get("/api/projects", () =>
        HttpResponse.json({
          "warn-proj": {
            title: "Warn Project",
            color: [128, 128, 128],
            cwd: "/tmp/warn",
            add_dirs: [],
            context_prompt: "",
          },
        }),
      ),
      http.post("/api/projects", async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json(
          {
            project: body.project,
            ...body,
            warnings: [
              { field: "cwd", code: "cwd_not_found", message: "directory does not exist yet" },
            ],
          },
          { status: 201 },
        );
      }),
    );

    renderWithQuery(<ProjectsEditor />);
    await screen.findByText("Warn Project");

    fireEvent.click(screen.getByText("New project"));
    fireEvent.change(screen.getByPlaceholderText("e.g. My App"), { target: { value: "Phantom" } });
    fireEvent.change(screen.getByPlaceholderText("~/Projects/my-app"), {
      target: { value: "/no/such/dir" },
    });
    fireEvent.click(screen.getByText("Create"));

    // Dialog closes on success; warnings are non-blocking (A13).
    await waitFor(() =>
      expect(screen.queryByPlaceholderText("e.g. My App")).not.toBeInTheDocument()
    );
  });
});
