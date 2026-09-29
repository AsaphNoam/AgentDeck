import React from "react";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { RunScreen, TaskScreen } from "./WorkScreens";
import { NewWorkScreen } from "./NewWorkScreen";
import { useConnection } from "./connection";
import type { AgentState } from "../api/types";

const calls: string[] = [];
let taskState = "interrupted";

const task = () => ({
  task_id: "t1", project: "my-app", display_name: "Fix login", instruction: "x", target_kind: "launch", role: "implementer",
  fast: false, state: taskState, outcome: "", attention_reason: "The agent exited.", created_by_kind: "person", assigned_agent_id: "a9",
  pending_release: false, continuation_pending: false, pending_yield: false, wait_version: 0, resume_needed: false,
  cleanup_unsafe: false, start_attempt_count: 1, revision: 3, created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
  arms: [], retry_eligible: taskState === "interrupted", attachments: [],
});

const template = {
  version: 2, title: "Ship", orchestrator_role: "orchestrator",
  inputs: [{ name: "ticket", description: "Ticket id", required: true }],
  stages: [{ id: "s1", title: "Build", objective: "o", inputs: [], outputs: [] }, { id: "s2", title: "Review", objective: "o", inputs: [], outputs: [] }],
};
const runDetail = {
  run: {
    run_id: "r1", template_id: "ship", template_snapshot: template, display_name: "Ship it", project: "my-app", goal: "g", inputs: {},
    source_kind: "person", updated_at: "", created_at: "", state: "paused", revision: 7, pending_action: "", current_stage_id: "s2",
    current_attempt_id: "at1", current_agent_id: "", attention_reason: "Waiting for approval", final_outcome: "",
  },
  template, inputs: {}, stage_tasks: [], values: [], diagnostics: [],
  controls: {
    continue: { eligible: true }, retry: { eligible: false }, replace: { eligible: false }, stop: { eligible: true }, repair_cleanup: { eligible: false },
  },
};

const server = setupServer(
  http.get("/api/tasks/t1", () => HttpResponse.json(task())),
  http.post("/api/tasks/t1/:action", async ({ params, request }) => {
    calls.push(`task ${String(params.action)} ${await request.text()}`);
    if (params.action === "cancel") return HttpResponse.json({ error: { code: "task_not_cancellable", message: "a finished task cannot be cancelled" } }, { status: 409 });
    return HttpResponse.json(task());
  }),
  http.get("/api/pipeline-runs/r1", () => HttpResponse.json(runDetail)),
  http.post("/api/pipeline-runs/r1/continue", async ({ request }) => {
    calls.push(`continue ${await request.text()}`);
    return HttpResponse.json(runDetail);
  }),
  http.get("/api/projects", () => HttpResponse.json({ "my-app": { title: "My app" }, old: { title: "Old", archived: true } })),
  http.get("/api/roles", () => HttpResponse.json({ implementer: { title: "Implementer" }, agentdecker: { title: "AgentDecker" } })),
  http.get("/api/pipelines", () => HttpResponse.json([{ id: "ship", template, valid: true, diagnostics: [] }])),
  http.post("/api/sessions", async ({ request }) => {
    calls.push(`launch ${await request.text()}`);
    return HttpResponse.json({ agent: { agent_id: "new-ad", name: "" } });
  }),
  http.post("/api/sessions/:id/prompt", async ({ params, request }) => {
    calls.push(`prompt ${String(params.id)} ${await request.text()}`);
    return HttpResponse.json({ accepted: true, delivery: "sent" });
  }),
  http.post("/api/tasks", async ({ request }) => {
    calls.push(`create ${await request.text()}`);
    return HttpResponse.json({ ...task(), task_id: "t2" }, { status: 201 });
  }),
  http.post("/api/pipeline-runs", async ({ request }) => {
    calls.push(`start ${await request.text()}`);
    return HttpResponse.json({ run: runDetail, replay: false, workspace_conflicts: [] }, { status: 201 });
  }),
);

const resident: AgentState = {
  agent_id: "ad1", name: "", role: "agentdecker", project: "my-app", backend: "", model: "", fast: false, fast_available: false,
  steering_available: false, interface: "chat", created_at: "", running: true, state: "idle", detail: "", context_pct: 0, updated_at: 0, archived: false,
};

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
beforeEach(() => {
  calls.length = 0;
  taskState = "interrupted";
  useConnection.setState({ link: "connected", agents: {}, revision: 0 });
  window.history.replaceState(null, "", "/");
});
afterEach(() => {
  cleanup();
  server.resetHandlers();
});
afterAll(() => server.close());

function renderWith(node: React.ReactNode) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}>{node}</QueryClientProvider>);
}

describe("phone task and run actions", () => {
  it("retries an interrupted task and shows the Mac's refusal", async () => {
    renderWith(<TaskScreen taskId="t1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Retry" }));
    await waitFor(() => expect(calls).toContain("task retry {}"));
    fireEvent.click(screen.getByRole("button", { name: "Cancel task" }));
    expect(await screen.findByText("a finished task cannot be cancelled")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Open conversation" })).toBeInTheDocument();
  });

  it("re-arms a dependency-failed task with or without prerequisites", async () => {
    taskState = "dependency_failed";
    renderWith(<TaskScreen taskId="t1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Re-arm without prerequisites" }));
    await waitFor(() => expect(calls).toContain('task rearm {"arms":[]}'));
  });

  // FS-16.R22/R23: Record result only on running/interrupted; Re-arm only on
  // armed/ready/dependency_failed; Cancel on everything unfinished.
  it.each([
    ["armed", { rearm: true, record: false, cancel: true }],
    ["ready", { rearm: true, record: false, cancel: true }],
    ["starting", { rearm: false, record: false, cancel: true }],
    ["running", { rearm: false, record: true, cancel: true }],
    ["interrupted", { rearm: false, record: true, cancel: true }],
    ["dependency_failed", { rearm: true, record: false, cancel: true }],
    ["finished", { rearm: false, record: false, cancel: false }],
  ])("offers only the controls a %s task accepts", async (state, expected) => {
    taskState = state;
    renderWith(<TaskScreen taskId="t1" />);
    await screen.findByRole("heading", { name: "Fix login" });
    expect(!!screen.queryByRole("button", { name: "Re-arm with its prerequisites" })).toBe(expected.rearm);
    expect(!!screen.queryByRole("form", { name: "Record result" })).toBe(expected.record);
    expect(!!screen.queryByRole("button", { name: "Cancel task" })).toBe(expected.cancel);
  });

  it("continues a paused run with input at its stage position", async () => {
    renderWith(<RunScreen runId="r1" />);
    expect(await screen.findByText(/Stage 2 of 2/)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Input for the stage (optional)"), { target: { value: "approved" } });
    fireEvent.click(screen.getByRole("button", { name: "Continue" }));
    await waitFor(() => expect(calls).toContain('continue {"revision":7,"input":"approved"}'));
    expect(screen.queryByRole("button", { name: "Retry stage" })).toBeNull();
  });
});

describe("New work", () => {
  it("asks the running AgentDecker without launching another", async () => {
    useConnection.setState({ agents: { ad1: resident } });
    renderWith(<NewWorkScreen />);
    fireEvent.change(await screen.findByLabelText("Instruction"), { target: { value: "Plan the release" } });
    fireEvent.click(screen.getByRole("button", { name: "Send to AgentDecker" }));
    await waitFor(() => expect(calls).toContain('prompt ad1 {"text":"Plan the release"}'));
    expect(calls.some((c) => c.startsWith("launch"))).toBe(false);
    expect(window.location.pathname).toBe("/agent/ad1");
  });

  it("launches AgentDecker with the Mac's default runtime when none is running", async () => {
    renderWith(<NewWorkScreen />);
    fireEvent.change(await screen.findByLabelText("Instruction"), { target: { value: "Plan" } });
    fireEvent.click(screen.getByRole("button", { name: "Send to AgentDecker" }));
    await waitFor(() => expect(calls).toContain('launch {"role":"agentdecker","project":"my-app"}'));
    await waitFor(() => expect(calls).toContain('prompt new-ad {"text":"Plan"}'));
  });

  it("creates a task with only the phone form's fields", async () => {
    renderWith(<NewWorkScreen />);
    fireEvent.click(await screen.findByRole("radio", { name: "New task" }));
    fireEvent.change(screen.getByLabelText("Task name"), { target: { value: "Fix login" } });
    fireEvent.change(screen.getByLabelText("Instruction"), { target: { value: "Repair the form" } });
    fireEvent.click(screen.getByRole("button", { name: "Create task" }));
    await waitFor(() =>
      expect(calls).toContain('create {"project":"my-app","display_name":"Fix login","instruction":"Repair the form","target_kind":"launch","role":"implementer"}'),
    );
  });

  it("starts a pipeline run from a template once required inputs are filled", async () => {
    renderWith(<NewWorkScreen />);
    fireEvent.click(await screen.findByRole("radio", { name: "Start pipeline" }));
    fireEvent.change(await screen.findByLabelText("Run goal"), { target: { value: "Release 2" } });
    const start = screen.getByRole("button", { name: "Start run" });
    expect(start).toBeDisabled();
    fireEvent.change(screen.getByLabelText("ticket"), { target: { value: "T-1" } });
    fireEvent.click(start);
    await waitFor(() => expect(calls.some((c) => c.startsWith("start ") && c.includes('"inputs":{"ticket":"T-1"}') && c.includes('"goal":"Release 2"'))).toBe(true));
    expect(window.location.pathname).toBe("/run/r1");
  });
});
