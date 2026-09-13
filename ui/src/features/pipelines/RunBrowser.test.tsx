import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, expect, it } from "vitest";
import { RunBrowser, RunsLedger } from "./RunBrowser";

const template = { version: 2, title: "Delivery", orchestrator_role: "implementer", inputs: [], stages: [{ id: "work", title: "Work", objective: "Do the work.", coordination: "standing", inputs: [], outputs: [] }] };
const controls = { continue: { eligible: false, reason: "" }, retry: { eligible: false, reason: "" }, replace: { eligible: false, reason: "" }, stop: { eligible: true, reason: "" }, repair_cleanup: { eligible: false, reason: "" } };
const run = {
  run_id: "run_1", template_id: "delivery", template_snapshot: template, display_name: "Ship", project: "app", goal: "Ship it", inputs: {}, assignments: { standing: { backend: "codex", model: "gpt-5.6-sol" } }, orchestrator: { backend: "codex", model: "gpt-5.6-sol" }, dedicated_assignments: {}, state: "running", revision: 4, pending_action: "dispatch_stage_task", current_stage_id: "work", current_task_id: "task_1", current_attempt_id: "", current_agent_id: "a_live", attention_reason: "", final_outcome: "", created_at: "2026-07-26T00:00:00Z", updated_at: "2026-07-26T00:00:00Z",
};
const stageTask = {
  task_id: "task_1", run_id: "run_1", stage_id: "work", stage_index: 0, attempt_number: 1, state: "running", assignment_text: "Do the work.", standing_owner: { agent_id: "a_live", name: "Standing", state: "busy", route: "live", runtime: { backend: "codex", model: "gpt-5.6-sol" } }, work: [], created_at: "2026-07-26T00:00:00Z", updated_at: "2026-07-26T00:00:00Z",
};
const detail = { run, template, inputs: {}, assignments: run.assignments, orchestrator: run.orchestrator, dedicated_assignments: {}, stage_tasks: [stageTask], attempts: [], values: [], diagnostics: [], controls };
const server = setupServer(http.get("/api/pipeline-runs", () => HttpResponse.json([])), http.get("/api/pipeline-runs/run_1", () => HttpResponse.json(detail)));

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => { cleanup(); server.resetHandlers(); });
afterAll(() => server.close());

function renderRun() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
  return render(<QueryClientProvider client={client}><MemoryRouter><RunBrowser selectedID="run_1" /></MemoryRouter></QueryClientProvider>);
}

it("renders only durable stage-task provenance", async () => {
  renderRun();
  expect(await screen.findByText("Standing")).toBeInTheDocument();
  expect(screen.getByText("Task task_1")).toBeInTheDocument();
  expect(screen.getByText("1 task")).toBeInTheDocument();
});

it("requires recovery input when blocked continuation is eligible", async () => {
  server.use(http.get("/api/pipeline-runs/run_1", () => HttpResponse.json({ ...detail, run: { ...run, state: "paused" }, stage_tasks: [{ ...stageTask, state: "finished", result: { outcome: "blocked", summary: "Need a decision", details: "", checks: "", outputs: {} } }], controls: { ...controls, continue: { eligible: true, reason: "Recovery input starts a new standing-owner task." } } })));
  renderRun();
  expect(await screen.findByRole("button", { name: "Continue stage" })).toBeDisabled();
  fireEvent.change(screen.getByRole("textbox"), { target: { value: "use the cache" } });
  expect(screen.getByRole("button", { name: "Continue stage" })).toBeEnabled();
});

it("surfaces retained stage cleanup and its repair action", async () => {
  server.use(http.get("/api/pipeline-runs/run_1", () => HttpResponse.json({
    ...detail,
    run: { ...run, state: "finishing", pending_action: "release_stage_task" },
    stage_tasks: [{ ...stageTask, state: "finished", cleanup: { state: "release_needs_attention", reason: "permission denied stopping the runtime" } }],
    controls: { ...controls, repair_cleanup: { eligible: true, reason: "Retries only the retained cleanup effects." } },
  })));
  renderRun();
  expect(await screen.findByText((_, element) => element?.tagName === "P" && element.textContent === "Cleanup: release needs attention · permission denied stopping the runtime")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Repair cleanup" })).toBeInTheDocument();
});

it("loads retained run pages to an exact complete-history state", async () => {
  const retained = Array.from({ length: 51 }, (_, index) => ({ run_id: `run_${index}`, template_id: "delivery", display_name: `Run ${index + 1}`, project: "app", state: "completed", revision: 1, pending_action: "", current_stage_id: "work", current_stage_title: "Work", current_agent_id: "", attention_reason: "", final_outcome: "success", updated_at: new Date(Date.UTC(2026, 6, 26, 0, 0, 51 - index)).toISOString(), diagnostics: [] }));
  server.use(http.get("/api/pipeline-runs", ({ request }) => { const url = new URL(request.url); const offset = Number(url.searchParams.get("offset") ?? 0); const limit = Number(url.searchParams.get("limit") ?? 50); return HttpResponse.json(retained.slice(offset, offset + limit), { headers: { "X-Total-Count": "51" } }); }));
  const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
  render(<QueryClientProvider client={client}><MemoryRouter><RunsLedger /></MemoryRouter></QueryClientProvider>);
  expect(await screen.findByText("50 of 51 retained runs")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "More runs" }));
  expect(await screen.findByText("51 of 51 retained runs")).toBeInTheDocument();
});
import React from "react";
