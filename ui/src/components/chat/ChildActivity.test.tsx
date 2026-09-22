import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { TranscriptEvent } from "../../api/types";
import { foldTranscript, useTranscriptStore } from "../../store/transcriptStore";
import { TranscriptView } from "./TranscriptView";

afterEach(() => {
  cleanup();
  useTranscriptStore.setState({ byAgent: {}, rawByAgent: {}, pending: {}, previewByAgent: {}, previewKindByAgent: {} });
});

// Raw wire events as the server stores and streams them (TS-01.R35).
const wire = (seq: number, type: string, data: Record<string, unknown>, activity_id?: string, parent_activity_id?: string): TranscriptEvent =>
  ({ agent_id: "a1", seq, type, ts: "t", data, ...(activity_id ? { activity_id } : {}), ...(parent_activity_id ? { parent_activity_id } : {}) });

const history = [
  wire(1, "user_text", { text: "Delegate" }),
  wire(2, "assistant_text", { delta: "Root before. " }),
  wire(3, "activity_started", { name: "researcher", task: "Find the bug" }, "act_c"),
  wire(4, "assistant_text", { delta: "Child says" }, "act_c"),
  wire(5, "assistant_text", { delta: "Root interleaved." }),
  wire(6, "activity_started", { name: "helper" }, "act_g", "act_c"),
  wire(7, "assistant_text", { delta: "Grandchild says" }, "act_g"),
  wire(8, "activity_state", { state: "stopped" }, "act_g", "act_c"),
  wire(9, "activity_state", { state: "completed" }, "act_c"),
  wire(10, "turn_end", { stop_reason: "end_turn" }),
];

function renderTranscript(events: TranscriptEvent[]) {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <TranscriptView agentId="a1" events={events} />
    </QueryClientProvider>,
  );
}

describe("native child activity (FS-03.A40)", () => {
  it("nests each child at its causal position without merging its text into the root", () => {
    renderTranscript(foldTranscript(history));
    const child = screen.getByRole("button", { name: /researcher · Completed/ });
    expect(child).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("Child says")).toBeNull();
    expect(screen.getByText("Root before.")).toBeInTheDocument();
    expect(screen.getByText("Root interleaved.")).toBeInTheDocument();

    fireEvent.click(child);
    const section = child.closest("[data-slot='child']") as HTMLElement;
    expect(section).toHaveAttribute("data-state", "completed");
    expect(within(section).getByText("Find the bug")).toBeInTheDocument();
    expect(within(section).getByText("Child says")).toBeInTheDocument();
    fireEvent.click(within(section).getByRole("button", { name: /researcher › helper · Stopped/ }));
    expect(within(section).getByText("Grandchild says")).toBeInTheDocument();
  });

  it("renders the same nesting live as on replay", () => {
    const store = useTranscriptStore.getState();
    for (const event of history) store.appendMessage("a1", event);
    const live = useTranscriptStore.getState().byAgent.a1;
    expect(live).toEqual(foldTranscript(history));
  });

  it("keeps a child with a pending permission open and a lost child reads as outcome unknown", () => {
    renderTranscript(foldTranscript([
      wire(1, "activity_started", { name: "researcher" }, "act_c"),
      wire(2, "permission_request", { tool_call_id: "act_c/tc_1", name: "Child rm", options: [] }, "act_c"),
      wire(3, "turn_end", { stop_reason: "end_turn" }),
    ]));
    const toggle = screen.getByRole("button", { name: /researcher · Outcome unknown/ });
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(toggle.closest("[data-slot='child']")).toHaveAttribute("data-state", "disconnected");
    expect(screen.getByText("Child rm")).toBeInTheDocument();
  });

  it("does not let child text become the agent's card preview", () => {
    const store = useTranscriptStore.getState();
    store.updatePreview("a1", wire(1, "assistant_text", { delta: "Root reply" }));
    store.updatePreview("a1", wire(2, "assistant_text", { delta: " child noise" }, "act_c"));
    expect(useTranscriptStore.getState().previewByAgent.a1).toBe("Root reply");
  });
});
