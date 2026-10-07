import { Link } from "react-router-dom";
import type { TranscriptEvent } from "../../api/types";
import type { ThinkTankResult } from "../../api/thinkTanks";
import { thinkTankPath } from "../../features/thinktank/roomText";
import { SanitizedMarkdown } from "./renderers/SanitizedMarkdown";

export const THINK_TANK_RESULT_KIND = "think_tank_result";

/** mergeThinkTankResults places each retained judge synthesis just before
 *  the turn end that completed it, so it reads as that turn's result rather
 *  than as collapsible activity (FS-21.R50, TS-14.R26). An anchor beyond the
 *  loaded window waits for it; a missing anchor inside the window (or none
 *  recorded) still shows the result, framed as source-unavailable. The
 *  provider transcript itself is unchanged. */
export function mergeThinkTankResults(events: TranscriptEvent[], results: ThinkTankResult[] | undefined): TranscriptEvent[] {
  if (!results?.length) return events;
  const seqs = events.map((event) => event.seq).filter((seq): seq is number => typeof seq === "number");
  const first = seqs.length ? Math.min(...seqs) : Infinity;
  const last = seqs.length ? Math.max(...seqs) : -Infinity;
  const anchored = new Map<number, TranscriptEvent[]>();
  const loose: TranscriptEvent[] = [];
  for (const result of results) {
    const row: TranscriptEvent = { kind: THINK_TANK_RESULT_KIND, result_id: result.result_id, result } as TranscriptEvent;
    if (result.event_seq > 0 && seqs.includes(result.event_seq)) {
      anchored.set(result.event_seq, [...(anchored.get(result.event_seq) ?? []), row]);
    } else if (result.event_seq === 0 || (result.event_seq > first && result.event_seq < last)) {
      loose.push({ ...row, source_unavailable: true } as TranscriptEvent);
    }
  }
  const out: TranscriptEvent[] = [];
  for (const event of events) {
    if (typeof event.seq === "number" && anchored.has(event.seq)) out.push(...anchored.get(event.seq)!);
    out.push(event);
  }
  return [...out, ...loose];
}

export function ThinkTankResultRow({ event }: { event: TranscriptEvent }) {
  const result = event.result as ThinkTankResult;
  return (
    <section className="think-tank-result" data-ui="think-tank" data-slot="result" aria-label={`Think Tank synthesis for ${result.room_title}`}>
      <header>
        <strong>Think Tank synthesis</strong>
        {result.room_available
          ? <Link to={thinkTankPath(result.room_id)}>{result.room_title}</Link>
          : <span>{result.room_title} (room deleted)</span>}
        {Boolean(event.source_unavailable) && <span>Its originating turn is not in this transcript.</span>}
      </header>
      <SanitizedMarkdown text={result.body} />
    </section>
  );
}
