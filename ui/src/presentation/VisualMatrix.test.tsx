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
