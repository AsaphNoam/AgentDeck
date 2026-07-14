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
      reviewer: { title: "Reviewer", system_prompt: "", skip_permissions: null },
    }),
  ),
  http.get("/api/projects", () =>
    HttpResponse.json({
      "my-app": { title: "My App", color: [100, 180, 255], cwd: "/tmp/my-app", add_dirs: [], context_prompt: "" },
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
            sonnet: { name: "Sonnet 4.6", model: "claude-sonnet-4-6" },
            haiku: { name: "Haiku 4.5", model: "claude-haiku-4-5" },
          },
        },
        codex: {
          name: "Codex",
          type: "codex-acp",
          default: false,
          default_model: "gpt-4o",
          models: {
            "gpt-4o": { name: "GPT-4o", model: "gpt-4o" },
          },
        },
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

describe("NewAgentModal", () => {
  it("renders role and project selects from API", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    expect(await screen.findByText(/Implementer/)).toBeInTheDocument();
    expect(await screen.findByText(/My App/)).toBeInTheDocument();
  });

  it("shows project titles without internal project ids in the chooser", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByText(/My App/);

    const projectSelect = screen.getAllByRole("combobox")[1] as HTMLSelectElement;
    expect(projectSelect).toHaveTextContent("My App");
    expect(projectSelect).not.toHaveTextContent("My App (my-app)");
  });

  it("auto-suggests name from the role", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    // The suggested name is just the (capitalized) role once the role default loads.
    await screen.findByText(/Implementer/);
    const nameInput = screen.getByPlaceholderText("e.g. Atlas") as HTMLInputElement;
    await waitFor(() => expect(nameInput.value).toBe("Implementer"));
  });

  it("model select shows only models for the chosen backend", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByText(/Implementer/);

    // Default backend is "claude" with sonnet and haiku
    expect(await screen.findByText(/Sonnet 4.6/)).toBeInTheDocument();
    expect(screen.queryByText(/GPT-4o/)).toBeNull();
  });

  it("changing backend resets model to that backend's default", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByText(/Implementer/);
    await screen.findByText(/Sonnet 4.6/);

    // Change backend to codex
    const backendSelect = screen.getAllByRole("combobox")[2]; // backend is 3rd select
    fireEvent.change(backendSelect, { target: { value: "codex" } });

    expect(await screen.findByText(/GPT-4o/)).toBeInTheDocument();
    expect(screen.queryByText(/Sonnet 4.6/)).toBeNull();
  });

  it("terminal interface option is enabled when capabilities allow it", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByText(/Implementer/);

    const terminalRadio = screen.getByRole("radio", { name: /Terminal/i });
    await waitFor(() => expect(terminalRadio).toBeEnabled());
  });

  it("disables the Terminal option for a non-claude backend", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByText(/Implementer/);

    const terminalRadio = screen.getByRole("radio", { name: /Terminal/i }) as HTMLInputElement;
    // Default backend is claude-acp → terminal enabled.
    await waitFor(() => expect(terminalRadio).toBeEnabled());

    // Switch to codex (codex-acp, no verified terminal path) → terminal disabled.
    const backendSelect = screen.getAllByRole("combobox")[2];
    fireEvent.change(backendSelect, { target: { value: "codex" } });
    await waitFor(() => expect(terminalRadio).toBeDisabled());
  });

  it("resets a terminal selection to chat when switching to a non-claude backend", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByText(/Implementer/);

    const terminalRadio = screen.getByRole("radio", { name: /Terminal/i }) as HTMLInputElement;
    const chatRadio = screen.getByRole("radio", { name: /Chat/i }) as HTMLInputElement;
    await waitFor(() => expect(terminalRadio).toBeEnabled());
    fireEvent.click(terminalRadio);
    expect(terminalRadio.checked).toBe(true);

    const backendSelect = screen.getAllByRole("combobox")[2];
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
    await screen.findByText(/Implementer/);
    await screen.findByText(/My App/);

    const roleSelect = screen.getAllByRole("combobox")[0] as HTMLSelectElement;
    const projectSelect = screen.getAllByRole("combobox")[1] as HTMLSelectElement;
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
    await screen.findByText(/Implementer/);

    fireEvent.click(screen.getByText("Launch"));

    expect(await screen.findByText(/project cwd does not exist/)).toBeInTheDocument();
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
    await screen.findByText(/Implementer/);

    fireEvent.click(screen.getByText("Launch"));

    await waitFor(() => expect(capturedBody).toBeDefined());
    const body = capturedBody as Record<string, unknown>;
    expect(body.role).toBeTruthy();
    expect(body.project).toBeTruthy();
    expect(body.interface).toBe("chat");
    await waitFor(() => expect(onClose.called).toBe(true));
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
    await screen.findByText(/Implementer/);
    // The stale/invalid bound source for the default (claude) backend is flagged
    // BEFORE launch, rather than only surfacing as a late server error.
    expect(await screen.findByText(/linked configuration needs attention/)).toBeInTheDocument();
    expect(screen.getByText(/source_invalid/)).toBeInTheDocument();
  });
});
