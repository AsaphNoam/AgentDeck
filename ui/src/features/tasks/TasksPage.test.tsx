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

const runSummary = {
  run_id: "pr_1", template_id: "release", display_name: "Release", project: "my-app", state: "completed",
  revision: 1, pending_action: "", current_stage_id: "", current_stage_title: "", current_agent_id: "",
  attention_reason: "", final_outcome: "success", updated_at: "2026-08-24T10:00:00Z", diagnostics: [],
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
  http.get("/api/projects", () => HttpResponse.json({ "my-app": { title: "My App", cwd: "/tmp" }, other: { title: "Other", cwd: "/tmp/other" } })),
  http.get("/api/roles", () => HttpResponse.json({ agentdecker: { title: "AgentDecker" }, impl: { title: "Impl" } })),
  http.get("/api/config", () => HttpResponse.json({ default_role: "impl" })),
  http.get("/api/backends", () => HttpResponse.json({ version: 2, backends: {
    codex: { name: "Codex", type: "codex-acp", default: true, default_model: "gpt-5", models: { "gpt-5": { name: "GPT-5", model: "gpt-5", fast: true } } },
  } })),
  http.get("/api/tasks", ({ request }) => {
    const project = new URL(request.url).searchParams.get("project");
    return HttpResponse.json({ tasks: project === "other" ? [{ ...baseTask, task_id: "tk_other", project: "other" }] : [baseTask, parked] });
  }),
  http.get("/api/pipeline-runs", () => HttpResponse.json([runSummary], { headers: { "X-Total-Count": "1" } })),
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

function renderPage(initialEntry = "/tasks?project=my-app") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const result = render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[initialEntry]}><TasksPage /></MemoryRouter>
    </QueryClientProvider>,
  );
  return { ...result, client };
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
    const parkedRow = screen.getByText("parked work").closest("li") as HTMLElement;

    expect(within(parkedRow).queryByRole("button", { name: "Retry" })).not.toBeInTheDocument();


    fireEvent.click(within(parkedRow).getByText("Advanced signals"));
    fireEvent.change(within(parkedRow).getByLabelText("Wait for signal"), { target: { value: "ci-green" } });
    fireEvent.click(within(parkedRow).getByRole("button", { name: "Re-arm" }));
    await waitFor(() => expect(lastRequest?.url).toBe("rearm:tk_2"));
    expect(lastRequest?.body).toEqual({ arms: [{ kind: "signal", signal_name: "ci-green" }] });
  });

  it("authors an existing-agent task with a pipeline arm, outcomes, and context", async () => {
		renderPage();
		await screen.findByText("New task");
		const createSurface = within(screen.getByText("New task").parentElement as HTMLElement);
		fireEvent.change(screen.getByLabelText("Name"), { target: { value: "review" } });
		fireEvent.change(screen.getByLabelText("Instruction"), { target: { value: "review it" } });
		fireEvent.change(screen.getByLabelText("Target"), { target: { value: "agent" } });
		// The fixture has no agents, so switch back to launch after proving the full
		// payload fields are available on the same form.
		fireEvent.change(screen.getByLabelText("Target"), { target: { value: "launch" } });
		fireEvent.change(createSurface.getByLabelText("Source type"), { target: { value: "pipeline_run" } });
		await screen.findByRole("option", { name: "Release · completed · pr_1" });
		fireEvent.change(createSurface.getByLabelText("Named prerequisite"), { target: { value: "pr_1" } });
		fireEvent.click(createSurface.getByLabelText("Failure"));
		fireEvent.click(screen.getByText("Advanced signal and context"));
		fireEvent.change(createSurface.getByLabelText("Context reference ID"), { target: { value: "cx_1" } });
		fireEvent.change(createSurface.getByLabelText("Context label"), { target: { value: "brief" } });
		fireEvent.click(screen.getByRole("button", { name: "Create task" }));
		await waitFor(() => expect(lastRequest?.url).toBe("create"));
		expect(lastRequest?.body).toMatchObject({ target_kind: "launch", arms: [{ source_kind: "pipeline_run", source_id: "pr_1", satisfying_outcomes: ["success", "failure"] }], attachments: [{ context_ref_id: "cx_1", label: "brief" }] });
	 });

  it("disambiguates duplicate task names and submits the selected stable ID", async () => {
    server.use(http.get("/api/tasks", () => HttpResponse.json({ tasks: [
      { ...baseTask, display_name: "Build", task_id: "tk_a" },
      { ...parked, display_name: "Build", task_id: "tk_b" },
    ] })));
    renderPage();
    const create = within(screen.getByText("New task").parentElement as HTMLElement);
    const named = create.getByLabelText("Named prerequisite");
    await within(named).findByRole("option", { name: "Build · armed · tk_a" });
    expect(within(named).getByRole("option", { name: "Build · dependency_failed · tk_b" })).toBeInTheDocument();
    fireEvent.change(named, { target: { value: "tk_b" } });
    fireEvent.change(create.getByLabelText("Name"), { target: { value: "follow up" } });
    fireEvent.change(create.getByLabelText("Instruction"), { target: { value: "continue" } });
    fireEvent.click(create.getByRole("button", { name: "Create task" }));
    await waitFor(() => expect(lastRequest?.url).toBe("create"));
    expect(lastRequest?.body).toMatchObject({ arms: [{ kind: "work_result", source_kind: "task", source_id: "tk_b", satisfying_outcomes: ["success"] }] });
  });

  it("keeps an unavailable outcome visible when changing source type and blocks submission until corrected", async () => {
    renderPage();
    const create = within(screen.getByText("New task").parentElement as HTMLElement);
    await within(create.getByLabelText("Named prerequisite")).findByRole("option", { name: "parked work · dependency_failed · tk_2" });
    fireEvent.change(create.getByLabelText("Named prerequisite"), { target: { value: "tk_2" } });
    fireEvent.click(create.getByLabelText("Blocked"));
    fireEvent.change(create.getByLabelText("Source type"), { target: { value: "pipeline_run" } });
    expect(create.getByLabelText("blocked (unavailable for pipeline runs)")).toBeChecked();
    fireEvent.click(create.getByRole("button", { name: "Create task" }));
    expect(await create.findByRole("alert")).toHaveTextContent("Remove outcomes that are unavailable for pipeline runs.");
    expect(lastRequest).toBeNull();
  });

  it("loads a later global run page without treating project-filtered partial results as empty", async () => {
    const firstPage = Array.from({ length: 50 }, (_, index) => ({ ...runSummary, run_id: `pr_other_${index}`, project: "other" }));
    server.use(http.get("/api/pipeline-runs", ({ request }) => {
      const offset = Number(new URL(request.url).searchParams.get("offset") ?? 0);
      return HttpResponse.json(offset === 0 ? firstPage : [{ ...runSummary, run_id: "pr_page2" }], { headers: { "X-Total-Count": "51" } });
    }));
    renderPage();
    const create = within(screen.getByText("New task").parentElement as HTMLElement);
    fireEvent.change(create.getByLabelText("Source type"), { target: { value: "pipeline_run" } });
    expect(await create.findByText("No matching runs in the loaded pages yet.")).toBeInTheDocument();
    fireEvent.click(create.getByRole("button", { name: "Load more runs" }));
    await create.findByRole("option", { name: "Release · completed · pr_page2" });
    fireEvent.change(create.getByLabelText("Named prerequisite"), { target: { value: "pr_page2" } });
    fireEvent.change(create.getByLabelText("Name"), { target: { value: "after release" } });
    fireEvent.change(create.getByLabelText("Instruction"), { target: { value: "continue" } });
    fireEvent.click(create.getByRole("button", { name: "Create task" }));
    await waitFor(() => expect(lastRequest?.url).toBe("create"));
    expect(lastRequest?.body).toMatchObject({ arms: [{ kind: "work_result", source_kind: "pipeline_run", source_id: "pr_page2", satisfying_outcomes: ["success"] }] });
  });

  it("keeps manual IDs in the same selection and preserves the Create draft after refusal", async () => {
    server.use(http.post("/api/tasks", async ({ request }) => {
      lastRequest = { url: "create", body: await request.json() };
      return HttpResponse.json({ error: { code: "validation", message: "source is no longer available" } }, { status: 422 });
    }));
    renderPage();
    const create = within(screen.getByText("New task").parentElement as HTMLElement);
    fireEvent.change(create.getByLabelText("Name"), { target: { value: "manual follow-up" } });
    fireEvent.change(create.getByLabelText("Instruction"), { target: { value: "continue the work" } });
    fireEvent.click(create.getByText("Advanced", { selector: "summary" }));
    fireEvent.click(create.getByLabelText("Enter an ID manually"));
    fireEvent.change(create.getByLabelText("Source ID"), { target: { value: "tk_manual" } });
    fireEvent.click(create.getByLabelText("Choose a named source"));
    fireEvent.click(create.getByLabelText("Enter an ID manually"));
    expect((create.getByLabelText("Source ID") as HTMLInputElement).value).toBe("tk_manual");
    fireEvent.click(create.getByRole("button", { name: "Create task" }));
    expect(await create.findByRole("alert")).toHaveTextContent("source is no longer available");
    expect(lastRequest?.body).toMatchObject({ arms: [{ kind: "work_result", source_kind: "task", source_id: "tk_manual", satisfying_outcomes: ["success"] }] });
    expect((create.getByLabelText("Name") as HTMLInputElement).value).toBe("manual follow-up");
    expect((create.getByLabelText("Instruction") as HTMLTextAreaElement).value).toBe("continue the work");
  });

  it("keeps a disappeared named source visible after a list refresh", async () => {
    let taskReads = 0;
    server.use(
      http.get("/api/tasks", () => {
        taskReads += 1;
        return HttpResponse.json({ tasks: taskReads === 1 ? [baseTask, parked] : [parked] });
      }),
    );
    const { client } = renderPage();
    const parkedRow = (await screen.findByText("parked work")).closest("li") as HTMLElement;
    const named = within(parkedRow).getByLabelText("Named prerequisite");
    await within(named).findByRole("option", { name: "build it · armed · tk_1" });
    fireEvent.change(named, { target: { value: "tk_1" } });
    await client.invalidateQueries({ queryKey: ["tasks", "my-app"] });
    await waitFor(() => expect(taskReads).toBeGreaterThan(1));
    expect((within(parkedRow).getByLabelText("Named prerequisite") as HTMLSelectElement).value).toBe("tk_1");
    expect(within(parkedRow).getByRole("option", { name: "Unavailable task · tk_1" })).toBeInTheDocument();
  });

  it("keeps run query errors distinct from an empty project history", async () => {
    server.use(http.get("/api/pipeline-runs", () => HttpResponse.json({ error: { message: "history unavailable" } }, { status: 503 })));
    renderPage();
    const create = within(screen.getByText("New task").parentElement as HTMLElement);
    fireEvent.change(create.getByLabelText("Source type"), { target: { value: "pipeline_run" } });
    expect(await create.findByText("Pipeline runs could not be loaded. The current selection is kept.")).toBeInTheDocument();
    expect(create.queryByText("No pipeline runs in this project.")).not.toBeInTheDocument();
  });

  it("clears the named selection when the project changes", async () => {
    renderPage();
    const create = within(screen.getByText("New task").parentElement as HTMLElement);
    await within(create.getByLabelText("Named prerequisite")).findByRole("option", { name: "build it · armed · tk_1" });
    fireEvent.change(create.getByLabelText("Named prerequisite"), { target: { value: "tk_1" } });
    fireEvent.change(screen.getByLabelText("Project"), { target: { value: "other" } });
    await screen.findByText("build it");
    const nextCreate = within(screen.getByText("New task").parentElement as HTMLElement);
    expect((nextCreate.getByLabelText("Named prerequisite") as HTMLSelectElement).value).toBe("");
  });

  it("keeps the full replacement draft and reports a refused Re-arm", async () => {
    server.use(http.post("/api/tasks/:id/rearm", () => HttpResponse.json({ error: { code: "conflict", message: "wait graph changed" } }, { status: 409 })));
    renderPage();
    const parkedRow = (await screen.findByText("parked work")).closest("li") as HTMLElement;
    expect(within(parkedRow).getByText(/Re-arm replaces this entire wait set/)).toBeInTheDocument();
    expect(within(parkedRow).getByText(/Task: Unavailable task · tk_0/)).toBeInTheDocument();
    fireEvent.change(within(parkedRow).getByLabelText("Named prerequisite"), { target: { value: "tk_1" } });
    fireEvent.click(within(parkedRow).getByRole("button", { name: "Re-arm" }));
    expect(await within(parkedRow).findByRole("alert")).toHaveTextContent("wait graph changed");
    expect((within(parkedRow).getByLabelText("Named prerequisite") as HTMLSelectElement).value).toBe("tk_1");
  });

  it("makes an empty Re-arm replacement explicit and submits no waits", async () => {
    renderPage();
    const parkedRow = (await screen.findByText("parked work")).closest("li") as HTMLElement;
    expect(within(parkedRow).getByText("None — this removes all waits.")).toBeInTheDocument();
    fireEvent.click(within(parkedRow).getByRole("button", { name: "Re-arm" }));
    await waitFor(() => expect(lastRequest?.url).toBe("rearm:tk_2"));
    expect(lastRequest?.body).toEqual({ arms: [] });
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

	 it("offers fast mode only for a capable launch model and sends it", async () => {
		renderPage();
		await screen.findByText("New task");
		fireEvent.change(screen.getByLabelText("Name"), { target: { value: "move quickly" } });
		fireEvent.change(screen.getByLabelText("Instruction"), { target: { value: "do it" } });
		fireEvent.click(await screen.findByRole("checkbox", { name: /Fast mode/ }));
		fireEvent.click(screen.getByRole("button", { name: "Create task" }));
		await waitFor(() => expect(lastRequest?.url).toBe("create"));
		expect(lastRequest?.body).toMatchObject({ target_kind: "launch", fast: true });

		fireEvent.change(screen.getByLabelText("Target"), { target: { value: "agent" } });
		expect(screen.queryByRole("checkbox", { name: /Fast mode/ })).not.toBeInTheDocument();
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
