import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach } from "vitest";
import { act, render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { NewAgentModal } from "./NewAgentModal";
import { BACKEND_SUPPORT_WIRE } from "../../test/backendSupport";
import { useAgentStore } from "../../store/agentStore";

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
  http.get("/api/backends", () => HttpResponse.json(backendsFixture())),
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

function backendsFixture() {
  return {
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
      backend_support: BACKEND_SUPPORT_WIRE as unknown,
      provider_runtimes: {
        claude: {
          sonnet: { source: "detected", state: "available", path: "/Users/me/.local/bin/claude", version: "2.1.300", checked_at: "2026-10-04T10:00:00Z" },
          haiku: { source: "model", state: "missing", path: "/old/claude" },
        },
        codex: { "gpt-4o": { source: "bundled", state: "available", path: "/app/runtime/codex" } },
      } as unknown,
  };
}

beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterEach(() => {
  cleanup();
  useAgentStore.setState({ agents: {}, order: [] });
  server.resetHandlers();
});
afterAll(() => server.close());

function renderWithQuery(ui: React.ReactElement) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

async function openOptions() {
  // Roles, projects, and backends load independently. Wait until the selected
  // backend's default model has reached component state before changing runtime
  // controls; the model select can display its first option while modelId is
  // still empty, and the pending defaulting effect would then overwrite a test
  // change made in that window.
  await screen.findByText("Sonnet 4.6", { selector: ".new-agent-runtime span" });
  fireEvent.click(screen.getByText("Options"));
}

describe("NewAgentModal", () => {
  it("renders role and project selects from API", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    expect(await screen.findByRole("option", { name: "Implementer (implementer)" })).toBeInTheDocument();
    expect(await screen.findByRole("option", { name: "My App (my-app)" })).toBeInTheDocument();
  });

  // FS-09.R72: New Agent reports the saved next-start provider for the chosen
  // backend/model and points a broken one at Settings, without a selector.
  it("reports the selected backend/model provider and its repair", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    expect(await screen.findByText(/Installed Claude Code · \/Users\/me\/\.local\/bin\/claude · version 2\.1\.300/)).toBeInTheDocument();
    await openOptions();
    fireEvent.change(screen.getByLabelText("Model"), { target: { value: "haiku" } });
    expect(await screen.findByText(/The Claude Code executable path was not found.*Settings → Backends/)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Backend"), { target: { value: "codex" } });
    expect(await screen.findByText(/Chuck bundle Codex · \/app\/runtime\/codex · version not checked/)).toBeInTheDocument();
    expect(screen.queryByRole("radio", { name: "Chuck bundle" })).toBeNull();
  });

  // FS-09.A40: a cache version difference is not an incompatibility warning.
  it("shows no Codex cache-version warning, even from a legacy runtime field", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });
    await openOptions();
    const backendSelect = screen.getByLabelText("Backend");
    fireEvent.change(backendSelect, { target: { value: "codex" } });
    expect(await screen.findByText("GPT-4o", { selector: ".new-agent-runtime span" })).toBeInTheDocument();
    expect(screen.queryByText(/Model auto-sync skipped cache/)).toBeNull();
    expect(screen.queryByText(/Codex runtime 0\.144\.0/)).toBeNull();
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

  it("shows a scoped launch's fixed project without rendering a picker", async () => {
    let capturedBody: unknown;
    server.use(
      http.post("/api/sessions", async ({ request }) => {
        capturedBody = await request.json();
        return HttpResponse.json({ agent: { agent_id: "a1", name: "Atlas" } }, { status: 201 });
      }),
    );

    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} fixedProject="billing" />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });

    expect(await screen.findByLabelText("Project")).toHaveTextContent("Billing");
    expect(screen.queryByRole("combobox", { name: "Project" })).toBeNull();
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
    await openOptions();
    // Default backend is "claude" with sonnet and haiku.
    expect(screen.getByLabelText("Model")).toHaveTextContent("Sonnet 4.6");
    expect(screen.getByLabelText("Model")).not.toHaveTextContent("GPT-4o");
  });

  it("changing backend resets model to that backend's default", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });
    await openOptions();

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
    await openOptions();

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
    await openOptions();

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
    await openOptions();

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
    await openOptions();

    const terminalRadio = screen.getByRole("radio", { name: /Terminal/i });
    await waitFor(() => expect(terminalRadio).toBeEnabled());
  });

  it("disables the Terminal option for a non-claude backend", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });
    await openOptions();

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
    await openOptions();

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

  // FS-09.A31: choices come from adapter launch support, not the backend type.
  it("offers only the options the selected backend/interface supports", async () => {
    server.use(http.get("/api/backends", () => {
      const doc = backendsFixture();
      // An adapter reporting no chat effort/fast hides them even for a model
      // that declares both, and a reported Terminal is offered for Codex.
      doc.backend_support = {
        ...BACKEND_SUPPORT_WIRE,
        "codex-acp": {
          chat: { available: true, effort: false, fast: false },
          terminal: { available: true, effort: true, fast: false },
        },
      };
      return HttpResponse.json(doc);
    }));
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });
    await openOptions();
    fireEvent.change(screen.getByLabelText("Backend"), { target: { value: "codex" } });
    const terminalRadio = screen.getByRole("radio", { name: /Terminal/i });
    await waitFor(() => expect(terminalRadio).toBeEnabled());
    expect(screen.queryByLabelText("Effort")).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/Fast mode/)).not.toBeInTheDocument();
    fireEvent.click(terminalRadio);
    expect(await screen.findByLabelText("Effort")).toBeInTheDocument();
    expect(screen.queryByLabelText(/Fast mode/)).not.toBeInTheDocument();
  });

  it("keeps verified Chat choices usable when Terminal support is malformed", async () => {
    let posted: Record<string, unknown> | undefined;
    server.use(
      http.get("/api/backends", () => {
        const doc = backendsFixture();
        doc.backend_support = {
          ...BACKEND_SUPPORT_WIRE,
          "claude-acp": {
            chat: { available: true, effort: true, fast: true },
            terminal: { available: "bad", effort: true, fast: false },
          },
        };
        return HttpResponse.json(doc);
      }),
      http.post("/api/sessions", async ({ request }) => {
        posted = await request.json() as Record<string, unknown>;
        return HttpResponse.json({ agent: { agent_id: "a4", name: "Atlas" } }, { status: 201 });
      }),
    );
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });
    await openOptions();
    fireEvent.change(screen.getByLabelText("Model"), { target: { value: "haiku" } });

    expect(await screen.findByLabelText("Effort")).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Terminal/i })).toBeDisabled();
    expect(screen.getByText(/Launch options could not be loaded for Terminal/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Launch" })).toBeEnabled();

    fireEvent.click(screen.getByRole("button", { name: "Launch" }));
    await waitFor(() => expect(posted).toBeDefined());
    expect(posted).toMatchObject({ interface: "chat", effort: "medium", fast: false });
  });

  it("keeps verified Terminal choices usable when Chat support is malformed", async () => {
    let posted: Record<string, unknown> | undefined;
    server.use(
      http.get("/api/backends", () => {
        const doc = backendsFixture();
        doc.backend_support = {
          ...BACKEND_SUPPORT_WIRE,
          "claude-acp": {
            chat: { available: true, effort: "bad", fast: true },
            terminal: { available: true, effort: true, fast: false },
          },
        };
        return HttpResponse.json(doc);
      }),
      http.post("/api/sessions", async ({ request }) => {
        posted = await request.json() as Record<string, unknown>;
        return HttpResponse.json({ agent: { agent_id: "a5", name: "Atlas" } }, { status: 201 });
      }),
    );
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });
    await openOptions();
    fireEvent.change(screen.getByLabelText("Model"), { target: { value: "haiku" } });
    const terminalRadio = screen.getByRole("radio", { name: /Terminal/i });
    await waitFor(() => expect(terminalRadio).toBeEnabled());
    fireEvent.click(terminalRadio);

    expect(await screen.findByLabelText("Effort")).toBeInTheDocument();
    expect(screen.getByText(/Launch options could not be loaded for Chat/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Launch" })).toBeEnabled();

    fireEvent.click(screen.getByRole("button", { name: "Launch" }));
    await waitFor(() => expect(posted).toBeDefined());
    expect(posted).toMatchObject({ interface: "terminal", effort: "medium", fast: false });
  });

  it("withholds Terminal when the host lacks it even if the backend supports it", async () => {
    server.use(http.get("/api/capabilities", () => HttpResponse.json({ terminal: { available: false } })));
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });
    await openOptions();
    await waitFor(() => expect(screen.getByRole("radio", { name: /Terminal/i })).toBeDisabled());
  });

  it("offers no optional choices and a retry when support metadata is missing, keeping ordinary chat launchable", async () => {
    let posted: Record<string, unknown> | undefined;
    server.use(
      http.get("/api/backends", () => {
        const { backend_support: _omit, ...doc } = backendsFixture();
        return HttpResponse.json(doc);
      }),
      http.post("/api/sessions", async ({ request }) => {
        posted = await request.json() as Record<string, unknown>;
        return HttpResponse.json({ agent: { agent_id: "a3", name: "Atlas" } }, { status: 201 });
      }),
    );
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });
    await openOptions();
    fireEvent.change(screen.getByLabelText("Backend"), { target: { value: "codex" } });
    expect(await screen.findByText(/Launch options could not be loaded/)).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Terminal/i })).toBeDisabled();
    expect(screen.queryByLabelText("Effort")).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/Fast mode/)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Launch" }));
    await waitFor(() => expect(posted).toBeDefined());
    expect(posted).toMatchObject({ backend: "codex", model: "gpt-4o", fast: false, interface: "chat" });
    expect(posted!.effort).toBeUndefined();
  });

  it("blocks a selection made before support went missing until retry or clear, preserving other input", async () => {
    let withSupport = true;
    server.use(http.get("/api/backends", () => {
      const doc = backendsFixture();
      if (!withSupport) delete (doc as { backend_support?: unknown }).backend_support;
      return HttpResponse.json(doc);
    }));
    const qc = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    render(<QueryClientProvider client={qc}><NewAgentModal open={true} onClose={() => {}} /></QueryClientProvider>);
    await screen.findByRole("option", { name: "Implementer (implementer)" });
    await openOptions();
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Kept" } });
    fireEvent.change(screen.getByLabelText("Model"), { target: { value: "haiku" } });
    const terminalRadio = screen.getByRole("radio", { name: /Terminal/i }) as HTMLInputElement;
    await waitFor(() => expect(terminalRadio).toBeEnabled());
    fireEvent.click(terminalRadio);

    withSupport = false;
    await act(() => qc.refetchQueries({ queryKey: ["backends"] }));
    expect(await screen.findByText(/Retry to verify your selected options/)).toBeInTheDocument();
    expect(terminalRadio.checked).toBe(true);
    expect(screen.getByRole("button", { name: "Launch" })).toBeDisabled();

    withSupport = true;
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(screen.queryByText(/Launch options could not be loaded/)).not.toBeInTheDocument());
    expect(terminalRadio.checked).toBe(true);
    expect((screen.getByLabelText("Name") as HTMLInputElement).value).toBe("Kept");
    expect((screen.getByLabelText("Model") as HTMLSelectElement).value).toBe("haiku");
    expect(screen.getByRole("button", { name: "Launch" })).toBeEnabled();

    withSupport = false;
    await act(() => qc.refetchQueries({ queryKey: ["backends"] }));
    fireEvent.click(await screen.findByRole("button", { name: "Clear options" }));
    expect((screen.getByRole("radio", { name: /Chat/i }) as HTMLInputElement).checked).toBe(true);
    expect(screen.getByRole("button", { name: "Launch" })).toBeEnabled();
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
    await openOptions();
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

  it.each([
    ["existing", "Core", "Core"],
    ["new", "New team", "New team"],
  ])("launches with an %s selected group", async (_kind, typed, expected) => {
    let capturedBody: Record<string, unknown> | undefined;
    useAgentStore.setState({ agents: { existing: { group: "Core", archived: false } as never }, order: ["existing"] });
    server.use(http.post("/api/sessions", async ({ request }) => {
      capturedBody = await request.json() as Record<string, unknown>;
      return HttpResponse.json({ agent: { agent_id: "a1", name: "Atlas" } }, { status: 201 });
    }));
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });
    await openOptions();
    const picker = screen.getByRole("combobox", { name: "Group" });
    fireEvent.focus(picker);
    fireEvent.change(picker, { target: { value: typed } });
    fireEvent.click(screen.getByRole("button", { name: expected === "Core" ? "Core" : /Create group/ }));
    fireEvent.click(screen.getByRole("button", { name: "Launch" }));
    await waitFor(() => expect(capturedBody).toBeDefined());
    expect(capturedBody?.group).toBe(expected);
  });

  it("launches blank as ungrouped and retains a selected group after refusal", async () => {
    let calls = 0;
    const bodies: Record<string, unknown>[] = [];
    server.use(http.post("/api/sessions", async ({ request }) => {
      calls += 1;
      bodies.push(await request.json() as Record<string, unknown>);
      return calls === 1
        ? HttpResponse.json({ agent: { agent_id: "a1", name: "Atlas" } }, { status: 201 })
        : calls === 2
          ? HttpResponse.json({ error: { message: "runtime unavailable" } }, { status: 422 })
          : HttpResponse.json({ agent: { agent_id: "a2", name: "Atlas 2" } }, { status: 201 });
    }));
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByRole("option", { name: "Implementer (implementer)" });
    await openOptions();
    fireEvent.click(screen.getByRole("button", { name: "Launch" }));
    await waitFor(() => expect(bodies[0]).toBeDefined());
    expect(bodies[0].group).toBeUndefined();
    const picker = screen.getByRole("combobox", { name: "Group" });
    fireEvent.focus(picker);
    fireEvent.change(picker, { target: { value: "Retry group" } });
    fireEvent.click(screen.getByRole("button", { name: /Create group/ }));
    fireEvent.click(screen.getByRole("button", { name: "Launch" }));
    await waitFor(() => expect(screen.getByRole("combobox", { name: "Group" })).toHaveValue("Retry group"));
    expect(bodies[1]?.group).toBe("Retry group");
    fireEvent.click(screen.getByRole("button", { name: "Launch" }));
    await waitFor(() => expect(calls).toBe(3));
  });
});
