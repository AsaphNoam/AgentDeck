import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { Link, MemoryRouter, Route, Routes } from "react-router-dom";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import type { AgentState } from "../../api/types";
import { useAgentStore } from "../../store/agentStore";
import { RunBrowser, RunsLedger } from "./RunBrowser";
import { PipelineRunPage } from "./PipelinesPage";

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

const detail = {
  run,
  template,
  inputs: {},
  assignments: { work: { backend: "codex", model: "gpt-5.6-sol" } },
  attempts: [attempt("at_1", "a_stopped", 1, "crashed"), attempt("at_2", "a_live", 2, "running")],
  values: [],
  diagnostics: [],
  agents_by_attempt: {
    at_1: { stage_agent: { agent_id: "a_stopped", name: "Stopped", running: false, state: "error", preview: "Stopped", route: "archive", available: true }, delegated_agents: [], delegated_total: 0, delegated_running_count: 0 },
    at_2: { stage_agent: { agent_id: "a_live", name: "Live", running: true, state: "busy", preview: "Working", route: "live", available: true }, delegated_agents: [], delegated_total: 0, delegated_running_count: 0 },
  },
};

const server = setupServer(
  http.get("/api/pipeline-runs", () => HttpResponse.json([{
    run_id: "run_1", template_id: "delivery", display_name: "Ship", project: "app", state: "running",
    revision: 4, pending_action: "await_result", current_stage_id: "work", current_agent_id: "a_live",
    attention_reason: "", final_outcome: "", updated_at: "2026-07-26T00:00:00Z", diagnostics: [],
  }])),
  http.get("/api/pipeline-runs/run_1", () => HttpResponse.json(detail)),
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

    const { container } = render(
      <QueryClientProvider client={client}>
        <MemoryRouter><RunBrowser selectedID="run_1" onSelect={() => {}} /></MemoryRouter>
      </QueryClientProvider>,
    );

    await screen.findByText("a_live");
    const links = [...container.querySelectorAll<HTMLAnchorElement>(".pipeline-agent-card")];
    expect(links.map((link) => link.getAttribute("href"))).toEqual(["/archive/a_stopped", "/agent/a_live"]);
  });

  // FS-14.A22: retained history is explicit and bounded without replacing
  // already-rendered pages during the next fetch.
  it("loads all retained run pages to an exact complete-history state", async () => {
    const retained = Array.from({ length: 121 }, (_, index) => ({
      run_id: `run_${String(index).padStart(3, "0")}`, template_id: "delivery", display_name: `Run ${index + 1}`,
      project: "app", state: "completed", revision: 1, pending_action: "", current_stage_id: "work",
      current_stage_title: "Frozen Work", current_agent_id: "", attention_reason: "", final_outcome: "success",
      updated_at: new Date(Date.UTC(2026, 6, 26, 0, 0, 121 - index)).toISOString(), diagnostics: [],
    }));
    server.use(http.get("/api/pipeline-runs", ({ request }) => {
      const url = new URL(request.url);
      const offset = Number(url.searchParams.get("offset") ?? 0);
      const limit = Number(url.searchParams.get("limit") ?? 50);
      return HttpResponse.json(retained.slice(offset, offset + limit), { headers: { "X-Total-Count": "121" } });
    }));
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    render(<QueryClientProvider client={client}><MemoryRouter><RunsLedger /></MemoryRouter></QueryClientProvider>);

    expect(await screen.findByText("50 of 121 retained runs")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "More runs" }));
    await waitFor(() => expect(screen.getByText("100 of 121 retained runs")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "More runs" }));
    expect(await screen.findByText("121 of 121 retained runs")).toBeInTheDocument();
    expect(screen.getByText("Complete history loaded")).toBeInTheDocument();
    expect(screen.getAllByText("Frozen Work")).toHaveLength(121);
  });
});

describe("RunDetail continuity and absence", () => {
  // FS-14.A21: only an attempt that arrives after the first render plays the
  // append transition; the attempts already on screen do not replay it.
  it("marks only the attempt appended after the first render", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    const { container } = render(
      <QueryClientProvider client={client}>
        <MemoryRouter><RunBrowser selectedID="run_1" /></MemoryRouter>
      </QueryClientProvider>,
    );

    await screen.findByRole("heading", { name: "Ship", level: 2 });
    expect(container.querySelectorAll('[data-slot="attempt"]')).toHaveLength(2);
    expect(container.querySelectorAll(".pipeline-timeline-appended")).toHaveLength(0);

    server.use(http.get("/api/pipeline-runs/run_1", () => HttpResponse.json({
      ...detail,
      attempts: [...detail.attempts, attempt("at_3", "a_live", 3, "running")],
    })));
    void client.invalidateQueries();

    await waitFor(() => expect(container.querySelectorAll('[data-slot="attempt"]')).toHaveLength(3));
    const appended = [...container.querySelectorAll(".pipeline-timeline-appended")];
    expect(appended).toHaveLength(1);
    expect(appended[0]).toBe(container.querySelectorAll('[data-slot="attempt"]')[2]);
  });

  // FS-14.R43: a deleted run explains itself in product language instead of
  // rendering the transport error string.
  it("explains a deleted run without the raw API message", async () => {
    server.use(http.get("/api/pipeline-runs/run_1", () => HttpResponse.json(
      { error: { code: "not_found", message: "pipeline resource not found" } },
      { status: 404 },
    )));
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter><RunBrowser selectedID="run_1" /></MemoryRouter>
      </QueryClientProvider>,
    );

    expect(await screen.findByText("This run is gone.")).toBeInTheDocument();
    expect(screen.queryByText("pipeline resource not found")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Return to Runs" })).toHaveAttribute("href", "/pipelines/runs");
  });
});

// FS-14.A17/A23: the delegated-agent cards shipped with no frontend test at all —
// every fixture here set `delegated_agents: []` — so a regression in card
// rendering, live/archive routing, the unavailable fallback, or the capped
// remaining-count line would have shipped silently.
describe("attempt agent cards", () => {
  const delegated = {
    ...detail,
    agents_by_attempt: {
      ...detail.agents_by_attempt,
      at_2: {
        stage_agent: { agent_id: "a_live", name: "Live", running: true, state: "busy", preview: "Working", route: "live", available: true },
        delegated_agents: [
          { agent_id: "a_task1", name: "Task one", running: true, state: "busy", preview: "Reviewing", route: "live", available: true, task_id: "tk_1", display_name: "Review the change", task_state: "running", outcome: "" },
          { agent_id: "a_task2", name: "Task two", running: false, state: "idle", preview: "Done", route: "archive", available: true, task_id: "tk_2", display_name: "Write the note", task_state: "finished", outcome: "success" },
          { agent_id: "a_gone", name: "a_gone", running: false, state: "unknown", preview: "", route: "unavailable", available: false, task_id: "tk_3", display_name: "Deleted work", task_state: "finished", outcome: "success" },
        ],
        delegated_total: 5,
        delegated_running_count: 2,
      },
    },
  };

  function renderDelegated() {
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    return render(
      <QueryClientProvider client={client}>
        <MemoryRouter><RunBrowser selectedID="run_1" /></MemoryRouter>
      </QueryClientProvider>,
    );
  }

  it("shows the stage agent beside its delegated agents and routes each by liveness", async () => {
    server.use(http.get("/api/pipeline-runs/run_1", () => HttpResponse.json(delegated)));
    const { container } = renderDelegated();

    await screen.findByText("Task one");
    // The first attempt carries its own agents section, so scope to the attempt
    // under test rather than to every card on the page.
    const cards = [...container.querySelectorAll('[data-slot="attempt"]')[1]
      .querySelectorAll(".pipeline-agent-card")];
    expect(cards).toHaveLength(4);
    expect(cards[0].textContent).toContain("Stage agent");
    expect(cards.slice(1).every((card) => card.textContent?.includes("Delegated work"))).toBe(true);
    expect(cards.slice(0, 3).map((card) => card.getAttribute("href")))
      .toEqual(["/agent/a_live", "/agent/a_task1", "/archive/a_task2"]);
  });

  it("renders one honest non-linking card for a delegated agent that is gone", async () => {
    server.use(http.get("/api/pipeline-runs/run_1", () => HttpResponse.json(delegated)));
    const { container } = renderDelegated();

    await screen.findByText("a_gone");
    const fallback = container.querySelector(".pipeline-agent-unavailable");
    expect(fallback).not.toBeNull();
    expect(fallback?.tagName).toBe("DIV");
    expect(fallback?.textContent).toContain("No recent activity");
  });

  it("states the distinct remaining count when the visible delegated cards are capped", async () => {
    server.use(http.get("/api/pipeline-runs/run_1", () => HttpResponse.json(delegated)));
    renderDelegated();

    await screen.findByText("Task one");
    expect(screen.getByText("2 delegated still running", { exact: false })).toBeInTheDocument();
    expect(screen.getByText("Showing 3 of 5 delegated", { exact: false })).toBeInTheDocument();
  });

  // A23's live-refresh half: a later state_update refreshes an available card in
  // place and moves it between the live and archive routes.
  it("refreshes a card in place from a later state_update", async () => {
    server.use(http.get("/api/pipeline-runs/run_1", () => HttpResponse.json(delegated)));
    const { container } = renderDelegated();

    await screen.findByText("Task two");
    const cardAt = (index: number) => container.querySelectorAll('[data-slot="attempt"]')[1]
      .querySelectorAll(".pipeline-agent-card")[index];
    expect(cardAt(2).getAttribute("href")).toBe("/archive/a_task2");

    useAgentStore.setState({
      agents: { a_task2: { ...agent("a_task2", true), name: "Task two renamed", detail: "Back at work" } },
      order: ["a_task2"], hydrated: true, hydrating: false,
    });

    await waitFor(() => {
      const card = cardAt(2);
      expect(card.getAttribute("href")).toBe("/agent/a_task2");
      expect(card.textContent).toContain("Task two renamed");
      expect(card.textContent).toContain("Back at work");
    });
  });
});

// FS-14.A16: a run that looped through its stages shows every visit as its own
// timeline entry in execution order, and an attempt that never reported says so.
describe("looping timeline", () => {
  it("shows each visit as its own entry and marks an unreported attempt", async () => {
    const stages = [
      ...template.stages,
      { id: "review", title: "Review", role: "reviewer", instruction: "Review it.", inputs: [], outputs: [], max_visits: 2, transitions: { success: { final: "success", approval: "automatic" }, failure: { final: "failure", approval: "required" } } },
    ];
    const looped = {
      ...detail,
      template: { ...template, stages },
      run: { ...run, template_snapshot: { ...template, stages } },
      attempts: [
        { ...attempt("at_1", "a_1", 1, "completed"), stage_id: "work", visit_no: 1, report_outcome: "success", report_summary: "Built it" },
        { ...attempt("at_2", "a_2", 2, "completed"), stage_id: "review", visit_no: 1, report_outcome: "failure", report_summary: "Needs a fix" },
        { ...attempt("at_3", "a_3", 3, "completed"), stage_id: "work", visit_no: 2, report_outcome: "success", report_summary: "Fixed it" },
        // Interrupted with no report: the entry must say no result was reported
        // rather than presenting the attempt as if it had finished cleanly.
        { ...attempt("at_4", "a_4", 4, "completed"), stage_id: "review", visit_no: 2 },
      ],
      agents_by_attempt: {},
    };
    server.use(http.get("/api/pipeline-runs/run_1", () => HttpResponse.json(looped)));
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    const { container } = render(
      <QueryClientProvider client={client}>
        <MemoryRouter><RunBrowser selectedID="run_1" /></MemoryRouter>
      </QueryClientProvider>,
    );

    await screen.findByText("4 attempts");
    const entries = [...container.querySelectorAll('[data-slot="attempt"]')];
    expect(entries).toHaveLength(4);
    expect(entries.map((entry) => entry.querySelector("strong")?.textContent))
      .toEqual(["Work", "Review", "Work", "Review"]);
    expect(entries.map((entry) => entry.querySelector("small")?.textContent))
      .toEqual([
        "Visit 1 · codex · gpt-5.6-sol",
        "Visit 1 · codex · gpt-5.6-sol",
        "Visit 2 · codex · gpt-5.6-sol",
        "Visit 2 · codex · gpt-5.6-sol",
      ]);
    expect(entries.slice(0, 3).map((entry) => entry.querySelector(".pipeline-state")?.textContent))
      .toEqual(["success", "failure", "success"]);
    expect(entries[3].querySelector(".pipeline-state")?.textContent).toBe("unreported");
    expect(entries[3].textContent).toContain("No stage result was reported for this attempt.");
  });
});

// INV §1: RunDetail holds a continuation draft, a mutation error, and the
// appended-attempts "seen" ref, none scoped to the run. Browser back/forward
// across two visited run pages reuses the route element, so without a key the
// previous run's unsent input stayed on screen against the new run.
describe("run page lifecycle boundary", () => {
  it("drops the previous run's unsent continuation when the route changes run", async () => {
    const blockedRun = {
      ...detail,
      run: { ...run, state: "paused", attention_reason: "blocked" },
      attempts: [{ ...attempt("at_2", "a_live", 2, "completed"), report_outcome: "blocked", report_summary: "Which fallback?" }],
    };
    server.use(
      http.get("/api/pipeline-runs/run_1", () => HttpResponse.json(blockedRun)),
      http.get("/api/pipeline-runs/run_2", () => HttpResponse.json({
        ...blockedRun,
        run: { ...blockedRun.run, run_id: "run_2", display_name: "Second" },
      })),
    );
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={["/pipelines/runs/run_1"]}>
          <Link to="/pipelines/runs/run_2">Go to the second run</Link>
          <Routes><Route path="/pipelines/runs/:runID" element={<PipelineRunPage />} /></Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    const box = await screen.findByRole("textbox");
    fireEvent.change(box, { target: { value: "use the cached copy" } });
    expect((box as HTMLTextAreaElement).value).toBe("use the cached copy");

    fireEvent.click(screen.getByRole("link", { name: "Go to the second run" }));

    await screen.findByRole("heading", { name: "Second", level: 2 });
    expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("");
  });
});

// FS-14.R48: a restart pause stopped the stage agent, Continue rejects that
// state, and an ordinary chat resume mints an unrelated generation whose report
// is refused forever — so Retry is the only route that moves the run. The run
// page used to offer Open agent there anyway, inviting the operator into a chat
// that leads nowhere.
describe("restart-recovery pause", () => {
  const recovered = {
    ...detail,
    run: { ...run, state: "paused", pending_action: "", attention_reason: "restart_recovery" },
  };

  function renderRun() {
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    return render(
      <QueryClientProvider client={client}>
        <MemoryRouter><RunBrowser selectedID="run_1" /></MemoryRouter>
      </QueryClientProvider>,
    );
  }

  it("withholds Open agent and names Retry as the route", async () => {
    server.use(http.get("/api/pipeline-runs/run_1", () => HttpResponse.json(recovered)));
    renderRun();

    await screen.findByRole("heading", { name: "Ship", level: 2 });
    expect(screen.queryByRole("link", { name: "Open agent" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry stage" })).toBeInTheDocument();
    expect(screen.getByText(/its chat can no longer report against this run/)).toBeInTheDocument();
  });

  it("still offers Open agent on an ordinary blocked pause", async () => {
    server.use(http.get("/api/pipeline-runs/run_1", () => HttpResponse.json({
      ...detail,
      run: { ...run, state: "paused", attention_reason: "blocked" },
      attempts: [{ ...attempt("at_2", "a_live", 2, "completed"), report_outcome: "blocked", report_summary: "Which fallback?" }],
    })));
    renderRun();

    await screen.findByRole("heading", { name: "Ship", level: 2 });
    expect(screen.getByRole("link", { name: "Open agent" })).toHaveAttribute("href", "/agent/a_live");
  });
});
