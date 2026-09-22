import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { useReasoningStore } from "../../store/reasoningStore";
import { useTranscriptStore } from "../../store/transcriptStore";
import { TranscriptView, withReasoning } from "./TranscriptView";

afterEach(() => {
  cleanup();
  useReasoningStore.getState().clearAll();
});

const delta = (span_id: string, text: string, generation = "g1") =>
  ({ agent_id: "a1", generation, span_id, kind: "reasoning_delta" as const, delta: text });

function renderTranscript() {
  const client = new QueryClient();
  return render(
    <QueryClientProvider client={client}>
      <TranscriptView agentId="a1" events={[{ kind: "user_text", seq: 1, text: "Go" }, { kind: "assistant_text", seq: 2, text: "Answer" }]} />
    </QueryClientProvider>,
  );
}

describe("live reasoning (FS-03.A39)", () => {
  it("renders one collapsed Thinking disclosure per span at its chronological slot and streams while open", () => {
    const { append } = useReasoningStore.getState();
    append(delta("r1", "Consider "), 1);
    append(delta("r1", "the plan."), 1);
    renderTranscript();

    const toggle = screen.getByRole("button", { name: /Thinking/ });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("Consider the plan.")).toBeNull();
    const items = [...document.querySelectorAll('[data-slot="event"]')].map((node) => node.getAttribute("data-variant"));
    expect(items).toEqual(["user", "thinking", "assistant"]);

    fireEvent.click(toggle);
    expect(screen.getByText("Consider the plan.")).toBeInTheDocument();
    act(() => append(delta("r1", " More."), 1));
    expect(screen.getByText("Consider the plan. More.")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: /Thinking/ })).toHaveLength(1);
  });

  it("never enters the transcript store and a new generation or reconnect discards it", () => {
    const { append, clearAll } = useReasoningStore.getState();
    append(delta("r1", "old"), 0);
    append(delta("r1", "new", "g2"), 0);
    expect(useReasoningStore.getState().byAgent.a1.spans.map((span) => span.text)).toEqual(["new"]);
    expect(useTranscriptStore.getState().byAgent.a1).toBeUndefined();
    clearAll();
    expect(useReasoningStore.getState().byAgent).toEqual({});
  });

  it("keeps span order when several open at the same slot", () => {
    const rows = withReasoning([{ kind: "user_text", seq: 1 }], [
      { spanId: "r1", text: "a", anchor: 1 },
      { spanId: "r2", text: "b", anchor: 1 },
    ]);
    expect(rows.map((row) => row.text ?? row.kind)).toEqual(["user_text", "a", "b"]);
  });
});
