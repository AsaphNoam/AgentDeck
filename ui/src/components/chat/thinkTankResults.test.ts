import { describe, expect, it } from "vitest";
import type { ThinkTankResult } from "../../api/thinkTanks";
import { mergeThinkTankResults, THINK_TANK_RESULT_KIND } from "./thinkTankResults";

const result = { result_id: 1, room_id: "tt_1", room_title: "Cache choice", room_available: true, entry_seq: 9,
  attempt_id: "att", body: "Exact synthesis", generation: "g", turn_id: "t", event_seq: 3, completed_at: "2026-10-07T00:00:00Z" } as ThinkTankResult;

// TS-14.R26: an unloaded transcript waits for the anchor; a settled empty one
// shows the retained result as source-unavailable.
describe("mergeThinkTankResults", () => {
  it("waits while the transcript is still loading", () => {
    expect(mergeThinkTankResults([], [result])).toEqual([]);
  });

  it("shows the result once an empty transcript has settled", () => {
    const rows = mergeThinkTankResults([], [result], true);
    expect(rows).toHaveLength(1);
    expect(rows[0]).toMatchObject({ kind: THINK_TANK_RESULT_KIND, source_unavailable: true });
  });

  it("still waits for an anchor beyond a settled non-empty window", () => {
    expect(mergeThinkTankResults([{ kind: "user_text", seq: 1 }], [result], true)).toHaveLength(1);
  });
});
