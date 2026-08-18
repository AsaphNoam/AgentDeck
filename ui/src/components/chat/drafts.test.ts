import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { discardChatDraft, getChatDraft, setChatDraft } from "./drafts";

beforeEach(() => localStorage.clear());
afterEach(() => vi.restoreAllMocks());

describe("chat drafts", () => {
  it("keeps exact per-agent text and removes it when emptied or discarded", () => {
    setChatDraft("a_one", "first\nline");
    setChatDraft("a_two", "second");

    expect(getChatDraft("a_one")).toBe("first\nline");
    expect(getChatDraft("a_two")).toBe("second");

    setChatDraft("a_one", "");
    discardChatDraft("a_two");
    expect(getChatDraft("a_one")).toBe("");
    expect(getChatDraft("a_two")).toBe("");
  });

  it("keeps the twenty most recently edited drafts", () => {
    for (let i = 0; i < 21; i++) setChatDraft(`a_${i}`, `draft ${i}`);

    expect(getChatDraft("a_0")).toBe("");
    expect(getChatDraft("a_1")).toBe("draft 1");
    expect(getChatDraft("a_20")).toBe("draft 20");
  });

  it("starts empty on malformed storage and leaves callers usable when storage fails", () => {
    localStorage.setItem("agentdeck-chat-drafts", "not json");
    expect(getChatDraft("a_one")).toBe("");

    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("quota exceeded"); });
    expect(() => setChatDraft("a_one", "still typing")).not.toThrow();
    expect(getChatDraft("a_one")).toBe("");
  });
});
