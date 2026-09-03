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

  // TS-08.R14/R48: the compact form differs in presentation only, so it must still report its
  // low/medium/high tone through the contract attribute a skin can select on — density is a
  // separate dimension, not a replacement for the tone.
  it("keeps the shared value derivation and its tone in compact form", () => {
    render(<ContextBar value={2} compact />);
    const meter = screen.getByLabelText("100% context used");
    expect(meter).toHaveClass("high");
    expect(meter).toHaveAttribute("data-variant", "high");
    expect(meter).toHaveAttribute("data-state", "compact");
  });

  it("marks only the compact form with the density state", () => {
    render(<ContextBar value={0.7} />);
    const meter = screen.getByLabelText("70% context used");
    expect(meter).toHaveAttribute("data-variant", "medium");
    expect(meter).not.toHaveAttribute("data-state");
  });
});
