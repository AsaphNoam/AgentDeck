import { describe, it, expect } from "vitest";
import { initialTab } from "./ChatPanel";

// initialTab drives which tab a chat panel opens on. The load-bearing case for
// the Finding 9 secondary fix: a terminal-interface agent must default to the
// Terminal tab so a WS attaches after launch (chat agents stay on transcript).
describe("initialTab", () => {
  it("defaults a terminal-interface agent to the Terminal tab", () => {
    expect(initialTab(null, "terminal")).toBe("terminal");
  });

  it("defaults a chat-interface agent to the transcript tab", () => {
    expect(initialTab(null, "acp")).toBe("transcript");
    expect(initialTab(null, undefined)).toBe("transcript");
  });

  it("honors an explicit ?tab= over the interface default", () => {
    expect(initialTab("terminal", "acp")).toBe("terminal");
    expect(initialTab("files", "terminal")).toBe("files");
  });
});
