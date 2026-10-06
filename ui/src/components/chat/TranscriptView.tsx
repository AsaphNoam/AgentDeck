import { useEffect, useLayoutEffect, useRef, useState, type MouseEvent, type ReactNode } from "react";
import type { AnnotationDraft, TranscriptEvent } from "../../api/types";
import { clipAnnotationExcerpt } from "../../lib/annotations";
import { copyText } from "../../lib/copyText";
import { ErrorBoundary } from "../ErrorBoundary";
import { AssistantText } from "./renderers/AssistantText";
import { DiffBlock } from "./renderers/DiffBlock";
import { PermissionPrompt } from "./renderers/PermissionPrompt";
import { ToolCall } from "./renderers/ToolCall";
import { ToolResult } from "./renderers/ToolResult";
import { groupTranscriptRows, kindOf, ToolRun } from "./toolRun";
import { TurnError } from "./renderers/TurnError";
import { AnnotationCard } from "./renderers/AnnotationCard";
import { NoticeRow } from "./renderers/NoticeRow";
import { ThinkingDisclosure } from "./renderers/ThinkingDisclosure";
import { ChildActivity } from "./renderers/ChildActivity";
import { BackgroundTaskList } from "./renderers/BackgroundTaskList";
import { collectTasks, markBackgrounded, nestActivities, withReasoning, type ChildNode } from "./runtimeActivity";
import { useReasoningStore } from "../../store/reasoningStore";
import { projectTurns } from "./turnActivity";
import { TurnList, useFocusReturn, useTurnChoices } from "./TurnList";
import { AnnotationTray } from "./AnnotationTray";
import { AnnotationContextMenu, type AnnotationMenuState } from "./AnnotationContextMenu";
import { FileViewer } from "./FileViewer";
import type { FileLink } from "./renderers/filePath";
import { useAnnotationStore } from "../../store/annotationStore";
import { useAgentStore } from "../../store/agentStore";
import { useHeldStore } from "../../store/heldStore";
import { withdrawHeldMessage } from "../../lib/heldMessage";
import { useUiStore } from "../../store/uiStore";

// openFile/onOpenFile are per-surface, exactly as annotationsEnabled already is
// (TS-08.R57). The agent and archived-agent screens pass the file their route
// carries and a handler that writes it back to the route; the dashboard chat pane
// passes no open file and a handler that navigates to the agent screen instead,
// which is that pane's existing route to the full surface (FS-03.R53).
// taskControl offers targeted background-task Stop; only a live session whose
// runtime negotiated it passes true, and the archive never does (FS-03.R59).
// reveal names an event the caller is about to scroll to; a fresh object per request.
export function TranscriptView({ agentId, events, sourceActive = false, annotationsEnabled = true, busy = false, openFile = null, onOpenFile, taskControl = false, reveal = null }: { agentId: string; events: TranscriptEvent[]; sourceActive?: boolean; annotationsEnabled?: boolean; busy?: boolean; openFile?: FileLink | null; onOpenFile?: (link: FileLink | null, options?: { replace?: boolean }) => void; taskControl?: boolean; reveal?: { seq: number } | null }) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const atBottomRef = useRef(true);
  const [atBottom, setAtBottom] = useState(true);
  const [menu, setMenu] = useState<AnnotationMenuState | null>(null);
  const addAnnotation = useAnnotationStore((state) => state.add);
  const pushError = useUiStore((state) => state.pushError);
  // The queued follow-up renders beside the event list, never inside it: the
  // server sends no event for a message it has not delivered (TS-08.R56).
  const held = useHeldStore((state) => state.byAgent[agentId]);
  const reasoning = useReasoningStore((state) => state.byAgent[agentId]?.spans);

  // Annotating is a right-click action on the event under the pointer: it captures the
  // highlighted text when there is a selection inside that event, otherwise the whole event.
  const openMenu = (mouse: MouseEvent<HTMLDivElement>, event: TranscriptEvent) => {
    if (!annotationsEnabled || !canAnnotate(event)) return;
    const selected = selectionWithin(mouse.currentTarget);
    mouse.preventDefault();
    // Copy keeps the exact selection; the annotation excerpt stays trimmed (FS-03.R63).
    const draft = selected ? { seq: Number(event.seq), excerpt: clipAnnotationExcerpt(selected.trim()), instruction: "" } : eventDraft(event);
    setMenu({
      x: mouse.clientX,
      y: mouse.clientY,
      label: selected ? "Annotate selection" : "Annotate whole event",
      copy: selected ? () => copyText(selected, pushError) : undefined,
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
  }, [events, busy, reasoning]);

  const renderEvents = eventRenderer({
    agentId,
    onAnnotate: (draft) => addAnnotation(agentId, draft),
    onContextMenu: openMenu,
    onOpenFile,
  });

  const nested = nestActivities(withReasoning(markBackgrounded(events), reasoning));
  const choices = useTurnChoices(agentId);
  const trackFocus = useFocusReturn(scrollRef);
  // A Files-tab Diff reveal opens the completed turn hiding that row before the
  // caller scrolls to it.
  const { setOpen } = choices;
  useLayoutEffect(() => {
    if (!reveal) return;
    const turn = projectTurns(nested).find((item) => item.completed && item.seqs.has(reveal.seq));
    if (turn) setOpen(turn.key, true);
    // Only a new reveal request reopens; later updates must not undo a close.
  }, [reveal]);

  const jumpToLatest = () => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  };

  return (
    <div className="transcript-wrap" data-ui="transcript">
      {openFile && onOpenFile && (
        <FileViewer
          agentId={agentId}
          link={openFile}
          onClose={() => onOpenFile(null)}
          onOpenFile={onOpenFile}
          onPathResolved={(path) => onOpenFile({ ...openFile, path }, { replace: true })}
          onSelectionMenu={annotationsEnabled ? ({ x, y, text, draft }) => setMenu({
            x,
            y,
            label: "Annotate selection",
            copy: () => copyText(text, pushError),
            annotate: () => addAnnotation(agentId, draft),
          }) : undefined}
        />
      )}
      <div className="transcript-view" data-slot="list" ref={scrollRef} onScroll={onScroll} onFocus={trackFocus}>
        <TurnList agentId={agentId} events={nested} choices={choices} renderEvents={(list) => renderEvents(list, [], 1)} />
        <BackgroundTaskList agentId={agentId} tasks={collectTasks(events)} controllable={taskControl} />
        {busy && (
          <div className="transcript-pending" aria-live="polite">
            <span className="spinner" aria-hidden="true" />
            <span>Working…</span>
          </div>
        )}
        {held !== undefined && <HeldMessage agentId={agentId} text={held} />}
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

// eventRenderer is the one row renderer for the root list and every nested
// child (TS-08.R59). A Think Tank room reuses it for each attempt's retained
// activity, scoped to that attempt's source agent (TS-14.R15).
export function eventRenderer({ agentId, onAnnotate, onContextMenu, onOpenFile }: {
  agentId: string;
  onAnnotate: (draft: AnnotationDraft) => void;
  onContextMenu: (mouse: MouseEvent<HTMLDivElement>, event: TranscriptEvent) => void;
  onOpenFile?: (link: FileLink | null) => void;
}) {
  const renderEvents = (list: TranscriptEvent[], ancestry: string[], depth: number): ReactNode =>
    groupTranscriptRows(list).map((row, index) => {
      if (row.kind === "tool-run") {
        return (
          <ToolRun
            key={`run-${keyOf(row.events[0], index)}`}
            events={row.events}
            renderEvent={(event, eventIndex) => (
              <TranscriptEventFrame
                agentId={agentId}
                event={event}
                key={keyOf(event, eventIndex)}
                onAnnotate={onAnnotate}
                onContextMenu={onContextMenu}
                onOpenFile={onOpenFile}
                className="tool-run-event"
              />
            )}
          />
        );
      }
      if (kindOf(row.event) === "activity") {
        return <ChildActivity key={keyOf(row.event, index)} node={row.event.node as ChildNode} ancestry={ancestry} depth={depth} renderEvents={renderEvents} />;
      }
      return (
        <TranscriptEventFrame
          agentId={agentId}
          event={row.event}
          key={keyOf(row.event, index)}
          onAnnotate={onAnnotate}
          onContextMenu={onContextMenu}
          onOpenFile={onOpenFile}
        />
      );
    });
  return renderEvents;
}

// HeldMessage is the pending tail: the message the person submitted while the
// agent was working, which is not yet sent and not yet seen by the agent. It must
// read that way rather than as a sent message awaiting a reply — the failure mode
// is believing the agent already has it — and it carries its own withdraw
// affordance (FS-03.R48, TS-08.R56, INV §8).
function HeldMessage({ agentId, text }: { agentId: string; text: string }) {
  const [error, setError] = useState<string | null>(null);
  return (
    <div className="transcript-held" data-ui="transcript" data-variant="held">
      <p className="transcript-held-label">Queued — sends when this turn ends</p>
      <p className="transcript-held-text">{text}</p>
      <button
        type="button"
        className="transcript-held-withdraw"
        onClick={() => {
          setError(null);
          withdrawHeldMessage(agentId).catch(() => setError("Could not withdraw — it may already have been sent."));
        }}
      >
        Withdraw
      </button>
      {error && <p className="transcript-held-error" role="alert">{error}</p>}
    </div>
  );
}

function TranscriptEventFrame({ agentId, event, onAnnotate, onContextMenu, onOpenFile, className = "transcript-item" }: {
  agentId: string;
  event: TranscriptEvent;
  onAnnotate: (draft: AnnotationDraft) => void;
  onContextMenu: (mouse: MouseEvent<HTMLDivElement>, event: TranscriptEvent) => void;
  onOpenFile?: (link: FileLink | null) => void;
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
        <TranscriptItem agentId={agentId} event={event} onAnnotate={onAnnotate} onOpenFile={onOpenFile} />
      </ErrorBoundary>
    </div>
  );
}

type TranscriptVariant = "assistant" | "user" | "tool-call" | "tool-result" | "diff" | "permission" | "error" | "turn" | "backend-switch" | "fork-boundary" | "annotation" | "notice" | "thinking" | "unknown";


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
  if (kind === "fork_boundary") return "fork-boundary";
  if (kind === "annotation") return "annotation";
  if (kind === "notice") return "notice";
  if (kind === "reasoning") return "thinking";
  return "unknown";
}

// Stable React key: prefer the runtime seq, then a local message_id, then index.
function keyOf(event: TranscriptEvent, index: number) {
  if (event.seq != null) return `s${event.seq}`;
  if (event.message_id) return `m${event.message_id}`;
  return `i${index}`;
}

function TranscriptItem({ agentId, event, onAnnotate, onOpenFile }: { agentId: string; event: TranscriptEvent; onAnnotate: (draft: AnnotationDraft) => void; onOpenFile?: (link: FileLink | null) => void }) {
  const kind = String(event.kind ?? event.type ?? "");
  if (kind === "assistant_text") return <AssistantText event={event} onOpenFile={onOpenFile} />;
  if (kind === "user_text")
    return <article className="message user-message" data-ui="transcript" data-variant="user">{String(event.text ?? "")}</article>;
  if (kind === "permission_request") return <PermissionPrompt agentId={agentId} event={event} />;
  if (kind === "diff") return <DiffBlock event={event} onAnnotate={onAnnotate} onOpenFile={onOpenFile} />;
  if (kind === "tool_call") return <ToolCall event={event} />;
  if (kind === "tool_result") return <ToolResult event={event} />;
  if (kind === "error") return <TurnError event={event} />;
  if (kind === "annotation") return <AnnotationCard event={event} />;
  if (kind === "notice") return <NoticeRow event={event} />;
  if (kind === "reasoning") return <ThinkingDisclosure text={String(event.text ?? "")} activityId={event.activity_id} />;
  if (kind === "turn_end") return <hr className="turn-end" />;
  if (kind === "backend_switch") {
    const from = String(event.from ?? "");
    const to = String(event.to ?? "");
    return <div className="backend-switch-divider">{from} {"->"} {to}</div>;
  }
  if (kind === "fork_boundary") return <ForkBoundary sourceId={String(event.forked_from_agent_id ?? "")} />;
  // permission_resolved is folded into its prompt by the store; nothing to render.
  if (kind === "permission_resolved" || kind === "session_meta") return null;
  return <pre className="tool-block">{JSON.stringify(event, null, 2)}</pre>;
}

// ForkBoundary marks where a clone's copied history ends (FS-01.R36).
function ForkBoundary({ sourceId }: { sourceId: string }) {
  const source = useAgentStore((state) => state.agents[sourceId]?.name);
  return <div className="backend-switch-divider">Cloned from {source || "another agent"}</div>;
}

export function canAnnotate(event: TranscriptEvent) {
  const kind = String(event.kind ?? event.type ?? "");
  return event.seq != null && !["session_meta", "permission_resolved", "turn_end", "annotation"].includes(kind);
}

// The highlighted text, but only when the highlight lives inside the right-clicked event.
export function selectionWithin(host: HTMLElement): string | null {
  const selection = typeof window.getSelection === "function" ? window.getSelection() : null;
  if (!selection || selection.isCollapsed || selection.rangeCount === 0) return null;
  if (!host.contains(selection.getRangeAt(0).commonAncestorContainer)) return null;
  const text = selection.toString();
  return text.trim() ? text : null;
}

export function eventDraft(event: TranscriptEvent): AnnotationDraft {
  const raw = event.text ?? event.delta ?? event.new_text ?? event.content ?? JSON.stringify(event, null, 2);
  const excerpt = clipAnnotationExcerpt(typeof raw === "string" ? raw : JSON.stringify(raw, null, 2));
  return { seq: Number(event.seq), excerpt, instruction: "" };
}
