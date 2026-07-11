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
    renderWithQuery(<SourceStep project="app" onDone={onDone} />);
    expect(await screen.findByText(/Link your CLI configuration/)).toBeInTheDocument();
    // The reused federation panel is present and expanded.
    expect(screen.getByText(/Configuration source/)).toBeInTheDocument();
    fireEvent.click(screen.getByText("Continue"));
    expect(onDone).toHaveBeenCalledTimes(1);
  });
});
