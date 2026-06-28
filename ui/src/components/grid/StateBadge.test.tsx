import React from "react";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { StateBadge } from "./StateBadge";

describe("StateBadge", () => {
  it("renders waiting_input as Waiting with a state class", () => {
    render(<StateBadge state="waiting_input" />);
    expect(screen.getByTestId("state-badge")).toHaveClass("waiting_input");
    expect(screen.getByText("Waiting")).toBeInTheDocument();
  });
});
