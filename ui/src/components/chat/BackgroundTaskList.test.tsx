import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import * as client from "../../api/client";
import type { TranscriptEvent } from "../../api/types";
import { foldTranscript } from "../../store/transcriptStore";
import { TranscriptView } from "./TranscriptView";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

const wire = (seq: number, type: string, data: Record<string, unknown>, activity_id?: string): TranscriptEvent =>
  ({ agent_id: "a1", seq, type, ts: "t", data, ...(activity_id ? { activity_id } : {}) });

const running = [
  wire(1, "tool_call", { tool_call_id: "tc_bg", name: "exec_command", title: "npm run dev", args: {} }),
  wire(2, "background_task_state", { task_id: "task_1", tool_call_id: "tc_bg", name: "npm run dev", state: "running", can_stop: true }),
  wire(3, "activity_started", { name: "researcher" }, "act_c"),
  wire(4, "background_task_state", { task_id: "act_c/task_1", tool_call_id: "act_c/tc_c", name: "go test", state: "running", can_stop: true }, "act_c"),
  wire(5, "background_task_state", { task_id: "act_c/task_1", tool_call_id: "act_c/tc_c", state: "completed" }, "act_c"),
];

function renderTranscript(events: TranscriptEvent[], taskControl = true) {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <TranscriptView agentId="a1" events={foldTranscript(events)} taskControl={taskControl} />
    </QueryClientProvider>,
  );
}

function rows() {
  return [...document.querySelectorAll("[data-slot='task']")].map((row) => `${row.getAttribute("data-state")}:${row.textContent}`);
}

describe("background tasks (FS-03.A41)", () => {
  it("lists one row per task at the tail, state first, and never inline", () => {
    renderTranscript(running);
    expect(screen.getByRole("button", { name: /Background tasks \(2\)/ })).toHaveAttribute("aria-expanded", "true");
    expect(rows()).toEqual(["running:Runningnpm run devStop", "completed:Completedgo testresearcher"]);
    expect(document.querySelectorAll("[data-variant='unknown']")).toHaveLength(0);
  });

  it("stops only the chosen task and waits for the runtime's terminal update", async () => {
    const stop = vi.spyOn(client, "stopBackgroundTask").mockResolvedValue({ accepted: true });
    const view = renderTranscript(running);
    fireEvent.click(screen.getByRole("button", { name: "Stop" }));
    expect(stop).toHaveBeenCalledWith("a1", "task_1", undefined);
    await waitFor(() => expect(screen.getByText("Stopping…")).toBeInTheDocument());

    view.rerender(
      <QueryClientProvider client={new QueryClient()}>
        <TranscriptView agentId="a1" taskControl events={foldTranscript([...running, wire(6, "background_task_state", { task_id: "task_1", tool_call_id: "tc_bg", state: "stopped" })])} />
      </QueryClientProvider>,
    );
    const toggle = screen.getByRole("button", { name: /Background tasks/ });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    fireEvent.click(toggle);
    expect(rows()[0]).toBe("stopped:Stoppednpm run dev");
    expect(screen.queryByRole("button", { name: "Stop" })).toBeNull();
  });

  it("keeps the last state and a retryable error when Stop fails", async () => {
    vi.spyOn(client, "stopBackgroundTask").mockRejectedValue(new Error("nope"));
    renderTranscript(running);
    fireEvent.click(screen.getByRole("button", { name: "Stop" }));
    const row = document.querySelector("[data-slot='task']") as HTMLElement;
    expect(await within(row).findByRole("alert")).toHaveTextContent("Could not stop this task. Try again.");
    expect(row).toHaveAttribute("data-state", "running");
    expect(within(row).getByRole("button", { name: "Stop" })).toBeEnabled();
  });

  it("offers no Stop without control, and a resume fences old running tasks", () => {
    renderTranscript(running, false);
    expect(screen.queryByRole("button", { name: "Stop" })).toBeNull();
    cleanup();
    renderTranscript([...running, wire(6, "session_meta", { resumed_at: "later" })]);
    const toggle = screen.getByRole("button", { name: /Background tasks/ });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    fireEvent.click(toggle);
    expect(rows()[0]).toBe("stopped:Ended with the previous sessionnpm run dev");
  });
});
