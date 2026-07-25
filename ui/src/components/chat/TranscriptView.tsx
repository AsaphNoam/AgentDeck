import { useEffect, useRef, useState } from "react";
import type { AnnotationDraft, TranscriptEvent } from "../../api/types";
import { clipAnnotationExcerpt } from "../../lib/annotations";
import { ErrorBoundary } from "../ErrorBoundary";
import { AssistantText } from "./renderers/AssistantText";
import { DiffBlock } from "./renderers/DiffBlock";
import { PermissionPrompt } from "./renderers/PermissionPrompt";
import { ToolCall } from "./renderers/ToolCall";
import { ToolResult } from "./renderers/ToolResult";
import { TurnError } from "./renderers/TurnError";
import { AnnotationCard } from "./renderers/AnnotationCard";
import { AnnotationTray } from "./AnnotationTray";
import { useAnnotationStore } from "../../store/annotationStore";

export function TranscriptView({ agentId, events, sourceActive = false, annotationsEnabled = true }: { agentId: string; events: TranscriptEvent[]; sourceActive?: boolean; annotationsEnabled?: boolean }) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const atBottomRef = useRef(true);
  const [atBottom, setAtBottom] = useState(true);
  const addAnnotation = useAnnotationStore((state) => state.add);

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
    <div className="transcript-wrap" data-ui="transcript">
      <div className="transcript-view" data-slot="list" ref={scrollRef} onScroll={onScroll}>
        {events.map((event, index) => (
          // data-seq lets the Files tab's "Diff" action scroll to this event
          // (present only when the event carries a runtime seq).
          <div key={keyOf(event, index)} className="transcript-item" data-slot="event" data-variant={variantOf(event)} data-seq={event.seq ?? undefined}>
            <ErrorBoundary
              label="message"
              fallback={<pre className="tool-block tool-result-error">Failed to render this event.</pre>}
            >
              <TranscriptItem agentId={agentId} event={event} annotationsEnabled={annotationsEnabled} onAnnotate={(draft) => addAnnotation(agentId, draft)} />
            </ErrorBoundary>
          </div>
        ))}
      </div>
      {!atBottom && (
        <button type="button" className="jump-to-latest" onClick={jumpToLatest}>
          Jump to latest
        </button>
      )}
      {annotationsEnabled && <AnnotationTray sourceId={agentId} sourceActive={sourceActive} />}
    </div>
  );
}

type TranscriptVariant = "assistant" | "user" | "tool-call" | "tool-result" | "diff" | "permission" | "error" | "turn" | "backend-switch" | "annotation" | "unknown";

function variantOf(event: TranscriptEvent): TranscriptVariant {
  const kind = String(event.kind ?? event.type ?? "");
  if (kind === "assistant_text") return "assistant";
  if (kind === "user_text") return "user";
  if (kind === "tool_call") return "tool-call";
  if (kind === "tool_result") return "tool-result";
  if (kind === "diff") return "diff";
  if (kind === "permission_request" || kind === "permission_resolved") return "permission";
  if (kind === "error") return "error";
  if (kind === "turn_end") return "turn";
  if (kind === "backend_switch") return "backend-switch";
  if (kind === "annotation") return "annotation";
  return "unknown";
}

// Stable React key: prefer the runtime seq, then a local message_id, then index.
function keyOf(event: TranscriptEvent, index: number) {
  if (event.seq != null) return `s${event.seq}`;
  if (event.message_id) return `m${event.message_id}`;
  return `i${index}`;
}

function TranscriptItem({ agentId, event, annotationsEnabled, onAnnotate }: { agentId: string; event: TranscriptEvent; annotationsEnabled: boolean; onAnnotate: (draft: AnnotationDraft) => void }) {
  const kind = String(event.kind ?? event.type ?? "");
  const action = annotationsEnabled && canAnnotate(event) ? <button type="button" className="annotation-event-trigger" onClick={() => onAnnotate(eventDraft(event))}>Annotate</button> : null;
  if (kind === "assistant_text") return <>{action}<AssistantText event={event} /></>;
  if (kind === "user_text")
    return <>{action}<article className="message user-message" data-ui="transcript" data-variant="user">{String(event.text ?? "")}</article></>;
  if (kind === "permission_request") return <>{action}<PermissionPrompt agentId={agentId} event={event} /></>;
  if (kind === "diff") return <DiffBlock event={event} onAnnotate={onAnnotate} />;
  if (kind === "tool_call") return <>{action}<ToolCall event={event} /></>;
  if (kind === "tool_result") return <>{action}<ToolResult event={event} /></>;
  if (kind === "error") return <>{action}<TurnError event={event} /></>;
  if (kind === "annotation") return <AnnotationCard event={event} />;
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

function canAnnotate(event: TranscriptEvent) {
  const kind = String(event.kind ?? event.type ?? "");
  return event.seq != null && !["session_meta", "permission_resolved", "turn_end", "annotation"].includes(kind);
}

function eventDraft(event: TranscriptEvent): AnnotationDraft {
  const raw = event.text ?? event.delta ?? event.new_text ?? event.content ?? JSON.stringify(event, null, 2);
  const excerpt = clipAnnotationExcerpt(typeof raw === "string" ? raw : JSON.stringify(raw, null, 2));
  return { seq: Number(event.seq), excerpt, instruction: "" };
}
