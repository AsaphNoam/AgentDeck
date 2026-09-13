import { afterEach, describe, expect, it, vi } from "vitest";
import { PipelineAPIError, listPipelineRuns, listPipelineTemplates, startPipelineRun } from "./pipelines";

afterEach(() => vi.unstubAllGlobals());

const template = {
  version: 2 as const,
  title: "Delivery",
  orchestrator_role: "implementer",
  inputs: [],
  stages: [{
    id: "work", title: "Work", objective: "Do the work.", coordination: "standing" as const,
    dedicated_role: "", approval_after_success: false, inputs: [], outputs: [],
  }],
};

describe("pipeline API", () => {
  it("keeps template collections non-null", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify([
      { id: "delivery", template, valid: true, diagnostics: [] },
    ]), { status: 200 })));

    await expect(listPipelineTemplates()).resolves.toEqual([
      { id: "delivery", template, valid: true, diagnostics: [] },
    ]);
  });

  it("sends exact run data and the explicit shared-workspace acknowledgement", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { code: "conflict", message: "shared", details: { code: "shared_workspace_confirmation_required", conflicts: [] } } }), { status: 409 }));
    vi.stubGlobal("fetch", fetchMock);
    const request = {
      request_id: "ui_1", template_id: "delivery", display_name: "Delivery", project: "app", goal: "Ship it", inputs: {},
      orchestrator: { backend: "claude", model: "sonnet" }, dedicated_assignments: {},
    };

    await expect(startPipelineRun(request, true)).rejects.toBeInstanceOf(PipelineAPIError);
    expect(JSON.parse(String(fetchMock.mock.calls[0][1]?.body))).toEqual({ ...request, acknowledge_shared_workspace: true });
  });

  it("parses bounded run summaries and preserves per-run diagnostics", async () => {
    const summary = {
      run_id: "pr_1", template_id: "delivery", display_name: "Delivery", project: "app", state: "paused",
      revision: 3, pending_action: "", current_stage_id: "work", current_agent_id: "a_1",
      attention_reason: "restart_state_invalid", final_outcome: "", updated_at: "2026-07-25T00:00:00Z",
      diagnostics: [{ field: "", code: "run_read_failed", message: "run detail could not be decoded" }],
    };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify([summary]), { status: 200 })));
    await expect(listPipelineRuns()).resolves.toEqual({ runs: [{ ...summary, current_stage_title: "" }], total: 1 });
  });
});
