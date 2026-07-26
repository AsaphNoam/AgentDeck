import React from "react";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, describe, it, expect, vi } from "vitest";
import { useAgentStore } from "../../store/agentStore";
import { useAnnotationStore } from "../../store/annotationStore";
import { ChatPanel, initialTab } from "./ChatPanel";

vi.mock("../../api/client", () => ({
  getTranscript: vi.fn(async (id: string) => ({ agent_id: id, events: [] })),
}));

vi.mock("../../api/sse", () => ({
  sseClient: { setOpenAgent: vi.fn() },
}));

afterEach(() => {
  cleanup();
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
