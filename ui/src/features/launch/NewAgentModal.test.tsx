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

  it("auto-suggests name from role and project", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    // Wait for both role and project data to load — the suggested name is empty
    // until both defaults are applied (suggest() returns "" if either is unset),
    // so awaiting only the role text would race the project default.
    await screen.findByText(/Implementer/);
    await screen.findByText(/My App/);
    const nameInput = screen.getByPlaceholderText("e.g. Atlas") as HTMLInputElement;
    // Name is suggested as "Implementer-my-app" or similar based on first loaded role/project.
    await waitFor(() => expect(nameInput.value).toBeTruthy());
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

  it("terminal interface option is disabled", async () => {
    renderWithQuery(<NewAgentModal open={true} onClose={() => {}} />);
    await screen.findByText(/Implementer/);

    const terminalRadio = screen.getByRole("radio", { name: /Terminal/i });
    expect(terminalRadio).toBeDisabled();
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
});
