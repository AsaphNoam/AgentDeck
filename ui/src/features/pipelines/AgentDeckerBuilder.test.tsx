import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import type { AgentState } from "../../api/types";
import { useAgentStore } from "../../store/agentStore";
import { AgentDeckerBuilder } from "./AgentDeckerBuilder";

const BUILDER_KEY = "agentdeck.pipeline-builder-agent";

const server = setupServer(
  http.get("/api/roles", () => HttpResponse.json({ agentdecker: { title: "AgentDecker", prompt: "" } })),
  http.get("/api/config", () => HttpResponse.json({ default_project: "app" })),
  http.get("/api/projects", () => HttpResponse.json({
    app: { title: "App", cwd: "~/Projects/app", color: [0, 0, 0], add_dirs: [], context_prompt: "", resource_dir: "" },
    other: { title: "Other", cwd: "~/Projects/other", color: [0, 0, 0], add_dirs: [], context_prompt: "", resource_dir: "" },
  })),
  http.get("/api/backends", () => HttpResponse.json({
    version: 2,
    backends: {
      codex: {
        name: "Codex", type: "codex-acp", default: true, default_model: "gpt-5.6-sol",
        models: { "gpt-5.6-sol": { name: "GPT-5.6-Sol", model: "gpt-5.6-sol" } },
      },
    },
  })),
  http.get("/api/sessions/:id/transcript", ({ params }) => HttpResponse.json({ agent_id: params.id, events: [] })),
  http.get("/api/pipeline-proposals", () => HttpResponse.json([])),
);

function agent(id: string, running: boolean): AgentState {
  return {
    agent_id: id, name: "Pipeline Builder", role: "agentdecker", project: "app", backend: "codex",
    model: "gpt-5.6-sol", interface: "chat", created_at: "2026-07-26T00:00:00Z", running,
    state: running ? "idle" : "error", detail: "", context_pct: 0, updated_at: 1,
  };
}

function renderBuilder() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter><AgentDeckerBuilder onTemplateProposal={() => {}} onRunProposal={() => {}} /></MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => {
  cleanup();
  server.resetHandlers();
  localStorage.clear();
  useAgentStore.setState({ agents: {}, order: [], hydrated: false, hydrating: false });
});
afterAll(() => server.close());

const templateProposal = {
  proposal_id: "pp_123",
  kind: "save_template",
  digest: "digest-123",
  payload: { id: "review", template: { version: 1, title: "Review", inputs: [], stages: [] } },
};

const runProposal = {
  proposal_id: "pp_456",
  kind: "start_run",
  digest: "digest-456",
  payload: {
    request_id: "req_1", template_id: "delivery", display_name: "Ship", project: "app",
    goal: "Ship it", inputs: {}, assignments: {},
  },
};

// FS-14.R42/A18: Runs only acts on `start_run` proposals and Templates only on
// `save_template` proposals, so each surface's `proposalKind` prop must filter
// the other kind out rather than offering an approval action it cannot own.
describe("AgentDeckerBuilder proposal filtering", () => {
  it("shows only the start_run proposal and its action for proposalKind=start_run", async () => {
    server.use(http.get("/api/pipeline-proposals", () => HttpResponse.json([templateProposal, runProposal])));
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter><AgentDeckerBuilder proposalKind="start_run" showLauncher={false} onRunProposal={() => {}} /></MemoryRouter>
      </QueryClientProvider>,
    );

    expect(await screen.findByRole("button", { name: "Review exact Start proposal" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Review exact Save proposal" })).not.toBeInTheDocument();
  });

  it("shows only the save_template proposal and its action for proposalKind=save_template", async () => {
    server.use(http.get("/api/pipeline-proposals", () => HttpResponse.json([templateProposal, runProposal])));
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter><AgentDeckerBuilder proposalKind="save_template" showLauncher onTemplateProposal={() => {}} /></MemoryRouter>
      </QueryClientProvider>,
    );

    expect(await screen.findByRole("button", { name: "Review exact Save proposal" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Review exact Start proposal" })).not.toBeInTheDocument();
  });

  it("shows every proposal when no proposalKind is given", async () => {
    server.use(http.get("/api/pipeline-proposals", () => HttpResponse.json([templateProposal, runProposal])));
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter><AgentDeckerBuilder onTemplateProposal={() => {}} onRunProposal={() => {}} /></MemoryRouter>
      </QueryClientProvider>,
    );

    expect(await screen.findByRole("button", { name: "Review exact Start proposal" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Review exact Save proposal" })).toBeInTheDocument();
  });
});

// INV §1: a stopped builder cannot keep a dead chat link, but a durable
// transcript proposal must remain reachable for the required human approval.
describe("AgentDeckerBuilder persisted session", () => {
  it("expires a persisted builder id once hydration shows it stopped", async () => {
    localStorage.setItem(BUILDER_KEY, "a_builder");
    useAgentStore.setState({
      agents: { a_builder: agent("a_builder", false) }, order: ["a_builder"], hydrated: true, hydrating: false,
    });

    renderBuilder();

    await waitFor(() => expect(localStorage.getItem(BUILDER_KEY)).toBeNull());
    expect(screen.queryByRole("link", { name: "Open AgentDecker chat" })).not.toBeInTheDocument();
  });

  it("keeps the session link while the builder is still running", async () => {
    localStorage.setItem(BUILDER_KEY, "a_builder");
    useAgentStore.setState({
      agents: { a_builder: agent("a_builder", true) }, order: ["a_builder"], hydrated: true, hydrating: false,
    });

    renderBuilder();

    const link = await screen.findByRole("link", { name: "Open AgentDecker chat" });
    expect(link.getAttribute("href")).toBe("/agent/a_builder");
    expect(localStorage.getItem(BUILDER_KEY)).toBe("a_builder");
  });

  it("retains the persisted id before hydration completes", async () => {
    localStorage.setItem(BUILDER_KEY, "a_builder");
    useAgentStore.setState({ agents: {}, order: [], hydrated: false, hydrating: true });

    renderBuilder();

    expect(screen.getByText("Loading builder session…")).toBeInTheDocument();
    expect(localStorage.getItem(BUILDER_KEY)).toBe("a_builder");
  });

  // FS-14.R27/A10 / TS-09.R15,R23: a new Pipelines view has no browser-local
  // builder pointer and a real ACP adapter may have stored the terminal result
  // as content:null. The durable proposal API is therefore the only authority.
  it("reviews one durable proposal without a builder pointer or transcript result", async () => {
    const reviewed: unknown[] = [];
    let saveCalls = 0;
    server.use(
      http.get("/api/pipeline-proposals", () => HttpResponse.json([templateProposal])),
      http.post("/api/pipelines", () => {
        saveCalls++;
        return HttpResponse.json({});
      }),
    );

    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    render(<QueryClientProvider client={client}><MemoryRouter><AgentDeckerBuilder onTemplateProposal={(proposal) => reviewed.push(proposal)} onRunProposal={() => {}} /></MemoryRouter></QueryClientProvider>);

    expect(localStorage.getItem(BUILDER_KEY)).toBeNull();
    const review = await screen.findByRole("button", { name: "Review exact Save proposal" });
    expect(screen.getAllByRole("button", { name: "Review exact Save proposal" })).toHaveLength(1);
    expect(saveCalls).toBe(0);
    fireEvent.click(review);
    expect(reviewed).toEqual([templateProposal]);
    expect(saveCalls).toBe(0);
  });

  // FS-14.R33/A13 / TS-09.R26: the approval surface follows the server's pending
  // records. Once the approved Save or Start commits, the server consumes that
  // proposal, so a refetch (SSE invalidation or a fresh page load) must stop
  // offering the same approval instead of leaving it listed forever.
  it("drops an approved proposal once the server no longer lists it", async () => {
    let pending = [templateProposal];
    server.use(http.get("/api/pipeline-proposals", () => HttpResponse.json(pending)));

    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    render(<QueryClientProvider client={client}><MemoryRouter><AgentDeckerBuilder onTemplateProposal={() => {}} onRunProposal={() => {}} /></MemoryRouter></QueryClientProvider>);

    await screen.findByRole("button", { name: "Review exact Save proposal" });
    expect(screen.getByText("Pending exact proposals")).toBeInTheDocument();

    pending = [];
    await client.invalidateQueries({ queryKey: ["pipelines", "proposals"] });

    await waitFor(() => expect(screen.queryByText("Pending exact proposals")).not.toBeInTheDocument());
    expect(screen.queryByRole("button", { name: "Review exact Save proposal" })).not.toBeInTheDocument();
  });
});

// FS-14.R26/A10: the builder launches an ordinary chat agent, which needs a real
// project directory. It previously sent `default_project` with no picker, so a
// seeded-but-absent default could only be discovered as a rejected launch and
// could not be changed from this page.
describe("AgentDeckerBuilder project selection", () => {
  async function openSetup() {
    renderBuilder();
    fireEvent.click(await screen.findByRole("button", { name: "Create with AgentDecker" }));
  }

  it("launches into the chosen project rather than the configured default", async () => {
    const launches: Array<Record<string, unknown>> = [];
    server.use(http.post("/api/sessions", async ({ request }) => {
      launches.push((await request.json()) as Record<string, unknown>);
      return HttpResponse.json({ agent: { agent_id: "a_new" } });
    }));
    server.use(http.post("/api/sessions/:id/prompt", () => HttpResponse.json({ ok: true })));

    await openSetup();
    const select = await screen.findByLabelText("Project");
    await waitFor(() => expect((select as HTMLSelectElement).value).toBe("app"));

    fireEvent.change(select, { target: { value: "other" } });
    fireEvent.change(screen.getByLabelText("Describe the pipeline"), { target: { value: "Build then review." } });
    fireEvent.click(screen.getByRole("button", { name: "Launch AgentDecker builder" }));

    await waitFor(() => expect(launches).toHaveLength(1));
    expect(launches[0].project).toBe("other");
  });

  it("clears a selection that leaves the catalog and holds the launch closed", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter><AgentDeckerBuilder onTemplateProposal={() => {}} onRunProposal={() => {}} /></MemoryRouter>
      </QueryClientProvider>,
    );
    fireEvent.click(await screen.findByRole("button", { name: "Create with AgentDecker" }));
    const select = await screen.findByLabelText("Project");
    await waitFor(() => expect((select as HTMLSelectElement).value).toBe("app"));

    fireEvent.change(select, { target: { value: "other" } });
    fireEvent.change(screen.getByLabelText("Describe the pipeline"), { target: { value: "Build then review." } });
    expect(screen.getByRole("button", { name: "Launch AgentDecker builder" })).toBeEnabled();

    // Another tab deletes the selected "other" and the seeded default "app"; the
    // refreshed catalog no longer contains the selection, so the stale id must be
    // cleared and the launch held closed rather than posting a rejected launch.
    server.use(http.get("/api/projects", () => HttpResponse.json({
      keep: { title: "Keep", cwd: "~/Projects/keep", color: [0, 0, 0], add_dirs: [], context_prompt: "", resource_dir: "" },
    })));
    await client.invalidateQueries({ queryKey: ["projects"] });

    await waitFor(() => expect((select as HTMLSelectElement).value).toBe(""));
    expect(screen.getByRole("button", { name: "Launch AgentDecker builder" })).toBeDisabled();
  });

  it("holds the launch closed when the default project no longer exists", async () => {
    server.use(http.get("/api/projects", () => HttpResponse.json({
      other: { title: "Other", cwd: "~/Projects/other", color: [0, 0, 0], add_dirs: [], context_prompt: "", resource_dir: "" },
    })));

    await openSetup();
    fireEvent.change(await screen.findByLabelText("Describe the pipeline"), { target: { value: "Build then review." } });

    // `default_project` is still "app", but it resolves to no configured project,
    // so nothing is seeded and the launch stays disabled instead of enabling a
    // button whose only outcome is a rejected launch.
    expect((await screen.findByLabelText("Project") as HTMLSelectElement).value).toBe("");
    expect(screen.getByRole("button", { name: "Launch AgentDecker builder" })).toBeDisabled();
  });
});
