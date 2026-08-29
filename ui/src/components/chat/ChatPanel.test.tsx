import React from "react";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, it, expect, vi } from "vitest";
import { useAgentStore } from "../../store/agentStore";
import { useAnnotationStore } from "../../store/annotationStore";
import { ChatPanel, initialTab } from "./ChatPanel";

const mocks = vi.hoisted(() => ({
  getTranscript: vi.fn(async (id: string) => ({ agent_id: id, events: [] })),
  switchRuntime: vi.fn(),
  useBackends: vi.fn(),
  useProjects: vi.fn(),
}));

vi.mock("../../api/client", () => ({
  getTranscript: mocks.getTranscript,
  switchRuntime: mocks.switchRuntime,
}));

vi.mock("../../api/config", async (importOriginal) => ({
  ...await importOriginal<typeof import("../../api/config")>(),
  useBackends: mocks.useBackends,
  useProjects: mocks.useProjects,
}));

vi.mock("../../api/sse", () => ({
  sseClient: { registerOpenAgent: vi.fn(() => vi.fn()) },
}));

beforeEach(() => {
  mocks.useBackends.mockReturnValue({ data: undefined });
  mocks.useProjects.mockReturnValue({ data: undefined });
});

afterEach(() => {
  cleanup();
  mocks.switchRuntime.mockReset();
  mocks.useBackends.mockReset();
  mocks.useProjects.mockReset();
  useAgentStore.setState({ agents: {}, order: [], hydrated: false, hydrating: false });
  useAnnotationStore.setState({ bySource: {}, overallBySource: {}, editedAt: {} });
});

// initialTab drives which tab a chat panel opens on. The load-bearing case for
// the Finding 9 secondary fix: a terminal-interface agent must default to the
// Terminal tab so a WS attaches after launch (chat agents stay on transcript).
describe("initialTab", () => {
  it("defaults a terminal-interface agent to the Terminal tab", () => {
    expect(initialTab(null, "terminal")).toBe("terminal");
  });

  it("defaults a chat-interface agent to the transcript tab", () => {
    expect(initialTab(null, "acp")).toBe("transcript");
    expect(initialTab(null, undefined)).toBe("transcript");
  });

  it("honors an explicit ?tab= over the interface default", () => {
    expect(initialTab("terminal", "acp")).toBe("terminal");
    expect(initialTab("files", "terminal")).toBe("files");
  });
});

function renderPanel(id: string) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[`/agent/${id}`]}>
        <Routes><Route path="/agent/:id" element={<ChatPanel />} /></Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function liveAgent(id: string) {
  return {
    agent_id: id, name: "Nova", role: "implementer", project: "app", backend: "claude",
    model: "sonnet", interface: "chat", running: true, state: "idle", context_pct: 12,
    created_at: "2026-07-26T00:00:00Z",
  } as never;
}

const backends = {
  version: 2 as const,
  backends: {
    claude: {
      name: "Claude", type: "claude-acp" as const, default_model: "sonnet",
      models: { sonnet: { name: "Sonnet", model: "sonnet" } },
    },
    codex: {
      name: "Codex", type: "codex-acp" as const, default_model: "gpt-5",
      models: { "gpt-5": { name: "GPT-5", model: "gpt-5", efforts: ["low", "high"], default_effort: "high" } },
    },
  },
};

// FS-13.R16/A8: the missing-source recovery is destructive, so it may only claim
// a source is gone once agent hydration has actually looked for it.
describe("ChatPanel missing-agent recovery", () => {
  it("lets a retained annotation tray be discarded when its source is gone", () => {
    useAgentStore.setState({ agents: {}, order: [], hydrated: true, hydrating: false });
    useAnnotationStore.getState().add("a_gone", { seq: 3, excerpt: "line", instruction: "check" });

    renderPanel("a_gone");

    expect(screen.getByText("1 pending annotation cannot be sent because the source agent no longer exists.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Discard pending annotations" }));
    expect(useAnnotationStore.getState().bySource.a_gone).toBeUndefined();
  });

  it("does not offer to discard drafts before the first hydration completes", () => {
    useAgentStore.setState({ agents: {}, order: [], hydrated: false, hydrating: true });
    useAnnotationStore.getState().add("a_pending", { seq: 3, excerpt: "line", instruction: "check" });

    renderPanel("a_pending");

    expect(screen.queryByText("Agent not found")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Discard pending annotations" })).not.toBeInTheDocument();
    expect(screen.getByText("Loading agent…")).toBeInTheDocument();
    expect(useAnnotationStore.getState().bySource.a_pending).toHaveLength(1);
  });

  it("shows the live workspace when hydration delivers the source", () => {
    useAgentStore.setState({ agents: { a_live: liveAgent("a_live") }, order: ["a_live"], hydrated: true, hydrating: false });
    useAnnotationStore.getState().add("a_live", { seq: 3, excerpt: "line", instruction: "check" });

    renderPanel("a_live");

    expect(screen.getByRole("heading", { name: "Nova" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Discard pending annotations" })).not.toBeInTheDocument();
    expect(useAnnotationStore.getState().bySource.a_live).toHaveLength(1);
  });
});

// FS-03.A12 (R27) — the chat header Back link targets the agent's project
// dashboard only when that project is a current, non-archived catalog member;
// otherwise it falls back to the projects home so a removed/archived project
// never strands the user on a dead-end route.
describe("ChatPanel back target", () => {
  it("targets the project dashboard for an active catalog project", () => {
    useAgentStore.setState({ agents: { a_live: liveAgent("a_live") }, order: ["a_live"], hydrated: true, hydrating: false });
    mocks.useProjects.mockReturnValue({ data: { app: { title: "App", archived: false } } });

    renderPanel("a_live");

    expect(screen.getByRole("link", { name: "Back" })).toHaveAttribute("href", "/project/app");
  });

  it("falls back to the projects home when the project is absent from the catalog", () => {
    useAgentStore.setState({ agents: { a_live: liveAgent("a_live") }, order: ["a_live"], hydrated: true, hydrating: false });
    mocks.useProjects.mockReturnValue({ data: {} });

    renderPanel("a_live");

    expect(screen.getByRole("link", { name: "Back" })).toHaveAttribute("href", "/");
  });

  it("falls back to the projects home when the project is archived", () => {
    useAgentStore.setState({ agents: { a_live: liveAgent("a_live") }, order: ["a_live"], hydrated: true, hydrating: false });
    mocks.useProjects.mockReturnValue({ data: { app: { title: "App", archived: true } } });

    renderPanel("a_live");

    expect(screen.getByRole("link", { name: "Back" })).toHaveAttribute("href", "/");
  });
});

// FS-03.A9 — the chat header is a second, explicit entry point to the existing
// runtime switch. It shares backend/model/effort reset behavior with launch and
// the dashboard switch dialog.
describe("ChatPanel runtime picker", () => {
  it("resets the model and effort, then switches a running chat agent", async () => {
    mocks.useBackends.mockReturnValue({ data: backends });
    mocks.switchRuntime.mockResolvedValue({ history_handoff: "native_resume" });
    useAgentStore.setState({ agents: { a_live: liveAgent("a_live") }, order: ["a_live"], hydrated: true, hydrating: false });

    renderPanel("a_live");

    const backend = await screen.findByLabelText("Backend");
    expect((screen.getByLabelText("Model") as HTMLSelectElement).value).toBe("sonnet");
    expect(screen.queryByLabelText("Effort")).not.toBeInTheDocument();
    fireEvent.change(backend, { target: { value: "codex" } });

    expect((screen.getByLabelText("Model") as HTMLSelectElement).value).toBe("gpt-5");
    expect((screen.getByLabelText("Effort") as HTMLSelectElement).value).toBe("high");
    fireEvent.click(screen.getByRole("button", { name: "Switch" }));

    await waitFor(() => expect(mocks.switchRuntime).toHaveBeenCalledWith("a_live", { backend: "codex", model: "gpt-5", effort: "high" }));
  });

  it("restores the current runtime and explains a rejected switch", async () => {
    mocks.useBackends.mockReturnValue({ data: backends });
    mocks.switchRuntime.mockRejectedValue(new Error("no runtime change requested"));
    useAgentStore.setState({ agents: { a_live: liveAgent("a_live") }, order: ["a_live"], hydrated: true, hydrating: false });

    renderPanel("a_live");

    fireEvent.change(await screen.findByLabelText("Backend"), { target: { value: "codex" } });
    fireEvent.click(screen.getByRole("button", { name: "Switch" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("no runtime change requested");
    expect((screen.getByLabelText("Backend") as HTMLSelectElement).value).toBe("claude");
    expect(screen.queryByRole("button", { name: "Switch" })).not.toBeInTheDocument();
  });

  it("keeps stopped agents' runtime identity static", () => {
    mocks.useBackends.mockReturnValue({ data: backends });
    useAgentStore.setState({ agents: { a_stopped: { ...liveAgent("a_stopped"), running: false } }, order: ["a_stopped"], hydrated: true, hydrating: false });

    renderPanel("a_stopped");

    expect(screen.getByText("claude · sonnet")).toBeInTheDocument();
    expect(screen.queryByLabelText("Backend")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Switch" })).not.toBeInTheDocument();
  });

  it("keeps an unavailable current runtime visible until a listed target is chosen", async () => {
    mocks.useBackends.mockReturnValue({ data: { version: 2, backends: { codex: backends.backends.codex } } });
    useAgentStore.setState({ agents: { a_live: liveAgent("a_live") }, order: ["a_live"], hydrated: true, hydrating: false });

    renderPanel("a_live");

    expect((await screen.findByLabelText("Backend") as HTMLSelectElement).value).toBe("claude");
    expect((screen.getByLabelText("Model") as HTMLSelectElement).value).toBe("sonnet");
    expect(screen.queryByRole("button", { name: "Switch" })).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Backend"), { target: { value: "codex" } });
    expect(screen.getByRole("button", { name: "Switch" })).toBeEnabled();
  });
});
