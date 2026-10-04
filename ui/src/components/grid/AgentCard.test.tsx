import React from "react";
import { afterEach, describe, it, expect, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { DndContext } from "@dnd-kit/core";
import { SortableContext, rectSortingStrategy } from "@dnd-kit/sortable";
import { AgentCard } from "./AgentCard";

afterEach(cleanup);

describe("AgentCard", () => {
  it("navigates to the agent chat when the card is clicked", () => {
    render(
      <MemoryRouter initialEntries={["/"]}>
        <DndContext>
          <SortableContext items={["a_1"]} strategy={rectSortingStrategy}>
            <Routes>
              <Route
                path="/"
                element={(
                  <AgentCard
                    agent={{
                      agent_id: "a_1",
                      name: "Atlas",
                      role: "implementer",
                      project: "my-app",
                      backend: "claude",
                      model: "sonnet-5",
                      interface: "chat",
                      state: "idle",
                      detail: "ready",
                      running: true,
                      context_pct: 0,
                    }}
                    projectTitle="Chuck demo"
                  />
                )}
              />
              <Route path="/agent/:id" element={<div>Chat view</div>} />
            </Routes>
          </SortableContext>
        </DndContext>
      </MemoryRouter>,
    );

    expect(screen.getByText("implementer · Chuck demo")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Atlas"));

    expect(screen.getByText("Chat view")).toBeInTheDocument();
  });

  it("falls back to the project id when its title is unavailable", () => {
    render(
      <MemoryRouter>
        <DndContext>
          <SortableContext items={["a_1"]} strategy={rectSortingStrategy}>
            <AgentCard agent={{
              agent_id: "a_1", name: "Atlas", role: "implementer", project: "chuck-v0-1-2-demo-20260726t230903z",
              backend: "claude", model: "sonnet", interface: "chat", state: "idle",
              detail: "ready", running: true, context_pct: 0,
            }} />
          </SortableContext>
        </DndContext>
      </MemoryRouter>,
    );

    expect(screen.getByText("implementer · chuck-v0-1-2-demo-20260726t230903z")).toBeInTheDocument();
  });

  it("links an associated stage agent back to its pipeline run", () => {
    render(
      <MemoryRouter initialEntries={["/"]}>
        <DndContext>
          <SortableContext items={["a_1"]} strategy={rectSortingStrategy}>
            <Routes>
              <Route path="/" element={<AgentCard agent={{
                agent_id: "a_1", name: "Atlas", role: "implementer", project: "my-app",
                backend: "claude", model: "sonnet", interface: "chat", state: "busy",
                detail: "working", running: true, context_pct: 0,
                pipeline: { run_id: "pr_1", run_name: "Delivery", stage_id: "work", attempt_id: "pa_1", attempt_no: 2 },
              }} />} />
              <Route path="/pipelines/runs/:runID" element={<div>Pipeline run</div>} />
            </Routes>
          </SortableContext>
        </DndContext>
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByText("Delivery · work · attempt 2"));

    expect(screen.getByText("Pipeline run")).toBeInTheDocument();
  });

  // FS-02.A34/A38/A41 — the boundary is the region, not a list of exempted controls: a
  // click and a right-click inside the pane's content reach neither the toggle nor
  // the card menu, the drag grip is withheld, and the header still collapses.
  it("narrows expanded activation to the header and exposes one labelled collapse control", () => {
    const toggle = vi.fn();
    render(
      <MemoryRouter>
        <DndContext>
          <SortableContext items={[]} strategy={rectSortingStrategy}>
            <AgentCard expanded onToggle={toggle} agent={{
              agent_id: "a_1", name: "Atlas", role: "implementer", project: "my-app",
              backend: "claude", model: "sonnet", interface: "chat", state: "idle",
              detail: "ready", running: true, context_pct: 0.72,
            }}><button type="button">Send</button></AgentCard>
          </SortableContext>
        </DndContext>
      </MemoryRouter>,
    );

    expect(screen.queryByLabelText("Reorder Atlas")).not.toBeInTheDocument();
    expect(screen.getByLabelText("72% context used")).toHaveAttribute("data-state", "compact");
    fireEvent.click(screen.getByText("Send"));
    expect(toggle).not.toHaveBeenCalled();
    fireEvent.contextMenu(screen.getByText("Send"));
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    const collapse = screen.getByRole("button", { name: "Collapse" });
    fireEvent.click(collapse);
    expect(toggle).toHaveBeenCalledOnce();
    fireEvent.click(screen.getByText("idle").closest('[data-slot="header"]')!);
    expect(toggle).toHaveBeenCalledTimes(2);
  });

  // FS-02.A41 — collapsed cards carry no context figure; the same shared
  // ContextBar appears in compact form only after expansion.
  it("omits context usage while collapsed", () => {
    render(
      <MemoryRouter><DndContext><SortableContext items={["a_1"]} strategy={rectSortingStrategy}>
        <AgentCard agent={{
          agent_id: "a_1", name: "Atlas", role: "implementer", project: "my-app",
          backend: "claude", model: "sonnet", interface: "chat", state: "idle",
          detail: "ready", running: true, context_pct: 0,
        }} />
      </SortableContext></DndContext></MemoryRouter>,
    );

    expect(screen.queryByLabelText("0% context used")).not.toBeInTheDocument();
  });

  // FS-02.R62/R63, A44/A45: the expanded card states the exact figure and the
  // same backend/model/effort runtime identity already shown collapsed.
  it("shows the exact context figure and runtime metadata while expanded", () => {
    const { rerender } = render(
      <MemoryRouter><DndContext><SortableContext items={["a_1"]} strategy={rectSortingStrategy}>
        <AgentCard expanded agent={{
          agent_id: "a_1", name: "Atlas", role: "implementer", project: "my-app",
          backend: "claude", model: "sonnet", effort: "high", interface: "chat", state: "idle",
          detail: "ready", running: true, context_pct: 0.06, context_used: 12345, context_size: 200000,
        }} />
      </SortableContext></DndContext></MemoryRouter>,
    );

    expect(screen.getByLabelText("12,345 / 200,000 tokens · 6% context used")).toBeInTheDocument();
    expect(screen.getByText("claude · sonnet · high")).toBeInTheDocument();

    // A live update replaces both numbers (and the percentage) in place.
    rerender(
      <MemoryRouter><DndContext><SortableContext items={["a_1"]} strategy={rectSortingStrategy}>
        <AgentCard expanded agent={{
          agent_id: "a_1", name: "Atlas", role: "implementer", project: "my-app",
          backend: "claude", model: "sonnet", effort: "high", interface: "chat", state: "idle",
          detail: "ready", running: true, context_pct: 0.5, context_used: 100000, context_size: 200000,
        }} />
      </SortableContext></DndContext></MemoryRouter>,
    );

    expect(screen.getByLabelText("100,000 / 200,000 tokens · 50% context used")).toBeInTheDocument();
    expect(screen.queryByLabelText("12,345 / 200,000 tokens · 6% context used")).not.toBeInTheDocument();
  });

  // An empty effort must not leave a dangling " · " separator in the reused
  // runtime-identity string (FS-02.R63/A45).
  it("renders the runtime identity with no dangling separator when effort is empty", () => {
    render(
      <MemoryRouter><DndContext><SortableContext items={["a_1"]} strategy={rectSortingStrategy}>
        <AgentCard expanded agent={{
          agent_id: "a_1", name: "Atlas", role: "implementer", project: "my-app",
          backend: "claude", model: "sonnet", interface: "chat", state: "idle",
          detail: "ready", running: true, context_pct: 0,
        }} />
      </SortableContext></DndContext></MemoryRouter>,
    );

    expect(screen.getByText("claude · sonnet")).toBeInTheDocument();
  });
});
