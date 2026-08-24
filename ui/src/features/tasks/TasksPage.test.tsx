import React from "react";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { TasksPage, needsAttention, waitingOn } from "./TasksPage";

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
  arms: [{ ...baseTask.arms[0], arm_id: "tk_2_arm00", task_id: "tk_2", state: "unsatisfiable" as const }],
};

let lastRequest: { url: string; body: unknown } | null = null;

const server = setupServer(
  http.get("/api/projects", () => HttpResponse.json({ "my-app": { title: "My App", cwd: "/tmp" } })),
  http.get("/api/roles", () => HttpResponse.json({ impl: { title: "Impl" } })),
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

  // FS-16.R23 / A11 — retry on a task parked by an unsatisfiable arm is refused
  // with the reason, and re-arming it is the repair the view offers next to it.
  it("surfaces the retry refusal and re-arms in place", async () => {
    renderPage();
    await screen.findByText("parked work");
    const rows = screen.getAllByRole("listitem");
    const parkedRow = rows[1];

    fireEvent.click(within(parkedRow).getByRole("button", { name: "Retry" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("re-arm it instead");

    fireEvent.change(within(parkedRow).getByLabelText("Wait for signal"), { target: { value: "ci-green" } });
    fireEvent.click(within(parkedRow).getByRole("button", { name: "Re-arm" }));
    await waitFor(() => expect(lastRequest?.url).toBe("rearm:tk_2"));
    expect(lastRequest?.body).toEqual({ arms: [{ kind: "signal", signal_name: "ci-green" }] });
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
});
