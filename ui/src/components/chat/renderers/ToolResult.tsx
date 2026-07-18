import { useState } from "react";
import type { TranscriptEvent } from "../../../api/types";

const LIMIT = 600;

export function ToolResult({ event }: { event: TranscriptEvent }) {
  const [expanded, setExpanded] = useState(false);
  const raw = event.content ?? event.error ?? event.result ?? "";
  const text = typeof raw === "string" ? raw : JSON.stringify(raw, null, 2);
  const isError = event.status === "failed" || event.error != null || event.is_error === true;
  const long = text.length > LIMIT;
  const shown = long && !expanded ? `${text.slice(0, LIMIT)}…` : text;
  return (
    <article className={`tool-block tool-result ${isError ? "tool-result-error" : ""}`} data-ui="tool-result" data-state={isError ? "error" : expanded ? "expanded" : "collapsed"}>
      <pre data-slot="content">{shown}</pre>
      {long && (
        <button type="button" className="tool-toggle" data-slot="actions" onClick={() => setExpanded((v) => !v)}>
          {expanded ? "Show less" : "Show more"}
        </button>
      )}
    </article>
  );
}
