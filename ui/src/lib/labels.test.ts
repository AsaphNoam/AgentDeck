import { describe, expect, it } from "vitest";
import { displayLabel, displayLabels } from "./labels";

describe("readable labels (FS-02.A51)", () => {
  const models = [["sonnet", "Claude Sonnet"], ["opus", "Claude Opus"], ["opus-1m", "Claude Opus"]] as const;

  it("shows the readable name alone when it is unique in the list", () => {
    expect(displayLabel(models, "sonnet")).toBe("Claude Sonnet");
  });

  it("adds the id only to entries whose names collide", () => {
    expect(displayLabels(models)).toEqual([
      ["sonnet", "Claude Sonnet"],
      ["opus", "Claude Opus (opus)"],
      ["opus-1m", "Claude Opus (opus-1m)"],
    ]);
  });

  it("returns an empty label for an id not in the list", () => {
    expect(displayLabel(models, "haiku")).toBe("");
  });
});
