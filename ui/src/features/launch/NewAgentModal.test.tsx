import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { NewAgentModal } from "./NewAgentModal";

const server = setupServer(
  http.get("/api/roles", () =>
    HttpResponse.json({
      implementer: { title: "Implementer", system_prompt: "", skip_permissions: null },
      "implementer-copy": { title: "Implementer", system_prompt: "", skip_permissions: null },
      reviewer: { title: "Reviewer", system_prompt: "", skip_permissions: null },
    }),
  ),
  http.get("/api/projects", () =>
    HttpResponse.json({
      "my-app": { title: "My App", color: [100, 180, 255], cwd: "/tmp/my-app", add_dirs: [], context_prompt: "" },
      "my-app-copy": { title: "My App", color: [100, 180, 255], cwd: "/tmp/my-app-copy", add_dirs: [], context_prompt: "" },
      billing: { title: "Billing", color: [200, 100, 50], cwd: "/tmp/billing", add_dirs: [], context_prompt: "" },
    }),
  ),
  http.get("/api/backends", () =>
    HttpResponse.json({
      version: 2,
      backends: {
        claude: {
          name: "Claude",
          type: "claude-acp",
          default: true,
          default_model: "sonnet",
          models: {
            // sonnet (the default) declares no efforts so the effort control is
            // hidden by default and the fixed combobox indices below stay stable.
            sonnet: { name: "Sonnet 4.6", model: "claude-sonnet-4-6", efforts: [] },
            haiku: { name: "Haiku 4.5", model: "claude-haiku-4-5", efforts: ["low", "medium", "high"], default_effort: "medium" },
          },
        },
        codex: {
          name: "Codex",
          type: "codex-acp",
          default: false,
          default_model: "gpt-4o",
          models: {
            "gpt-4o": { name: "GPT-4o", model: "gpt-4o", efforts: ["low", "high"], default_effort: "high", fast: true },
          },
        },
      },
      codex_runtime: {
        path: "/private/runtime/codex",
        version: "0.144.0",
        cache_version: "0.153.4",
        catalog_status: "mismatch",
      },
    }),
  ),
  http.post("/api/sessions", () =>
    HttpResponse.json(
      { agent: { agent_id: "a1", name: "Atlas", role: "implementer", project: "my-app" } },
      { status: 201 },
    ),
  ),
  http.get("/api/capabilities", () =>
    HttpResponse.json({
      terminal: { available: true, default_driver: "xterm", drivers: { xterm: true } },
    }),
  ),
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

function openOptions() {
  fireEvent.click(screen.getByText("Options"));
}

describe("NewAgentModal", () => {
  it("renders role and project selects from API", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    expect(await screen.findByRole("option", { name: "Implementer (implementer)" })).toBeInTheDocument();
    expect(await screen.findByRole("option", { name: "My App (my-app)" })).toBeInTheDocument();
  });

  it("warns before launch when the Codex cache is newer than the packaged runtime", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });
    openOptions();
    const backendSelect = screen.getByLabelText("Backend");
    fireEvent.change(backendSelect, { target: { value: "codex" } });
    expect(await screen.findByText(/Codex runtime 0\.144\.0/)).toHaveTextContent("Model auto-sync skipped cache from 0.153.4");
    expect(screen.getByText(/Codex runtime 0\.144\.0/)).toBeVisible();
  });

  it("shows project titles without internal project ids in the chooser", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByRole("option", { name: "My App (my-app)" });

    const projectSelect = screen.getByLabelText("Project") as HTMLSelectElement;
    expect(projectSelect).toHaveTextContent("My App (my-app)");
    expect(projectSelect).toHaveTextContent("My App (my-app-copy)");
    expect(projectSelect).toHaveTextContent("Billing");
    expect(projectSelect).not.toHaveTextContent("Billing (billing)");
    const roleSelect = screen.getByLabelText("Role");
    expect(roleSelect).toHaveTextContent("Implementer (implementer)");
    expect(roleSelect).toHaveTextContent("Implementer (implementer-copy)");
  });

  it("locks a scoped launch to its fixed project without rendering a picker", async () => {
    let capturedBody: unknown;
    server.use(
      http.post("/api/sessions", async ({ request }) => {
        capturedBody = await request.json();
        return HttpResponse.json({ agent: { agent_id: "a1", name: "Atlas" } }, { status: 201 });
      }),
    );

    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} fixedProject="billing" />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });

    expect(screen.queryByText("Project")).toBeNull();
    const launchButton = screen.getByRole("button", { name: "Launch" });
    await waitFor(() => expect(launchButton).toBeEnabled());
    fireEvent.click(launchButton);

    await waitFor(() => expect(capturedBody).toBeDefined());
    expect((capturedBody as Record<string, unknown>).project).toBe("billing");
  });

  it("updates the fixed project when a scoped route changes", async () => {
    let capturedBody: unknown;
    server.use(
      http.post("/api/sessions", async ({ request }) => {
        capturedBody = await request.json();
        return HttpResponse.json({ agent: { agent_id: "a1", name: "Atlas" } }, { status: 201 });
      }),
    );

    const qc = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    const onClose = () => {};
    const view = render(
      <QueryClientProvider client={qc}>
        <NewAgentModal open={true} onClose={onClose} fixedProject="my-app" />
      </QueryClientProvider>,
    );
    await screen.findByRole("option", { name: "Implementer (implementer)" });

    view.rerender(
      <QueryClientProvider client={qc}>
        <NewAgentModal open={true} onClose={onClose} fixedProject="billing" />
      </QueryClientProvider>,
    );
    const launchButton = screen.getByRole("button", { name: "Launch" });
    await waitFor(() => expect(launchButton).toBeEnabled());
    fireEvent.click(launchButton);

    await waitFor(() => expect(capturedBody).toBeDefined());
    expect((capturedBody as Record<string, unknown>).project).toBe("billing");
  });

  it("auto-suggests name from the role", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    // The suggested name is just the (capitalized) role once the role default loads.
    await screen.findByRole("option", { name: "Implementer (implementer)" });
    const nameInput = screen.getByPlaceholderText("e.g. Atlas") as HTMLInputElement;
    await waitFor(() => expect(nameInput.value).toBe("Implementer"));
  });

  it("model select shows only models for the chosen backend", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });
    openOptions();
    // Default backend is "claude" with sonnet and haiku.
    expect(screen.getByLabelText("Model")).toHaveTextContent("Sonnet 4.6");
    expect(screen.getByLabelText("Model")).not.toHaveTextContent("GPT-4o");
  });

  it("changing backend resets model to that backend's default", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });
    openOptions();

    // Change backend to codex
    const backendSelect = screen.getByLabelText("Backend");
    fireEvent.change(backendSelect, { target: { value: "codex" } });

    expect(screen.getByLabelText("Model")).toHaveTextContent("GPT-4o");
    expect(screen.getByLabelText("Model")).not.toHaveTextContent("Sonnet 4.6");
  });

  // FS-09.R37/A14 — the effort control is offered only for models that declare
  // efforts, preselects default_effort, and follows the selected model.
  it("shows an effort control preselecting the model's default_effort", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });
    openOptions();

    // Sonnet declares no efforts → no control until a model with efforts is picked.
    expect(screen.queryByText("Effort")).toBeNull();
    const modelSelect = screen.getByLabelText("Model");
    fireEvent.change(modelSelect, { target: { value: "haiku" } });

    const effortSelect = (await screen.findByText("Effort")).parentElement!.querySelector("select") as HTMLSelectElement;
    await waitFor(() => expect(effortSelect.value).toBe("medium"));
    expect(effortSelect).toHaveTextContent("low");
    expect(effortSelect).toHaveTextContent("high");
  });

  it("hides the effort control for a model that declares no efforts", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });
    openOptions();

    // Reveal it on haiku, then switch back to effort-less sonnet: it disappears.
    const modelSelect = screen.getByLabelText("Model");
    fireEvent.change(modelSelect, { target: { value: "haiku" } });
    await screen.findByText("Effort");
    fireEvent.change(modelSelect, { target: { value: "sonnet" } });
    await waitFor(() => expect(screen.queryByText("Effort")).toBeNull());
  });

  it("resets effort to the new model's default when the model changes", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });
    openOptions();

    // Switch backend to codex → gpt-4o defaults its effort to "high".
    const backendSelect = screen.getByLabelText("Backend");
    fireEvent.change(backendSelect, { target: { value: "codex" } });
    expect(screen.getByLabelText("Model")).toHaveTextContent("GPT-4o");
    const effortSelect = (await screen.findByText("Effort")).parentElement!.querySelector("select") as HTMLSelectElement;
    await waitFor(() => expect(effortSelect.value).toBe("high"));
  });

  it("terminal interface option is enabled when capabilities allow it", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });
    openOptions();

    const terminalRadio = screen.getByRole("radio", { name: /Terminal/i });
    await waitFor(() => expect(terminalRadio).toBeEnabled());
  });

  it("disables the Terminal option for a non-claude backend", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });
    openOptions();

    const terminalRadio = screen.getByRole("radio", { name: /Terminal/i }) as HTMLInputElement;
    // Default backend is claude-acp → terminal enabled.
    await waitFor(() => expect(terminalRadio).toBeEnabled());

    // Switch to codex (codex-acp, no verified terminal path) → terminal disabled.
    const backendSelect = screen.getByLabelText("Backend");
    fireEvent.change(backendSelect, { target: { value: "codex" } });
    await waitFor(() => expect(terminalRadio).toBeDisabled());
  });

  it("resets a terminal selection to chat when switching to a non-claude backend", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });
    openOptions();

    const terminalRadio = screen.getByRole("radio", { name: /Terminal/i }) as HTMLInputElement;
    const chatRadio = screen.getByRole("radio", { name: /Chat/i }) as HTMLInputElement;
    await waitFor(() => expect(terminalRadio).toBeEnabled());
    fireEvent.click(terminalRadio);
    expect(terminalRadio.checked).toBe(true);

    const backendSelect = screen.getByLabelText("Backend");
    fireEvent.change(backendSelect, { target: { value: "codex" } });
    await waitFor(() => expect(chatRadio.checked).toBe(true));
    expect(terminalRadio.checked).toBe(false);
  });

  it("preselects configured default_role / default_project over the first entry", async () => {
    server.use(
      http.get("/api/config", () =>
        HttpResponse.json({
          onboarding_complete: true,
          default_role: "reviewer",
          default_project: "billing",
        }),
      ),
    );

    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });
    await screen.findByRole("option", { name: "My App (my-app)" });

    const roleSelect = screen.getByLabelText("Role") as HTMLSelectElement;
    const projectSelect = screen.getByLabelText("Project") as HTMLSelectElement;
    // Configured defaults (the second entries) must win over the first entry.
    await waitFor(() => expect(roleSelect.value).toBe("reviewer"));
    await waitFor(() => expect(projectSelect.value).toBe("billing"));
  });

  it("surfaces the server error message when launch fails", async () => {
    server.use(
      http.post("/api/sessions", () =>
        HttpResponse.json(
          { error: { code: "runtime_start_failed", message: "project cwd does not exist: ~/Projects/my-app" } },
          { status: 502 },
        ),
      ),
    );

    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });

    const launchButton = screen.getByRole("button", { name: "Launch" });
    await waitFor(() => expect(launchButton).toBeEnabled());
    fireEvent.click(launchButton);

    expect(await screen.findByText(/project cwd does not exist/)).toBeInTheDocument();
    expect(screen.getByLabelText("Backend")).toBeVisible();
  });

  it("submits correct payload on Launch", async () => {
    let capturedBody: unknown;
    server.use(
      http.post("/api/sessions", async ({ request }) => {
        capturedBody = await request.json();
        return HttpResponse.json(
          { agent: { agent_id: "a1", name: "Atlas" } },
          { status: 201 },
        );
      }),
    );

    const onClose = { called: false };
    renderWithQuery(<NewAgentModal open={true} onClose={() => { onClose.called = true; }} />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });

    const launchButton = screen.getByRole("button", { name: "Launch" });
    await waitFor(() => expect(launchButton).toBeEnabled());
    fireEvent.click(launchButton);

    await waitFor(() => expect(capturedBody).toBeDefined());
    const body = capturedBody as Record<string, unknown>;
    expect(body.role).toBeTruthy();
    expect(body.project).toBeTruthy();
    expect(body.backend).toBe("claude");
    expect(body.model).toBe("sonnet");
    expect(body.effort).toBeUndefined();
    expect(body.fast).toBe(false);
    expect(body.interface).toBe("chat");
    expect(screen.getByText("Options").closest("details")).not.toHaveAttribute("open");
    await waitFor(() => expect(onClose.called).toBe(true));
  });

  it("keeps customized launch values through Options toggles and submits the summarized runtime", async () => {
    let capturedBody: Record<string, unknown> | undefined;
    server.use(http.post("/api/sessions", async ({ request }) => {
      capturedBody = await request.json() as Record<string, unknown>;
      return HttpResponse.json({ agent: { agent_id: "a2", name: "Custom Worker" } }, { status: 201 });
    }));

    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });
    openOptions();
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Custom Worker" } });
    fireEvent.change(screen.getByLabelText("Backend"), { target: { value: "codex" } });
    fireEvent.change(screen.getByLabelText("Effort"), { target: { value: "low" } });
    fireEvent.click(screen.getByLabelText(/Fast mode/));

    fireEvent.click(screen.getByText("Options"));
    expect(screen.getByLabelText("Backend")).not.toBeVisible();
    expect(screen.getByText("GPT-4o", { selector: ".new-agent-runtime span" })).toBeInTheDocument();
    expect(screen.getByText("low effort")).toBeInTheDocument();
    expect(screen.getByText("Fast mode")).toBeInTheDocument();
    fireEvent.click(screen.getByText("Options"));
    expect((screen.getByLabelText("Name") as HTMLInputElement).value).toBe("Custom Worker");
    expect((screen.getByLabelText("Backend") as HTMLSelectElement).value).toBe("codex");
    expect((screen.getByLabelText("Effort") as HTMLSelectElement).value).toBe("low");
    expect((screen.getByLabelText(/Fast mode/) as HTMLInputElement).checked).toBe(true);

    fireEvent.click(screen.getByRole("button", { name: "Launch" }));
    await waitFor(() => expect(capturedBody).toBeDefined());
    expect(capturedBody).toMatchObject({
      name: "Custom Worker",
      backend: "codex",
      model: "gpt-4o",
      effort: "low",
      fast: true,
      interface: "chat",
    });
  });

  it("warns before launch when the chosen backend's linked source needs attention", async () => {
    server.use(
      http.get("/api/config-sources", () =>
        HttpResponse.json({
          bindings: [
            { backend_id: "claude", provider: "claude-code", mode: "linked", root: "/h/.claude", claims: [], approved_roots: [], health: "source_invalid", stale: true },
          ],
          candidates: [],
        }),
      ),
    );

    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });
    // The stale/invalid bound source for the default (claude) backend is flagged
    // BEFORE launch, rather than only surfacing as a late server error.
    expect(await screen.findByText(/linked configuration needs attention/)).toBeInTheDocument();
    expect(screen.getByText(/source_invalid/)).toBeInTheDocument();
  });
});
