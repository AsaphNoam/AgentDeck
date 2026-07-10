import React from "react";
import { describe, it, expect } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { DndContext } from "@dnd-kit/core";
import { SortableContext, rectSortingStrategy } from "@dnd-kit/sortable";
import { AgentCard } from "./AgentCard";

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
                  />
                )}
              />
              <Route path="/agent/:id" element={<div>Chat view</div>} />
            </Routes>
          </SortableContext>
        </DndContext>
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByText("Atlas"));

    expect(screen.getByText("Chat view")).toBeInTheDocument();
  });
});
