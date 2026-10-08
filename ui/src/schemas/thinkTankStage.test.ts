import { describe, expect, it } from "vitest";
import { taskActions, taskSchema } from "./task";
import { pipelineStageTaskSchema } from "./pipeline";

// Shapes produced by internal/server (taskDetail, pipelineTaskRunProjection)
// for a room-backed Think Tank stage (FS-16.R48, TS-09.R55, INV §11).
const roomTask = {
  task_id: "tk_room", project: "app", display_name: "Debate", instruction: "Think Tank stage",
  target_kind: "think_tank", target_agent_id: "", role: "", backend: "", model: "", state: "waiting",
  created_by_kind: "pipeline", created_at: "2026-10-08T00:00:00Z", updated_at: "2026-10-08T00:00:00Z",
  revision: 1, attention_reason: "Accepting judge output",
  room: { room_id: "tt_1", run_id: "pr_1", stage_id: "debate", phase: "ended", judge_status: "completed" },
};

describe("Think Tank stage wire shapes", () => {
  it("parses a room-backed task and withholds generic task controls", () => {
    const parsed = taskSchema.safeParse(roomTask);
    expect(parsed.success).toBe(true);
    if (!parsed.success) return;
    expect(parsed.data.room?.room_id).toBe("tt_1");
    expect(taskActions(parsed.data)).toEqual({ cancel: false, retry: false, recordResult: false, rearm: false });
  });

  it("parses a run stage row with Think Tank execution", () => {
    const parsed = pipelineStageTaskSchema.safeParse({
      task_id: "tk_room", run_id: "pr_1", stage_id: "debate", stage_index: 1, attempt_number: 1, state: "waiting",
      assignment_text: "", execution_kind: "think_tank", attention_reason: "Accepting judge output",
      room: { room_id: "tt_1", run_id: "pr_1", stage_id: "debate", phase: "ended", judge_status: "completed" },
      standing_owner: { route: "unavailable", runtime: { backend: "", model: "" } },
      created_at: "2026-10-08T00:00:00Z", updated_at: "2026-10-08T00:00:00Z",
    });
    expect(parsed.success).toBe(true);
    if (parsed.success) expect(parsed.data.room?.source_entry_seq).toBe(0);
  });
});
