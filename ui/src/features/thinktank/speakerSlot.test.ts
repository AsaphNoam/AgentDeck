import { describe, expect, it } from "vitest";
import { thinkTankDetailSchema } from "../../schemas/thinkTank";
import { speakerSlot } from "./speakerSlot";
import fixture from "./fixtures/room.json";

describe("retained room speaker identity", () => {
  const member = thinkTankDetailSchema.parse(fixture.room).members[0];
  it("assigns every bounded participant and a separate judge", () => {
    const slots = Array.from({ length: 32 }, (_, order) => speakerSlot({ ...member, order }));
    expect(new Set(slots).size).toBe(32);
    expect(slots[0]).toBe("0");
    expect(slots[31]).toBe("31");
    expect(speakerSlot({ ...member, role: "judge", order: 0 })).toBe("judge");
  });
  it("preserves fixed identity after departure/deletion and refuses unknown slots", () => {
    expect(speakerSlot({ ...member, order: 7, state: "departed", exists: false })).toBe("7");
    expect(speakerSlot(undefined)).toBeUndefined();
    expect(speakerSlot({ ...member, order: 32 })).toBeUndefined();
  });
});
