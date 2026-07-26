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

  it("keeps a text label alongside every state indicator", () => {
    const { container, rerender } = render(<StateBadge state="busy" />);
    for (const [state, label] of [["busy", "busy"], ["idle", "idle"], ["waiting_input", "Waiting"], ["done", "done"], ["error", "error"], ["unknown", "-"]] as const) {
      rerender(<StateBadge state={state} />);
      const badge = container.querySelector('[data-testid="state-badge"]');
      expect(badge).not.toBeNull();
      expect(badge).toHaveTextContent(label);
      expect(badge.querySelector('[data-slot="indicator"]')).not.toBeNull();
      expect(badge.querySelector('[data-slot="label"]')).toHaveTextContent(label);
    }
  });
});
