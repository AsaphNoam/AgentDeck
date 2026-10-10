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
    render(<ContextBar value={2} />);
    const meter = screen.getByLabelText("100% context used");
    expect(meter).toHaveClass("high");
    expect(meter).toHaveAttribute("data-variant", "high");
    expect(meter).toHaveAttribute("data-state", "compact");
  });

  // FS-12.R65: the card row leads with the percentage and follows with the exact pair.
  it("labels the card row with Context and the percentage before the pair", () => {
    render(<ContextBar value={0.26} used={52680} size={200000} />);
    const meter = screen.getByLabelText("53K / 200K tokens · 26% context used");
    expect(meter).toHaveTextContent("Context26% · 53K / 200K tokens");
  });

  it("marks only the card form with the density state", () => {
    render(<ContextBar value={0.7} detailed />);
    const meter = screen.getByLabelText("70% context used");
    expect(meter).toHaveAttribute("data-variant", "medium");
    expect(meter).not.toHaveAttribute("data-state");
  });

  // FS-12.R67 (superseding FS-02.R62's unabridged display): the pair reads in whole
  // thousands beside the existing rounded percentage, never a second label.
  it("renders the used/size pair in whole thousands beside the percentage", () => {
    render(<ContextBar value={0.06} used={12345} size={200000} />);
    expect(screen.getByLabelText("12K / 200K tokens · 6% context used")).toBeInTheDocument();
  });

  it("rounds to the nearest thousand and shows counts below 1,000 as is", () => {
    render(<ContextBar value={0.01} used={999} size={1500} />);
    expect(screen.getByLabelText("999 / 2K tokens · 1% context used")).toBeInTheDocument();
  });

  it("renders zero used as zero rather than an empty track", () => {
    render(<ContextBar value={0} used={0} size={200000} />);
    expect(screen.getByLabelText("0 / 200K tokens · 0% context used")).toBeInTheDocument();
  });

  // A1.R62: a reported used count beyond size is still reported as given;
  // only the percentage is capped by the shared clamp.
  it("keeps the reported counts when used exceeds size, capping only the percentage", () => {
    render(<ContextBar value={1.25} used={250000} size={200000} />);
    expect(screen.getByLabelText("250K / 200K tokens · 100% context used")).toBeInTheDocument();
  });

  it("falls back to the percentage-only label when the pair is absent", () => {
    render(<ContextBar value={0.5} />);
    expect(screen.getByLabelText("50% context used")).toBeInTheDocument();
  });

  it("falls back to the percentage-only label when only one of the pair is present", () => {
    render(<ContextBar value={0.42} used={100000} />);
    expect(screen.getByLabelText("42% context used")).toBeInTheDocument();
    render(<ContextBar value={0.42} size={200000} />);
    expect(screen.getAllByLabelText("42% context used")).toHaveLength(2);
  });
});
