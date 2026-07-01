import React from "react";
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { render, screen, act, cleanup } from "@testing-library/react";
import { NotificationCenter } from "./NotificationCenter";
import { useUiStore } from "../../store/uiStore";

describe("NotificationCenter per-toast timers", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    useUiStore.setState({ toasts: [] });
  });
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  // Regression (review fix): each toast auto-dismisses on its own 6s clock. The
  // previous single effect depended on the whole toasts array, so a newly pushed
  // toast restarted every timer and older toasts lingered.
  it("dismisses each toast independently; a new toast does not restart older timers", () => {
    render(<NotificationCenter />);

    act(() => {
      useUiStore.getState().pushError("first");
    });
    act(() => {
      vi.advanceTimersByTime(3_000);
    });
    act(() => {
      useUiStore.getState().pushError("second");
    });
    // 3s later: "first" has lived 6s → gone; "second" only 3s → still shown.
    act(() => {
      vi.advanceTimersByTime(3_000);
    });

    expect(screen.queryByText("first")).not.toBeInTheDocument();
    expect(screen.getByText("second")).toBeInTheDocument();
  });
});
