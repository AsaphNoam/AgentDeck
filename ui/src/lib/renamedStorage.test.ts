import { beforeEach, describe, expect, it } from "vitest";
import { copyForwardRenamedStorage } from "./renamedStorage";

// TS-08.R58: pre-rename browser keys move forward once, never overwrite newer
// state, and never block the app when storage fails.
describe("copyForwardRenamedStorage", () => {
  beforeEach(() => localStorage.clear());

  it("moves each old key to an empty new key and removes the old one", () => {
    localStorage.setItem("agentdeck-chat-drafts", '{"a1":{"text":"unsent","editedAt":1}}');
    localStorage.setItem("agentdeck-annotation-tray", '{"state":{}}');
    localStorage.setItem("agentdeck.pipeline-builder-agent", "a_builder");

    copyForwardRenamedStorage();

    expect(localStorage.getItem("chuck-chat-drafts")).toBe('{"a1":{"text":"unsent","editedAt":1}}');
    expect(localStorage.getItem("chuck-annotation-tray")).toBe('{"state":{}}');
    expect(localStorage.getItem("chuck.pipeline-builder-agent")).toBe("a_builder");
    expect(localStorage.getItem("agentdeck-chat-drafts")).toBeNull();
    expect(localStorage.getItem("agentdeck-annotation-tray")).toBeNull();
    expect(localStorage.getItem("agentdeck.pipeline-builder-agent")).toBeNull();
  });

  it("keeps newer state under the new key and is a no-op when run again", () => {
    localStorage.setItem("chuck-chat-drafts", "newer");
    localStorage.setItem("agentdeck-chat-drafts", "older");

    copyForwardRenamedStorage();
    copyForwardRenamedStorage();

    expect(localStorage.getItem("chuck-chat-drafts")).toBe("newer");
    expect(localStorage.getItem("agentdeck-chat-drafts")).toBeNull();
  });

  it("tolerates storage that throws and keeps the old key for a later load", () => {
    const removed: string[] = [];
    const failing = {
      getItem: (key: string) => (key.startsWith("agentdeck") ? "old" : null),
      setItem: () => {
        throw new Error("quota");
      },
      removeItem: (key: string) => removed.push(key),
    } as unknown as Storage;

    expect(() => copyForwardRenamedStorage(failing)).not.toThrow();
    expect(removed).toEqual([]);
  });
});
