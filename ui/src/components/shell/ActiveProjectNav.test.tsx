import React from "react";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { AgentState } from "../../api/types";
import { useAgentStore } from "../../store/agentStore";
import { ActiveProjectNav, deriveActiveProjects } from "./ActiveProjectNav";

const mocks = vi.hoisted(() => ({ useProjects: vi.fn() }));

vi.mock("../../api/config", () => ({ useProjects: mocks.useProjects }));

const projects = Object.fromEntries(
  ["Alpha", "Bravo", "Charlie", "Delta", "Echo", "Foxtrot"].map((title, index) => [
    title.toLowerCase(),
    { title, color: [20 + index, 40, 60] as [number, number, number], cwd: `/tmp/${title}`, add_dirs: [], context_prompt: "", archived: false },
  ]),
);

function agent(id: string, project: string, running = true, archived = false): AgentState {
  return {
    agent_id: id, name: id, role: "implementer", project, backend: "test", model: "test",
    interface: "chat", created_at: "2026-08-31T00:00:00Z", running, state: "idle", detail: "",
    context_pct: 0, updated_at: 0, archived,
  };
}

beforeEach(() => {
  mocks.useProjects.mockReturnValue({ data: projects });
  useAgentStore.setState({ agents: {}, order: [], hydrated: true, hydrating: false });
});

afterEach(() => {
  cleanup();
  mocks.useProjects.mockReset();
});

describe("deriveActiveProjects", () => {
  it("derives live membership and excludes stopped, archived, and unavailable entries", () => {
    const agents = {
      alpha: agent("alpha", "alpha"),
      stopped: agent("stopped", "bravo", false),
      archived: agent("archived", "charlie", true, true),
      missing: agent("missing", "missing"),
    };

    expect(deriveActiveProjects(projects, agents, undefined).visible.map(({ id }) => id)).toEqual(["alpha"]);
    expect(deriveActiveProjects(projects, {}, "bravo").visible.map(({ id }) => id)).toEqual(["bravo"]);
  });

  it("keeps a later current project visible and alphabetizes both sets", () => {
    const agents = Object.fromEntries(Object.keys(projects).map((id) => [id, agent(id, id)]));

    const result = deriveActiveProjects(projects, agents, "foxtrot");
    expect(result.visible.map(({ id }) => id)).toEqual(["alpha", "bravo", "charlie", "delta", "foxtrot"]);
    expect(result.overflow.map(({ id }) => id)).toEqual(["echo"]);
  });
});

describe("ActiveProjectNav", () => {
  it("selects an agent's project and exposes overflow navigation", () => {
    useAgentStore.setState({
      agents: Object.fromEntries(Object.keys(projects).map((id) => [id, agent(id, id)])),
      order: Object.keys(projects), hydrated: true, hydrating: false,
    });

    render(
      <MemoryRouter initialEntries={["/agent/foxtrot"]}>
        <Routes><Route path="/agent/:agentId" element={<ActiveProjectNav />} /></Routes>
      </MemoryRouter>,
    );

    expect(screen.getByRole("link", { name: "Foxtrot" })).toHaveClass("active");
    expect(screen.queryByRole("link", { name: "Echo" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "1 more active project" }));
    expect(screen.getByRole("menuitem", { name: "Echo" })).toHaveAttribute("href", "/project/echo");
    fireEvent.keyDown(screen.getByRole("button", { name: "1 more active project" }), { key: "Escape" });
    expect(screen.queryByRole("menuitem", { name: "Echo" })).not.toBeInTheDocument();
  });

  it("reacts to store membership changes without a retained copy", () => {
    render(<MemoryRouter><ActiveProjectNav /></MemoryRouter>);
    expect(screen.queryByRole("navigation", { name: "Active projects" })).not.toBeInTheDocument();

    act(() => useAgentStore.getState().applyStateUpdate(agent("alpha", "alpha")));
    expect(screen.getByRole("link", { name: "Alpha" })).toBeInTheDocument();

    act(() => useAgentStore.getState().hydrateComplete([]));
    expect(screen.queryByRole("navigation", { name: "Active projects" })).not.toBeInTheDocument();
  });
});
