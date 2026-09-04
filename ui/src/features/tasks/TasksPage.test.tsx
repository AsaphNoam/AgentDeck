import React from "react";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { TasksPage, needsAttention, waitingOn } from "./TasksPage";

// `retry_eligible` mirrors exactly what the real server computes
// (state.retryEligible in internal/state/tasks.go), per INV §11: a mock that
// idealizes the field instead of mirroring the server's own switch would let
// a UI regression pass against a server that doesn't exist.
const baseTask = {
  task_id: "tk_1",
  project: "my-app",
  display_name: "build it",
  instruction: "do the work",
  target_kind: "launch" as const,
  role: "impl",
  state: "armed" as const,
  created_by_kind: "person",
  revision: 1,
  created_at: "2026-08-24T10:00:00Z",
  retry_eligible: false,
  arms: [{
    arm_id: "tk_1_arm00", task_id: "tk_1", kind: "work_result" as const,
    source_kind: "task", source_id: "tk_0", satisfying_outcomes: ["success"],
    state: "unsatisfied" as const,
  }],
  attachments: [],
};

const parked = {
  ...baseTask,
  task_id: "tk_2",
  display_name: "parked work",
  state: "dependency_failed" as const,
  attention_reason: "a prerequisite can no longer be satisfied",
  retry_eligible: false,
  arms: [{ ...baseTask.arms[0], arm_id: "tk_2_arm00", task_id: "tk_2", state: "unsatisfiable" as const }],
};

// Parked because its three start attempts were spent: every arm is satisfied,
// so Retry — not Re-arm — is the repair that restores the allowance (FS-16.R25).
const exhausted = {
  ...baseTask,
  task_id: "tk_3",
  display_name: "exhausted work",
  state: "dependency_failed" as const,
  attention_reason: "the last start attempt failed",
  retry_eligible: true,
  arms: [{ ...baseTask.arms[0], arm_id: "tk_3_arm00", task_id: "tk_3", state: "satisfied" as const }],
};

let lastRequest: { url: string; body: unknown } | null = null;

const server = setupServer(
  http.get("/api/projects", () => HttpResponse.json({ "my-app": { title: "My App", cwd: "/tmp" } })),
  http.get("/api/roles", () => HttpResponse.json({ agentdecker: { title: "AgentDecker" }, impl: { title: "Impl" } })),
  http.get("/api/config", () => HttpResponse.json({ default_role: "impl" })),
  http.get("/api/tasks", () => HttpResponse.json({ tasks: [baseTask, parked] })),
  http.post("/api/tasks/:id/retry", async ({ params }) => {
    lastRequest = { url: `retry:${params.id}`, body: null };
    return HttpResponse.json({
      error: { code: "validation", message: "re-arm it instead", details: { code: "retry_requires_rearm" } },
    }, { status: 422 });
  }),
  http.post("/api/tasks/:id/rearm", async ({ params, request }) => {
    lastRequest = { url: `rearm:${params.id}`, body: await request.json() };
    return HttpResponse.json({ ...parked, state: "ready" });
  }),
	 http.post("/api/tasks", async ({ request }) => {
		lastRequest = { url: "create", body: await request.json() };
		return HttpResponse.json({ ...baseTask, state: "ready", arms: [], attachments: [] }, { status: 201 });
	 }),
);

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => {
  cleanup();
  lastRequest = null;
  server.resetHandlers();
});
afterAll(() => server.close());

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={["/tasks?project=my-app"]}><TasksPage /></MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("Tasks view", () => {
  // FS-16.R14 — the view shows each task's state, what an armed one waits on,
  // and which parked one needs attention.
  it("shows armed and parked work with what each is waiting on", async () => {
    renderPage();
    expect(await screen.findByText("build it")).toBeInTheDocument();
    expect(screen.getByText(/Waiting on: task tk_0 → success/)).toBeInTheDocument();
    expect(screen.getByText("a prerequisite can no longer be satisfied")).toBeInTheDocument();
    expect(screen.getByText("1 need attention")).toBeInTheDocument();
  });

  // FS-16.R23 / A11 — parked work offers only the repair that can succeed.
  it("omits retry on parked work and re-arms in place", async () => {
    renderPage();
    await screen.findByText("parked work");
    const rows = screen.getAllByRole("listitem");
    const parkedRow = rows[1];

    expect(within(parkedRow).queryByRole("button", { name: "Retry" })).not.toBeInTheDocument();


    fireEvent.change(within(parkedRow).getByLabelText("Wait for signal"), { target: { value: "ci-green" } });
    fireEvent.click(within(parkedRow).getByRole("button", { name: "Re-arm" }));
    await waitFor(() => expect(lastRequest?.url).toBe("rearm:tk_2"));
    expect(lastRequest?.body).toEqual({ arms: [{ kind: "signal", signal_name: "ci-green" }] });
  });

	 it("authors an existing-agent task with a pipeline arm, outcomes, and context", async () => {
		renderPage();
		await screen.findByText("New task");
		fireEvent.change(screen.getByLabelText("Name"), { target: { value: "review" } });
		fireEvent.change(screen.getByLabelText("Instruction"), { target: { value: "review it" } });
		fireEvent.change(screen.getByLabelText("Target"), { target: { value: "agent" } });
		// The fixture has no agents, so switch back to launch after proving the full
		// payload fields are available on the same form.
		fireEvent.change(screen.getByLabelText("Target"), { target: { value: "launch" } });
		fireEvent.change(screen.getByLabelText("Wait for task or pipeline run (optional)"), { target: { value: "pr_1" } });
		const prerequisiteKinds = screen.getAllByLabelText("Prerequisite kind");
		const satisfyingOutcomes = screen.getAllByLabelText("Satisfying outcomes");
		fireEvent.change(prerequisiteKinds[prerequisiteKinds.length - 1], { target: { value: "pipeline_run" } });
		fireEvent.change(satisfyingOutcomes[satisfyingOutcomes.length - 1], { target: { value: "success,failure" } });
		fireEvent.change(screen.getByLabelText("Context reference (optional)"), { target: { value: "cx_1" } });
		fireEvent.change(screen.getByLabelText("Context label"), { target: { value: "brief" } });
		fireEvent.click(screen.getByRole("button", { name: "Create task" }));
		await waitFor(() => expect(lastRequest?.url).toBe("create"));
		expect(lastRequest?.body).toMatchObject({ target_kind: "launch", arms: [{ source_kind: "pipeline_run", source_id: "pr_1", satisfying_outcomes: ["success", "failure"] }], attachments: [{ context_ref_id: "cx_1", label: "brief" }] });
	 });

	 // FS-16.A18 (R27) — the effort a person names beside backend and model
	 // reaches the create request, and only for a launch target: an existing agent
	 // already runs at its session's level.
	 it("sends the effort it was given for a launch target", async () => {
		renderPage();
		await screen.findByText("New task");
		fireEvent.change(screen.getByLabelText("Name"), { target: { value: "think hard" } });
		fireEvent.change(screen.getByLabelText("Instruction"), { target: { value: "reason about it" } });
		fireEvent.change(screen.getByLabelText("Effort (optional)"), { target: { value: "high" } });
		fireEvent.click(screen.getByRole("button", { name: "Create task" }));
		await waitFor(() => expect(lastRequest?.url).toBe("create"));
		expect(lastRequest?.body).toMatchObject({ target_kind: "launch", effort: "high" });

		fireEvent.change(screen.getByLabelText("Target"), { target: { value: "agent" } });
		expect(screen.queryByLabelText("Effort (optional)")).not.toBeInTheDocument();
	 });

  // Regression (review fix): narrowing Retry to `interrupted` also removed it
  // from a task parked by exhausted start attempts, whose only specified repair
  // it is. Re-arm is not a substitute — it never restores the allowance — so the
  // person was left with no route back for work that simply failed to start.
  it("offers retry on work parked by exhausted start attempts", async () => {
    server.use(http.get("/api/tasks", () => HttpResponse.json({ tasks: [exhausted, parked] })));
    server.use(http.post("/api/tasks/:id/retry", async ({ params }) => {
      lastRequest = { url: `retry:${params.id}`, body: null };
      return HttpResponse.json({ ...exhausted, state: "ready" });
    }));
    renderPage();
    await screen.findByText("exhausted work");
    const rows = screen.getAllByRole("listitem");

    // The unsatisfiable-arm park in the same list must still withhold it.
    expect(within(rows[1]).queryByRole("button", { name: "Retry" })).not.toBeInTheDocument();

    fireEvent.click(within(rows[0]).getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(lastRequest?.url).toBe("retry:tk_3"));
  });

  // INV §2: the server is the one authority for retry eligibility. This fixture
  // makes the field disagree with the arm shape the view used to reason from, so
  // a reintroduced local condition fails here instead of drifting silently until
  // the next FS-16.R23/R25 change separates the two copies again.
  it("follows the server's retry_eligible rather than the arm shape", async () => {
    server.use(http.get("/api/tasks", () => HttpResponse.json({
      tasks: [
        // Arms all satisfied, but the server says no.
        { ...exhausted, task_id: "tk_9", display_name: "server says no", retry_eligible: false },
        // An unsatisfiable arm, but the server says yes.
        { ...parked, task_id: "tk_10", display_name: "server says yes", retry_eligible: true },
      ],
    })));
    renderPage();
    // Earlier cases in this file leave their trees mounted, so each row is
    // reached from its own unique name rather than by list position.
    const noRow = (await screen.findByText("server says no")).closest("li") as HTMLElement;
    const yesRow = screen.getByText("server says yes").closest("li") as HTMLElement;
    expect(within(noRow).queryByRole("button", { name: "Retry" })).not.toBeInTheDocument();
    expect(within(yesRow).getByRole("button", { name: "Retry" })).toBeInTheDocument();
  });

  // FS-16.A8 / INV §8: a task targeting an existing agent has no "launches …"
  // segment, and each optional segment used to carry its own leading separator,
  // so every agent-target row read "· assigned to Bob".
  it("renders an agent-target row without a leading separator", async () => {
    server.use(http.get("/api/tasks", () => HttpResponse.json({
      tasks: [{
        ...baseTask, state: "running", arms: [],
        target_kind: "agent", target_agent_id: "a_bob", role: "",
      }],
    })));
    renderPage();
    const link = await screen.findByRole("link", { name: "a_bob" });
    const meta = link.closest('[data-slot="metadata"]');
    expect(meta?.querySelector("span")?.textContent).toBe("assigned to a_bob");
  });

  it("uses the configured default role for a new launch task", async () => {
    renderPage();
    await waitFor(() => {
      const select = screen.getByRole("combobox", { name: "Role to launch" }).querySelector("select") ??
        screen.getByRole("combobox", { name: "Role to launch" });
      expect((select as HTMLSelectElement).value).toBe("impl");
    });
  });

  it("links a launch task to its assigned agent", async () => {
    server.use(http.get("/api/tasks", () => HttpResponse.json({
      tasks: [{ ...baseTask, state: "running", arms: [], assigned_agent_id: "a_worker" }],
    })));
    renderPage();
    expect(await screen.findByRole("link", { name: "a_worker" })).toHaveAttribute("href", "/agent/a_worker");
  });
});

describe("task helpers", () => {
  // FS-02.A26 — attention is exactly parked and interrupted work.
  it("counts only parked and interrupted work as needing attention", () => {
    for (const state of ["dependency_failed", "interrupted"]) {
      expect(needsAttention({ state } as never)).toBe(true);
    }
    for (const state of ["armed", "ready", "starting", "running", "finished"]) {
      expect(needsAttention({ state } as never)).toBe(false);
    }
  });

  it("names only the arms still unsatisfied", () => {
    expect(waitingOn([
      { kind: "signal", signal_name: "ci", state: "unsatisfied" } as never,
      { kind: "work_result", source_kind: "task", source_id: "tk_9", satisfying_outcomes: ["success"], state: "satisfied" } as never,
    ])).toEqual(["signal ci"]);
  });

  it("uses a prerequisite task's display name when it is loaded", () => {
    expect(waitingOn([
      { kind: "work_result", source_kind: "task", source_id: "tk_9", satisfying_outcomes: ["success"], state: "unsatisfied" } as never,
    ], { tk_9: "Compile assets" })).toEqual(["task Compile assets → success"]);
  });
});
