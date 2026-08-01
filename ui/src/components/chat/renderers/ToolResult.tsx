import { useState } from "react";
import type { TranscriptEvent } from "../../../api/types";

const LIMIT = 600;

export function ToolResult({ event }: { event: TranscriptEvent }) {
  const [expanded, setExpanded] = useState(false);
  const raw = firstDisplayable(event.content, event.error, event.result);
  const text = formatResult(raw);
  const isError = event.status === "failed" || event.error != null || event.is_error === true;
  const empty = text.trim() === "";
  if (empty && !isError) return null;
  const long = text.length > LIMIT;
  const shown = long && !expanded ? `${text.slice(0, LIMIT)}…` : text;
  const state = isError ? "error" : expanded ? "expanded" : "collapsed";
  return (
    <article className={`tool-block tool-result ${isError ? "tool-result-error" : ""}`} data-ui="tool-result" data-state={state}>
      {empty ? <span data-slot="content">Failed</span> : <pre data-slot="content">{shown}</pre>}
      {!empty && long && (
        <button type="button" className="tool-toggle" data-slot="actions" onClick={() => setExpanded((v) => !v)}>
          {expanded ? "Show less" : "Show more"}
        </button>
      )}
    </article>
  );
}

export function shouldRenderToolResult(event: TranscriptEvent) {
  const text = formatResult(firstDisplayable(event.content, event.error, event.result));
  return text.trim() !== "" || event.status === "failed" || event.error != null || event.is_error === true;
}

function firstDisplayable(...values: unknown[]) {
  return values.find((value) => formatResult(value).trim() !== "");
}

function formatResult(value: unknown): string {
  if (value == null) return "";
  if (typeof value === "string") return value;
  const json = JSON.stringify(value, null, 2);
  return json ?? "";
}
