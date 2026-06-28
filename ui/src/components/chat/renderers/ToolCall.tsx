import { useState } from "react";
import type { TranscriptEvent } from "../../../api/types";

export function ToolCall({ event }: { event: TranscriptEvent }) {
  const [open, setOpen] = useState(false);
  const name = String(event.name ?? event.tool ?? "tool");
  const args = event.args;
  const hasArgs = args !== undefined && args !== null;
  return (
    <article className="tool-block tool-call">
      <button type="button" className="tool-toggle" onClick={() => setOpen((v) => !v)}>
        {hasArgs ? (open ? "▾" : "▸") : ""} Tool call: {name}
      </button>
      {hasArgs && open && <pre className="tool-args">{JSON.stringify(args, null, 2)}</pre>}
    </article>
  );
}
