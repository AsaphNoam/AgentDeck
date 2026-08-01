import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { useAnnotationStore } from "../../store/annotationStore";
import { TranscriptView } from "./TranscriptView";

afterEach(() => {
  cleanup();
  window.getSelection()?.removeAllRanges();
  useAnnotationStore.setState({ bySource: {}, overallBySource: {}, editedAt: {} });
});

const events = [{ kind: "assistant_text", seq: 7, text: "First line\nSecond line" }];

function renderTranscript(annotationsEnabled = true, transcriptEvents = events) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
  return render(
    <QueryClientProvider client={client}>
      <TranscriptView agentId="a1" events={transcriptEvents} sourceActive annotationsEnabled={annotationsEnabled} />
    </QueryClientProvider>,
  );
}

function selectWithin(node: Node) {
  const range = document.createRange();
  range.selectNodeContents(node);
  const selection = window.getSelection();
  selection?.removeAllRanges();
  selection?.addRange(range);
}

describe("TranscriptView annotation entry point", () => {
  it("shows no standing Annotate control on transcript events", () => {
    renderTranscript();

    expect(screen.queryByRole("button", { name: /annotate/i })).toBeNull();
  });

  it("captures the highlighted text when the selection is right-clicked", () => {
    const { container } = renderTranscript();
    const item = container.querySelector('[data-slot="event"]') as HTMLElement;
    const text = document.createTextNode("Second line");
    item.appendChild(text);
    selectWithin(text);

    fireEvent.contextMenu(item, { clientX: 20, clientY: 30 });
    fireEvent.click(screen.getByRole("button", { name: "Annotate selection" }));

    expect(useAnnotationStore.getState().bySource.a1).toEqual([
      { seq: 7, excerpt: "Second line", instruction: "" },
    ]);
  });

  it("falls back to the whole event when nothing is highlighted", () => {
    const { container } = renderTranscript();
    const item = container.querySelector('[data-slot="event"]') as HTMLElement;

    fireEvent.contextMenu(item, { clientX: 20, clientY: 30 });
    fireEvent.click(screen.getByRole("button", { name: "Annotate whole event" }));

    expect(useAnnotationStore.getState().bySource.a1).toEqual([
      { seq: 7, excerpt: "First line\nSecond line", instruction: "" },
    ]);
  });

  it("leaves the native menu alone where annotations are disabled", () => {
    const { container } = renderTranscript(false);
    const item = container.querySelector('[data-slot="event"]') as HTMLElement;

    const opened = fireEvent.contextMenu(item, { clientX: 20, clientY: 30 });

    expect(opened).toBe(true); // preventDefault was not called
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("collapses uninterrupted tool activity and omits successful no-payload outcomes", () => {
    renderTranscript(false, [
      { kind: "tool_call", seq: 1, name: "Read", args: { path: "a.ts" } },
      { kind: "tool_result", seq: 2, status: "completed", content: null },
      { kind: "tool_call", seq: 3, name: "Search" },
      { kind: "tool_result", seq: 4, status: "completed", content: "Found one match." },
    ]);

    const summary = screen.getByRole("button", { name: "Ran 2 tools" });
    expect(screen.queryByText(/Tool call:/)).toBeNull();
    expect(screen.queryByText("Completed")).toBeNull();

    fireEvent.click(summary);

    expect(screen.getAllByText(/Tool call:/)).toHaveLength(2);
    expect(screen.getByText("Found one match.")).toBeInTheDocument();
    expect(screen.queryByText("Completed")).toBeNull();
  });

  it("starts a new tool run after non-tool activity", () => {
    renderTranscript(false, [
      { kind: "tool_call", seq: 1, name: "Read" },
      { kind: "tool_result", seq: 2, status: "completed", content: null },
      { kind: "assistant_text", seq: 3, text: "I found the file." },
      { kind: "tool_call", seq: 4, name: "Edit" },
      { kind: "tool_result", seq: 5, status: "completed", content: null },
    ]);

    expect(screen.getAllByRole("button", { name: "Ran 1 tool" })).toHaveLength(2);
    expect(screen.getByText("I found the file.")).toBeInTheDocument();
  });

  it("keeps expanded tool calls individually annotatable", () => {
    const { container } = renderTranscript(true, [
      { kind: "tool_call", seq: 1, name: "Read" },
      { kind: "tool_result", seq: 2, status: "completed", content: null },
    ]);

    fireEvent.click(screen.getByRole("button", { name: "Ran 1 tool" }));
    fireEvent.contextMenu(container.querySelector('[data-seq="1"]') as HTMLElement, { clientX: 20, clientY: 30 });
    fireEvent.click(screen.getByRole("button", { name: "Annotate whole event" }));

    expect(useAnnotationStore.getState().bySource.a1).toMatchObject([{ seq: 1 }]);
  });
});
