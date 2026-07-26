import { beforeEach, describe, expect, it } from "vitest";
import { useTranscriptStore } from "./transcriptStore";

beforeEach(() => {
  useTranscriptStore.setState({ byAgent: {}, pending: {} });
});

describe("transcriptStore", () => {
  it("concatenates assistant text deltas with the same message_id", () => {
    useTranscriptStore.getState().appendMessage("a_1", {
      kind: "assistant_text",
      message_id: "m_1",
      text: "hel",
    });
    useTranscriptStore.getState().appendMessage("a_1", {
      kind: "assistant_text",
      message_id: "m_1",
      text: "lo",
    });
    expect(useTranscriptStore.getState().byAgent.a_1).toEqual([
      { kind: "assistant_text", message_id: "m_1", text: "hello" },
    ]);
  });

  it("normalizes the nested runtime wire shape and merges consecutive deltas", () => {
    // The real wire shape: payload lives under `data`, not at the top level.
    useTranscriptStore.getState().appendMessage("a_2", {
      agent_id: "a_2",
      seq: 1,
      type: "assistant_text",
      ts: "t1",
      data: { delta: "hel" },
    });
    useTranscriptStore.getState().appendMessage("a_2", {
      agent_id: "a_2",
      seq: 2,
      type: "assistant_text",
      ts: "t2",
      data: { delta: "lo" },
    });
    const events = useTranscriptStore.getState().byAgent.a_2;
    expect(events).toHaveLength(1);
    expect(events[0].kind).toBe("assistant_text");
    expect(events[0].text).toBe("hello");
  });

  it("surfaces tool_call payload fields at the top level after normalization", () => {
    useTranscriptStore.getState().appendMessage("a_3", {
      agent_id: "a_3",
      seq: 1,
      type: "tool_call",
      ts: "t1",
      data: { tool_call_id: "tc_1", name: "Edit", args: { path: "x" } },
    });
    const event = useTranscriptStore.getState().byAgent.a_3[0];
    expect(event.kind).toBe("tool_call");
    expect(event.name).toBe("Edit");
    expect(event.tool_call_id).toBe("tc_1");
  });

  it("folds permission_resolved into the matching prompt instead of rendering it", () => {
    useTranscriptStore.getState().appendMessage("a_4", {
      agent_id: "a_4",
      seq: 1,
      type: "permission_request",
      ts: "t1",
      data: { tool_call_id: "tc_9", name: "Bash", reason: "run" },
    });
    useTranscriptStore.getState().appendMessage("a_4", {
      agent_id: "a_4",
      seq: 2,
      type: "permission_resolved",
      ts: "t2",
      data: { tool_call_id: "tc_9", decision: "approve" },
    });
    const events = useTranscriptStore.getState().byAgent.a_4;
    expect(events).toHaveLength(1);
    expect(events[0].kind).toBe("permission_request");
    expect(events[0].resolved).toBe("approve");
    expect(useTranscriptStore.getState().pending.a_4).toBeNull();
  });

  it("normalizes the nested shape in setTranscript (REST refetch path)", () => {
    useTranscriptStore.getState().setTranscript("a_5", [
      { agent_id: "a_5", seq: 1, type: "assistant_text", ts: "t1", data: { delta: "hi" } },
    ]);
    const event = useTranscriptStore.getState().byAgent.a_5[0];
    expect(event.kind).toBe("assistant_text");
    expect(event.text ?? event.delta).toBe("hi");
  });

  it("merges consecutive assistant deltas on transcript replay", () => {
    useTranscriptStore.getState().setTranscript("a_replay", [
      { agent_id: "a_replay", seq: 1, type: "assistant_text", ts: "t1", data: { delta: "Sure, " } },
      { agent_id: "a_replay", seq: 2, type: "assistant_text", ts: "t2", data: { delta: "I'll " } },
      { agent_id: "a_replay", seq: 3, type: "assistant_text", ts: "t3", data: { delta: "do that." } },
      { agent_id: "a_replay", seq: 4, type: "turn_end", ts: "t4", data: { reason: "completed" } },
    ]);

    const events = useTranscriptStore.getState().byAgent.a_replay;
    expect(events).toHaveLength(2);
    expect(events[0]).toMatchObject({ kind: "assistant_text", text: "Sure, I'll do that." });
    expect(events[1]).toMatchObject({ kind: "turn_end" });
  });

  it("replaces an optimistic user bubble with its durable SSE event", () => {
    useTranscriptStore.getState().appendMessage("a_5", {
      kind: "user_text", text: "persist this", message_id: "local-1",
    });
    useTranscriptStore.getState().appendMessage("a_5", {
      agent_id: "a_5", seq: 1, type: "user_text", ts: "t1", data: { text: "persist this" },
    });
    const events = useTranscriptStore.getState().byAgent.a_5;
    expect(events).toHaveLength(1);
    expect(events[0]).toMatchObject({ kind: "user_text", seq: 1, text: "persist this" });
  });

  it("folds permission_resolved on setTranscript (refetch/archive replay)", () => {
    useTranscriptStore.getState().setTranscript("a_6", [
      { agent_id: "a_6", seq: 1, type: "permission_request", ts: "t1", data: { tool_call_id: "tc_7", name: "Bash", reason: "run" } },
      { agent_id: "a_6", seq: 2, type: "permission_resolved", ts: "t2", data: { tool_call_id: "tc_7", decision: "deny" } },
    ]);
    const events = useTranscriptStore.getState().byAgent.a_6;
    // The resolution event is folded into the request, not rendered on its own.
    expect(events).toHaveLength(1);
    expect(events[0].kind).toBe("permission_request");
    expect(events[0].resolved).toBe("deny");
  });

  it("resolves only the newest permission request when a tool-call id is reused", () => {
    const request = (seq: number) => ({
      agent_id: "a_repeat", seq, type: "permission_request", ts: `t${seq}`,
      data: { tool_call_id: "tc_repeat", name: "Bash", reason: "run" },
    });
    const resolution = (seq: number, decision: "approve" | "deny") => ({
      agent_id: "a_repeat", seq, type: "permission_resolved", ts: `t${seq}`,
      data: { tool_call_id: "tc_repeat", decision },
    });

    useTranscriptStore.getState().appendMessage("a_repeat", request(1));
    useTranscriptStore.getState().appendMessage("a_repeat", resolution(2, "approve"));
    useTranscriptStore.getState().appendMessage("a_repeat", request(3));
    useTranscriptStore.getState().appendMessage("a_repeat", resolution(4, "deny"));
    expect(useTranscriptStore.getState().byAgent.a_repeat.map((event) => event.resolved)).toEqual(["approve", "deny"]);

    useTranscriptStore.getState().setTranscript("a_action", [request(1), request(3)]);
    useTranscriptStore.getState().resolvePermission("a_action", "tc_repeat", "approve");
    expect(useTranscriptStore.getState().byAgent.a_action.map((event) => event.resolved)).toEqual([undefined, "approve"]);
  });

  it("preserves cancelled and timed-out permission outcomes live and on replay", () => {
    useTranscriptStore.getState().appendMessage("a_cancel", {
      agent_id: "a_cancel", seq: 1, type: "permission_request", ts: "t1",
      data: { tool_call_id: "tc_cancel", name: "Bash", reason: "run" },
    });
    useTranscriptStore.getState().appendMessage("a_cancel", {
      agent_id: "a_cancel", seq: 2, type: "permission_resolved", ts: "t2",
      data: { tool_call_id: "tc_cancel", decision: "cancelled" },
    });
    expect(useTranscriptStore.getState().byAgent.a_cancel[0].resolved).toBe("cancelled");

    useTranscriptStore.getState().setTranscript("a_timeout", [
      { agent_id: "a_timeout", seq: 1, type: "permission_request", ts: "t1", data: { tool_call_id: "tc_timeout", name: "Edit", reason: "write" } },
      { agent_id: "a_timeout", seq: 2, type: "permission_resolved", ts: "t2", data: { tool_call_id: "tc_timeout", decision: "timeout" } },
    ]);
    expect(useTranscriptStore.getState().byAgent.a_timeout[0].resolved).toBe("timeout");
  });
});
