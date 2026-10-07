import React from "react";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { http, HttpResponse, delay } from "msw";
import { setupServer } from "msw/node";
import { TasksPage, needsAttention } from "./TasksPage";
import { useAgentStore } from "../../store/agentStore";
import fixture from "./fixtures/taskLists.json";

// Task lists come from the Go-marshalled fixture (internal/server/
// task_wire_fixture_test.go), so the view is tested against the server's real
// wire shape — including `waiting`, lineage and cleanup fields (INV §11/§17).
type WireTask = (typeof fixture)["my-app"]["tasks"][number];
const wire = (project: keyof typeof fixture): WireTask[] => structuredClone(fixture[project].tasks) as WireTask[];
const byID = (id: string) => [...wire("my-app"), ...wire("other")].find((task) => task.task_id === id)!;

const runSummary = {
  run_id: "pr_1", template_id: "release", display_name: "Release", project: "my-app", state: "completed",
  revision: 1, pending_action: "", current_stage_id: "", current_stage_title: "", current_agent_id: "",
  attention_reason: "", final_outcome: "success", updated_at: "2026-08-24T10:00:00Z", diagnostics: [],
};

let lastRequest: { url: string; body: unknown } | null = null;
let taskLists: Record<string, unknown[] | "error"> = {};

const server = setupServer(
  http.get("/api/projects", () => HttpResponse.json({
    "my-app": { title: "My App", cwd: "/tmp" },
    other: { title: "Other", cwd: "/tmp/other" },
    quiet: { title: "Quiet", cwd: "/tmp/quiet" },
  })),
  http.get("/api/roles", () => HttpResponse.json({ chucky: { title: "Chucky" }, impl: { title: "Impl" } })),
  http.get("/api/config", () => HttpResponse.json({ default_role: "impl" })),
  http.get("/api/backends", () => HttpResponse.json({ version: 2, backends: {
    codex: { name: "Codex", type: "codex-acp", default: true, default_model: "gpt-5", models: { "gpt-5": { name: "GPT-5", model: "gpt-5", fast: true } } },
  } })),
  http.get("/api/tasks", ({ request }) => {
    const project = new URL(request.url).searchParams.get("project") ?? "";
    const list = taskLists[project] ?? [];
    if (list === "error") return HttpResponse.json({ error: { code: "internal", message: "store unavailable" } }, { status: 500 });
    return HttpResponse.json({ tasks: list });
  }),
  http.get("/api/pipeline-runs", () => HttpResponse.json([runSummary], { headers: { "X-Total-Count": "1" } })),
  http.get("/api/pipeline-runs/:id", () => HttpResponse.json({ error: { code: "internal", message: "run unavailable" } }, { status: 500 })),
  http.post("/api/tasks/:id/retry", ({ params }) => {
    lastRequest = { url: `retry:${params.id}`, body: null };
    return HttpResponse.json({ error: { code: "validation", message: "re-arm it instead", details: { code: "retry_requires_rearm" } } }, { status: 422 });
  }),
  http.post("/api/tasks/:id/rearm", async ({ params, request }) => {
    lastRequest = { url: `rearm:${params.id}`, body: await request.json() };
    return HttpResponse.json({ ...byID(String(params.id)), state: "ready" });
  }),
  http.post("/api/tasks/:id/result", async ({ params, request }) => {
    lastRequest = { url: `result:${params.id}`, body: await request.json() };
    return HttpResponse.json({ ...byID(String(params.id)), state: "finished", outcome: "success" });
  }),
  http.post("/api/tasks", async ({ request }) => {
    lastRequest = { url: "create", body: await request.json() };
    return HttpResponse.json({ ...byID("tk_r"), state: "ready" }, { status: 201 });
  }),
  http.post("/api/signals", async ({ request }) => {
    lastRequest = { url: "signal", body: await request.json() };
    return HttpResponse.json({ released: 1 });
  }),
);

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
beforeEach(() => {
  taskLists = { "my-app": wire("my-app"), other: wire("other"), quiet: [] };
  useAgentStore.setState({ agents: {}, order: [], hydrated: true, hydrating: false });
});
afterEach(() => {
  cleanup();
  lastRequest = null;
  server.resetHandlers();
});
afterAll(() => server.close());

function renderPage(initialEntry = "/tasks") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const result = render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[initialEntry]}><TasksPage /></MemoryRouter>
    </QueryClientProvider>,
  );
  return { ...result, client };
}

/** rowOf returns the list item for a task, by its row's summary button. */
async function rowOf(name: string) {
  const button = await screen.findByRole("button", { name: new RegExp(`^${name}`) });
  return button.closest("li") as HTMLElement;
}

async function openTask(name: string) {
  const row = await rowOf(name);
  fireEvent.click(within(row).getByRole("button", { name: new RegExp(`^${name}`) }));
  return row;
}

function createForm() {
  return within(screen.getByText("Create task manually").closest("details") as HTMLElement);
}

describe("Tasks work in motion", () => {
  // FS-02.A52 — archived projects leave the all-projects view and filter, but
  // an explicit focus on one still shows its tasks.
  it("omits archived projects from all projects while an explicit focus still works", async () => {
    server.use(http.get("/api/projects", () => HttpResponse.json({
      "my-app": { title: "My App", cwd: "/tmp" },
      other: { title: "Other", cwd: "/tmp/other", archived: true },
    })));
    renderPage();
    await screen.findByRole("heading", { name: "My App" });
    expect(screen.queryByRole("heading", { name: "Other" })).not.toBeInTheDocument();
    const filter = within(document.querySelector("[data-slot=\"toolbar\"]") as HTMLElement).getByRole("combobox", { name: "Project" });
    expect(within(filter).queryByRole("option", { name: "Other" })).not.toBeInTheDocument();
    cleanup();

    renderPage("/tasks?project=other");
    expect(await screen.findByText("Rotate keys")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(within(document.querySelector("[data-slot=\"toolbar\"]") as HTMLElement).getByRole("combobox", { name: "Project" })).toHaveDisplayValue("Other");
  });

  // FS-16.A27/A29 — no project means All projects, grouped by project and by
  // recorded relationships, with authoring closed below the work.
  it("opens on all projects with connected groups and closed authoring at the bottom", async () => {
    renderPage();
    const myApp = (await screen.findByRole("heading", { name: "My App" })).closest("section") as HTMLElement;
    const other = screen.getByRole("heading", { name: "Other" }).closest("section") as HTMLElement;
    expect(screen.queryByRole("heading", { name: "Quiet" })).not.toBeInTheDocument();
    await within(myApp).findByText("Implement API");
    expect(within(myApp).getByText(/11 unfinished tasks/)).toBeInTheDocument();
    expect(within(myApp).getByText("2 need attention")).toBeInTheDocument();
    expect(within(other).getByText("Rotate keys")).toBeInTheDocument();

    const chain = (await rowOf("Write API docs")).closest("ol") as HTMLElement;
    expect(within(chain).getAllByRole("listitem").map((item) => within(item).getAllByRole("button")[0].textContent)).toEqual([
      expect.stringMatching(/^Design schema/), expect.stringMatching(/^Implement API/), expect.stringMatching(/^Write API docs/),
    ]);
    expect(within(await rowOf("Write API docs")).getByText("Waits for Implement API → success")).toBeInTheDocument();
    expect(within(await rowOf("Draft release notes")).getByText(/delegated by Coordinate release/)).toBeInTheDocument();
    expect(within(await rowOf("Coordinate release")).queryByText(/leads to/)).not.toBeInTheDocument();
    expect(within(await rowOf("Design schema")).getByText(/leads to Implement API/)).toBeInTheDocument();
    expect(within(await rowOf("Follow up on removed work")).getByText(/unavailable task tk_gone/)).toBeInTheDocument();
    expect(within(await rowOf("Migrate orders")).getByText("Waiting for work updates")).toBeInTheDocument();
    expect(within(await rowOf("Publish notes")).getByText("Finishing cleanup")).toBeInTheDocument();

    const history = within(myApp).getByText(/Completed history · 1 group, 2 tasks/).closest("details") as HTMLDetailsElement;
    expect(history.open).toBe(false);
    expect(within(history).getByText("Failure")).toBeInTheDocument();
    expect(within(history).getByText("Cancelled")).toBeInTheDocument();

    const authoring = screen.getByText("Create task manually").closest("details") as HTMLDetailsElement;
    expect(authoring.open).toBe(false);
    expect(myApp.compareDocumentPosition(authoring) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(other.compareDocumentPosition(authoring) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("narrows to an existing project link and explains a focused empty project", async () => {
    renderPage("/tasks?project=other");
    expect(await screen.findByText("Rotate keys")).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "My App" })).not.toBeInTheDocument();
    cleanup();
    renderPage("/tasks?project=quiet");
    expect(await screen.findByText(/No tasks in this project/)).toBeInTheDocument();
  });

  it("reports an unknown focused project instead of substituting another", async () => {
    renderPage("/tasks?project=gone");
    expect(await screen.findByText(/No project named “gone” exists/)).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "My App" })).not.toBeInTheDocument();
  });

  it("explains a globally empty page without promoting manual creation", async () => {
    taskLists = {};
    renderPage();
    expect(await screen.findByText(/No tasks yet\. When agents hand work to each other/)).toBeInTheDocument();
    expect((screen.getByText("Create task manually").closest("details") as HTMLDetailsElement).open).toBe(false);
  });

  it("keeps one failed project distinct from loaded and empty ones", async () => {
    taskLists.other = "error";
    renderPage();
    expect(await screen.findByText("Tasks for Other could not be loaded.", { exact: false })).toBeInTheDocument();
    expect(await screen.findByText("Implement API")).toBeInTheDocument();
    expect(screen.getByText(/\(some projects not loaded\)/)).toBeInTheDocument();
  });

  it("marks retained data stale when a refresh fails", async () => {
    const { client } = renderPage("/tasks?project=my-app");
    await screen.findByText("Implement API");
    taskLists["my-app"] = "error";
    await client.invalidateQueries({ queryKey: ["tasks", "my-app"] });
    expect(await screen.findByText(/Refreshing failed; showing the last loaded tasks/)).toBeInTheDocument();
    expect(screen.getByText("Implement API")).toBeInTheDocument();
  });

  // TS-08.R82 — at most four project reads are in flight at once.
  it("bounds concurrent project reads to four", async () => {
    let active = 0;
    let peak = 0;
    const names = Array.from({ length: 7 }, (_, index) => `p${index}`);
    server.use(
      http.get("/api/projects", () => HttpResponse.json(Object.fromEntries(names.map((name) => [name, { title: name, cwd: "/tmp" }])))),
      http.get("/api/tasks", async ({ request }) => {
        active++;
        peak = Math.max(peak, active);
        await delay(20);
        active--;
        const project = new URL(request.url).searchParams.get("project")!;
        return HttpResponse.json({ tasks: [{ ...byID("tk_r"), task_id: `tk_${project}`, project, display_name: `work ${project}` }] });
      }),
    );
    renderPage();
    for (const name of names) expect(await screen.findByText(`work ${name}`)).toBeInTheDocument();
    expect(peak).toBeLessThanOrEqual(4);
    expect(peak).toBeGreaterThan(1);
  });

  // FS-16.A28 — an open detail survives its group settling.
  it("keeps an inspected task's group and detail in place when it finishes", async () => {
    const { client } = renderPage("/tasks?project=my-app");
    const row = await openTask("Implement API");
    expect(within(row).getByText("Instruction for Implement API.")).toBeInTheDocument();
    taskLists["my-app"] = wire("my-app").map((task) =>
      task.task_id === "tk_b" || task.task_id === "tk_c" ? { ...task, state: "finished", outcome: "success", finished_at: "2026-10-01T10:00:00Z" } : task);
    await client.invalidateQueries({ queryKey: ["tasks", "my-app"] });
    const settled = await rowOf("Implement API");
    await within(settled).findByText("Success", { selector: "span *, span" });
    expect(within(settled).getByText("Instruction for Implement API.")).toBeInTheDocument();
    expect(settled.closest("details")).toBeNull();
  });

  it("shows result, outputs, creator and assignee identity in detail", async () => {
    useAgentStore.setState({ agents: {
      ag_a: { agent_id: "ag_a", name: "Schema author", archived: false } as never,
      ag_lead: { agent_id: "ag_lead", name: "Lead", archived: true } as never,
    } });
    renderPage("/tasks?project=my-app");
    const row = await openTask("Design schema");
    expect(within(row).getByText(/Result: success · recorded by agent/)).toBeInTheDocument();
    expect(within(row).getByText("docs/schema.md")).toBeInTheDocument();
    expect(within(row).getByRole("link", { name: "Schema author" })).toHaveAttribute("href", "/agent/ag_a");
    // FS-02.A51 — an archived agent reads as its name plus an **archived** tag.
    const tag = within(row).getByText("archived").closest('[data-ui="badge"]')!;
    expect(tag).not.toBeNull();
    expect(tag.parentElement).toHaveTextContent(/^Lead archived$/);
    expect(within(row).queryByRole("link", { name: /Lead/ })).not.toBeInTheDocument();
    const missing = await rowOf("Draft release notes");
    expect(within(missing).getByText("ag_gone (unavailable)")).toBeInTheDocument();
  });

  // FS-16.R23/A11 — parked work offers only the repair that can succeed.
  it("omits retry on parked work and re-arms in place", async () => {
    renderPage("/tasks?project=my-app");
    const row = await openTask("Follow up on removed work");
    expect(within(row).queryByRole("button", { name: "Retry" })).not.toBeInTheDocument();
    expect(within(row).getByText(/Task: Unavailable task · tk_gone/)).toBeInTheDocument();
    fireEvent.click(within(row).getByText("Advanced signals"));
    fireEvent.change(within(row).getByLabelText("Wait for signal"), { target: { value: "ci-green" } });
    fireEvent.click(within(row).getByRole("button", { name: "Re-arm" }));
    await waitFor(() => expect(lastRequest?.url).toBe("rearm:tk_s"));
    expect(lastRequest?.body).toEqual({ arms: [{ kind: "signal", signal_name: "ci-green" }] });
  });

  it("keeps the full replacement draft and reports a refused Re-arm", async () => {
    server.use(http.post("/api/tasks/:id/rearm", () => HttpResponse.json({ error: { code: "conflict", message: "wait graph changed" } }, { status: 409 })));
    renderPage("/tasks?project=my-app");
    const row = await openTask("Follow up on removed work");
    expect(within(row).getByText(/Re-arm replaces this entire wait set/)).toBeInTheDocument();
    const named = within(row).getByLabelText("Named prerequisite");
    await within(named).findByRole("option", { name: "Tidy fixtures · ready · tk_r" });
    fireEvent.change(named, { target: { value: "tk_r" } });
    fireEvent.click(within(row).getByRole("button", { name: "Re-arm" }));
    expect(await within(row).findByRole("alert")).toHaveTextContent("wait graph changed");
    expect((within(row).getByLabelText("Named prerequisite") as HTMLSelectElement).value).toBe("tk_r");
  });

  it("makes an empty Re-arm replacement explicit and submits no waits", async () => {
    renderPage("/tasks?project=my-app");
    const row = await openTask("Follow up on removed work");
    expect(within(row).getByText("None — this removes all waits.")).toBeInTheDocument();
    fireEvent.click(within(row).getByRole("button", { name: "Re-arm" }));
    await waitFor(() => expect(lastRequest?.url).toBe("rearm:tk_s"));
    expect(lastRequest?.body).toEqual({ arms: [] });
  });

  it("keeps a disappeared named source visible after a list refresh", async () => {
    const { client } = renderPage("/tasks?project=my-app");
    const row = await openTask("Follow up on removed work");
    const named = within(row).getByLabelText("Named prerequisite");
    await within(named).findByRole("option", { name: "Tidy fixtures · ready · tk_r" });
    fireEvent.change(named, { target: { value: "tk_r" } });
    taskLists["my-app"] = wire("my-app").filter((task) => task.task_id !== "tk_r");
    await client.invalidateQueries({ queryKey: ["tasks", "my-app"] });
    await waitFor(() => expect(screen.queryByRole("button", { name: /^Tidy fixtures/ })).not.toBeInTheDocument());
    expect((within(row).getByLabelText("Named prerequisite") as HTMLSelectElement).value).toBe("tk_r");
    expect(within(row).getByRole("option", { name: "Unavailable task · tk_r" })).toBeInTheDocument();
  });

  // INV §2 — retry follows the server's retry_eligible, and a refusal stays visible.
  it("offers retry from server eligibility and reports a refusal", async () => {
    renderPage("/tasks?project=my-app");
    const row = await openTask("Draft release notes");
    fireEvent.click(within(row).getByRole("button", { name: "Retry" }));
    expect(await within(row).findByRole("alert")).toHaveTextContent("re-arm it instead");
    taskLists["my-app"] = wire("my-app").map((task) => task.task_id === "tk_q" ? { ...task, retry_eligible: false } : task);
    cleanup();
    renderPage("/tasks?project=my-app");
    const again = await openTask("Draft release notes");
    expect(within(again).queryByRole("button", { name: "Retry" })).not.toBeInTheDocument();
  });

  it("records a person's result on running work", async () => {
    renderPage("/tasks?project=my-app");
    const row = await openTask("Implement API");
    fireEvent.change(within(row).getByLabelText("Result summary"), { target: { value: "done by hand" } });
    fireEvent.click(within(row).getByRole("button", { name: "Record result" }));
    await waitFor(() => expect(lastRequest?.url).toBe("result:tk_b"));
    expect(lastRequest?.body).toMatchObject({ outcome: "success", summary: "done by hand" });
  });

  // TS-08.R83 — run lineage withholds stage-restricted controls until the run
  // confirms ownership, and always offers the run link.
  it("withholds stage-restricted controls while run ownership is unavailable", async () => {
    renderPage("/tasks?project=my-app");
    const row = await openTask("Build stage");
    expect(within(row).getAllByRole("link", { name: "pr_1" })[0]).toHaveAttribute("href", "/pipelines/runs/pr_1");
    expect(await within(row).findByText(/Stage ownership could not be loaded/)).toBeInTheDocument();
    expect(within(row).queryByRole("button", { name: "Record result" })).not.toBeInTheDocument();
    expect(within(row).queryByRole("button", { name: "Delete" })).not.toBeInTheDocument();
    expect(within(row).getByRole("button", { name: "Cancel" })).toBeInTheDocument();
  });

  it("routes a signal wait to the explicit signal control without firing it", async () => {
    renderPage();
    const row = await openTask("Ship release");
    fireEvent.click(within(row).getByRole("button", { name: "Go to signal control" }));
    expect(lastRequest).toBeNull();
    const signal = within(screen.getByText("Fire a signal").closest("details") as HTMLElement);
    expect((signal.getByLabelText("Signal project") as HTMLSelectElement).value).toBe("my-app");
    expect((signal.getByLabelText("Signal name") as HTMLInputElement).value).toBe("ci-green");
    fireEvent.click(signal.getByRole("button", { name: "Fire" }));
    await waitFor(() => expect(lastRequest?.url).toBe("signal"));
    expect(lastRequest?.body).toEqual({ project: "my-app", name: "ci-green" });
    expect(await signal.findByText(/Fired ci-green in my-app; 1 wait released/)).toBeInTheDocument();
  });
});

describe("Create task manually", () => {
  it("requires a concrete project in All projects and preselects a focused one", async () => {
    renderPage();
    await screen.findByText("Implement API");
    expect((createForm().getByLabelText("Project") as HTMLSelectElement).value).toBe("");
    cleanup();
    renderPage("/tasks?project=other");
    await screen.findByText("Rotate keys");
    expect((createForm().getByLabelText("Project") as HTMLSelectElement).value).toBe("other");
  });

  it("keeps the draft when closed and reopened, and clears only dependency choices on project change", async () => {
    renderPage("/tasks?project=my-app");
    const disclosure = screen.getByText("Create task manually").closest("details") as HTMLDetailsElement;
    disclosure.open = true;
    const create = createForm();
    fireEvent.change(create.getByLabelText("Name"), { target: { value: "draft name" } });
    const named = create.getByLabelText("Named prerequisite");
    await within(named).findByRole("option", { name: "Tidy fixtures · ready · tk_r" });
    fireEvent.change(named, { target: { value: "tk_r" } });
    disclosure.open = false;
    disclosure.open = true;
    expect((create.getByLabelText("Name") as HTMLInputElement).value).toBe("draft name");
    fireEvent.change(create.getByLabelText("Project"), { target: { value: "other" } });
    expect((create.getByLabelText("Named prerequisite") as HTMLSelectElement).value).toBe("");
    expect((create.getByLabelText("Name") as HTMLInputElement).value).toBe("draft name");
  });

  it("authors a task with a pipeline arm, outcomes, and context", async () => {
    renderPage("/tasks?project=my-app");
    const create = createForm();
    fireEvent.change(create.getByLabelText("Name"), { target: { value: "review" } });
    fireEvent.change(create.getByLabelText("Instruction"), { target: { value: "review it" } });
    fireEvent.change(create.getByLabelText("Source type"), { target: { value: "pipeline_run" } });
    await create.findByRole("option", { name: "Release · completed · pr_1" });
    fireEvent.change(create.getByLabelText("Named prerequisite"), { target: { value: "pr_1" } });
    fireEvent.click(create.getByLabelText("Failure"));
    fireEvent.change(create.getByLabelText("Context reference ID"), { target: { value: "cx_1" } });
    fireEvent.change(create.getByLabelText("Context label"), { target: { value: "brief" } });
    fireEvent.click(create.getByRole("button", { name: "Create task" }));
    await waitFor(() => expect(lastRequest?.url).toBe("create"));
    expect(lastRequest?.body).toMatchObject({ project: "my-app", target_kind: "launch", arms: [{ source_kind: "pipeline_run", source_id: "pr_1", satisfying_outcomes: ["success", "failure"] }], attachments: [{ context_ref_id: "cx_1", label: "brief" }] });
    expect(await create.findByText(/Created “review” in my-app/)).toBeInTheDocument();
  });

  it("disambiguates duplicate task names and submits the selected stable ID", async () => {
    taskLists["my-app"] = [{ ...byID("tk_r"), display_name: "Build", task_id: "tk_x" }, { ...byID("tk_e"), display_name: "Build", task_id: "tk_y", arms: [] }];
    renderPage("/tasks?project=my-app");
    const create = createForm();
    const named = create.getByLabelText("Named prerequisite");
    await within(named).findByRole("option", { name: "Build · ready · tk_x" });
    expect(within(named).getByRole("option", { name: "Build · ready · tk_y" })).toBeInTheDocument();
    fireEvent.change(named, { target: { value: "tk_y" } });
    fireEvent.change(create.getByLabelText("Name"), { target: { value: "follow up" } });
    fireEvent.change(create.getByLabelText("Instruction"), { target: { value: "continue" } });
    fireEvent.click(create.getByRole("button", { name: "Create task" }));
    await waitFor(() => expect(lastRequest?.url).toBe("create"));
    expect(lastRequest?.body).toMatchObject({ arms: [{ kind: "work_result", source_kind: "task", source_id: "tk_y", satisfying_outcomes: ["success"] }] });
  });

  it("keeps an unavailable outcome visible when changing source type and blocks submission until corrected", async () => {
    renderPage("/tasks?project=my-app");
    const create = createForm();
    await within(create.getByLabelText("Named prerequisite")).findByRole("option", { name: "Tidy fixtures · ready · tk_r" });
    fireEvent.change(create.getByLabelText("Named prerequisite"), { target: { value: "tk_r" } });
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
    renderPage("/tasks?project=my-app");
    const create = createForm();
    fireEvent.change(create.getByLabelText("Source type"), { target: { value: "pipeline_run" } });
    expect(await create.findByText("No matching runs in the loaded pages yet.")).toBeInTheDocument();
    fireEvent.click(create.getByRole("button", { name: "Load more runs" }));
    await create.findByRole("option", { name: "Release · completed · pr_page2" });
  });

  it("keeps manual IDs in the same selection and preserves the draft after refusal", async () => {
    server.use(http.post("/api/tasks", async ({ request }) => {
      lastRequest = { url: "create", body: await request.json() };
      return HttpResponse.json({ error: { code: "validation", message: "source is no longer available" } }, { status: 422 });
    }));
    renderPage("/tasks?project=my-app");
    await screen.findByText("Implement API");
    const create = createForm();
    fireEvent.change(create.getByLabelText("Name"), { target: { value: "manual follow-up" } });
    fireEvent.change(create.getByLabelText("Instruction"), { target: { value: "continue the work" } });
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

  it("keeps run query errors distinct from an empty project history", async () => {
    server.use(http.get("/api/pipeline-runs", () => HttpResponse.json({ error: { message: "history unavailable" } }, { status: 503 })));
    renderPage("/tasks?project=my-app");
    const create = createForm();
    fireEvent.change(create.getByLabelText("Source type"), { target: { value: "pipeline_run" } });
    expect(await create.findByText("Pipeline runs could not be loaded. The current selection is kept.")).toBeInTheDocument();
    expect(create.queryByText("No pipeline runs in this project.")).not.toBeInTheDocument();
  });

  // FS-16.A18 (R27) — effort reaches the request only for a launch target.
  it("sends the effort it was given for a launch target", async () => {
    renderPage("/tasks?project=my-app");
    await screen.findByText("Implement API");
    const create = createForm();
    fireEvent.change(create.getByLabelText("Name"), { target: { value: "think hard" } });
    fireEvent.change(create.getByLabelText("Instruction"), { target: { value: "reason about it" } });
    fireEvent.change(create.getByLabelText("Effort (optional)"), { target: { value: "high" } });
    fireEvent.click(create.getByRole("button", { name: "Create task" }));
    await waitFor(() => expect(lastRequest?.url).toBe("create"));
    expect(lastRequest?.body).toMatchObject({ target_kind: "launch", effort: "high" });
    fireEvent.change(create.getByLabelText("Target"), { target: { value: "agent" } });
    expect(create.queryByLabelText("Effort (optional)")).not.toBeInTheDocument();
  });

  it("offers fast mode only for a capable launch model and sends it", async () => {
    renderPage("/tasks?project=my-app");
    const create = createForm();
    fireEvent.change(create.getByLabelText("Name"), { target: { value: "move quickly" } });
    fireEvent.change(create.getByLabelText("Instruction"), { target: { value: "do it" } });
    fireEvent.click(await create.findByRole("checkbox", { name: /Fast mode/ }));
    fireEvent.click(create.getByRole("button", { name: "Create task" }));
    await waitFor(() => expect(lastRequest?.url).toBe("create"));
    expect(lastRequest?.body).toMatchObject({ target_kind: "launch", fast: true });
    fireEvent.change(create.getByLabelText("Target"), { target: { value: "agent" } });
    expect(create.queryByRole("checkbox", { name: /Fast mode/ })).not.toBeInTheDocument();
  });

  it("uses the configured default role for a new launch task", async () => {
    renderPage("/tasks?project=my-app");
    await waitFor(() => expect((createForm().getByLabelText("Role to launch") as HTMLSelectElement).value).toBe("impl"));
  });
});

describe("task helpers", () => {
  // FS-02.A26 — attention is exactly parked and interrupted work.
  it("counts only parked and interrupted work as needing attention", () => {
    for (const state of ["dependency_failed", "interrupted"]) expect(needsAttention({ state } as never)).toBe(true);
    for (const state of ["armed", "ready", "starting", "running", "waiting", "finished"]) expect(needsAttention({ state } as never)).toBe(false);
  });
});
