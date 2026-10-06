import { describe, expect, it } from "vitest";
import type { TranscriptEvent } from "../../api/types";
import { foldTranscript } from "../../store/transcriptStore";
import { nestActivities } from "./runtimeActivity";
import { projectTurns, turnOutcome } from "./turnActivity";

const project = (raw: TranscriptEvent[]) => projectTurns(nestActivities(foldTranscript(raw)));
const shown = (raw: TranscriptEvent[]) =>
  project(raw).map((turn) => turn.parts.filter((part) => !part.hidden).flatMap((part) => part.events.map((event) => event.seq ?? event.kind)));
const tucked = (raw: TranscriptEvent[]) =>
  project(raw).map((turn) => turn.parts.filter((part) => part.hidden).flatMap((part) => part.events.map((event) => event.seq ?? event.kind)));

describe("turn projection (FS-03.A55–A56, TS-08.R100–R101)", () => {
  // Independently authored: a long turn interleaving passages, a Steer, a
  // child and tools, then a turn with no text after its final tool.
  const long: TranscriptEvent[] = [
    { kind: "user_text", seq: 1, text: "Fix the build" },
    { kind: "assistant_text", seq: 2, text: "Looking " },
    { kind: "assistant_text", seq: 3, text: "at it." },
    { kind: "tool_call", seq: 4, tool_call_id: "t1", name: "Read" },
    { kind: "tool_result", seq: 5, tool_call_id: "t1", status: "completed", content: "ok" },
    { kind: "user_text", seq: 6, text: "Also check lint" },
    { kind: "activity_started", seq: 7, activity_id: "c1", name: "Scout" },
    { kind: "assistant_text", seq: 8, activity_id: "c1", text: "child says" },
    { kind: "activity_state", seq: 9, activity_id: "c1", state: "completed" },
    { kind: "diff", seq: 10, path: "a.go", new_text: "x" },
    { kind: "assistant_text", seq: 11, text: "Done: build fixed." },
    { kind: "turn_end", seq: 12, stop_reason: "end_turn" },
    { kind: "user_text", seq: 13, text: "Run tests" },
    { kind: "assistant_text", seq: 14, text: "Running them now." },
    { kind: "tool_call", seq: 15, tool_call_id: "t2", name: "Bash" },
    { kind: "tool_result", seq: 16, tool_call_id: "t2", status: "completed", content: "pass" },
    { kind: "turn_end", seq: 17, stop_reason: "end_turn" },
  ];

  it("keeps input, Steer, the last merged passage and the outcome visible in causal order", () => {
    expect(shown(long)).toEqual([[1, 6, 11, 12], [13, 14, 17]]);
    expect(tucked(long)).toEqual([[2, 4, 5, "activity", 10], [15, 16]]);
    // Merged chunks are one passage; the projection never splits them.
    expect(project(long)[0].parts[1].events[0].text).toBe("Looking at it.");
  });

  it("keys a turn by the boundary that opened it so a live turn keeps its identity", () => {
    const live = project(long.slice(0, 14));
    expect(live.map((turn) => [turn.key, turn.completed])).toEqual([["start", true], ["12", false]]);
    expect(project(long).map((turn) => turn.key)).toEqual(["start", "12"]);
    expect(project([{ kind: "user_text", text: "optimistic" }])[0].key).toBe("start");
  });

  it("never collapses an unfinished tail, a child turn_end or a permission wait", () => {
    const raw: TranscriptEvent[] = [
      { kind: "user_text", seq: 1, text: "go" },
      { kind: "assistant_text", seq: 2, text: "first" },
      { kind: "activity_started", seq: 3, activity_id: "c1", name: "Scout" },
      { kind: "turn_end", seq: 4, activity_id: "c1", stop_reason: "end_turn" },
      { kind: "tool_call", seq: 5, tool_call_id: "t1" },
      { kind: "permission_request", seq: 6, tool_call_id: "t1" },
    ];
    const turns = project(raw);
    expect(turns).toHaveLength(1);
    expect(turns[0].completed).toBe(false);
    expect(turns[0].parts).toEqual([{ hidden: false, events: expect.any(Array) }]);
  });

  it("keeps a still-pending approval outside activity and a tool-only turn has no invented response", () => {
    const raw: TranscriptEvent[] = [
      { kind: "user_text", seq: 1, text: "go" },
      { kind: "tool_call", seq: 2, tool_call_id: "t1" },
      { kind: "permission_request", seq: 3, tool_call_id: "t1" },
      { kind: "permission_request", seq: 4, tool_call_id: "t2" },
      { kind: "permission_resolved", seq: 5, tool_call_id: "t2", decision: "approve" },
      { kind: "turn_end", seq: 6, stop_reason: "cancelled" },
    ];
    expect(shown(raw)).toEqual([[1, 3, 6]]);
    expect(tucked(raw)).toEqual([[2, 4]]);
    expect(project(raw)[0].hasResponse).toBe(false);
    expect(turnOutcome(project(raw)[0])).toBe("Cancelled");
  });

  it("fences a turn cut by a resume or backend switch without claiming an outcome", () => {
    const raw: TranscriptEvent[] = [
      { kind: "user_text", seq: 1, text: "go" },
      { kind: "assistant_text", seq: 2, text: "partial" },
      { kind: "tool_call", seq: 3, tool_call_id: "t1" },
      { kind: "session_meta", seq: 4, resumed_at: "2026-10-06T10:00:00Z" },
      { kind: "user_text", seq: 5, text: "again" },
      { kind: "assistant_text", seq: 6, text: "answer" },
      { kind: "turn_end", seq: 7, stop_reason: "end_turn" },
    ];
    const turns = project(raw);
    expect(turns.map((turn) => [turn.key, turn.completed])).toEqual([["start", false], ["4", true]]);
    expect(turnOutcome(turns[0])).toBeNull();
    expect(shown(raw)[0]).toEqual([1, 2, 3]);
  });

  it("labels a cut-short response partial and claims nothing for unknown reasons", () => {
    expect(turnOutcome({ completed: true, stopReason: "error", hasResponse: true })).toBe("Failed — response is partial");
    expect(turnOutcome({ completed: true, stopReason: "end_turn", hasResponse: true })).toBeNull();
    expect(turnOutcome({ completed: true, stopReason: "novel_reason", hasResponse: true })).toBeNull();
    expect(turnOutcome({ completed: false, stopReason: "cancelled", hasResponse: true })).toBeNull();
  });

  it("collects nested child seqs so a reveal can find the turn that hides them", () => {
    expect([...project(long)[0].seqs]).toEqual(expect.arrayContaining([8, 10]));
  });
});
