import type { ThinkTankDetail, ThinkTankEntry, ThinkTankMember, ThinkTankSummary } from "../../schemas/thinkTank";

/** Plain-language room vocabulary shared by the room page and room lists
 *  (FS-21.R9, TS-08.R87). Every state names its reason in words. */

export function thinkTankPath(roomID: string) {
  return `/think-tank/${encodeURIComponent(roomID)}`;
}

export const roomAnnotationSource = (roomID: string) => `room:${roomID}`;

export function phaseLabel(phase: ThinkTankSummary["phase"]): string {
  switch (phase) {
    case "setup": return "Setting up";
    case "openings": return "Independent openings";
    case "discussion": return "Discussion";
    case "closing": return "Closing message";
    case "ended": return "Discussion ended";
  }
}

export function endReasonText(reason: string): string {
  switch (reason) {
    case "operator": return "You ended the discussion.";
    case "participants_left": return "Participants left the discussion.";
    case "allowance_exhausted": return "Participants reached their turn limits.";
    case "left_and_exhausted": return "Participants left or reached their turn limits.";
    default: return "";
  }
}

export function judgeText(status: string, error = ""): string {
  switch (status) {
    case "waiting": return "Synthesis runs after the discussion ends.";
    case "ready":
    case "launching":
    case "starting": return "Starting the judge for a final synthesis.";
    case "running": return "The judge is writing the synthesis.";
    case "failed": return `Synthesis failed${error ? `: ${error}` : "."}`;
    case "completed": return "Synthesis complete.";
    default: return "";
  }
}

const TURN_WORDS: Record<string, string> = {
  opening: "opening",
  discussion: "turn",
  closing: "closing message",
  judge: "synthesis",
};

export function memberName(room: ThinkTankDetail, agentID: string): string {
  return room.members.find((m) => m.agent_id === agentID)?.name ?? "A participant";
}

export type RoomStatus = { tone: "active" | "waiting" | "attention" | "paused" | "ended"; text: string };

/** roomStatus is the single "current action" line: who holds the floor, what
 *  the room waits for, or why it stopped (FS-21.R9, R18–R20, R28). */
export function roomStatus(room: ThinkTankDetail): RoomStatus {
  const active = room.active ? memberName(room, room.active.agent_id) : "";
  const turn = room.active ? TURN_WORDS[room.active.turn] ?? "turn" : "";
  if (room.phase === "ended") {
    const parts = [endReasonText(room.end_reason), judgeText(room.judge_status, room.judge.error)].filter(Boolean);
    if (room.active) parts.push(`${active} is writing the ${turn}.`);
    return { tone: room.judge_status === "failed" ? "attention" : "ended", text: parts.join(" ") || "Discussion ended." };
  }
  if (room.hold) return { tone: "attention", text: room.hold };
  if (room.control === "end_requested") return { tone: "waiting", text: `Ending after ${active}'s ${turn} finishes.` };
  if (room.control === "pause_requested") return { tone: "waiting", text: `Pausing after ${active}'s ${turn} finishes.` };
  if (room.control === "paused") return { tone: "paused", text: "Paused. No new turn starts until you resume." };
  if (room.active) return { tone: "active", text: `${active} is taking a ${turn}.` };
  if (room.phase === "setup") return { tone: "waiting", text: "Starting new participants." };
  if (room.next?.waiting) return { tone: "waiting", text: `${room.next.waiting}. The room keeps this speaker.` };
  if (room.next) return { tone: "active", text: `Starting ${memberName(room, room.next.agent_id)}'s ${TURN_WORDS[room.next.turn] ?? "turn"}.` };
  return { tone: "waiting", text: phaseLabel(room.phase) };
}

export function memberState(room: ThinkTankDetail, m: ThinkTankMember): string {
  if (m.setup_state === "failed") return "Launch failed";
  if (m.setup_state === "pending" || m.setup_state === "launching") return "Launching";
  if (m.setup_state === "abandoned") return "Launch skipped";
  if (room.active?.agent_id === m.agent_id) return m.role === "judge" ? "Writing synthesis" : "Speaking";
  if (m.role === "judge") return "Judge";
  if (m.state === "departed") return "Left";
  if (m.state === "exhausted" || m.completed >= m.limit) return "Limit reached";
  if (room.next?.agent_id === m.agent_id && room.next.waiting) return "Busy — next up";
  if (room.next?.agent_id === m.agent_id) return "Next";
  return "";
}

export function entryLabel(entry: ThinkTankEntry): string {
  switch (entry.kind) {
    case "opening": return "Opening";
    case "departure": return entry.body ? "Left with a final message" : "Left the discussion";
    case "closing": return "Closing message";
    case "annotation": return "Annotations";
    case "synthesis": return "Synthesis";
    case "missing_opening": return "No opening submitted";
    default: return "";
  }
}

export function entryAuthor(entry: ThinkTankEntry): string {
  if (entry.input_id) return "You";
  return entry.agent_name || "A participant";
}
