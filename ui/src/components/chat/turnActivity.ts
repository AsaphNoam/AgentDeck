import type { TranscriptEvent } from "../../api/types";
import { hasPendingPermission, type ChildNode } from "./runtimeActivity";

// The turn projection (FS-03.R74, TS-08.R100–R101): one pure pass over the
// nested root list that partitions it into root turns and, for each proven
// completed turn, marks which rows belong behind its one activity control.
// It never reorders, copies or rewrites events; every surface renders the
// returned parts in order with its own row renderer (INV §2).

export interface TurnPart {
  hidden: boolean;
  events: TranscriptEvent[];
}

export interface Turn {
  /** Stable for the turn's whole life: the boundary that opened it. */
  key: string;
  /** Only a root turn_end proves completion. */
  completed: boolean;
  /** The root turn_end's stop_reason when completed. */
  stopReason?: string;
  /** Whether a visible root response survives (partial labelling). */
  hasResponse: boolean;
  parts: TurnPart[];
  /** Every runtime seq the turn holds, including nested child rows. */
  seqs: Set<number>;
}

// Rows that end the provable extent of an unfinished turn: a resumed session,
// a backend switch or a clone's copied history. The turn they cut stays
// incomplete and is never labelled or collapsed (FS-03.R76).
function isFence(event: TranscriptEvent) {
  const kind = kindOf(event);
  return kind === "backend_switch" || kind === "fork_boundary" || (kind === "session_meta" && Boolean(event.resumed_at));
}

function isRootTurnEnd(event: TranscriptEvent) {
  return kindOf(event) === "turn_end" && !event.activity_id;
}

// A turn key is the seq of the boundary before it, so a live turn keeps one
// identity from its first optimistic row through its terminal event. Boundary
// rows are durable, so the key never depends on optimistic input (TS-08.R102).
function boundaryKey(event: TranscriptEvent) {
  return String(event.seq ?? "unsequenced");
}

// projectTurns partitions a root list produced by nestActivities.
export function projectTurns(events: TranscriptEvent[], scope = ""): Turn[] {
  const turns: Turn[] = [];
  let buffer: TranscriptEvent[] = [];
  let key = `${scope}start`;
  const close = (completed: boolean, stopReason?: string) => {
    if (buffer.length) turns.push(build(key, buffer, completed, stopReason));
    buffer = [];
  };
  for (const event of events) {
    if (isFence(event)) {
      close(false);
      key = `${scope}${boundaryKey(event)}`;
      buffer.push(event);
      continue;
    }
    buffer.push(event);
    if (isRootTurnEnd(event)) {
      close(true, String(event.stop_reason ?? ""));
      key = `${scope}${boundaryKey(event)}`;
    }
  }
  close(false);
  return turns;
}

// openTurnKey names the turn a live row admitted now belongs to: the same key
// projectTurns gives the unfinished tail of this folded list (TS-08.R103).
export function openTurnKey(events: TranscriptEvent[]): string {
  for (let index = events.length - 1; index >= 0; index--) {
    if (isFence(events[index]) || isRootTurnEnd(events[index])) return boundaryKey(events[index]);
  }
  return "start";
}

// slotTurnKeys gives, for each insertion slot 0..n of this list, the key of the
// turn a row placed there belongs to, matching openTurnKey (TS-08.R103).
export function slotTurnKeys(events: TranscriptEvent[]): string[] {
  const keys = ["start"];
  for (const event of events) keys.push(isFence(event) || isRootTurnEnd(event) ? boundaryKey(event) : keys[keys.length - 1]);
  return keys;
}

function build(key: string, events: TranscriptEvent[], completed: boolean, stopReason?: string): Turn {
  const seqs = new Set<number>();
  collectSeqs(events, seqs);
  let response = -1;
  events.forEach((event, index) => {
    if (kindOf(event) === "assistant_text") response = index;
  });
  if (!completed) return { key, completed, hasResponse: response >= 0, parts: [{ hidden: false, events }], seqs };
  const parts: TurnPart[] = [];
  events.forEach((event, index) => {
    const hidden = index !== response && isActivity(event);
    const last = parts[parts.length - 1];
    if (last && last.hidden === hidden) last.events.push(event);
    else parts.push({ hidden, events: [event] });
  });
  return { key, completed, stopReason, hasResponse: response >= 0, parts, seqs };
}

// isActivity names what a completed turn tucks away: thoughts, tools and their
// diffs, settled child activity, settled permissions and earlier root passages.
// Input, the response, errors, notices, outcomes and anything still waiting on
// the person stay in the reading path; unknown rows stay visible (FS-03.R76).
function isActivity(event: TranscriptEvent) {
  const kind = kindOf(event);
  if (kind === "activity") return !hasPendingPermission(event.node as ChildNode);
  if (kind === "permission_request") return Boolean(event.resolved);
  return kind === "assistant_text" || kind === "reasoning" || kind === "tool_call" || kind === "tool_result" || kind === "diff";
}

function collectSeqs(events: TranscriptEvent[], seqs: Set<number>) {
  for (const event of events) {
    if (typeof event.seq === "number") seqs.add(event.seq);
    if (kindOf(event) === "activity") collectSeqs((event.node as ChildNode).items, seqs);
  }
}

// ACP stop reasons that cut a turn short, in the person's words. Anything else,
// including an unknown future reason, claims no outcome (INV §8).
const SHORT_OUTCOMES: Record<string, string> = {
  cancelled: "Cancelled",
  error: "Failed",
  max_tokens: "Stopped at the token limit",
  max_turn_requests: "Stopped at the request limit",
  refusal: "Refused",
};

// The visible outcome of a turn that did not end normally. The response above
// it, when any, reads as partial rather than final (FS-03.R74).
export function turnOutcome(turn: Pick<Turn, "completed" | "stopReason" | "hasResponse">): string | null {
  const reason = turn.completed ? SHORT_OUTCOMES[turn.stopReason ?? ""] : undefined;
  if (!reason) return null;
  return turn.hasResponse ? `${reason} — response is partial` : reason;
}

function kindOf(event: TranscriptEvent) {
  return String(event.kind ?? event.type ?? "");
}
