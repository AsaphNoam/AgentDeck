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
});
