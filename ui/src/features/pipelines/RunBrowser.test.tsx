import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import type { AgentState } from "../../api/types";
import { useAgentStore } from "../../store/agentStore";
import { RunBrowser } from "./RunBrowser";

const template = {
  version: 1,
  title: "Delivery",
  inputs: [],
  stages: [{
    id: "work", title: "Work", role: "implementer", instruction: "Do the work.", inputs: [], outputs: [], max_visits: 2,
    transitions: {
      success: { final: "success", approval: "automatic" },
      failure: { final: "failure", approval: "required" },
    },
  }],
};

function attempt(attemptID: string, agentID: string, attemptNo: number, state: string) {
  return {
    attempt_id: attemptID, run_id: "run_1", stage_id: "work", attempt_no: attemptNo, visit_no: attemptNo,
    agent_id: agentID, agent_generation: `gen-${attemptNo}`, backend: "codex", model: "gpt-5.6-sol",
    state, assignment_text: "Do the work.", assignment_hash: "hash", assignment_version: 1,
    report_outputs: {}, created_at: "2026-07-26T00:00:00Z", updated_at: "2026-07-26T00:00:00Z",
  };
}

const run = {
  run_id: "run_1", template_id: "delivery", template_snapshot: template, display_name: "Ship",
  project: "app", goal: "Ship it", inputs: {}, assignments: { work: { backend: "codex", model: "gpt-5.6-sol" } },
  state: "running", revision: 4, pending_action: "await_result", current_stage_id: "work",
  current_attempt_id: "at_2", current_agent_id: "a_live", attention_reason: "", final_outcome: "",
  created_at: "2026-07-26T00:00:00Z", updated_at: "2026-07-26T00:00:00Z",
};

const server = setupServer(
  http.get("/api/pipeline-runs", () => HttpResponse.json([{
    run_id: "run_1", template_id: "delivery", display_name: "Ship", project: "app", state: "running",
    revision: 4, pending_action: "await_result", current_stage_id: "work", current_agent_id: "a_live",
    attention_reason: "", final_outcome: "", updated_at: "2026-07-26T00:00:00Z", diagnostics: [],
  }])),
  http.get("/api/pipeline-runs/run_1", () => HttpResponse.json({
    run,
    template,
    inputs: {},
    assignments: { work: { backend: "codex", model: "gpt-5.6-sol" } },
    attempts: [attempt("at_1", "a_stopped", 1, "crashed"), attempt("at_2", "a_live", 2, "running")],
    values: [],
    diagnostics: [],
  })),
);

function agent(id: string, running: boolean): AgentState {
  return {
    agent_id: id, name: id, role: "implementer", project: "app", backend: "codex", model: "gpt-5.6-sol",
    interface: "chat", created_at: "2026-07-26T00:00:00Z", running, state: running ? "busy" : "error",
    detail: "", context_pct: 0, updated_at: 1,
  };
}

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => {
  cleanup();
  server.resetHandlers();
  useAgentStore.setState({ agents: {}, order: [], hydrated: false, hydrating: false });
});
afterAll(() => server.close());

// FS-14.R8/R11: a stopped attempt's transcript lives in the Archive. Hydration
// keeps a stopped agent's identity row, so presence in the agent store must not
// be read as liveness — that routed every finished attempt to "Agent not found".
describe("RunBrowser attempt transcripts", () => {
  it("routes a stopped attempt to Archive and a running attempt to the live route", async () => {
    useAgentStore.setState({
      agents: { a_stopped: agent("a_stopped", false), a_live: agent("a_live", true) },
      order: ["a_stopped", "a_live"], hydrated: true, hydrating: false,
    });
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });

    render(
      <QueryClientProvider client={client}>
        <MemoryRouter><RunBrowser selectedID="run_1" onSelect={() => {}} /></MemoryRouter>
      </QueryClientProvider>,
    );

    const links = await screen.findAllByRole("link", { name: "Open transcript" });
    expect(links.map((link) => link.getAttribute("href"))).toEqual(["/archive/a_stopped", "/agent/a_live"]);
  });
});
