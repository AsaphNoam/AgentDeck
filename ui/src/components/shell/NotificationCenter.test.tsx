import React from "react";
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { render, screen, act, cleanup, fireEvent } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useParams } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { NotificationCenter } from "./NotificationCenter";
import { ChatPanel } from "../chat/ChatPanel";
import { useAgentStore } from "../../store/agentStore";
import { useUiStore } from "../../store/uiStore";
import type { NotificationPayload } from "../../api/types";

vi.mock("../../api/config", async (importOriginal) => ({
  ...await importOriginal<typeof import("../../api/config")>(),
  useBackends: () => ({ data: undefined }),
  useProjects: () => ({ data: undefined }),
}));

vi.mock("../../api/sse", () => ({
  sseClient: { registerOpenAgent: () => () => {} },
}));

function AgentRoute() {
  return <p>conversation {useParams().id}</p>;
}

function renderCenter() {
  return render(
    <MemoryRouter initialEntries={["/tasks"]}>
      <Routes>
        <Route path="/tasks" element={<p>tasks page</p>} />
        <Route path="/agent/:id" element={<AgentRoute />} />
      </Routes>
      <NotificationCenter />
    </MemoryRouter>,
  );
}

function notify(type: NotificationPayload["notification_type"], agentId: string, title: string) {
  act(() => {
    useUiStore.getState().pushToast({ type: "notification", notification_type: type, agent_id: agentId, title, ts: title });
  });
}

describe("NotificationCenter per-toast timers", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    useUiStore.setState({ toasts: [] });
  });
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  // Regression (review fix): each toast auto-dismisses on its own 6s clock. The
  // previous single effect depended on the whole toasts array, so a newly pushed
  // toast restarted every timer and older toasts lingered.
  it("dismisses each toast independently; a new toast does not restart older timers", () => {
    renderCenter();

    act(() => {
      useUiStore.getState().pushError("first");
    });
    act(() => {
      vi.advanceTimersByTime(3_000);
    });
    act(() => {
      useUiStore.getState().pushError("second");
    });
    // 3s later: "first" has lived 6s → gone; "second" only 3s → still shown.
    act(() => {
      vi.advanceTimersByTime(3_000);
    });

    expect(screen.queryByText("first")).not.toBeInTheDocument();
    expect(screen.getByText("second")).toBeInTheDocument();
  });
});

describe("NotificationCenter opens the agent's conversation (FS-02.A46)", () => {
  beforeEach(() => useUiStore.setState({ toasts: [] }));
  afterEach(() => {
    cleanup();
    useAgentStore.setState({ agents: {}, order: [], hydrated: false, hydrating: false });
  });

  it("opens the conversation for every agent notification type and dismisses the toast", () => {
    renderCenter();
    for (const type of ["permission_required", "waiting_input", "done", "budget_exceeded"] as const) {
      notify(type, "a 1/x", `${type} toast`);
      fireEvent.click(screen.getByText(`${type} toast`));
      expect(screen.getByText("conversation a 1/x")).toBeInTheDocument();
      expect(screen.queryByText(`${type} toast`)).not.toBeInTheDocument();
    }
  });

  it("close control dismisses without navigating", () => {
    renderCenter();
    notify("permission_required", "a_1", "Needs permission");
    fireEvent.click(screen.getByRole("button", { name: "Dismiss notification" }));
    expect(screen.queryByText("Needs permission")).not.toBeInTheDocument();
    expect(screen.getByText("tasks page")).toBeInTheDocument();
  });

  // The agent vanished after its toast was raised: the real conversation route
  // must show its existing recovery view rather than a stale conversation.
  it("a toast for an agent removed before the click shows Agent not found", () => {
    useAgentStore.setState({ agents: {}, order: [], hydrated: true, hydrating: false });
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter initialEntries={["/tasks"]}>
          <Routes>
            <Route path="/tasks" element={<p>tasks page</p>} />
            <Route path="/agent/:id" element={<ChatPanel />} />
          </Routes>
          <NotificationCenter />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    notify("permission_required", "gone", "Needs permission");
    fireEvent.click(screen.getByText("Needs permission"));
    expect(screen.getByRole("heading", { name: "Agent not found" })).toBeInTheDocument();
    expect(screen.queryByText("Needs permission")).not.toBeInTheDocument();
  });

  it("error toast only dismisses", () => {
    renderCenter();
    act(() => useUiStore.getState().pushError("Save failed"));
    fireEvent.click(screen.getByText("Save failed"));
    expect(screen.queryByText("Save failed")).not.toBeInTheDocument();
    expect(screen.getByText("tasks page")).toBeInTheDocument();
  });
});
