import React from "react";
import { describe, it, expect, vi, beforeAll, afterAll, afterEach } from "vitest";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { SourceStep } from "./SourceStep";

const server = setupServer(
  http.get("/api/projects", () => HttpResponse.json({ app: { title: "App", cwd: "/tmp/app" } })),
  http.get("/api/config", () => HttpResponse.json({ default_project: "app", default_role: "impl", onboarding_complete: false })),
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

describe("SourceStep", () => {
  it("is optional and Continue advances the wizard", async () => {
    const onDone = vi.fn();
    renderWithQuery(<SourceStep project="app" backendId="claude" backendType="claude-acp" onDone={onDone} />);
    expect(await screen.findByText(/Link your CLI configuration/)).toBeInTheDocument();
    // The reused federation panel is present and expanded, targeting Claude.
    expect(screen.getByText(/Configuration source \(Claude Code\)/)).toBeInTheDocument();
    fireEvent.click(screen.getByText("Continue"));
    expect(onDone).toHaveBeenCalledTimes(1);
  });

  it("links the selected provider (Codex), not a hard-coded Claude", async () => {
    renderWithQuery(<SourceStep project="app" backendId="codex" backendType="codex-acp" onDone={vi.fn()} />);
    // The panel must target Codex, proving the chosen backend is carried through.
    expect(await screen.findByText(/Configuration source \(Codex\)/)).toBeInTheDocument();
    expect(screen.queryByText(/Configuration source \(Claude Code\)/)).not.toBeInTheDocument();
  });

  it("shows no federation controls for a non-federated backend", async () => {
    const onDone = vi.fn();
    renderWithQuery(<SourceStep project="app" backendId="opencode" backendType="opencode-acp" onDone={onDone} />);
    expect(await screen.findByText(/configured directly in Settings/)).toBeInTheDocument();
    expect(screen.queryByText(/Configuration source/)).not.toBeInTheDocument();
    fireEvent.click(screen.getByText("Continue"));
    expect(onDone).toHaveBeenCalledTimes(1);
  });
});
