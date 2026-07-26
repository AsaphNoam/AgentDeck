import React from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { DiffBlock } from "./DiffBlock";

vi.mock("react-diff-viewer-continued", () => ({
  default: ({ onLineNumberClick }: { onLineNumberClick: (lineId: string) => void }) => (
    <button type="button" onClick={() => onLineNumberClick("L-2")}>Old line 2</button>
  ),
}));

describe("DiffBlock", () => {
  it("captures a line selected with the diff viewer's line-id format", () => {
    const onAnnotate = vi.fn();
    render(
      <DiffBlock
        event={{ seq: 7, path: "main.go", old_text: "first\nsecond\nthird", new_text: "replacement" }}
        onAnnotate={onAnnotate}
      />,
    );

    expect(screen.getByText("Click line numbers to select a range.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Old line 2" }));
    fireEvent.click(screen.getByRole("button", { name: "Annotate lines 2–2" }));

    expect(onAnnotate).toHaveBeenCalledWith({
      seq: 7,
      path: "main.go",
      side: "old",
      start_line: 2,
      end_line: 2,
      excerpt: "second",
      instruction: "",
    });
  });
});
