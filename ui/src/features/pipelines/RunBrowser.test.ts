import { describe, expect, it } from "vitest";
import { attemptTranscriptPath } from "./RunBrowser";

describe("pipeline attempt transcript links", () => {
  it("routes stopped attempt agents to the durable archive transcript", () => {
    expect(attemptTranscriptPath("a_finished", false)).toBe("/archive/a_finished");
    expect(attemptTranscriptPath("a_live", true)).toBe("/agent/a_live");
  });
});
