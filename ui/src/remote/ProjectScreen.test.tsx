import React from "react";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { ProjectScreen } from "./ProjectScreen";
import { useConnection } from "./connection";
import type { AgentState } from "../api/types";

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
  http.post("/api/projects/:project/groups/:action", async ({ params, request }) => {
    groupActions.push({ project: params.project, action: params.action, body: await request.json() });
    return HttpResponse.json({ project: params.project, group: "Alpha", results: [{ agent_id: "a", ok: true }, { agent_id: "b", ok: true }] });
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

const phoneAgent = (agent_id: string, group: string | undefined, running: boolean): AgentState => ({
  agent_id, name: agent_id, role: "implementer", project: "app", backend: "codex", model: "preferred", fast: false,
  fast_available: false, steering_available: false, interface: "chat", group, created_at: "", running,
  state: running ? "busy" : "idle", detail: "", context_pct: 0, updated_at: 0, archived: false,
});

let launches: unknown[] = [];
let starts: unknown[] = [];
let launchStatus = 201;
let startStatus = 201;
let groupActions: unknown[] = [];

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
beforeEach(() => {
  useConnection.setState({ link: "connected", revision: 0, agents: {} });
  window.history.replaceState(null, "", "/");
  launches = [];
  starts = [];
  groupActions = [];
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
    const opener = await screen.findByRole("button", { name: "New agent" });
    opener.focus();
    fireEvent.click(opener);
    const role = await screen.findByRole("combobox", { name: "Role" });
    await waitFor(() => expect(role).toHaveValue("reviewer"));
    expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue("Reviewer");
    expect(screen.getByRole("combobox", { name: "Model" })).toHaveValue("preferred");
    fireEvent.change(screen.getByRole("textbox", { name: "Name" }), { target: { value: "Remote reviewer" } });
    expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue("Remote reviewer");
  });

  it("returns focus to the New agent opener after dismissal", async () => {
    renderScreen();
    const opener = await screen.findByRole("button", { name: "New agent" });
    opener.focus();
    fireEvent.click(opener);
    fireEvent.keyDown(await screen.findByRole("dialog", { name: "New agent" }), { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "New agent" })).toBeNull());
    expect(document.activeElement).toBe(opener);
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

describe("ProjectScreen groups", () => {
  it("projects collapsible groups with counts, running-first order, and Ungrouped last", async () => {
    useConnection.setState({ agents: {
      z: phoneAgent("z", undefined, false),
      b: phoneAgent("b", "Beta", false),
      a: phoneAgent("a", "Alpha", true),
      c: phoneAgent("c", "Alpha", false),
    } });
    renderScreen();
    expect(await screen.findByRole("region", { name: "Alpha group" })).toBeInTheDocument();
    expect([...document.querySelectorAll(".phone-agent-group")].map((node) => node.textContent?.split(" agents")[0])).toHaveLength(3);
    const sections = [...document.querySelectorAll<HTMLElement>(".phone-agent-group")];
    expect(sections.map((section) => section.querySelector("strong")?.textContent)).toEqual(["Alpha", "Beta", "Ungrouped"]);
    expect(within(sections[0]).getAllByRole("button").map((button) => button.textContent).join(" ")).toContain("a");
    fireEvent.click(within(sections[0]).getByRole("button", { name: /Alpha/ }));
    expect(within(sections[0]).queryByText("a")).toBeNull();
    act(() => useConnection.setState({ agents: { ...useConnection.getState().agents, d: phoneAgent("d", "Alpha", true) } }));
    expect(await within(sections[0]).findByText(/2 running/)).toBeInTheDocument();
  });

  it("sends explicit existing, new, and blank group values and retains a refused launch", async () => {
    useConnection.setState({ agents: { existing: phoneAgent("existing", "Existing", true) } });
    renderScreen();
    fireEvent.click(await screen.findByRole("button", { name: "New agent" }));
    const picker = await screen.findByRole("combobox", { name: "Group" });
    fireEvent.focus(picker);
    fireEvent.click(screen.getByRole("button", { name: "Existing" }));
    fireEvent.click(screen.getByRole("button", { name: "Create agent" }));
    await waitFor(() => expect(launches[0]).toEqual(expect.objectContaining({ group: "Existing" })));

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    launchStatus = 400;
    fireEvent.click(screen.getByRole("button", { name: "New agent" }));
    const secondPicker = await screen.findByRole("combobox", { name: "Group" });
    fireEvent.change(secondPicker, { target: { value: "Fresh" } });
    fireEvent.click(screen.getByRole("button", { name: 'Create group “Fresh”' }));
    fireEvent.click(screen.getByRole("button", { name: "Create agent" }));
    expect(await screen.findByText("unknown model")).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Group" })).toHaveValue("Fresh");
    fireEvent.focus(screen.getByRole("combobox", { name: "Group" }));
    fireEvent.click(screen.getByRole("button", { name: "Ungrouped" }));
    fireEvent.click(screen.getByRole("button", { name: "Create agent" }));
    await waitFor(() => expect(launches[2]).not.toHaveProperty("group"));
    expect(screen.getByRole("combobox", { name: "Group" })).toHaveValue("");
  });

  it("keeps scoped group action feedback after members disappear", async () => {
    useConnection.setState({ agents: { a: phoneAgent("a", "Alpha", true), b: phoneAgent("b", "Alpha", false) } });
    renderScreen();
    const section = await screen.findByRole("region", { name: "Alpha group" });
    fireEvent.click(within(section).getByRole("button", { name: "Archive group" }));
    const dialog = await screen.findByRole("dialog", { name: /Archive group/ });
    expect(dialog).toHaveTextContent("2 members · 1 running");
    fireEvent.click(within(dialog).getByRole("button", { name: "Archive group" }));
    await waitFor(() => expect(screen.getByRole("status", { name: "Group action result" })).toHaveTextContent("2 succeeded"));
    expect(groupActions).toEqual([{ project: "app", action: "archive", body: { group: "Alpha" } }]);
    act(() => useConnection.setState({ agents: {} }));
    expect(screen.getByRole("status", { name: "Group action result" })).toBeInTheDocument();
  });
});
