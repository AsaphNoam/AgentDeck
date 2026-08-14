import { useEffect, useRef, useState, type MouseEvent, type ReactNode } from "react";
import type { AnnotationDraft, TranscriptEvent } from "../../api/types";
import { clipAnnotationExcerpt } from "../../lib/annotations";
import { ErrorBoundary } from "../ErrorBoundary";
import { AssistantText } from "./renderers/AssistantText";
import { DiffBlock } from "./renderers/DiffBlock";
import { PermissionPrompt } from "./renderers/PermissionPrompt";
import { ToolCall } from "./renderers/ToolCall";
import { shouldRenderToolResult, ToolResult } from "./renderers/ToolResult";
import { TurnError } from "./renderers/TurnError";
import { AnnotationCard } from "./renderers/AnnotationCard";
import { AnnotationTray } from "./AnnotationTray";
import { AnnotationContextMenu, type AnnotationMenuState } from "./AnnotationContextMenu";
import { useAnnotationStore } from "../../store/annotationStore";

export function TranscriptView({ agentId, events, sourceActive = false, annotationsEnabled = true, busy = false }: { agentId: string; events: TranscriptEvent[]; sourceActive?: boolean; annotationsEnabled?: boolean; busy?: boolean }) {
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
  }, [events, busy]);

  const jumpToLatest = () => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  };

  return (
    <div className="transcript-wrap" data-ui="transcript">
      <div className="transcript-view" data-slot="list" ref={scrollRef} onScroll={onScroll}>
        {groupTranscriptRows(events).map((row, index) => row.kind === "tool-run" ? (
          <ToolRun
            key={`run-${keyOf(row.events[0], index)}`}
            events={row.events}
            renderEvent={(event, eventIndex) => (
              <TranscriptEventFrame
                agentId={agentId}
                event={event}
                key={keyOf(event, eventIndex)}
                onAnnotate={(draft) => addAnnotation(agentId, draft)}
                onContextMenu={openMenu}
                className="tool-run-event"
              />
            )}
          />
        ) : (
          <TranscriptEventFrame
            agentId={agentId}
            event={row.event}
            key={keyOf(row.event, index)}
            onAnnotate={(draft) => addAnnotation(agentId, draft)}
            onContextMenu={openMenu}
          />
        ))}
        {busy && (
          <div className="transcript-pending" aria-live="polite">
            <span className="spinner" aria-hidden="true" />
            <span>Working…</span>
          </div>
        )}
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

function TranscriptEventFrame({ agentId, event, onAnnotate, onContextMenu, className = "transcript-item" }: {
  agentId: string;
  event: TranscriptEvent;
  onAnnotate: (draft: AnnotationDraft) => void;
  onContextMenu: (mouse: MouseEvent<HTMLDivElement>, event: TranscriptEvent) => void;
  className?: string;
}) {
  return (
    // data-seq lets the Files tab's "Diff" action scroll to this event
    // (present only when the event carries a runtime seq).
    <div
      className={className}
      data-slot="event"
      data-variant={variantOf(event)}
      data-seq={event.seq ?? undefined}
      onContextMenu={(mouse) => onContextMenu(mouse, event)}
    >
      <ErrorBoundary
        label="message"
        fallback={<pre className="tool-block tool-result-error">Failed to render this event.</pre>}
      >
        <TranscriptItem agentId={agentId} event={event} onAnnotate={onAnnotate} />
      </ErrorBoundary>
    </div>
  );
}

function ToolRun({ events, renderEvent }: { events: TranscriptEvent[]; renderEvent: (event: TranscriptEvent, index: number) => ReactNode }) {
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

type TranscriptVariant = "assistant" | "user" | "tool-call" | "tool-result" | "diff" | "permission" | "error" | "turn" | "backend-switch" | "annotation" | "unknown";

type TranscriptRow = { kind: "event"; event: TranscriptEvent } | { kind: "tool-run"; events: TranscriptEvent[] };

export function groupTranscriptRows(events: TranscriptEvent[]): TranscriptRow[] {
  const rows: TranscriptRow[] = [];
  for (let index = 0; index < events.length;) {
    const event = events[index];
    if (!isToolEvent(event)) {
      if (shouldRenderTranscriptEvent(event)) rows.push({ kind: "event", event });
      index++;
      continue;
    }
    const run: TranscriptEvent[] = [];
    while (index < events.length && isToolEvent(events[index])) run.push(events[index++]);
    if (run.some((item) => kindOf(item) === "tool_call")) rows.push({ kind: "tool-run", events: run });
    else run.filter(shouldRenderTranscriptEvent).forEach((item) => rows.push({ kind: "event", event: item }));
  }
  return rows;
}

function isToolEvent(event: TranscriptEvent) {
  const kind = kindOf(event);
  return kind === "tool_call" || kind === "tool_result";
}

function shouldRenderToolEvent(event: TranscriptEvent) {
  return kindOf(event) !== "tool_result" || shouldRenderToolResult(event);
}

function shouldRenderTranscriptEvent(event: TranscriptEvent) {
  return shouldRenderToolEvent(event);
}

function kindOf(event: TranscriptEvent) {
  return String(event.kind ?? event.type ?? "");
}

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
