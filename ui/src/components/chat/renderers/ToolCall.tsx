import { useState } from "react";
import type { TranscriptEvent } from "../../../api/types";

export function ToolCall({ event }: { event: TranscriptEvent }) {
  const [open, setOpen] = useState(false);
  const name = String(event.name ?? event.tool ?? "tool");
  const args = event.args;
  const hasArgs = args !== undefined && args !== null;
  return (
    <article className="tool-block tool-call" data-ui="tool-call" data-state={open ? "expanded" : "collapsed"}>
      <button type="button" className="tool-toggle" data-slot="trigger" onClick={() => setOpen((v) => !v)}>
        {hasArgs ? (open ? "▾" : "▸") : ""} Tool call: {name}
      </button>
      {hasArgs && open && <pre className="tool-args" data-slot="content">{JSON.stringify(args, null, 2)}</pre>}
    </article>
  );
}
