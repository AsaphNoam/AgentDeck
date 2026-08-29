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
                    projectTitle="AgentDeck demo"
                  />
                )}
              />
              <Route path="/agent/:id" element={<div>Chat view</div>} />
            </Routes>
          </SortableContext>
        </DndContext>
      </MemoryRouter>,
    );

    expect(screen.getByText("implementer · AgentDeck demo")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Atlas"));

    expect(screen.getByText("Chat view")).toBeInTheDocument();
  });

  it("falls back to the project id when its title is unavailable", () => {
    render(
      <MemoryRouter>
        <DndContext>
          <SortableContext items={["a_1"]} strategy={rectSortingStrategy}>
            <AgentCard agent={{
              agent_id: "a_1", name: "Atlas", role: "implementer", project: "agentdeck-v0-1-2-demo-20260726t230903z",
              backend: "claude", model: "sonnet", interface: "chat", state: "idle",
              detail: "ready", running: true, context_pct: 0,
            }} />
          </SortableContext>
        </DndContext>
      </MemoryRouter>,
    );

    expect(screen.getByText("implementer · agentdeck-v0-1-2-demo-20260726t230903z")).toBeInTheDocument();
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

  // FS-02.A34 — the boundary is the region, not a list of exempted controls: a
  // click and a right-click inside the pane's content reach neither the toggle nor
  // the card menu, the drag grip is withheld, and the header still collapses.
  it("narrows expanded activation and context menu handling to the header", () => {
    const toggle = vi.fn();
    render(
      <MemoryRouter>
        <DndContext>
          <SortableContext items={[]} strategy={rectSortingStrategy}>
            <AgentCard expanded onToggle={toggle} agent={{
              agent_id: "a_1", name: "Atlas", role: "implementer", project: "my-app",
              backend: "claude", model: "sonnet", interface: "chat", state: "idle",
              detail: "ready", running: true, context_pct: 0,
            }}><button type="button">Send</button></AgentCard>
          </SortableContext>
        </DndContext>
      </MemoryRouter>,
    );

    expect(screen.queryByLabelText("Reorder Atlas")).not.toBeInTheDocument();
    fireEvent.click(screen.getByText("Send"));
    expect(toggle).not.toHaveBeenCalled();
    fireEvent.contextMenu(screen.getByText("Send"));
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    fireEvent.click(screen.getByText("idle").closest('[data-slot="header"]')!);
    expect(toggle).toHaveBeenCalledOnce();
  });
});
