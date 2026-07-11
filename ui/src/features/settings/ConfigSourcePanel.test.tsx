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

  it("discovers then links a native source", async () => {
    let bound = false;
    server.use(
      http.post("/api/config-sources/preview", () =>
        HttpResponse.json({
          preview_token: "tok123",
          expires_at: new Date(Date.now() + 600000).toISOString(),
          effective: { ...emptyEffective, model: "user-model", provenance: { model: { scope: "user", path: "/h/.claude/settings.json", key: "model" } } },
          report: { source_digest: "abc", files_read: [], skipped: [], unknown_keys: [], warnings: [], fingerprints: [], approved_roots: [] },
        }),
      ),
      http.put("/api/config-sources/:id", () => {
        bound = true;
        return HttpResponse.json({ backend_id: "claude", provider: "claude-code", mode: "linked", root: "/h/.claude", health: "ok", stale: false });
      }),
    );

    renderWithQuery(<ConfigSourcePanel backendId="claude" backendType="claude-acp" />);

    // The Discover button is disabled until the project id resolves from the
    // async projects query; wait for it to enable before clicking.
    const discover = (await screen.findByText("Discover native config")) as HTMLButtonElement;
    await waitFor(() => expect(discover).not.toBeDisabled());
    fireEvent.click(discover);

    // The redacted effective model with its provenance label appears.
    expect(await screen.findByText(/user-model — inherited from user/)).toBeInTheDocument();

    // Link (Linked) binds with the preview token.
    fireEvent.click(screen.getByText(/Link \(Linked/));
    await waitFor(() => expect(bound).toBe(true));
  });

  // Regression (review fix): "Discover" previews Linked to show the effective view;
  // clicking "Link (Mirrored)" must bind a token minted FOR mirrored (the server
  // derives the bound mode from the token), not reuse the linked discovery token.
  it("Link (Mirrored) binds a mirrored-minted token, not the linked discovery token", async () => {
    const previewedModes: string[] = [];
    let boundToken: string | null = null;
    server.use(
      http.post("/api/config-sources/preview", async ({ request }) => {
        const body = (await request.json()) as { mode: string };
        previewedModes.push(body.mode);
        return HttpResponse.json({
          preview_token: `tok-${body.mode}`,
          expires_at: new Date(Date.now() + 600000).toISOString(),
          effective: { ...emptyEffective, model: "user-model", provenance: { model: { scope: "user", path: "/h/.claude/settings.json", key: "model" } } },
          report: { source_digest: "abc", files_read: [], skipped: [], unknown_keys: [], warnings: [], fingerprints: [], approved_roots: [] },
        });
      }),
      http.put("/api/config-sources/:id", async ({ request }) => {
        const body = (await request.json()) as { preview_token: string };
        boundToken = body.preview_token;
        return HttpResponse.json({ backend_id: "claude", provider: "claude-code", mode: "mirrored", root: "/h/.claude", health: "ok", stale: false });
      }),
    );

    renderWithQuery(<ConfigSourcePanel backendId="claude" backendType="claude-acp" />);
    const discover = (await screen.findByText("Discover native config")) as HTMLButtonElement;
    await waitFor(() => expect(discover).not.toBeDisabled());
    fireEvent.click(discover); // discovery previews Linked
    expect(await screen.findByText(/user-model/)).toBeInTheDocument();

    fireEvent.click(screen.getByText(/Link \(Mirrored/));
    await waitFor(() => expect(boundToken).toBe("tok-mirrored"));
    expect(previewedModes).toContain("mirrored");
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
