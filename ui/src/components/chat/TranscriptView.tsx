import { useEffect, useRef, useState } from "react";
import type { TranscriptEvent } from "../../api/types";
import { ErrorBoundary } from "../ErrorBoundary";
import { AssistantText } from "./renderers/AssistantText";
import { DiffBlock } from "./renderers/DiffBlock";
import { PermissionPrompt } from "./renderers/PermissionPrompt";
import { ToolCall } from "./renderers/ToolCall";
import { ToolResult } from "./renderers/ToolResult";
import { TurnError } from "./renderers/TurnError";

export function TranscriptView({ agentId, events }: { agentId: string; events: TranscriptEvent[] }) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const atBottomRef = useRef(true);
  const [atBottom, setAtBottom] = useState(true);

  const onScroll = () => {
    const el = scrollRef.current;
    if (!el) return;
    const stuck = el.scrollHeight - el.scrollTop - el.clientHeight < 24;
    atBottomRef.current = stuck;
    setAtBottom(stuck);
  };

  useEffect(() => {
    const el = scrollRef.current;
    if (el && atBottomRef.current) el.scrollTop = el.scrollHeight;
  }, [events]);

  const jumpToLatest = () => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  };

  return (
    <div className="transcript-wrap">
      <div className="transcript-view" ref={scrollRef} onScroll={onScroll}>
        {events.map((event, index) => (
          // data-seq lets the Files tab's "Diff" action scroll to this event
          // (present only when the event carries a runtime seq).
          <div key={keyOf(event, index)} className="transcript-item" data-seq={event.seq ?? undefined}>
            <ErrorBoundary
              label="message"
              fallback={<pre className="tool-block tool-result-error">Failed to render this event.</pre>}
            >
              <TranscriptItem agentId={agentId} event={event} />
            </ErrorBoundary>
          </div>
        ))}
      </div>
      {!atBottom && (
        <button type="button" className="jump-to-latest" onClick={jumpToLatest}>
          Jump to latest
        </button>
      )}
    </div>
  );
}

// Stable React key: prefer the runtime seq, then a local message_id, then index.
function keyOf(event: TranscriptEvent, index: number) {
  if (event.seq != null) return `s${event.seq}`;
  if (event.message_id) return `m${event.message_id}`;
  return `i${index}`;
}

function TranscriptItem({ agentId, event }: { agentId: string; event: TranscriptEvent }) {
  const kind = String(event.kind ?? event.type ?? "");
  if (kind === "assistant_text") return <AssistantText event={event} />;
  if (kind === "user_text")
    return <article className="message user-message">{String(event.text ?? "")}</article>;
  if (kind === "permission_request") return <PermissionPrompt agentId={agentId} event={event} />;
  if (kind === "diff") return <DiffBlock event={event} />;
  if (kind === "tool_call") return <ToolCall event={event} />;
  if (kind === "tool_result") return <ToolResult event={event} />;
  if (kind === "error") return <TurnError event={event} />;
  if (kind === "turn_end") return <hr className="turn-end" />;
  if (kind === "backend_switch") {
    const from = String(event.from ?? "");
    const to = String(event.to ?? "");
    return <div className="backend-switch-divider">{from} {"->"} {to}</div>;
  }
  // permission_resolved is folded into its prompt by the store; nothing to render.
  if (kind === "permission_resolved" || kind === "session_meta") return null;
  return <pre className="tool-block">{JSON.stringify(event, null, 2)}</pre>;
}
