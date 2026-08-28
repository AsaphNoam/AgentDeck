import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { RunStartForm } from "./RunStartForm";

const template = {
  version: 1,
  title: "Delivery",
  inputs: [],
  stages: [{
    id: "work", title: "Work", role: "implementer", instruction: "Do the work.", inputs: [], outputs: [], max_visits: 1,
    transitions: {
      success: { final: "success", approval: "automatic" },
      failure: { final: "failure", approval: "required" },
    },
  }],
};

const server = setupServer(
  http.get("/api/pipelines", () => HttpResponse.json([{ id: "delivery", template, valid: true, diagnostics: [] }])),
  http.get("/api/projects", () => HttpResponse.json({ app: { title: "App", cwd: "/tmp/app", color: [1, 2, 3], add_dirs: [], context_prompt: "" } })),
  http.get("/api/backends", () => HttpResponse.json({
    version: 2,
    backends: {
      codex: {
        name: "Codex", type: "codex-acp", default: true, default_model: "gpt-5.6-sol",
        models: { "gpt-5.6-sol": { name: "GPT-5.6-Sol", model: "gpt-5.6-sol" } },
      },
    },
  })),
  http.get("/api/config", () => HttpResponse.json({ default_project: "app" })),
  http.post("/api/pipeline-runs", () => HttpResponse.json({
    error: {
      code: "validation_failed",
      message: "run cannot start",
      details: {
        diagnostics: [{ field: "assignments.work", code: "unavailable", message: "unknown model \"gpt-5.6-sol\"" }],
      },
    },
  }, { status: 422 })),
);

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => {
  cleanup();
  server.resetHandlers();
});
afterAll(() => server.close());

describe("RunStartForm", () => {
  it("renders field diagnostics from a rejected run start", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    render(<QueryClientProvider client={client}><RunStartForm onStarted={() => {}} /></QueryClientProvider>);

    await screen.findByRole("option", { name: "Delivery (delivery)" });
    fireEvent.change(screen.getByLabelText("Template"), { target: { value: "delivery" } });
    fireEvent.change(screen.getByLabelText("Run goal"), { target: { value: "Ship it" } });
    const start = screen.getByRole("button", { name: "Start run" });
    await waitFor(() => expect(start).toBeEnabled());
    fireEvent.click(start);

    expect(await screen.findByText("assignments.work")).toBeInTheDocument();
    expect(screen.getByText(/unknown model "gpt-5.6-sol"/)).toBeInTheDocument();
    expect(screen.getByLabelText("Backend")).toHaveFocus();
  });

  // FS-14.A20/A21: the dialog sequence preserves the draft while moving
  // through one stable Setup → Runtimes → Review flow.
  it("preserves setup values across the three start steps", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    render(<QueryClientProvider client={client}><RunStartForm stepMode onCancel={() => {}} onStarted={() => {}} /></QueryClientProvider>);

    await screen.findByRole("option", { name: "Delivery (delivery)" });
    fireEvent.change(screen.getByLabelText("Template"), { target: { value: "delivery" } });
    fireEvent.change(screen.getByLabelText("Run display name"), { target: { value: "Release train" } });
    fireEvent.change(screen.getByLabelText("Run goal"), { target: { value: "Ship it" } });
    await waitFor(() => expect(screen.getByRole("button", { name: "Next" })).toBeEnabled());
    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    expect(screen.getByText("Stage runtimes")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    expect(screen.getByRole("heading", { name: "Release train" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(screen.getByLabelText("Run display name")).toHaveValue("Release train");
    expect(screen.getByLabelText("Run goal")).toHaveValue("Ship it");
  });
});
