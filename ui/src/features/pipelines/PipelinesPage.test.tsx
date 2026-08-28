import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { PipelineTemplatePage, PipelinesIndex, PipelinesLayout, RunsPage, TemplatesPage } from "./PipelinesPage";

const template = {
  version: 1, title: "Delivery", inputs: [],
  stages: [{ id: "work", title: "Work", role: "implementer", instruction: "Do the work", inputs: [], outputs: [], max_visits: 1, transitions: { success: { final: "success", approval: "automatic" }, failure: { final: "failure", approval: "required" } } }],
};
const server = setupServer(
  http.get("/api/pipeline-proposals", () => HttpResponse.json([])),
  http.get("/api/pipeline-runs", () => HttpResponse.json([], { headers: { "X-Total-Count": "0" } })),
  http.get("/api/pipelines", () => HttpResponse.json([{ id: "delivery", template, valid: true, diagnostics: [] }])),
  http.get("/api/roles", () => HttpResponse.json({ implementer: { title: "Implementer", system_prompt: "", skip_permissions: false } })),
  http.get("/api/projects", () => HttpResponse.json({ app: { title: "App", cwd: "/tmp/app", color: [1, 2, 3], add_dirs: [], context_prompt: "" } })),
  http.get("/api/backends", () => HttpResponse.json({ version: 2, backends: {} })),
  http.get("/api/config", () => HttpResponse.json({ default_project: "app" })),
);

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

function renderPipelines(path: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
  return render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><Routes>
    <Route path="/pipelines" element={<PipelinesLayout />}>
      <Route index element={<PipelinesIndex />} />
      <Route path="runs" element={<RunsPage />} />
      <Route path="templates" element={<TemplatesPage />} />
      <Route path="templates/:templateID" element={<PipelineTemplatePage />} />
      <Route path="runs/:runID" element={<div>Run route</div>} />
    </Route>
  </Routes></MemoryRouter></QueryClientProvider>);
}

describe("Pipelines routing", () => {
  // FS-14.A14/A19: the destination opens on Runs, each job owns its route, and
  // old selected-run links resolve to the addressable run page.
  it("separates Runs and Templates and reloads a template editor route", async () => {
    renderPipelines("/pipelines");
    expect(await screen.findByRole("heading", { name: "Runs", level: 2 })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("link", { name: "Templates" }));
    expect(await screen.findByRole("heading", { name: "Templates", level: 2 })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Start run" })).not.toBeInTheDocument();
    fireEvent.click(await screen.findByRole("link", { name: /Delivery/ }));
    expect(await screen.findByRole("heading", { name: "Delivery", level: 2 })).toBeInTheDocument();
    expect(screen.queryByText("Operational ledger")).not.toBeInTheDocument();
  });

  it("redirects a legacy selected-run link to the run page", async () => {
    renderPipelines("/pipelines?run=run_legacy");
    expect(await screen.findByText("Run route")).toBeInTheDocument();
  });
});
