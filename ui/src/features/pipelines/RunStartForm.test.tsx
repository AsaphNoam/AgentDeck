import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { RunStartForm } from "./RunStartForm";

const template = {
  version: 2,
  title: "Delivery",
  orchestrator_role: "implementer",
  inputs: [],
  stages: [
    { id: "work", title: "Work", objective: "Do the work.", coordination: "standing", inputs: [], outputs: [] },
    { id: "design", title: "Design", objective: "Design the work.", coordination: "dedicated", dedicated_role: "designer", inputs: [], outputs: [] },
    { id: "review", title: "Review", objective: "Review the work.", coordination: "dedicated", dedicated_role: "reviewer", inputs: [], outputs: [] },
  ],
};

let starts: unknown[] = [];

const server = setupServer(
  http.get("/api/pipelines", () => HttpResponse.json([{ id: "delivery", template, valid: true, diagnostics: [] }])),
  http.get("/api/projects", () => HttpResponse.json({ app: { title: "App", cwd: "/tmp/app", color: [1, 2, 3], add_dirs: [], context_prompt: "" } })),
  http.get("/api/backends", () => HttpResponse.json({
    version: 2,
    backends: {
      codex: {
        name: "Codex", type: "codex-acp", default: true, default_model: "gpt-5.6-sol",
        models: { "gpt-5.6-sol": { name: "GPT-5.6-Sol", model: "gpt-5.6-sol", fast: true } },
      },
      alternate: {
        name: "Alternate", type: "codex-acp", default: false, default_model: "opus",
        models: { opus: { name: "Opus", model: "opus", fast: true, efforts: ["low", "high"], default_effort: "high" } },
      },
    },
  })),
  http.get("/api/config", () => HttpResponse.json({ default_project: "app" })),
  http.post("/api/pipeline-runs", async ({ request }) => {
    starts.push(await request.json());
    return HttpResponse.json({
    error: {
      code: "validation_failed",
      message: "run cannot start",
      details: {
        diagnostics: [{ field: "orchestrator", code: "unavailable", message: "unknown model \"gpt-5.6-sol\"" }],
      },
    },
    }, { status: 422 });
  }),
);

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
  afterEach(() => {
  cleanup();
  starts = [];
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

    expect(await screen.findByText("orchestrator")).toBeInTheDocument();
    expect(screen.getByText(/unknown model "gpt-5.6-sol"/)).toBeInTheDocument();
    expect(screen.getAllByLabelText("Backend")[0]).toHaveFocus();
  });

  // FS-14.A15: a disabled Review names the value it is still waiting for, and the
  // empty required input is marked, so the gate is not silent.
  it("names the missing required input while Next stays disabled", async () => {
    server.use(http.get("/api/pipelines", () => HttpResponse.json([{
      id: "delivery",
      template: { ...template, inputs: [{ name: "spec_url", description: "Where the spec lives", required: true }] },
      valid: true,
      diagnostics: [],
    }])));
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    const { container } = render(<QueryClientProvider client={client}><RunStartForm stepMode onCancel={() => {}} onStarted={() => {}} /></QueryClientProvider>);

    await screen.findByRole("option", { name: "Delivery (delivery)" });
    fireEvent.change(screen.getByLabelText("Template"), { target: { value: "delivery" } });
    fireEvent.change(screen.getByLabelText("Run goal"), { target: { value: "Ship it" } });

    expect(await screen.findByText("Fill the required named input: spec_url")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Review" })).toBeDisabled();
    expect(container.querySelector('[data-field="inputs.spec_url"]')).toHaveClass("pipeline-field-missing");

    fireEvent.change(screen.getByLabelText(/spec_url/), { target: { value: "https://example.invalid/spec" } });
    await waitFor(() => expect(screen.getByRole("button", { name: "Review" })).toBeEnabled());
    expect(screen.queryByText(/Fill the required named input/)).not.toBeInTheDocument();
    expect(container.querySelector('[data-field="inputs.spec_url"]')).not.toHaveClass("pipeline-field-missing");
  });

  it("reaches Review with configured defaults and submits every coordinator assignment", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    render(<QueryClientProvider client={client}><RunStartForm stepMode onCancel={() => {}} onStarted={() => {}} /></QueryClientProvider>);

    await screen.findByRole("option", { name: "Delivery (delivery)" });
    fireEvent.change(screen.getByLabelText("Template"), { target: { value: "delivery" } });
    fireEvent.change(screen.getByLabelText("Run goal"), { target: { value: "Ship it" } });
    await waitFor(() => expect(screen.getByRole("button", { name: "Review" })).toBeEnabled());
    expect(screen.getByText("Configured defaults")).toBeInTheDocument();
    expect(screen.getByText("Design coordinator · designer")).toBeInTheDocument();
    expect(screen.getByText("Review coordinator · reviewer")).toBeInTheDocument();
    expect(screen.getByText("Customize runtimes").closest("details")).not.toHaveAttribute("open");

    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    expect(screen.getByRole("heading", { name: "Delivery" })).toBeInTheDocument();
    expect(screen.getAllByText(/Codex \(codex\).*GPT-5.6-Sol/)).toHaveLength(3);
    fireEvent.click(screen.getByRole("button", { name: "Start run" }));
    await waitFor(() => expect(starts).toHaveLength(1));
    expect(starts[0]).toMatchObject({
      template_id: "delivery",
      orchestrator: { backend: "codex", model: "gpt-5.6-sol", fast: false },
      dedicated_assignments: {
        design: { backend: "codex", model: "gpt-5.6-sol", fast: false },
        review: { backend: "codex", model: "gpt-5.6-sol", fast: false },
      },
    });
  });

  it("keeps customized runtimes and fast mode through disclosure toggles and review", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    render(<QueryClientProvider client={client}><RunStartForm stepMode onCancel={() => {}} onStarted={() => {}} /></QueryClientProvider>);

    await screen.findByRole("option", { name: "Delivery (delivery)" });
    fireEvent.change(screen.getByLabelText("Template"), { target: { value: "delivery" } });
    fireEvent.change(screen.getByLabelText("Run goal"), { target: { value: "Ship it" } });
    fireEvent.click(screen.getByText("Customize runtimes"));
    const backendFields = screen.getAllByLabelText("Backend");
    fireEvent.change(backendFields[0], { target: { value: "alternate" } });
    await waitFor(() => expect(screen.getAllByLabelText("Model")[0]).toHaveValue("opus"));
    fireEvent.click(screen.getAllByRole("checkbox")[0]);

    const disclosure = screen.getByText("Customize runtimes").closest("details");
    expect(disclosure).not.toBeNull();
    fireEvent.click(screen.getByText("Customize runtimes"));
    expect(disclosure).not.toHaveAttribute("open");
    expect(screen.getByText(/Alternate \(alternate\).*Opus \(opus\).*Fast mode on/)).toBeInTheDocument();
    fireEvent.click(screen.getByText("Customize runtimes"));
    expect(screen.getAllByRole("checkbox")[0]).toBeChecked();

    fireEvent.change(screen.getByLabelText("Run display name"), { target: { value: "Release train" } });
    await waitFor(() => expect(screen.getByRole("button", { name: "Review" })).toBeEnabled());
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    expect(screen.getByRole("heading", { name: "Release train" })).toBeInTheDocument();
    expect(screen.getByText(/Alternate \(alternate\).*Opus \(opus\).*Fast mode on/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(screen.getByLabelText("Run display name")).toHaveValue("Release train");
    expect(screen.getByLabelText("Run goal")).toHaveValue("Ship it");
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    fireEvent.click(screen.getByRole("button", { name: "Start run" }));
    await waitFor(() => expect(starts).toHaveLength(1));
    expect(starts[0]).toMatchObject({ orchestrator: { backend: "alternate", model: "opus", fast: true } });
  });

  it("keeps proposal runtimes exact while customizing is toggled and the catalog refreshes", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    const proposal = {
      proposal_id: "proposal-1", digest: "digest-1", created_at: "2026-09-26T00:00:00Z", kind: "start_run" as const,
      payload: {
        request_id: "proposal-request", template_id: "delivery", display_name: "Proposed run", project: "app", goal: "Ship it", inputs: {},
        orchestrator: { backend: "alternate", model: "opus", effort: "high", fast: true },
        dedicated_assignments: {
          design: { backend: "codex", model: "gpt-5.6-sol", effort: "", fast: false },
          review: { backend: "alternate", model: "opus", effort: "low", fast: false },
        },
      },
    };
    render(<QueryClientProvider client={client}><RunStartForm stepMode proposal={proposal} onCancel={() => {}} onStarted={() => {}} /></QueryClientProvider>);

    expect(await screen.findByText("Proposal selections")).toBeInTheDocument();
    expect(screen.getByText(/Alternate \(alternate\).*Opus \(opus\).*high.*Fast mode on/)).toBeInTheDocument();
    fireEvent.click(screen.getByText("Customize runtimes"));
    expect(screen.getAllByLabelText("Backend")[0]).toHaveValue("alternate");
    expect(screen.getAllByLabelText("Model")[0]).toHaveValue("opus");

    client.setQueryData(["backends"], {
      version: 2,
      backends: {
        codex: { name: "Updated Codex", type: "codex-acp", default: false, default_model: "gpt-5.6-sol", models: { "gpt-5.6-sol": { name: "Updated GPT", model: "gpt-5.6-sol", fast: true } } },
        alternate: { name: "Updated Alternate", type: "codex-acp", default: true, default_model: "opus", models: { opus: { name: "Updated Opus", model: "opus", fast: true, efforts: ["low", "high"], default_effort: "high" } } },
      },
    });
    expect(await screen.findAllByText(/Updated Alternate \(alternate\).*Updated Opus \(opus\)/)).toHaveLength(2);
    expect(screen.getAllByLabelText("Backend")[0]).toHaveValue("alternate");
    expect(screen.getAllByLabelText("Model")[0]).toHaveValue("opus");
    expect(screen.getAllByRole("checkbox")[0]).toBeChecked();
    fireEvent.click(screen.getByText("Customize runtimes"));
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    expect(screen.getByRole("button", { name: "Confirm and start exact proposal" })).toBeEnabled();
  });
});
