import React from "react";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { ProjectScreen } from "./ProjectScreen";
import { useConnection } from "./connection";

const server = setupServer(
  http.get("/api/projects", () => HttpResponse.json({ app: { title: "App", color: [1, 2, 3] } })),
  http.get("/api/remote/home", () => HttpResponse.json({ needs_you: [], active_runs: [] })),
  http.get("/api/roles", () => HttpResponse.json({ implementer: { title: "Implementer" }, reviewer: { title: "Reviewer" } })),
  http.get("/api/config", () => HttpResponse.json({ default_role: "reviewer" })),
  http.get("/api/remote/runtime-options", () => HttpResponse.json({ backends: [{ id: "codex", name: "Codex", default: true, default_model: "preferred", models: [{ id: "fallback", name: "Fallback", efforts: [], fast: false }, { id: "preferred", name: "Preferred", efforts: ["high"], default_effort: "high", fast: true }] }] })),
  http.post("/api/sessions", async ({ request }) => {
    launches.push(await request.json());
    if (launchStatus === 400) return HttpResponse.json({ error: { code: "invalid_request", message: "unknown model" } }, { status: 400 });
    return HttpResponse.json({ agent: { agent_id: "a9" } }, { status: 201 });
  }),
  http.get("/api/pipelines", () => HttpResponse.json([{ id: "delivery", template, valid: true, diagnostics: [] }])),
  http.post("/api/pipeline-runs", async ({ request }) => {
    starts.push(await request.json());
    if (startStatus === 422) return HttpResponse.json({ error: { code: "validation_failed", message: "run cannot start", details: { diagnostics: [{ field: "orchestrator", code: "unavailable", message: "unknown model" }] } } }, { status: 422 });
    return HttpResponse.json({ run: detail, replay: false, workspace_conflicts: [] }, { status: 201 });
  }),
);

const template = { version: 2, title: "Delivery", orchestrator_role: "implementer", inputs: [], stages: [{ id: "work", title: "Work", objective: "Do the work.", coordination: "standing", inputs: [], outputs: [] }] };
const controls = { continue: { eligible: false, reason: "" }, retry: { eligible: false, reason: "" }, replace: { eligible: false, reason: "" }, stop: { eligible: true, reason: "" }, repair_cleanup: { eligible: false, reason: "" } };
const run = { run_id: "run_1", template_id: "delivery", template_snapshot: template, display_name: "Delivery", project: "app", goal: "Ship it", inputs: {}, assignments: {}, orchestrator: { backend: "codex", model: "preferred" }, dedicated_assignments: {}, state: "running", revision: 1, pending_action: "", current_stage_id: "work", current_task_id: "", current_attempt_id: "", current_agent_id: "", attention_reason: "", final_outcome: "", created_at: "2026-10-05T00:00:00Z", updated_at: "2026-10-05T00:00:00Z" };
const detail = { run, template, inputs: {}, orchestrator: run.orchestrator, dedicated_assignments: {}, stage_tasks: [], assignments: {}, values: [], diagnostics: [], controls };

let launches: unknown[] = [];
let starts: unknown[] = [];
let launchStatus = 201;
let startStatus = 201;

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
beforeEach(() => {
  useConnection.setState({ link: "connected", revision: 0, agents: {} });
  window.history.replaceState(null, "", "/");
  launches = [];
  starts = [];
  launchStatus = 201;
  startStatus = 201;
});
afterEach(() => { cleanup(); server.resetHandlers(); });
afterAll(() => server.close());

function renderScreen() {
  render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><ProjectScreen projectID="app" /></QueryClientProvider>);
}

describe("ProjectScreen new agent", () => {
  it("uses the configured default role, an editable suggested name, and backend default model", async () => {
    renderScreen();
    fireEvent.click(await screen.findByRole("button", { name: "New agent" }));
    const role = await screen.findByRole("combobox", { name: "Role" });
    await waitFor(() => expect(role).toHaveValue("reviewer"));
    expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue("Reviewer");
    expect(screen.getByRole("combobox", { name: "Model" })).toHaveValue("preferred");
    fireEvent.change(screen.getByRole("textbox", { name: "Name" }), { target: { value: "Remote reviewer" } });
    expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue("Remote reviewer");
  });

  // FS-20.A10 — a non-default runtime reaches the launch request; a refusal keeps the form.
  async function fillLaunch() {
    renderScreen();
    fireEvent.click(await screen.findByRole("button", { name: "New agent" }));
    await waitFor(() => expect(screen.getByRole("combobox", { name: "Role" })).toHaveValue("reviewer"));
    fireEvent.change(screen.getByRole("textbox", { name: "Name" }), { target: { value: "Remote reviewer" } });
    fireEvent.change(screen.getByRole("combobox", { name: "Model" }), { target: { value: "fallback" } });
    fireEvent.change(screen.getByRole("combobox", { name: "Model" }), { target: { value: "preferred" } });
    fireEvent.change(screen.getByRole("combobox", { name: "Effort" }), { target: { value: "" } });
    fireEvent.click(screen.getByRole("checkbox", { name: "Fast mode" }));
    fireEvent.click(screen.getByRole("button", { name: "Create agent" }));
  }
  const launchBody = { project: "app", role: "reviewer", name: "Remote reviewer", backend: "codex", model: "preferred", effort: "", fast: true };

  it("launches with the chosen runtime and opens the new agent", async () => {
    await fillLaunch();
    await waitFor(() => expect(launches).toEqual([launchBody]));
    await waitFor(() => expect(window.location.pathname).toBe("/agent/a9"));
  });

  it("shows the Mac's reason and keeps entered values when the launch is refused", async () => {
    launchStatus = 400;
    await fillLaunch();
    expect(await screen.findByText("unknown model")).toBeInTheDocument();
    expect(launches).toEqual([launchBody]);
    expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue("Remote reviewer");
    expect(screen.getByRole("checkbox", { name: "Fast mode" })).toBeChecked();
    expect(window.location.pathname).toBe("/");
  });
});

// FS-20.A10 — Start pipeline from the project page starts a run in that project.
describe("ProjectScreen start pipeline", () => {
  async function fillStart() {
    renderScreen();
    fireEvent.click(await screen.findByRole("button", { name: "Start pipeline" }));
    fireEvent.change(await screen.findByRole("textbox", { name: "Run goal" }), { target: { value: "Ship it" } });
    fireEvent.click(screen.getByRole("dialog", { name: "Start pipeline" }).querySelector('button[type="submit"]')!);
  }

  it("starts a run in this project and opens it", async () => {
    await fillStart();
    await waitFor(() => expect(window.location.pathname).toBe("/run/run_1"));
    expect(starts).toEqual([expect.objectContaining({ template_id: "delivery", display_name: "Delivery", project: "app", goal: "Ship it", acknowledge_shared_workspace: false })]);
  });

  it("shows the Mac's reason and keeps the goal when the start is refused", async () => {
    startStatus = 422;
    await fillStart();
    expect(await screen.findByText(/run cannot start: unknown model/)).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Run goal" })).toHaveValue("Ship it");
    expect(window.location.pathname).toBe("/");
  });
});

// FS-20.R39 — archiving the project on the desktop reaches an open project screen.
describe("ProjectScreen archival", () => {
  it("shows the archived state after a desktop archive", async () => {
    let archived = false;
    server.use(http.get("/api/projects", () => HttpResponse.json({ app: { title: "App", color: [1, 2, 3], archived } })));
    render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><ProjectScreen projectID="app" /></QueryClientProvider>);
    expect(await screen.findByRole("button", { name: "New agent" })).toBeInTheDocument();
    archived = true;
    act(() => useConnection.setState({ revision: 1 }));
    expect(await screen.findByText(/Archived on the Mac/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "New agent" })).toBeNull();
  });
});
