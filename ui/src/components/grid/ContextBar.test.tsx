import React from "react";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ContextBar } from "./ContextBar";

describe("ContextBar", () => {
  it("labels and ramps context usage", () => {
    render(<ContextBar value={0.9} />);
    expect(screen.getByLabelText("90% context used")).toHaveClass("high");
  });
});
