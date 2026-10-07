import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { TranscriptEvent } from "../../api/types";
import { useReasoningStore } from "../../store/reasoningStore";
import { useTranscriptStore } from "../../store/transcriptStore";
import { TranscriptView } from "./TranscriptView";
import { withReasoning } from "./runtimeActivity";

afterEach(() => {
  cleanup();
  useReasoningStore.getState().clearAll();
});

const delta = (span_id: string, text: string, generation = "g1") =>
  ({ agent_id: "a1", generation, span_id, kind: "reasoning_delta" as const, delta: text });

const live: TranscriptEvent[] = [
  { kind: "user_text", seq: 1, text: "Go" },
  { kind: "activity_started", seq: 2, activity_id: "c1", name: "Scout" },
  { kind: "assistant_text", seq: 3, text: "Answer" },
];

function view(events: TranscriptEvent[]) {
  return (
    <QueryClientProvider client={new QueryClient()}>
      <TranscriptView agentId="a1" events={events} />
    </QueryClientProvider>
  );
}

const thinking = () => screen.getAllByRole("button", { name: /Thinking/ });

describe("live reasoning (FS-03.A39, A54)", () => {
  it("starts open at its chronological slot and streams in place", () => {
    const { append } = useReasoningStore.getState();
    append(delta("r1", "Consider "), 1, "start");
    append(delta("r1", "the plan."), 1, "start");
    render(view(live));

    expect(thinking()[0]).toHaveAttribute("aria-expanded", "true");
    const items = [...document.querySelectorAll('[data-slot="event"]')].map((node) => node.getAttribute("data-variant"));
    expect(items).toEqual(["user", "thinking", "assistant"]);
    act(() => append(delta("r1", " More."), 1, "start"));
    expect(screen.getByText("Consider the plan. More.")).toBeInTheDocument();
    expect(thinking()).toHaveLength(1);
  });

  it("holds a manual collapse for later chunks and spans in that scope, then completion closes everything and a new turn opens again", () => {
    const { append } = useReasoningStore.getState();
    append(delta("r1", "root one"), 1, "start");
    append({ ...delta("k1", "child one"), activity_id: "c1" }, 2, "start");
    const { rerender } = render(view(live));

    fireEvent.click(thinking()[0]);
    act(() => {
      append(delta("r1", " more"), 1, "start");
      append(delta("r2", "root two"), 3, "start");
    });
    expect(screen.queryByText("root one more")).toBeNull();
    expect(screen.queryByText("root two")).toBeNull();
    // The running child's scope keeps its own choice; reopening root resumes it.
    expect(screen.getByText("child one")).toBeInTheDocument();
    fireEvent.click(thinking()[0]);
    expect(screen.getByText("root one more")).toBeInTheDocument();
    expect(screen.getByText("root two")).toBeInTheDocument();

    const done = [...live, { kind: "activity_state", seq: 4, activity_id: "c1", state: "completed" }, { kind: "turn_end", seq: 5, stop_reason: "end_turn" }];
    rerender(view(done));
    expect(screen.queryByRole("button", { name: /Thinking/ })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: /Show activity/ }));
    expect(thinking().every((button) => button.getAttribute("aria-expanded") === "false")).toBe(true);

    act(() => append(delta("r1", "next turn"), 6, "5"));
    rerender(view([...done, { kind: "user_text", seq: 6, text: "Again" }]));
    const next = thinking().at(-1)!;
    expect(next).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("next turn")).toBeInTheDocument();
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

describe("reasoning turn association (TS-08.R103)", () => {
  it("does not show a span whose slot now lies in another turn", () => {
    const events: TranscriptEvent[] = [
      { kind: "user_text", seq: 1 },
      { kind: "turn_end", seq: 2 },
      { kind: "user_text", seq: 3 },
    ];
    const rows = withReasoning(events, [
      { spanId: "stale", text: "backfilled", anchor: 0, turn: "2" },
      { spanId: "old", text: "owned old", anchor: 1, turn: "start" },
      { spanId: "now", text: "owned now", anchor: 3, turn: "2" },
    ]);
    expect(rows.map((row) => row.text ?? row.kind)).toEqual(["user_text", "owned old", "turn_end", "user_text", "owned now"]);
  });

  it("never grows a finished turn's span from a repeated id and drops live choices with the generation", () => {
    const { append, setCollapsed } = useReasoningStore.getState();
    append(delta("r1", "first"), 1, "start");
    append(delta("r1", "second"), 4, "3");
    expect(useReasoningStore.getState().byAgent.a1.spans.map((span) => [span.text, span.anchor])).toEqual([["first", 1], ["second", 4]]);
    setCollapsed("a1", "3|", true);
    expect(useReasoningStore.getState().byAgent.a1.collapsed).toEqual(["3|"]);
    append(delta("r9", "fresh", "g2"), 0, "start");
    expect(useReasoningStore.getState().byAgent.a1.collapsed).toEqual([]);
  });
});
