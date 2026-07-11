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
});
