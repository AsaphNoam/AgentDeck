import React from "react";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ContextBar } from "./ContextBar";

describe("ContextBar", () => {
  // FS-02.A13: a blank track resembles an unloaded placeholder, especially on
  // an otherwise active card, so zero must remain visibly meaningful.
  it("labels zero context usage", () => {
    render(<ContextBar value={0} />);
    expect(screen.getByLabelText("0% context used")).toHaveTextContent("0% context used");
  });

  it("labels and ramps context usage", () => {
    render(<ContextBar value={0.9} />);
    expect(screen.getByLabelText("90% context used")).toHaveClass("high");
  });

  it("keeps the shared value derivation in compact form", () => {
    render(<ContextBar value={2} compact />);
    expect(screen.getByLabelText("100% context used")).toHaveClass("high");
    expect(screen.getByLabelText("100% context used")).toHaveAttribute("data-variant", "compact");
  });
});
