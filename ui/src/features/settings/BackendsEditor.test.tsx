import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { BackendsEditor } from "./BackendsEditor";
import { BACKEND_SUPPORT_WIRE } from "../../test/backendSupport";

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
  http.get("/api/backends", () => HttpResponse.json({ ...defaultBackendsDoc, backend_support: BACKEND_SUPPORT_WIRE })),
  http.put("/api/backends", () =>
    HttpResponse.json({
      ...defaultBackendsDoc,
      backend_support: BACKEND_SUPPORT_WIRE,
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

  // ---- Model capability controls (FS-09.R62 / A32) ----

  function fastCheckbox() {
    return screen.getByText("Fast mode capability").parentElement!.querySelector("input") as HTMLInputElement;
  }

  it("keeps fast known when chat is valid and terminal metadata is malformed", async () => {
    server.use(http.get("/api/backends", () => HttpResponse.json({
      ...defaultBackendsDoc,
      backend_support: {
        "claude-acp": {
          chat: { available: true, effort: false, fast: true },
          terminal: { available: true, effort: "invalid", fast: false },
        },
      },
    })));
    renderWithQuery(<BackendsEditor />);
    await screen.findByDisplayValue("Claude");
    fireEvent.click(screen.getByRole("button", { name: /▾ env/ }));

    expect(screen.getByRole("status").textContent).toContain("Effort support could not be loaded.");
    expect(screen.getByRole("status").textContent).not.toContain("Fast support could not be loaded.");
    expect(screen.queryByPlaceholderText("low, medium, high")).not.toBeInTheDocument();
    expect(fastCheckbox().disabled).toBe(false);
  });

  it("keeps effort known when terminal is valid and chat metadata is malformed", async () => {
    server.use(http.get("/api/backends", () => HttpResponse.json({
      ...defaultBackendsDoc,
      backend_support: {
        "claude-acp": {
          chat: { available: true, effort: "invalid", fast: false },
          terminal: { available: true, effort: true, fast: false },
        },
      },
    })));
    renderWithQuery(<BackendsEditor />);
    await screen.findByDisplayValue("Claude");
    fireEvent.click(screen.getByRole("button", { name: /▾ env/ }));

    expect(screen.getByRole("status").textContent).not.toContain("Effort support could not be loaded.");
    expect(screen.getByRole("status").textContent).toContain("Fast support could not be loaded.");
    expect(screen.getByPlaceholderText("low, medium, high")).toBeInTheDocument();
    expect(screen.queryByText("Fast mode capability")).not.toBeInTheDocument();
  });

  it("lets a supported Claude model gain effort and fast declarations", async () => {
    renderWithQuery(<BackendsEditor />);
    await screen.findByDisplayValue("Claude");
    fireEvent.click(screen.getByRole("button", { name: /▾ env/ }));

    const levels = screen.getByPlaceholderText("low, medium, high") as HTMLInputElement;
    expect(levels.disabled).toBe(false);
    fireEvent.change(levels, { target: { value: "low, high" } });
    await waitFor(() => expect(levels.value).toBe("low, high"));

    const fast = fastCheckbox();
    expect(fast.disabled).toBe(false);
    fireEvent.click(fast);
    await waitFor(() => expect(fast.checked).toBe(true));
  });

  it("preserves and explains now-unsupported values after changing type to OpenCode, offers explicit clearing, and saves after clearing", async () => {
    let putBody: { backends: Record<string, { models: Record<string, { efforts?: string[]; default_effort?: string; fast?: boolean }> }> } | null = null;
    server.use(
      http.put("/api/backends", async ({ request }) => {
        putBody = (await request.json()) as typeof putBody;
        return HttpResponse.json({ ...defaultBackendsDoc, backend_support: BACKEND_SUPPORT_WIRE, credentials: {} });
      }),
    );
    renderWithQuery(<BackendsEditor />);
    await screen.findByDisplayValue("Claude");
    fireEvent.click(screen.getByRole("button", { name: /▾ env/ }));
    fireEvent.change(screen.getByPlaceholderText("low, medium, high"), { target: { value: "low, high" } });
    await waitFor(() => expect((screen.getByPlaceholderText("low, medium, high") as HTMLInputElement).value).toBe("low, high"));
    fireEvent.click(fastCheckbox());
    await waitFor(() => expect(fastCheckbox().checked).toBe(true));

    const typeSelect = screen.getByDisplayValue(/Claude \(claude-acp\)/) as HTMLSelectElement;
    fireEvent.change(typeSelect, { target: { value: "opencode-acp" } });

    // The values stay, but as disabled controls with a reason and a clear action.
    await waitFor(() => expect(screen.getAllByText("This backend type doesn't support this.")).toHaveLength(2));
    const stillLevels = screen.getByDisplayValue("low, high") as HTMLInputElement;
    expect(stillLevels.disabled).toBe(true);
    const stillFast = fastCheckbox();
    expect(stillFast.checked).toBe(true);
    expect(stillFast.disabled).toBe(true);

    fireEvent.click(screen.getByText("Clear effort levels"));
    fireEvent.click(screen.getByText("Clear fast mode"));

    await waitFor(() => expect(screen.queryByDisplayValue("low, high")).not.toBeInTheDocument());
    expect(screen.queryByText("Fast mode capability")).not.toBeInTheDocument();

    fireEvent.click(screen.getByText("Save"));
    await waitFor(() => expect(putBody).not.toBeNull());
    const savedModel = putBody!.backends.claude.models.sonnet;
    expect(savedModel.efforts).toEqual([]);
    expect(savedModel.default_effort).toBeUndefined();
    expect(savedModel.fast).toBe(false);
  });

  it("shows a collapsed model's now-unsupported declaration without expanding it", async () => {
    server.use(http.get("/api/backends", () => HttpResponse.json({
      version: 2,
      backend_support: BACKEND_SUPPORT_WIRE,
      backends: { claude: { ...defaultBackendsDoc.backends.claude, models: { sonnet: { name: "Sonnet", model: "s", efforts: ["low", "high"], default_effort: "low" } } } },
    })));
    renderWithQuery(<BackendsEditor />);
    await screen.findByDisplayValue("Claude");
    expect(screen.queryByText("Clear effort levels")).not.toBeInTheDocument();
    fireEvent.change(screen.getByDisplayValue(/Claude \(claude-acp\)/), { target: { value: "opencode-acp" } });
    expect(await screen.findByText("Clear effort levels")).toBeInTheDocument();
  });

  it("retains unsupported values when switching back to a supporting type before clearing", async () => {
    renderWithQuery(<BackendsEditor />);
    await screen.findByDisplayValue("Claude");
    fireEvent.click(screen.getByRole("button", { name: /▾ env/ }));
    fireEvent.change(screen.getByPlaceholderText("low, medium, high"), { target: { value: "low, high" } });
    await waitFor(() => expect((screen.getByPlaceholderText("low, medium, high") as HTMLInputElement).value).toBe("low, high"));

    const typeSelect = screen.getByDisplayValue(/Claude \(claude-acp\)/) as HTMLSelectElement;
    fireEvent.change(typeSelect, { target: { value: "opencode-acp" } });
    await waitFor(() => expect(screen.getByDisplayValue("low, high")).toBeInTheDocument());

    fireEvent.change(typeSelect, { target: { value: "claude-acp" } });
    await waitFor(() => expect((screen.getByPlaceholderText("low, medium, high") as HTMLInputElement).value).toBe("low, high"));
  });

  it("does not let an empty model under OpenCode gain declarations through the controls", async () => {
    server.use(
      http.get("/api/backends", () =>
        HttpResponse.json({
          version: 2,
          backends: {
            oc: {
              name: "OC",
              type: "opencode-acp",
              default: true,
              default_model: "m1",
              models: { m1: { name: "Model 1", model: "oc-model" } },
            },
          },
          backend_support: BACKEND_SUPPORT_WIRE,
        }),
      ),
    );
    renderWithQuery(<BackendsEditor />);
    await screen.findByDisplayValue("OC");
    fireEvent.click(screen.getByRole("button", { name: /▾ env/ }));

    expect(screen.queryByPlaceholderText("low, medium, high")).not.toBeInTheDocument();
    expect(screen.queryByText("Fast mode capability")).not.toBeInTheDocument();
  });

  it("shows retry guidance when backend_support is missing, and Retry refetches support without truncating the unsaved draft or leaking backend_support into the PUT body", async () => {
    let getCount = 0;
    server.use(
      http.get("/api/backends", () => {
        getCount += 1;
        return HttpResponse.json(
          getCount === 1 ? defaultBackendsDoc : { ...defaultBackendsDoc, backend_support: BACKEND_SUPPORT_WIRE },
        );
      }),
    );
    let putBody: Record<string, unknown> | null = null;
    server.use(
      http.put("/api/backends", async ({ request }) => {
        putBody = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json({ ...defaultBackendsDoc, credentials: {} });
      }),
    );

    renderWithQuery(<BackendsEditor />);
    await screen.findByDisplayValue("Claude");
    fireEvent.click(screen.getByRole("button", { name: /▾ env/ }));

    expect(await screen.findByText(/could not be loaded/)).toBeInTheDocument();
    expect(screen.queryByPlaceholderText("low, medium, high")).not.toBeInTheDocument();

    // An unsaved edit made while support is missing must survive the retry.
    const nameInput = screen.getByDisplayValue("Claude") as HTMLInputElement;
    fireEvent.change(nameInput, { target: { value: "Renamed Claude" } });

    fireEvent.click(screen.getByText("Retry"));

    await waitFor(() => expect(screen.getByPlaceholderText("low, medium, high")).toBeInTheDocument());
    expect(screen.getByDisplayValue("Renamed Claude")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Save"));
    await waitFor(() => expect(putBody).not.toBeNull());
    expect(putBody!.backend_support).toBeUndefined();
  });

  // FS-09.R72/R75–R76/A40/A43/A45 — the provider choice, executable path,
  // next-start metadata and Refresh provider in Settings.
  describe("provider", () => {
    const runtimes = (version?: string) => ({
      claude: { sonnet: { source: "backend", state: "available", path: "/Users/me/bin/claude", ...(version ? { version, checked_at: "2026-10-04T10:00:00Z" } : {}) } },
    });
    const savedDoc = {
      ...defaultBackendsDoc,
      backends: {
        claude: {
          ...defaultBackendsDoc.backends.claude,
          autosync_models: true,
          env: { CLAUDE_CODE_EXECUTABLE: "/Users/me/bin/claude", ANTHROPIC_BASE_URL: "https://proxy" },
        },
      },
    };

    it("shows the saved next-start provider and saves Bundle without deleting overrides", async () => {
      let putBody: { backends: Record<string, { provider_mode?: string; env?: Record<string, string> }> } | null = null;
      server.use(
        http.get("/api/backends", () => HttpResponse.json({ ...savedDoc, backend_support: BACKEND_SUPPORT_WIRE, provider_runtimes: runtimes() })),
        http.put("/api/backends", async ({ request }) => {
          putBody = (await request.json()) as typeof putBody;
          return HttpResponse.json({ ...savedDoc, credentials: {} });
        }),
      );
      renderWithQuery(<BackendsEditor />);
      expect(await screen.findByText(/Next start: Installed \(backend path\) Claude Code · \/Users\/me\/bin\/claude · version not checked/)).toBeInTheDocument();
      expect(screen.getByRole("radio", { name: "Installed provider (default)" })).toBeChecked();
      expect(screen.getByLabelText("Executable path (advanced)")).toHaveValue("/Users/me/bin/claude");

      fireEvent.click(screen.getByRole("radio", { name: "Chuck bundle" }));
      expect(screen.getByText(/are inactive while the Chuck bundle is selected/)).toBeInTheDocument();
      expect(screen.getByText("Save to check provider.")).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Refresh provider" })).toBeDisabled();

      fireEvent.click(screen.getByText("Save"));
      await waitFor(() => expect(putBody).not.toBeNull());
      expect(putBody!.backends.claude.provider_mode).toBe("bundled");
      expect(putBody!.backends.claude.env).toEqual({ CLAUDE_CODE_EXECUTABLE: "/Users/me/bin/claude", ANTHROPIC_BASE_URL: "https://proxy" });
      expect((putBody as unknown as Record<string, unknown>).provider_runtimes).toBeUndefined();
    });

    it("edits only the executable entry and clears provider mode on a type change", async () => {
      let putBody: { backends: Record<string, { type: string; provider_mode?: string; env?: Record<string, string> }> } | null = null;
      server.use(
        http.get("/api/backends", () => HttpResponse.json({ ...savedDoc, backends: { claude: { ...savedDoc.backends.claude, provider_mode: "bundled" } }, backend_support: BACKEND_SUPPORT_WIRE })),
        http.put("/api/backends", async ({ request }) => {
          putBody = (await request.json()) as typeof putBody;
          return HttpResponse.json({ ...savedDoc, credentials: {} });
        }),
      );
      renderWithQuery(<BackendsEditor />);
      fireEvent.click(await screen.findByRole("radio", { name: "Installed provider (default)" }));
      fireEvent.change(screen.getByLabelText("Executable path (advanced)"), { target: { value: "claude-beta" } });
      fireEvent.click(screen.getByText("Save"));
      await waitFor(() => expect(putBody).not.toBeNull());
      expect(putBody!.backends.claude.provider_mode).toBe("installed");
      expect(putBody!.backends.claude.env).toEqual({ CLAUDE_CODE_EXECUTABLE: "claude-beta", ANTHROPIC_BASE_URL: "https://proxy" });

      putBody = null;
      fireEvent.change(screen.getAllByRole("combobox")[0], { target: { value: "opencode-acp" } });
      expect(screen.queryByRole("radio", { name: "Chuck bundle" })).not.toBeInTheDocument();
      fireEvent.click(screen.getByText("Save"));
      await waitFor(() => expect(putBody).not.toBeNull());
      expect(putBody!.backends.claude.provider_mode).toBeUndefined();
    });

    it("refreshes a clean saved backend and folds new models in without changing the default", async () => {
      let refreshed = false;
      let ifMatch: string | null = null;
      server.use(
        http.get("/api/backends", () => {
          const doc = refreshed
            ? { ...savedDoc, backends: { claude: { ...savedDoc.backends.claude, models: { ...savedDoc.backends.claude.models, opus: { name: "claude-opus-5-5", model: "claude-opus-5-5" } } } } }
            : savedDoc;
          return HttpResponse.json(
            { ...doc, backend_support: BACKEND_SUPPORT_WIRE, provider_runtimes: runtimes(refreshed ? "2.1.300" : undefined) },
            { headers: { ETag: refreshed ? '"v2"' : '"v1"' } },
          );
        }),
        http.post("/api/backends/claude/refresh-provider", async ({ request }) => {
          ifMatch = request.headers.get("If-Match");
          expect(await request.json()).toEqual({ model_id: "sonnet" });
          refreshed = true;
          return HttpResponse.json({
            runtime: runtimes("2.1.300").claude.sonnet,
            credentials: { status: "ok" },
            catalog: { status: "added", added_count: 1 },
          }, { headers: { ETag: '"v2"' } });
        }),
      );
      renderWithQuery(<BackendsEditor />);
      const refresh = await screen.findByRole("button", { name: "Refresh provider" });
      await waitFor(() => expect(refresh).toBeEnabled());
      fireEvent.click(refresh);
      expect(await screen.findByText("Version 2.1.300; signed in; 1 new model added.")).toBeInTheDocument();
      expect(ifMatch).toBe('"v1"');
      expect((await screen.findAllByDisplayValue("claude-opus-5-5")).length).toBe(2);
      expect(screen.getByText(/version 2\.1\.300, last checked/)).toBeInTheDocument();
      expect(screen.getAllByRole("radio", { name: /Default/ }).length).toBeGreaterThan(0);
    });

    it("keeps edits made while a refresh runs and folds in only the added models", async () => {
      let refreshed = false;
      let release!: () => void;
      const held = new Promise<void>((resolve) => { release = resolve; });
      let putBody: { catalogEtag?: string; backends: Record<string, { name: string; models: Record<string, unknown> }> } | null = null;
      let putIfMatch: string | null = null;
      server.use(
        http.get("/api/backends", () => {
          const doc = refreshed
            ? { ...savedDoc, backends: { claude: { ...savedDoc.backends.claude, models: { ...savedDoc.backends.claude.models, opus: { name: "claude-opus-5-5", model: "claude-opus-5-5" } } } } }
            : savedDoc;
          return HttpResponse.json(
            { ...doc, backend_support: BACKEND_SUPPORT_WIRE, provider_runtimes: runtimes(refreshed ? "2.1.300" : undefined) },
            { headers: { ETag: refreshed ? '"v2"' : '"v1"' } },
          );
        }),
        http.post("/api/backends/claude/refresh-provider", async () => {
          await held;
          refreshed = true;
          return HttpResponse.json({
            runtime: runtimes("2.1.300").claude.sonnet,
            credentials: { status: "ok" },
            catalog: { status: "added", added_count: 1 },
          }, { headers: { ETag: '"v2"' } });
        }),
        http.put("/api/backends", async ({ request }) => {
          putIfMatch = request.headers.get("If-Match");
          putBody = (await request.json()) as typeof putBody;
          return HttpResponse.json({ ...savedDoc, credentials: {} });
        }),
      );
      renderWithQuery(<BackendsEditor />);
      const refresh = await screen.findByRole("button", { name: "Refresh provider" });
      await waitFor(() => expect(refresh).toBeEnabled());
      fireEvent.click(refresh);
      await screen.findByRole("button", { name: "Checking…" });

      fireEvent.change(screen.getByDisplayValue("Claude"), { target: { value: "Claude edited" } });
      release();

      expect((await screen.findAllByDisplayValue("claude-opus-5-5")).length).toBe(2);
      expect(screen.getByDisplayValue("Claude edited")).toBeInTheDocument();

      fireEvent.click(screen.getByText("Save"));
      await waitFor(() => expect(putBody).not.toBeNull());
      expect(putBody!.backends.claude.name).toBe("Claude edited");
      expect(Object.keys(putBody!.backends.claude.models)).toContain("opus");
      expect(putIfMatch).toBe('"v2"');
    });

    it("runs one refresh at a time across backends", async () => {
      const twoDoc = {
        ...savedDoc,
        backends: { ...savedDoc.backends, other: { ...savedDoc.backends.claude, name: "Claude two", default: false } },
      };
      let release!: () => void;
      const held = new Promise<void>((resolve) => { release = resolve; });
      server.use(
        http.get("/api/backends", () => HttpResponse.json(
          { ...twoDoc, backend_support: BACKEND_SUPPORT_WIRE, provider_runtimes: { ...runtimes(), other: runtimes().claude } },
          { headers: { ETag: '"v1"' } },
        )),
        http.post("/api/backends/claude/refresh-provider", async () => {
          await held;
          return HttpResponse.json({
            runtime: runtimes("2.1.300").claude.sonnet,
            credentials: { status: "ok" },
            catalog: { status: "unchanged", added_count: 0 },
          }, { headers: { ETag: '"v1"' } });
        }),
      );
      renderWithQuery(<BackendsEditor />);
      await waitFor(() => expect(screen.getAllByRole("button", { name: "Refresh provider" })).toHaveLength(2));
      const [first] = screen.getAllByRole("button", { name: "Refresh provider" });
      await waitFor(() => expect(first).toBeEnabled());
      fireEvent.click(first);
      await screen.findByRole("button", { name: "Checking…" });
      expect(screen.getByRole("button", { name: "Refresh provider" })).toBeDisabled();

      release();
      expect(await screen.findByText("Version 2.1.300; signed in; no new models.")).toBeInTheDocument();
      await waitFor(() => screen.getAllByRole("button", { name: "Refresh provider" }).forEach((b) => expect(b).toBeEnabled()));
    });

    it("keeps the catalog editable when provider metadata is malformed", async () => {
      server.use(
        http.get("/api/backends", () => HttpResponse.json({ ...savedDoc, backend_support: BACKEND_SUPPORT_WIRE, provider_runtimes: { claude: { sonnet: { source: "weird" } } } })),
      );
      renderWithQuery(<BackendsEditor />);
      expect(await screen.findByDisplayValue("Claude")).toBeInTheDocument();
      expect(screen.getByText("Save to check provider.")).toBeInTheDocument();
    });

    it("explains a missing installed provider with an install link", async () => {
      server.use(
        http.get("/api/backends", () => HttpResponse.json({ ...defaultBackendsDoc, backend_support: BACKEND_SUPPORT_WIRE, provider_runtimes: { claude: { sonnet: { source: "detected", state: "missing" } } } })),
      );
      renderWithQuery(<BackendsEditor />);
      expect(await screen.findByText(/Claude Code was not found/)).toBeInTheDocument();
      expect(screen.getByRole("link", { name: "Install Claude Code" })).toHaveAttribute("href", "https://code.claude.com/docs/en/setup");
    });
  });
});
