import React from "react";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it } from "vitest";
import { VisualMatrix } from "./VisualMatrix";

afterEach(() => {
  cleanup();
  document.documentElement.removeAttribute("data-skin");
});

describe("VisualMatrix", () => {
  it("changes only presentation when the high-variance contract fixture is enabled", () => {
    const { container } = render(<MemoryRouter><VisualMatrix /></MemoryRouter>);
    const root = container.querySelector(".visual-matrix")!;
    expect(screen.getByLabelText("153,482 / 200,000 tokens · 74% context used")).toBeInTheDocument();
    const copyBefore = root.textContent;
    const routesBefore = [...root.querySelectorAll("a")].map((link) => link.getAttribute("href"));
    const actionsBefore = [...root.querySelectorAll("button")].map((button) => button.textContent);
    const statesBefore = [...root.querySelectorAll("[data-state]")].map((node) => node.getAttribute("data-state"));

    fireEvent.click(screen.getByRole("checkbox", { name: "High-variance contract" }));

    expect(root).toHaveClass("visual-matrix-high-variance");
    expect(root.textContent).toBe(copyBefore);
    expect([...root.querySelectorAll("a")].map((link) => link.getAttribute("href"))).toEqual(routesBefore);
    expect([...root.querySelectorAll("button")].map((button) => button.textContent)).toEqual(actionsBefore);
    expect([...root.querySelectorAll("[data-state]")].map((node) => node.getAttribute("data-state"))).toEqual(statesBefore);
  });

  // FS-12.A37 (superseding FS-02.A23 on agent cards): the fixture renders the real
  // AgentCard for every live state plus stopped and expanded, toned by state alone.
  it("renders every agent-card state from the real card", () => {
    const { container } = render(<MemoryRouter><VisualMatrix /></MemoryRouter>);
    const cards = [...container.querySelectorAll('[data-ui="agent-card"]')];
    expect(cards).toHaveLength(8);
    expect(cards.map((card) => card.getAttribute("data-state"))).toEqual(["busy", "idle", "waiting_input", "done", "error", "unknown", "stopped", "busy"]);
    expect(cards.every((card) => !(card.getAttribute("style") ?? "").includes("--ad-project-accent"))).toBe(true);
  });

  // FS-02.A40 and FS-12.A16/A37: the deterministic dashboard fixture carries
  // wrapped and unbroken names and the context row on collapsed and expanded cards.
  it("renders the card legibility and context-placement matrix", () => {
    const { container } = render(<MemoryRouter><VisualMatrix /></MemoryRouter>);
    expect(screen.getByText("Needs a decision from the orchestration and delivery reviewer")).toBeInTheDocument();
    expect(screen.getByText("unbroken-agent-name-that-must-not-escape-the-card-boundary")).toBeInTheDocument();
    const cards = [...container.querySelectorAll('[data-ui="agent-card"]')];
    expect(cards.every((card) => card.querySelector('[data-ui="context-meter"][data-state="compact"]'))).toBe(true);
  });

  it("renders the project color picker inside its context menu fixture", () => {
    render(<MemoryRouter><VisualMatrix /></MemoryRouter>);
    const menu = document.querySelector('[data-ui="context-menu"]')!;
    expect(menu.querySelectorAll(".project-color-preset")).toHaveLength(6);
  });

  // FS-12.A25: the deterministic chat fixture must retain the high-height states
  // used by the rendered width/skin check, not only the compact happy path.
  it("renders the staged runtime action and visible chat error state", () => {
    render(<MemoryRouter><VisualMatrix /></MemoryRouter>);
    expect(screen.getByRole("button", { name: "Switch" })).toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent("Runtime switch failed; current settings were restored.");
  });

  it("renders zero, one, five and overflowing active-project navigation fixtures", () => {
    render(<MemoryRouter><VisualMatrix /></MemoryRouter>);
    const fixture = screen.getByRole("heading", { name: "Active-project shell navigation" }).parentElement!;
    expect(fixture.querySelectorAll('nav[aria-label="Active projects"]')).toHaveLength(3);
    expect(fixture.querySelectorAll('button[aria-label="1 more active project"]')).toHaveLength(1);
    expect(screen.getAllByRole("link", { name: "Alpha" })).toHaveLength(3);
  });

  it("renders the timeline-first Pipelines contract fixture", () => {
    const { container } = render(<MemoryRouter><VisualMatrix /></MemoryRouter>);
    const run = container.querySelector('[data-ui="pipeline-run"]')!;
    expect(run.querySelector('[data-slot="live"]')).toBeTruthy();
    expect(run.querySelector('[data-slot="timeline"]')).toBeTruthy();
    expect(run.querySelectorAll('[data-slot="attempt"]')).toHaveLength(1);
    expect(run.querySelector('[data-slot="agents"]')).toBeTruthy();
  });

  it("switches between Core, Sky & Grove, and Studio without changing product structure", () => {
    const { container, unmount } = render(<MemoryRouter><VisualMatrix /></MemoryRouter>);
    const root = container.querySelector(".visual-matrix")!;
    const copyBefore = root.textContent;
    const routesBefore = [...root.querySelectorAll("a")].map((link) => link.getAttribute("href"));
    const actionsBefore = [...root.querySelectorAll("button")].map((button) => button.textContent);
    const statesBefore = [...root.querySelectorAll("[data-state]")].map((node) => node.getAttribute("data-state"));

    for (const skin of ["sky-grove", "studio"]) {
      fireEvent.change(screen.getByLabelText("Fixture appearance"), { target: { value: skin } });

      expect(document.documentElement.dataset.skin).toBe(skin);
      expect(root.textContent).toBe(copyBefore);
      expect([...root.querySelectorAll("a")].map((link) => link.getAttribute("href"))).toEqual(routesBefore);
      expect([...root.querySelectorAll("button")].map((button) => button.textContent)).toEqual(actionsBefore);
      expect([...root.querySelectorAll("[data-state]")].map((node) => node.getAttribute("data-state"))).toEqual(statesBefore);
    }

    unmount();
    expect(document.documentElement).not.toHaveAttribute("data-skin");
  });
});
