import { useState } from "react";
import type { TranscriptEvent } from "../../../api/types";

export function ToolCall({ event }: { event: TranscriptEvent }) {
  const [open, setOpen] = useState(false);
  const name = String(event.name ?? event.tool ?? "tool");
  const args = event.args;
  const hasArgs = args !== undefined && args !== null;
  const background = event.background_state === "running" ? "Continues in background" : event.background_state ? "Ran in background" : null;
  return (
    <article className="tool-block tool-call" data-ui="tool-call" data-state={open ? "expanded" : "collapsed"}>
      <button type="button" className="tool-toggle" data-slot="trigger" onClick={() => setOpen((v) => !v)}>
        {hasArgs ? (open ? "▾" : "▸") : ""} Tool call: {name}
      </button>
      {background && <span className="tool-call-background">{background}</span>}
      {hasArgs && open && <pre className="tool-args" data-slot="content">{JSON.stringify(args, null, 2)}</pre>}
    </article>
  );
}
