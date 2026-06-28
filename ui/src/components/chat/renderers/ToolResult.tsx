import { useState } from "react";
import type { TranscriptEvent } from "../../../api/types";

const LIMIT = 600;

export function ToolResult({ event }: { event: TranscriptEvent }) {
  const [expanded, setExpanded] = useState(false);
  const raw = event.result ?? event.status ?? "";
  const text = typeof raw === "string" ? raw : JSON.stringify(raw, null, 2);
  const isError = event.is_error === true;
  const long = text.length > LIMIT;
  const shown = long && !expanded ? `${text.slice(0, LIMIT)}…` : text;
  return (
    <article className={`tool-block tool-result ${isError ? "tool-result-error" : ""}`}>
      <pre>{shown}</pre>
      {long && (
        <button type="button" className="tool-toggle" onClick={() => setExpanded((v) => !v)}>
          {expanded ? "Show less" : "Show more"}
        </button>
      )}
    </article>
  );
}
