import { useState, type ReactNode } from "react";
import type { TranscriptEvent } from "../../api/types";
import { shouldRenderToolResult } from "./renderers/ToolResult";

// Consecutive tool calls and their results fold into one collapsed "Ran N
// tools" row. The desktop transcript and the phone conversation share this
// grouping so a result never renders outside its tool line (FS-20.R13, INV §2).

export type TranscriptRow = { kind: "event"; event: TranscriptEvent } | { kind: "tool-run"; events: TranscriptEvent[] };

export function groupTranscriptRows(events: TranscriptEvent[]): TranscriptRow[] {
  const rows: TranscriptRow[] = [];
  for (let index = 0; index < events.length;) {
    const event = events[index];
    if (!isToolEvent(event)) {
      if (shouldRenderToolEvent(event)) rows.push({ kind: "event", event });
      index++;
      continue;
    }
    const run: TranscriptEvent[] = [];
    while (index < events.length && isToolEvent(events[index])) run.push(events[index++]);
    if (run.some((item) => kindOf(item) === "tool_call")) rows.push({ kind: "tool-run", events: run });
    else run.filter(shouldRenderToolEvent).forEach((item) => rows.push({ kind: "event", event: item }));
  }
  return rows;
}

export function ToolRun({ events, renderEvent }: { events: TranscriptEvent[]; renderEvent: (event: TranscriptEvent, index: number) => ReactNode }) {
  const [open, setOpen] = useState(false);
  const count = events.filter((event) => kindOf(event) === "tool_call").length;
  return (
    <section className="tool-run" data-ui="tool-run" data-state={open ? "expanded" : "collapsed"}>
      <button type="button" className="tool-toggle" data-slot="trigger" aria-label={`Ran ${count} tool${count === 1 ? "" : "s"}`} aria-expanded={open} onClick={() => setOpen((value) => !value)}>
        {open ? "▾" : "▸"} Ran {count} tool{count === 1 ? "" : "s"}
      </button>
      {open && <div className="tool-run-content" data-slot="content">{events.filter(shouldRenderToolEvent).map(renderEvent)}</div>}
    </section>
  );
}

export function kindOf(event: TranscriptEvent) {
  return String(event.kind ?? event.type ?? "");
}

function isToolEvent(event: TranscriptEvent) {
  const kind = kindOf(event);
  return kind === "tool_call" || kind === "tool_result";
}

function shouldRenderToolEvent(event: TranscriptEvent) {
  return kindOf(event) !== "tool_result" || shouldRenderToolResult(event);
}
