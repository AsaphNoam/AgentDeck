import React from "react";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { GroupPicker } from "./GroupPicker";

afterEach(cleanup);

describe("GroupPicker", () => {
  it("shows choices on focus, filters case-insensitively, and reuses exact labels", () => {
    const onChange = vi.fn();
    render(<GroupPicker value="" groups={["Core", "Frontend"]} onChange={onChange} />);
    const input = screen.getByRole("combobox", { name: "Group" });
    fireEvent.focus(input);
    expect(screen.getByRole("button", { name: "Core" })).toBeInTheDocument();
    fireEvent.change(input, { target: { value: "front" } });
    expect(screen.getByRole("button", { name: "Frontend" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Frontend" }));
    expect(onChange).toHaveBeenCalledWith("Frontend");
  });

  it("offers explicit create, blank ungrouped, and rejects the reserved key", () => {
    const onChange = vi.fn();
    render(<GroupPicker value="" groups={[]} onChange={onChange} />);
    const input = screen.getByRole("combobox", { name: "Group" });
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: " New team " } });
    fireEvent.click(screen.getByRole("button", { name: /Create group/ }));
    expect(onChange).toHaveBeenCalledWith("New team");
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: "_ungrouped" } });
    expect(screen.getByRole("status")).toHaveTextContent("reserved");
    expect(screen.queryByRole("button", { name: /Create group/ })).toBeNull();
  });
});
