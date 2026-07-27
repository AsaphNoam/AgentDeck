import { useEffect, useRef, useState, type MouseEvent } from "react";
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
import { AnnotationContextMenu, type AnnotationMenuState } from "./AnnotationContextMenu";
import { useAnnotationStore } from "../../store/annotationStore";

export function TranscriptView({ agentId, events, sourceActive = false, annotationsEnabled = true }: { agentId: string; events: TranscriptEvent[]; sourceActive?: boolean; annotationsEnabled?: boolean }) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const atBottomRef = useRef(true);
  const [atBottom, setAtBottom] = useState(true);
  const [menu, setMenu] = useState<AnnotationMenuState | null>(null);
  const addAnnotation = useAnnotationStore((state) => state.add);

  // Annotating is a right-click action on the event under the pointer: it captures the
  // highlighted text when there is a selection inside that event, otherwise the whole event.
  const openMenu = (mouse: MouseEvent<HTMLDivElement>, event: TranscriptEvent) => {
    if (!annotationsEnabled || !canAnnotate(event)) return;
    const selected = selectionWithin(mouse.currentTarget);
    mouse.preventDefault();
    const draft = selected ? { seq: Number(event.seq), excerpt: clipAnnotationExcerpt(selected), instruction: "" } : eventDraft(event);
    setMenu({
      x: mouse.clientX,
      y: mouse.clientY,
      label: selected ? "Annotate selection" : "Annotate whole event",
      annotate: () => addAnnotation(agentId, draft),
    });
  };

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
          <div
            key={keyOf(event, index)}
            className="transcript-item"
            data-slot="event"
            data-variant={variantOf(event)}
            data-seq={event.seq ?? undefined}
            onContextMenu={(mouse) => openMenu(mouse, event)}
          >
            <ErrorBoundary
              label="message"
              fallback={<pre className="tool-block tool-result-error">Failed to render this event.</pre>}
            >
              <TranscriptItem agentId={agentId} event={event} onAnnotate={(draft) => addAnnotation(agentId, draft)} />
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
      <AnnotationContextMenu menu={menu} onClose={() => setMenu(null)} />
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

function TranscriptItem({ agentId, event, onAnnotate }: { agentId: string; event: TranscriptEvent; onAnnotate: (draft: AnnotationDraft) => void }) {
  const kind = String(event.kind ?? event.type ?? "");
  if (kind === "assistant_text") return <AssistantText event={event} />;
  if (kind === "user_text")
    return <article className="message user-message" data-ui="transcript" data-variant="user">{String(event.text ?? "")}</article>;
  if (kind === "permission_request") return <PermissionPrompt agentId={agentId} event={event} />;
  if (kind === "diff") return <DiffBlock event={event} onAnnotate={onAnnotate} />;
  if (kind === "tool_call") return <ToolCall event={event} />;
  if (kind === "tool_result") return <ToolResult event={event} />;
  if (kind === "error") return <TurnError event={event} />;
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

// The highlighted text, but only when the highlight lives inside the right-clicked event.
function selectionWithin(host: HTMLElement): string | null {
  const selection = typeof window.getSelection === "function" ? window.getSelection() : null;
  if (!selection || selection.isCollapsed || selection.rangeCount === 0) return null;
  if (!host.contains(selection.getRangeAt(0).commonAncestorContainer)) return null;
  return selection.toString().trim() || null;
}

function eventDraft(event: TranscriptEvent): AnnotationDraft {
  const raw = event.text ?? event.delta ?? event.new_text ?? event.content ?? JSON.stringify(event, null, 2);
  const excerpt = clipAnnotationExcerpt(typeof raw === "string" ? raw : JSON.stringify(raw, null, 2));
  return { seq: Number(event.seq), excerpt, instruction: "" };
}
