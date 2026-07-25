import { describe, expect, it } from "vitest";
import { extractPipelineProposals } from "./AgentDeckerBuilder";

describe("extractPipelineProposals", () => {
  it("projects only validated proposal tool results from the transcript", () => {
    const proposal = {
      proposal_id: "pp_1234",
      kind: "save_template",
      digest: "1234567890abcdef",
      payload: {
        id: "delivery",
        template: {
          version: 1,
          title: "Delivery",
          inputs: [],
          stages: [{
            id: "work", title: "Work", role: "implementer", instruction: "Do the work.", inputs: [], outputs: [], max_visits: 1,
            transitions: {
              success: { final: "success", approval: "automatic" },
              failure: { final: "failure", approval: "required" },
            },
          }],
        },
      },
    } as const;
    const events = [
      { kind: "tool_call", tool_call_id: "tc_other", name: "send_message" },
      { kind: "tool_result", tool_call_id: "tc_other", content: [{ type: "text", text: JSON.stringify({ ok: true, proposal }) }] },
      { kind: "tool_call", tool_call_id: "tc_proposal", name: "other", title: "propose_pipeline_template" },
      { kind: "tool_result", tool_call_id: "tc_proposal", content: [{ type: "text", text: JSON.stringify({ ok: true, proposal }) }] },
    ];

    expect(extractPipelineProposals(events)).toMatchObject([proposal]);
  });

  it("does not treat chat wording or malformed tool content as approval data", () => {
    expect(extractPipelineProposals([
      { kind: "assistant_text", text: "I propose saving this pipeline." },
      { kind: "tool_call", tool_call_id: "tc_1", name: "propose_pipeline_run" },
      { kind: "tool_result", tool_call_id: "tc_1", content: "not json" },
    ])).toEqual([]);
  });
});
