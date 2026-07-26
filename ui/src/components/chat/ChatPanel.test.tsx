import React from "react";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
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

describe("ChatPanel missing-agent recovery", () => {
  it("lets a retained annotation tray be discarded when its source is gone", () => {
    useAnnotationStore.getState().add("a_gone", { seq: 3, excerpt: "line", instruction: "check" });

    render(
      <MemoryRouter initialEntries={["/agent/a_gone"]}>
        <Routes><Route path="/agent/:id" element={<ChatPanel />} /></Routes>
      </MemoryRouter>,
    );

    expect(screen.getByText("1 pending annotation cannot be sent because the source agent no longer exists.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Discard pending annotations" }));
    expect(useAnnotationStore.getState().bySource.a_gone).toBeUndefined();
  });
});
