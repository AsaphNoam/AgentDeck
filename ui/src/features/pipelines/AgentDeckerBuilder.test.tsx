import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import type { AgentState } from "../../api/types";
import { useAgentStore } from "../../store/agentStore";
import { useTranscriptStore } from "../../store/transcriptStore";
import { AgentDeckerBuilder } from "./AgentDeckerBuilder";

const BUILDER_KEY = "agentdeck.pipeline-builder-agent";

const server = setupServer(
  http.get("/api/roles", () => HttpResponse.json({ agentdecker: { title: "AgentDecker", prompt: "" } })),
  http.get("/api/config", () => HttpResponse.json({ default_project: "app" })),
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
  useTranscriptStore.setState({ byAgent: {} });
});
afterAll(() => server.close());

// INV §1: a stopped builder keeps its identity row in the agent store, so the
// persisted browser id must be classified by `running` — presence alone left a
// dead "Open AgentDecker chat" link pointing at the live route forever.
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

    await screen.findByRole("link", { name: "Open AgentDecker chat" });
    expect(localStorage.getItem(BUILDER_KEY)).toBe("a_builder");
  });
});
