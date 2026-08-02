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

  // FS-09.R45/A18 — a claude-acp backend offers the configured-model import
  // opt-in with Claude-specific copy, and the toggle updates its state.
  it("offers the Claude configured-model import toggle on a claude-acp backend", async () => {
    renderWithQuery(<BackendsEditor />);
    await screen.findByDisplayValue("Claude");

    const toggle = screen.getByLabelText(/Import configured models from Claude on startup/) as HTMLInputElement;
    expect(toggle.checked).toBe(false);
    fireEvent.click(toggle);
    await waitFor(() => expect(toggle.checked).toBe(true));
  });

  it("offers all four backend types in the type dropdown", async () => {
    renderWithQuery(<BackendsEditor />);
    await screen.findByDisplayValue("Claude");

    const typeSelect = screen.getByDisplayValue(/Claude \(claude-acp\)/) as HTMLSelectElement;
    const values = Array.from(typeSelect.options).map((o) => o.value);
    expect(values).toEqual(["claude-acp", "codex-acp", "opencode-acp", "openhands-acp"]);
  });

  // ---- Add backend dialog (FS-04.A20 / R40) ----

  const codexStarter = {
    name: "Codex / OpenAI",
    type: "codex-acp",
    default: false,
    default_model: "gpt-5.6-sol",
    models: { "gpt-5.6-sol": { name: "GPT-5.6-Sol", model: "gpt-5.6-sol" } },
  };

  async function openAddDialog() {
    renderWithQuery(<BackendsEditor />);
    await screen.findByDisplayValue("Claude");
    fireEvent.click(screen.getByText("Add backend"));
    return screen.findByLabelText("Provider");
  }

  it("suggests the chosen provider's name and creates only that backend", async () => {
    let created: { backend_id: string; name: string; type: string; connect_native_configuration?: boolean } | null = null;
    server.use(
      http.post("/api/backends", async ({ request }) => {
        created = (await request.json()) as typeof created;
        return HttpResponse.json({ backend_id: "codex-openai", backend: codexStarter }, { status: 201 });
      }),
    );

    const providerSelect = (await openAddDialog()) as HTMLSelectElement;
    // A Claude default gives a Claude name; choosing Codex re-suggests Codex.
    expect((screen.getByLabelText("Name") as HTMLInputElement).value).toBe("Claude");
    fireEvent.change(providerSelect, { target: { value: "codex-acp" } });
    await waitFor(() => expect((screen.getByLabelText("Name") as HTMLInputElement).value).toBe("Codex / OpenAI"));

    fireEvent.click(screen.getByText("Create backend"));
    await waitFor(() => expect(created).not.toBeNull());
    expect(created!).toMatchObject({ name: "Codex / OpenAI", type: "codex-acp", connect_native_configuration: false });

    // The created card is merged in with its usable starter model, and the
    // existing backend is still there.
    expect(await screen.findByDisplayValue("Codex / OpenAI")).toBeInTheDocument();
    expect(screen.getByDisplayValue("gpt-5.6-sol")).toBeInTheDocument();
    expect(screen.getByDisplayValue("Claude")).toBeInTheDocument();
  });

  // The whole-catalog draft is browser-local: creating one backend must not
  // submit, save, or discard an unrelated unsaved edit.
  it("preserves an unrelated dirty draft across a create", async () => {
    server.use(
      http.post("/api/backends", () =>
        HttpResponse.json({ backend_id: "codex-openai", backend: codexStarter }, { status: 201 }),
      ),
    );
    renderWithQuery(<BackendsEditor />);
    const nameInput = (await screen.findByDisplayValue("Claude")) as HTMLInputElement;
    fireEvent.change(nameInput, { target: { value: "Renamed but unsaved" } });

    fireEvent.click(screen.getByText("Add backend"));
    await screen.findByLabelText("Provider");
    fireEvent.click(screen.getByText("Create backend"));

    expect(await screen.findByDisplayValue("Codex / OpenAI")).toBeInTheDocument();
    expect(screen.getByDisplayValue("Renamed but unsaved")).toBeInTheDocument();
  });

  it("keeps the dialog open with the server message when creation fails", async () => {
    server.use(
      http.post("/api/backends", () =>
        HttpResponse.json({ errors: [{ field: "backend_id", message: "backend id must be a slug" }] }, { status: 400 }),
      ),
    );
    await openAddDialog();
    fireEvent.click(screen.getByText("Create backend"));

    expect(await screen.findByText(/backend id must be a slug/)).toBeInTheDocument();
    expect(screen.getByLabelText("Provider")).toBeInTheDocument();
  });

  // FS-04.A20: create-and-connect returns a connected backend; a connection
  // failure still leaves it saved, visible, and retryable.
  it("reports a connected create and a saved-but-unbound failure", async () => {
    server.use(
      http.post("/api/backends", async ({ request }) => {
        const body = (await request.json()) as { name: string };
        return HttpResponse.json(
          {
            backend_id: "codex-openai",
            backend: { ...codexStarter, name: body.name },
            connection: { status: "connected", model_sync_enabled: true, models_added: 4 },
          },
          { status: 201 },
        );
      }),
    );
    const providerSelect = (await openAddDialog()) as HTMLSelectElement;
    fireEvent.change(providerSelect, { target: { value: "codex-acp" } });
    fireEvent.click(screen.getByText("Create and use my configuration"));

    expect(await screen.findByText(/imported 4 configured models/)).toBeInTheDocument();
    expect(screen.getByText(/not a check of availability or entitlement/)).toBeInTheDocument();

    cleanup();
    server.use(
      http.post("/api/backends", () =>
        HttpResponse.json(
          {
            backend_id: "codex-openai",
            backend: codexStarter,
            connection: { status: "unbound", error: { code: "source_not_found", message: "no native configuration found" } },
          },
          { status: 201 },
        ),
      ),
    );
    const retrySelect = (await openAddDialog()) as HTMLSelectElement;
    fireEvent.change(retrySelect, { target: { value: "codex-acp" } });
    fireEvent.click(screen.getByText("Create and use my configuration"));

    expect(await screen.findByText(/is not connected: no native configuration found/)).toBeInTheDocument();
    // The valid backend is still created and rendered so the person can retry.
    expect(screen.getByDisplayValue("Codex / OpenAI")).toBeInTheDocument();
  });

  // FS-08.A11 / TS-03.R23 / INV §1: a create-and-connect binds the source
  // server-side, so the already-mounted global config-source query is stale and
  // must be refetched — otherwise a sibling backend's panel would still read
  // bindings:[] and the new card would offer to connect an already-bound source.
  it("refetches the config-source query after a connected create", async () => {
    let sourceGets = 0;
    server.use(
      http.get("/api/config-sources", () => {
        sourceGets += 1;
        return HttpResponse.json({
          bindings: sourceGets === 1
            ? []
            : [{ backend_id: "codex-openai", provider: "codex", mode: "linked", root: "/native/codex", health: "ok", stale: false }],
          candidates: [],
        });
      }),
      http.post("/api/backends", () =>
        HttpResponse.json(
          {
            backend_id: "codex-openai",
            backend: codexStarter,
            connection: { status: "connected", model_sync_enabled: true, models_added: 1 },
          },
          { status: 201 },
        ),
      ),
    );

    renderWithQuery(<BackendsEditor />);
    await screen.findByDisplayValue("Claude");
    // The seeded claude-acp backend mounts the global config-source query once.
    await waitFor(() => expect(sourceGets).toBeGreaterThanOrEqual(1));
    const before = sourceGets;

    fireEvent.click(screen.getByText("Add backend"));
    fireEvent.change(await screen.findByLabelText("Provider"), { target: { value: "codex-acp" } });
    fireEvent.click(screen.getByText("Create and use my configuration"));

    await waitFor(() => expect(sourceGets).toBeGreaterThan(before));
    expect(await screen.findByText("/native/codex")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Use my Codex configuration" })).not.toBeInTheDocument();
  });

  it("only offers native configuration for federated providers", async () => {
    const providerSelect = (await openAddDialog()) as HTMLSelectElement;
    expect(screen.getByText("Create and use my configuration")).toBeInTheDocument();

    fireEvent.change(providerSelect, { target: { value: "opencode-acp" } });
    await waitFor(() =>
      expect(screen.queryByText("Create and use my configuration")).not.toBeInTheDocument(),
    );
  });

  it("changes nothing when the dialog is cancelled", async () => {
    let posts = 0;
    server.use(http.post("/api/backends", () => { posts += 1; return HttpResponse.json({}, { status: 201 }); }));
    await openAddDialog();
    fireEvent.click(screen.getByText("Cancel"));

    await waitFor(() => expect(screen.queryByLabelText("Provider")).not.toBeInTheDocument());
    expect(posts).toBe(0);
    expect(screen.getByDisplayValue("Claude")).toBeInTheDocument();
  });
});
