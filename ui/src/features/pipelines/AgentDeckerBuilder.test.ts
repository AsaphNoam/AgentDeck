import { describe, expect, it } from "vitest";
import { shouldDropBuilderSession } from "./AgentDeckerBuilder";

describe("builder session lifecycle", () => {
  it("drops a stopped builder after hydration regardless of transcript content", () => {
    expect(shouldDropBuilderSession("a_stale", false, false, false, false)).toBe(false);
    expect(shouldDropBuilderSession("a_stale", false, true, true, false)).toBe(false);
    expect(shouldDropBuilderSession("a_stale", false, true, false, false)).toBe(true);
    expect(shouldDropBuilderSession("a_live", true, true, false, false)).toBe(false);
    expect(shouldDropBuilderSession("a_new", false, true, false, true)).toBe(false);
  });
});
