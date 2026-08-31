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
    expect(screen.getByText("0% context used")).toBeInTheDocument();
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

  // FS-02.A23: the visual fixture carries every declared project accent while
  // preserving the independent live-state top bars.
  it("renders all six project accents on dashboard cards", () => {
    const { container } = render(<MemoryRouter><VisualMatrix /></MemoryRouter>);
    const cards = [...container.querySelectorAll('[data-ui="agent-card"]')];
    expect(cards).toHaveLength(7);
    expect(cards.slice(0, 6).map((card) => card.getAttribute("style"))).toEqual([
      expect.stringContaining("rgb(100,116,139)"),
      expect.stringContaining("rgb(59,130,246)"),
      expect.stringContaining("rgb(34,197,94)"),
      expect.stringContaining("rgb(245,158,11)"),
      expect.stringContaining("rgb(244,63,94)"),
      expect.stringContaining("rgb(139,92,246)"),
    ]);
  });

  it("renders the project color picker inside its context menu fixture", () => {
    render(<MemoryRouter><VisualMatrix /></MemoryRouter>);
    const menu = document.querySelector('[data-ui="context-menu"]')!;
    expect(menu.querySelectorAll(".project-color-preset")).toHaveLength(6);
  });

  it("renders zero, one, and overflowing active-project navigation fixtures", () => {
    render(<MemoryRouter><VisualMatrix /></MemoryRouter>);
    const fixture = screen.getByRole("heading", { name: "Active-project shell navigation" }).parentElement!;
    expect(fixture.querySelectorAll('nav[aria-label="Active projects"]')).toHaveLength(2);
    expect(fixture.querySelectorAll('button[aria-label="1 more active project"]')).toHaveLength(1);
    expect(screen.getAllByRole("link", { name: "Alpha" })).toHaveLength(2);
  });

  it("renders the timeline-first Pipelines contract fixture", () => {
    const { container } = render(<MemoryRouter><VisualMatrix /></MemoryRouter>);
    const run = container.querySelector('[data-ui="pipeline-run"]')!;
    expect(run.querySelector('[data-slot="live"]')).toBeTruthy();
    expect(run.querySelector('[data-slot="timeline"]')).toBeTruthy();
    expect(run.querySelectorAll('[data-slot="attempt"]')).toHaveLength(1);
    expect(run.querySelector('[data-slot="agents"]')).toBeTruthy();
  });

  it("switches between Core and Sky & Grove without changing product structure", () => {
    const { container, unmount } = render(<MemoryRouter><VisualMatrix /></MemoryRouter>);
    const root = container.querySelector(".visual-matrix")!;
    const copyBefore = root.textContent;
    const routesBefore = [...root.querySelectorAll("a")].map((link) => link.getAttribute("href"));
    const actionsBefore = [...root.querySelectorAll("button")].map((button) => button.textContent);
    const statesBefore = [...root.querySelectorAll("[data-state]")].map((node) => node.getAttribute("data-state"));

    fireEvent.change(screen.getByLabelText("Fixture appearance"), { target: { value: "sky-grove" } });

    expect(document.documentElement.dataset.skin).toBe("sky-grove");
    expect(root.textContent).toBe(copyBefore);
    expect([...root.querySelectorAll("a")].map((link) => link.getAttribute("href"))).toEqual(routesBefore);
    expect([...root.querySelectorAll("button")].map((button) => button.textContent)).toEqual(actionsBefore);
    expect([...root.querySelectorAll("[data-state]")].map((node) => node.getAttribute("data-state"))).toEqual(statesBefore);

    unmount();
    expect(document.documentElement).not.toHaveAttribute("data-skin");
  });
});
