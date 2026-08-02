import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { ConfigSourcePanel } from "./ConfigSourcePanel";

const emptyEffective = {
  model: null,
  effort: null,
  provider: null,
  models: [],
  assets: [],
  environment_keys: [],
  mcp_servers: [],
  provenance: {},
};

const server = setupServer(
  http.get("/api/projects", () => HttpResponse.json({ app: { title: "App", cwd: "/tmp/app" } })),
  http.get("/api/config", () => HttpResponse.json({ default_project: "app", default_role: "impl", onboarding_complete: true })),
  http.get("/api/config-sources", () => HttpResponse.json({ bindings: [], candidates: [] })),
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

describe("ConfigSourcePanel", () => {
  it("renders nothing for non-federation backend types", () => {
    const { container } = renderWithQuery(
      <ConfigSourcePanel backendId="oc" backendType={"opencode-acp" as never} />,
    );
    expect(container.firstChild).toBeNull();
  });

  it("requires a Settings backend to be saved before it can be linked", async () => {
    renderWithQuery(<ConfigSourcePanel backendId="backend-draft" backendType="codex-acp" persisted={false} />);

    expect(await screen.findByText("Save this backend before linking a configuration source.")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Use my/ })).not.toBeInTheDocument();
  });

  // FS-08.A11 (R34): one visible action performs a project-free, auto-root,
  // Linked preview and its matching bind with the model import enabled. It
  // exposes no project, root, profile, claim, or mode chooser.
  it("connects a native source through one action with no project or mode chooser", async () => {
    const previewBodies: Array<{ mode: string; root: string; project?: string }> = [];
    let bindBody: { preview_token: string; enable_model_sync: boolean } | null = null;
    server.use(
      http.post("/api/config-sources/preview", async ({ request }) => {
        previewBodies.push((await request.json()) as { mode: string; root: string; project?: string });
        return HttpResponse.json({
          preview_token: "tok123",
          expires_at: new Date(Date.now() + 600000).toISOString(),
          effective: { ...emptyEffective, model: "user-model" },
          report: { source_digest: "abc", files_read: [], skipped: [], unknown_keys: [], warnings: [], fingerprints: [], approved_roots: [] },
        });
      }),
      http.put("/api/config-sources/:id", async ({ request }) => {
        bindBody = (await request.json()) as { preview_token: string; enable_model_sync: boolean };
        return HttpResponse.json({
          backend_id: "claude", provider: "claude-code", mode: "linked", root: "/h/.claude",
          health: "ok", stale: false, model_sync_enabled: true, models_added: 2,
        });
      }),
    );

    let connected = 0;
    renderWithQuery(
      <ConfigSourcePanel backendId="claude" backendType="claude-acp" onConnected={() => (connected += 1)} />,
    );

    // No project select and no mirrored fallback control anywhere.
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
    expect(screen.queryByText(/Mirrored/)).not.toBeInTheDocument();

    fireEvent.click(await screen.findByText("Use my Claude Code configuration"));

    await waitFor(() => expect(bindBody).not.toBeNull());
    expect(previewBodies).toEqual([
      { provider: "claude-code", root: "auto", mode: "linked", claims: ["launch_defaults", "model_catalog", "setup"] },
    ]);
    expect(bindBody!).toMatchObject({ preview_token: "tok123", enable_model_sync: true });
    // The import result is reported without any support/entitlement claim.
    expect(await screen.findByText(/Imported 2 configured models\./)).toBeInTheDocument();
    expect(screen.getByText(/does not check availability or entitlement/)).toBeInTheDocument();
    expect(connected).toBe(1);
  });

  // FS-08.A11: a failed connection stays visible and retryable and creates no
  // binding; the normal flow never offers Mirrored as the recovery.
  it("keeps a failed connection retryable without offering mirrored", async () => {
    let previews = 0;
    server.use(
      http.post("/api/config-sources/preview", () => {
        previews += 1;
        if (previews === 1) {
          return HttpResponse.json(
            { error: { code: "source_not_found", message: "no native configuration found for claude-code" } },
            { status: 404 },
          );
        }
        return HttpResponse.json({
          preview_token: "tok-retry",
          expires_at: new Date(Date.now() + 600000).toISOString(),
          effective: { ...emptyEffective },
          report: { source_digest: "abc", files_read: [], skipped: [], unknown_keys: [], warnings: [], fingerprints: [], approved_roots: [] },
        });
      }),
      http.put("/api/config-sources/:id", () =>
        HttpResponse.json({
          backend_id: "claude", provider: "claude-code", mode: "linked", root: "/h/.claude",
          health: "ok", stale: false, model_sync_enabled: true, models_added: 0,
        }),
      ),
    );

    renderWithQuery(<ConfigSourcePanel backendId="claude" backendType="claude-acp" />);
    const connect = await screen.findByText("Use my Claude Code configuration");
    fireEvent.click(connect);

    expect(await screen.findByText(/no native configuration found/)).toBeInTheDocument();
    expect(screen.queryByText(/Mirrored/)).not.toBeInTheDocument();

    // The same action retries and succeeds.
    fireEvent.click(screen.getByText("Use my Claude Code configuration"));
    expect(await screen.findByText(/No new configured models to import/)).toBeInTheDocument();
  });

  // Regression (review fix): Codex emits plural asset kinds (instructions,
  // mcp_servers), so the inventory groups must match them or a Codex user sees
  // neither AGENTS.md nor MCP servers.
  it("renders Codex plural asset kinds (instructions, mcp_servers)", async () => {
    server.use(
      http.get("/api/config-sources", () =>
        HttpResponse.json({
          bindings: [{ backend_id: "codex", provider: "codex", mode: "linked", root: "/h/.codex", claims: [], approved_roots: [], health: "ok", stale: false }],
          candidates: [],
        }),
      ),
      http.post("/api/config-sources/codex/refresh", () =>
        HttpResponse.json({
          binding: { backend_id: "codex", provider: "codex", mode: "linked", root: "/h/.codex", health: "ok", stale: false },
          effective: {
            ...emptyEffective,
            assets: [
              { kind: "instructions", name: "AGENTS.md", path: "/p/AGENTS.md", scope: "project", detachability: "copyable", status: "native_passthrough" },
              { kind: "mcp_servers", name: "my-server", path: "/h/.codex/config.toml", scope: "user", detachability: "reference_only", status: "native_passthrough" },
            ],
          },
        }),
      ),
    );
    renderWithQuery(<ConfigSourcePanel backendId="codex" backendType="codex-acp" />);
    fireEvent.click(await screen.findByText("Load effective view"));
    expect(await screen.findByText("AGENTS.md")).toBeInTheDocument();
    expect(screen.getByText("my-server")).toBeInTheDocument();
  });

  it("shows a bound source with health and an unlink action", async () => {
    server.use(
      http.get("/api/config-sources", () =>
        HttpResponse.json({
          bindings: [{ backend_id: "claude", provider: "claude-code", mode: "linked", root: "/h/.claude", claims: [], approved_roots: [], health: "ok", stale: false }],
          candidates: [],
        }),
      ),
    );
    renderWithQuery(<ConfigSourcePanel backendId="claude" backendType="claude-acp" />);
    expect(await screen.findByText("Unlink")).toBeInTheDocument();
    expect(screen.getByText("linked")).toBeInTheDocument();
  });

  // Regression (review fix): a bound source must offer override + reset-to-inherit.
  // Apply re-previews the same source for a token, then re-binds with the overrides;
  // Reset re-binds with null overrides.
  it("applies a model override and resets a bound source to inherit", async () => {
    const boundOverrides: Array<{ model: string | null; effort: string | null }> = [];
    server.use(
      http.get("/api/config-sources", () =>
        HttpResponse.json({
          bindings: [{ backend_id: "claude", provider: "claude-code", mode: "linked", root: "/h/.claude", claims: [], approved_roots: [], overrides: { model: null, effort: null }, health: "ok", stale: false }],
          candidates: [],
        }),
      ),
      http.post("/api/config-sources/preview", () =>
        HttpResponse.json({
          preview_token: "tokX",
          expires_at: new Date(Date.now() + 600000).toISOString(),
          effective: { ...emptyEffective },
          report: { source_digest: "abc", files_read: [], skipped: [], unknown_keys: [], warnings: [], fingerprints: [], approved_roots: [] },
        }),
      ),
      http.put("/api/config-sources/:id", async ({ request }) => {
        const body = (await request.json()) as { overrides: { model: string | null; effort: string | null } };
        boundOverrides.push(body.overrides);
        return HttpResponse.json({ backend_id: "claude", provider: "claude-code", mode: "linked", root: "/h/.claude", health: "ok", stale: false });
      }),
    );

    renderWithQuery(<ConfigSourcePanel backendId="claude" backendType="claude-acp" />);
    const modelInput = (await screen.findByLabelText("Model override")) as HTMLInputElement;
    fireEvent.change(modelInput, { target: { value: "opus" } });
    fireEvent.click(screen.getByText("Apply override"));
    await waitFor(() => expect(boundOverrides).toContainEqual({ model: "opus", effort: null }));

    fireEvent.click(screen.getByText("Reset to inherit"));
    await waitFor(() => expect(boundOverrides).toContainEqual({ model: null, effort: null }));
  });
});
