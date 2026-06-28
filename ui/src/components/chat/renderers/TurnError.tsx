import type { TranscriptEvent } from "../../../api/types";

export function TurnError({ event }: { event: TranscriptEvent }) {
  return <p className="turn-error">{String(event.message ?? "Error")}</p>;
}
