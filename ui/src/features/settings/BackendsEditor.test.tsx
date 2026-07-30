import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { BackendsEditor } from "./BackendsEditor";

const defaultBackendsDoc = {
  version: 2,
  backends: {
    claude: {
      name: "Claude",
      type: "claude-acp",
      default: true,
      default_model: "sonnet",
      models: {
        sonnet: { name: "Sonnet 4.6", model: "claude-sonnet-4-6" },
      },
    },
  },
};

const server = setupServer(
  http.get("/api/backends", () => HttpResponse.json(defaultBackendsDoc)),
  http.put("/api/backends", () =>
    HttpResponse.json({
      ...defaultBackendsDoc,
      credentials: { claude: { status: "ok", detail: "" } },
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

describe("BackendsEditor", () => {
  it("renders backend name from GET /api/backends", async () => {
    renderWithQuery(<BackendsEditor />);
    expect(await screen.findByDisplayValue("Claude")).toBeInTheDocument();
  });

  it("does not crash when a malformed response contains null collections", async () => {
    server.use(
      http.get("/api/backends", () => HttpResponse.json({ version: 2, backends: { claude: { ...defaultBackendsDoc.backends.claude, models: null } } })),
    );
    renderWithQuery(<BackendsEditor />);
    expect(await screen.findByDisplayValue("Claude")).toBeInTheDocument();
    expect(screen.getByText("+ Add model")).toBeInTheDocument();
  });

  it("shows ok cred chip after Save", async () => {
    renderWithQuery(<BackendsEditor />);
    await screen.findByDisplayValue("Claude");

    fireEvent.click(screen.getByText("Save"));

    expect(await screen.findByText("ok")).toBeInTheDocument();
  });

  it("shows failed cred chip when credentials fail", async () => {
    server.use(
      http.put("/api/backends", () =>
        HttpResponse.json({
          ...defaultBackendsDoc,
          credentials: { claude: { status: "failed", detail: "invalid_api_key" } },
        }),
      ),
    );
    renderWithQuery(<BackendsEditor />);
    await screen.findByDisplayValue("Claude");

    fireEvent.click(screen.getByText("Save"));

    expect(await screen.findByText("failed")).toBeInTheDocument();
  });

  // FS-09.R37/A14 — the model editor lets a person declare/reorder effort levels
  // and choose the default from among them.
  it("edits a model's effort levels and default in the expanded editor", async () => {
    renderWithQuery(<BackendsEditor />);
    await screen.findByDisplayValue("Claude");

    // Expand the sonnet model row's env/effort editor (the ▾ toggle button).
    fireEvent.click(screen.getByRole("button", { name: /▾ env/ }));

    const levels = screen.getByPlaceholderText("low, medium, high") as HTMLInputElement;
    fireEvent.change(levels, { target: { value: "low, medium, high" } });
    await waitFor(() => expect(levels.value).toBe("low, medium, high"));

    // The default-effort select now appears and seeds to the first level.
    const defaultSelect = screen.getByText("Default effort").parentElement!.querySelector("select") as HTMLSelectElement;
    expect(Array.from(defaultSelect.options).map((o) => o.value)).toEqual(["low", "medium", "high"]);
    await waitFor(() => expect(defaultSelect.value).toBe("low"));

    fireEvent.change(defaultSelect, { target: { value: "medium" } });
    await waitFor(() => expect(defaultSelect.value).toBe("medium"));
  });

  it("offers all four backend types in the type dropdown", async () => {
    renderWithQuery(<BackendsEditor />);
    await screen.findByDisplayValue("Claude");

    const typeSelect = screen.getByDisplayValue(/Claude \(claude-acp\)/) as HTMLSelectElement;
    const values = Array.from(typeSelect.options).map((o) => o.value);
    expect(values).toEqual(["claude-acp", "codex-acp", "opencode-acp", "openhands-acp"]);
  });
});
