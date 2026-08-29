import { beforeEach, describe, expect, it } from "vitest";
import { useTranscriptStore } from "./transcriptStore";

beforeEach(() => {
  useTranscriptStore.setState({ byAgent: {}, rawByAgent: {}, pending: {} });
});

describe("card preview clipping", () => {
  // FS-02.R9 / INV §2: the live preview and the server's reconcile sweep build
  // the same card artifact, so they must keep the same end of a long message.
  // The server keeps the last 120 runes (clipPreview, internal/server/reconcile.go);
  // if this path kept the head, one agent's card would show the start of its last
  // message after a reload and the end of it while streaming.
  it("keeps the end of a long streamed message", () => {
    const long = "h".repeat(200) + "the tail the reader wants";
    useTranscriptStore.getState().updatePreview("a_clip", {
      agent_id: "a_clip", seq: 1, type: "assistant_text", ts: "t1", data: { delta: long },
    });
    const preview = useTranscriptStore.getState().previewByAgent.a_clip;
    expect(preview.endsWith("the tail the reader wants")).toBe(true);
    expect([...preview].length).toBe(120);
  });

  // A plain `.slice(-120)` cuts UTF-16 code units, so a boundary landing inside a
  // surrogate pair rendered a replacement character on the card — exactly what the
  // server helper documents itself as avoiding.
  it("clips by code point so a surrogate pair is never split", () => {
    const long = "😀".repeat(200);
    useTranscriptStore.getState().updatePreview("a_emoji", {
      agent_id: "a_emoji", seq: 1, type: "assistant_text", ts: "t1", data: { delta: long },
    });
    const preview = useTranscriptStore.getState().previewByAgent.a_emoji;
    expect([...preview].length).toBe(120);
    expect(preview).toBe("😀".repeat(120));
    expect(preview).not.toContain("\uFFFD");
  });

  // The replay path folds the same events and must land on the same preview, or a
  // refetch would flip the card between the two ends (INV §2).
  it("derives the same preview from a replayed transcript", () => {
    const long = "h".repeat(200) + "the tail the reader wants";
    useTranscriptStore.getState().setTranscript("a_replay", [
      { agent_id: "a_replay", seq: 1, type: "assistant_text", ts: "t1", data: { delta: long } },
    ]);
    const preview = useTranscriptStore.getState().previewByAgent.a_replay;
    expect(preview.endsWith("the tail the reader wants")).toBe(true);
    expect([...preview].length).toBe(120);
  });
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

  it("retains delivered events newer than a transcript response", () => {
    useTranscriptStore.getState().beginReconciliation("a_6");
    useTranscriptStore.getState().appendMessage("a_6", {
      agent_id: "a_6", seq: 2, type: "assistant_text", ts: "t2", data: { delta: "new" },
    });
    useTranscriptStore.getState().setTranscript("a_6", [
      { agent_id: "a_6", seq: 1, type: "user_text", ts: "t1", data: { text: "old" } },
    ]);
    expect(useTranscriptStore.getState().byAgent.a_6.map((event) => event.seq)).toEqual([1, 2]);
  });

  it("retains the exact newer assistant delta without duplicating the fetched prefix", () => {
    useTranscriptStore.getState().beginReconciliation("a_7");
    useTranscriptStore.getState().appendMessage("a_7", { seq: 1, kind: "assistant_text", text: "old" });
    useTranscriptStore.getState().appendMessage("a_7", { seq: 2, kind: "assistant_text", text: "new" });
    useTranscriptStore.getState().setTranscript("a_7", [{ seq: 1, kind: "assistant_text", text: "old" }]);
    expect(useTranscriptStore.getState().byAgent.a_7).toMatchObject([{ text: "oldnew" }]);
  });

  it("retains raw deltas only while a reconciliation is in flight", () => {
    for (let seq = 1; seq <= 2_000; seq++) {
      useTranscriptStore.getState().appendMessage("a_stream", { seq, kind: "assistant_text", text: "x" });
    }
    expect(useTranscriptStore.getState().rawByAgent.a_stream).toBeUndefined();

    useTranscriptStore.getState().beginReconciliation("a_stream");
    useTranscriptStore.getState().appendMessage("a_stream", { seq: 2_001, kind: "assistant_text", text: "y" });
    expect(useTranscriptStore.getState().rawByAgent.a_stream).toHaveLength(1);
    useTranscriptStore.getState().endReconciliation("a_stream");
    expect(useTranscriptStore.getState().rawByAgent.a_stream).toBeUndefined();
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
  // FS-03.A20: a streamed diagram fence and its replay must fold to the identical text, so the
  // rendered message is the same live, after a reload, and in an archived transcript.
  it("folds a streamed diagram fence into the same text a replay produces", () => {
    const deltas = ["Here it is:\n\n```mer", "maid\ngraph TD;\n  Start-->Fin", "ish;\n```\n"];
    deltas.forEach((delta, index) => {
      useTranscriptStore.getState().appendMessage("a_diagram", {
        agent_id: "a_diagram", seq: index + 1, type: "assistant_text", ts: `t${index + 1}`,
        data: { message_id: "m_diagram", delta },
      });
    });

    useTranscriptStore.getState().setTranscript("a_replay", deltas.map((delta, index) => ({
      agent_id: "a_replay", seq: index + 1, type: "assistant_text", ts: `t${index + 1}`,
      data: { message_id: "m_diagram", delta },
    })));

    const live = useTranscriptStore.getState().byAgent.a_diagram;
    const replayed = useTranscriptStore.getState().byAgent.a_replay;
    expect(live).toHaveLength(1);
    expect(live[0].text).toBe("Here it is:\n\n```mermaid\ngraph TD;\n  Start-->Finish;\n```\n");
    expect(replayed.map((event) => ({ ...event, agent_id: undefined }))).toEqual(live.map((event) => ({ ...event, agent_id: undefined })));
  });
});
