import type { ThinkTankMember } from "../../schemas/thinkTank";

export const SPEAKER_SLOTS = [
  "0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15",
  "16", "17", "18", "19", "20", "21", "22", "23", "24", "25", "26", "27", "28", "29", "30", "31",
] as const;

/** Retained member order survives departure/deletion; judge retries share a role slot. */
export function speakerSlot(member: ThinkTankMember | undefined): typeof SPEAKER_SLOTS[number] | "judge" | undefined {
  if (!member) return undefined;
  if (member.role === "judge") return "judge";
  return Number.isInteger(member.order) ? SPEAKER_SLOTS[member.order] : undefined;
}
