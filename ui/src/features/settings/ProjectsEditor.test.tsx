import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach } from "vitest";
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
    expect(screen.getByText("Active")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Archive" })).toBeInTheDocument();
  });

  it("shows archived state and restores the project from Settings", async () => {
    let restoreCalls = 0;
    server.use(
      http.get("/api/projects", () => HttpResponse.json({
        "my-app": {
          title: "My App", color: [100, 180, 255], cwd: "/tmp/my-app", add_dirs: [], context_prompt: "",
          archived: true,
        },
      })),
      http.post("/api/projects/my-app/restore", () => {
        restoreCalls += 1;
        return HttpResponse.json({
          project: "my-app", title: "My App", color: [100, 180, 255], cwd: "/tmp/my-app", add_dirs: [], context_prompt: "", archived: false,
        });
      }),
    );

    renderWithQuery(<ProjectsEditor />);
    expect(await screen.findByText("Archived")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Restore" }));
    await waitFor(() => expect(restoreCalls).toBe(1));
  });

  it("create project invalidates query so new project appears", async () => {
    let calls = 0;
    let created: Record<string, unknown> | null = null;
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
      http.post("/api/projects", async ({ request }) => {
        created = await request.json() as Record<string, unknown>;
        return HttpResponse.json({ project: "billing", ...created }, { status: 201 });
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

    expect(screen.getAllByRole("button", { name: /project color$/ })).toHaveLength(6);
    expect(screen.getByRole("button", { name: "Slate project color" })).toHaveAttribute("aria-pressed", "true");
    fireEvent.click(screen.getByRole("button", { name: "Violet project color" }));

    fireEvent.click(screen.getByText("Create"));

    await waitFor(() => expect(calls).toBeGreaterThan(1));
    expect(await screen.findByText("Billing")).toBeInTheDocument();
    expect(created).toMatchObject({ color: [139, 92, 246] });
  });

  it("shows the read-only shared-resources path when editing (FS-11.R4)", async () => {
    renderWithQuery(<ProjectsEditor />);
    await screen.findByText("My App");
    fireEvent.click(screen.getByText("Edit"));

    const field = (await screen.findByDisplayValue(RESOURCE_DIR)) as HTMLInputElement;
    expect(field.readOnly).toBe(true);
    expect(screen.getByText(/outside the repository/i)).toBeInTheDocument();
  });

  it("delete dialog states the resources directory is retained and cancels without deleting (FS-11.R5)", async () => {
    let deletes = 0;
    server.use(http.delete("/api/projects/:id", () => { deletes += 1; return new HttpResponse(null, { status: 204 }); }));
    renderWithQuery(<ProjectsEditor />);
    await screen.findByText("My App");
    fireEvent.click(screen.getByText("Delete"));
    expect(await screen.findByText(new RegExp(RESOURCE_DIR.replaceAll("/", "\\/")))).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(deletes).toBe(0);
  });

  // FS-04.A22: Browse fills cwd from the host folder panel, a dismissed panel
  // leaves the typed value alone, and neither one saves the project.
  it("browses for cwd, keeps the value on cancel, and never saves by itself", async () => {
    let posts = 0;
    let pickerCalls = 0;
    server.use(
      http.post("/api/projects", async ({ request }) => {
        posts += 1;
        const body = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json({ project: body.project, ...body }, { status: 201 });
      }),
      http.post("/api/directory-picker", () => {
        pickerCalls += 1;
        // First click selects; the second is a dismissed panel (204, no body).
        return pickerCalls === 1
          ? HttpResponse.json({ path: "/Users/me/picked" })
          : new HttpResponse(null, { status: 204 });
      }),
    );

    renderWithQuery(<ProjectsEditor />);
    await screen.findByText("My App");
    fireEvent.click(screen.getByText("New project"));

    const cwdInput = screen.getByPlaceholderText("~/Projects/my-app") as HTMLInputElement;
    fireEvent.click(screen.getByRole("button", { name: "Browse for working directory" }));
    await waitFor(() => expect(cwdInput.value).toBe("/Users/me/picked"));

    fireEvent.click(screen.getByRole("button", { name: "Browse for working directory" }));
    await waitFor(() => expect(pickerCalls).toBe(2));
    expect(cwdInput.value).toBe("/Users/me/picked");
    expect(posts).toBe(0);
  });

  // FS-04.A22: browsing an additional directory fills the pending entry only —
  // Add is still what puts it in the list.
  it("browses an additional directory into the pending entry until Add is clicked", async () => {
    server.use(
      http.post("/api/directory-picker", () => HttpResponse.json({ path: "/Users/me/extra" })),
    );

    renderWithQuery(<ProjectsEditor />);
    await screen.findByText("My App");
    fireEvent.click(screen.getByText("New project"));

    const pending = screen.getByPlaceholderText("~/extra-dir") as HTMLInputElement;
    fireEvent.click(screen.getByRole("button", { name: "Browse for an additional directory" }));
    await waitFor(() => expect(pending.value).toBe("/Users/me/extra"));
    expect(screen.queryByRole("button", { name: "Remove /Users/me/extra" })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Add" }));
    expect(await screen.findByRole("button", { name: "Remove /Users/me/extra" })).toBeInTheDocument();
    expect(pending.value).toBe("");
  });

  // INV §8: a failed browse is visible and still leaves the form usable.
  it("surfaces a browse failure without touching the field", async () => {
    server.use(
      http.post("/api/directory-picker", () =>
        HttpResponse.json(
          { error: { code: "directory_picker_failed", message: "could not complete the folder selection" } },
          { status: 500 },
        ),
      ),
    );

    renderWithQuery(<ProjectsEditor />);
    await screen.findByText("My App");
    fireEvent.click(screen.getByText("New project"));

    const cwdInput = screen.getByPlaceholderText("~/Projects/my-app") as HTMLInputElement;
    fireEvent.change(cwdInput, { target: { value: "/tmp/typed" } });
    fireEvent.click(screen.getByRole("button", { name: "Browse for working directory" }));

    expect(await screen.findByText(/could not complete the folder selection/i)).toBeInTheDocument();
    expect(cwdInput.value).toBe("/tmp/typed");
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
