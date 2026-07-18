import React from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { VisualMatrix } from "./VisualMatrix";

describe("VisualMatrix", () => {
  it("changes only presentation when the high-variance contract fixture is enabled", () => {
    const { container } = render(<MemoryRouter><VisualMatrix /></MemoryRouter>);
    const root = container.querySelector(".visual-matrix")!;
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
});
