import { useEffect, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  cancelTurn,
  decidePermission,
  getHeldPrompt,
  getTranscriptWindow,
  resumeAgent,
  sendPrompt,
  steerPrompt,
  stopAgent,
  withdrawPrompt,
} from "../api/client";
import type { AgentState, AnnotationDraft, TranscriptEvent } from "../api/types";
import { useAnnotationStore } from "../store/annotationStore";
import { normalizeEvent } from "../store/transcriptStore";
import { AssistantText } from "../components/chat/renderers/AssistantText";
import { DiffBlock } from "../components/chat/renderers/DiffBlock";
import { ToolCall } from "../components/chat/renderers/ToolCall";
import { shouldRenderToolResult, ToolResult } from "../components/chat/renderers/ToolResult";
import { PhoneAnnotationForm } from "./AnnotationForm";
import { useConnection } from "./connection";

// The phone reads a bounded window and keeps at most EARLIER_PAGES older ones
// (FS-20.R13).
const TRANSCRIPT_PAGE = 150;
const EARLIER_PAGES = 3;
const errorText = (error: unknown) => (error instanceof Error ? error.message : String(error));

export function agentTitle(agent: Pick<AgentState, "name" | "role" | "project">) {
  return agent.name || `${agent.role}@${agent.project}`;
}

/** withResolutions marks each permission request with its terminal decision. */
function withResolutions(events: TranscriptEvent[]): TranscriptEvent[] {
  const decisions = new Map<string, string>();
  for (const event of events) {
    if (event.kind === "permission_resolved" && event.tool_call_id) decisions.set(String(event.tool_call_id), String(event.decision ?? ""));
  }
  return events.map((event) =>
    event.kind === "permission_request" && decisions.has(String(event.tool_call_id))
      ? ({ ...event, resolved: decisions.get(String(event.tool_call_id)) } as TranscriptEvent)
      : event,
  );
}

/** requestSummary shows the command or the file a permission is for (FS-20.R12). */
function requestSummary(event: TranscriptEvent): string {
  const args = (event.args ?? {}) as Record<string, unknown>;
  const pick = (...keys: string[]) => keys.map((key) => args[key]).find((value) => typeof value === "string") as string | undefined;
  const command = pick("command", "cmd");
  if (command) return command;
  const path = pick("file_path", "path", "notebook_path");
  if (path) return `${String(event.name ?? "Edit")} ${path}`;
  const text = JSON.stringify(event.args ?? {});
  return text === "{}" ? "" : text;
}

const clip = (text: string, max: number) => (text.length > max ? `${text.slice(0, max)}…` : text);

function PermissionCard({ agent, event, latest, disabled, onSettled }: {
  agent: AgentState;
  event: TranscriptEvent;
  latest: string;
  disabled: boolean;
  onSettled: () => void;
}) {
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const decide = async (decision: "approve" | "deny") => {
    setBusy(true);
    setError(null);
    try {
      await decidePermission(agent.agent_id, String(event.tool_call_id), decision);
    } catch (err) {
      // The first decision wins; a later one learns what happened (FS-20.R24).
      setError(/already resolved/i.test(errorText(err)) ? "Already answered on the Mac or another phone." : errorText(err));
    } finally {
      setBusy(false);
      onSettled();
    }
  };
  const summary = requestSummary(event);
  return (
    <section className="phone-card phone-decision" aria-label="Permission request">
      <p className="phone-card-kicker">{agentTitle(agent)} needs permission</p>
      <h2>{String(event.name ?? "Tool")}</h2>
      {summary && <pre className="phone-pre">{clip(summary, 1200)}</pre>}
      {event.reason ? <p>{String(event.reason)}</p> : null}
      {latest && (
        <blockquote className="phone-quote">
          <span>Latest message</span>
          {clip(latest, 600)}
        </blockquote>
      )}
      {error && <p className="phone-error">{error}</p>}
      <div className="phone-actions">
        <button type="button" className="phone-primary" disabled={disabled || busy} onClick={() => void decide("approve")}>
          Approve
        </button>
        <button type="button" disabled={disabled || busy} onClick={() => void decide("deny")}>
          Deny
        </button>
      </div>
    </section>
  );
}

function EventRow({ event, onAnnotate }: { event: TranscriptEvent; onAnnotate: (draft: AnnotationDraft) => void }) {
  switch (event.kind) {
    case "user_text":
      return <p className="phone-user">{String(event.text ?? "")}</p>;
    case "assistant_text":
      return <AssistantText event={event} />;
    case "tool_call":
      return <ToolCall event={event} />;
    case "tool_result":
      return shouldRenderToolResult(event) ? <ToolResult event={event} /> : null;
    case "diff":
      return <DiffBlock event={event} onAnnotate={onAnnotate} selectHint="Tap line numbers to select a range." />;
    case "permission_request":
      return event.resolved ? <p className="phone-meta">{String(event.name ?? "Permission")} · {String(event.resolved)}</p> : null;
    case "error":
      return <p className="phone-error">{String(event.message ?? event.text ?? "The turn failed.")}</p>;
    default:
      return null;
  }
}

export function AgentScreen({ agentId }: { agentId: string }) {
  const agent = useConnection((state) => state.agents[agentId]);
  const rev = useConnection((state) => state.transcriptRev[agentId] ?? 0);
  const link = useConnection((state) => state.link);
  const offline = link !== "connected";
  const client = useQueryClient();
  const [text, setText] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const chat = agent?.interface !== "terminal";
  const addAnnotation = useAnnotationStore((state) => state.add);

  // Older windows cover seqs before `anchor`; while they are shown, the live
  // window reads from the anchor so the two stay contiguous.
  const [earlier, setEarlier] = useState<{ anchor: number; events: TranscriptEvent[]; pages: number; hasMore: boolean } | null>(null);
  const [loadingEarlier, setLoadingEarlier] = useState(false);

  const transcript = useQuery({
    queryKey: ["transcript", agentId, rev, earlier?.anchor],
    queryFn: () => getTranscriptWindow(agentId, { limit: TRANSCRIPT_PAGE, sinceSeq: earlier ? earlier.anchor - 1 : undefined }),
    enabled: chat,
    placeholderData: (prev) => prev,
  });
  // More than a window arrived after the anchor: the older pages no longer
  // adjoin the live window, so drop them rather than show a gap.
  const gap = !!earlier && !transcript.isPlaceholderData && !!transcript.data?.has_more;
  useEffect(() => {
    if (gap) setEarlier(null);
  }, [gap]);
  const held = useQuery({
    queryKey: ["held", agentId, rev, agent?.state],
    queryFn: () => getHeldPrompt(agentId).catch(() => null),
    enabled: chat && !!agent?.running,
  });

  const events = useMemo(
    () => withResolutions([...(earlier?.events ?? []), ...(transcript.data?.events ?? [])].map((event) => normalizeEvent(event))),
    [earlier, transcript.data],
  );
  // The server derives both from the whole session, so they hold when the
  // request or the reply falls before the window.
  const pendingEvent = transcript.data?.pending_permission;
  const pending = pendingEvent ? normalizeEvent(pendingEvent) : undefined;
  const latest = transcript.data?.latest_assistant ?? "";
  const refresh = () => void client.invalidateQueries({ queryKey: ["transcript", agentId] });
  const olderAvailable = earlier ? earlier.hasMore : !!transcript.data?.has_more;

  const showEarlier = async () => {
    const anchor = earlier?.anchor ?? transcript.data?.events[0]?.seq;
    const before = earlier?.events[0]?.seq ?? anchor;
    if (!anchor || !before) return;
    setLoadingEarlier(true);
    try {
      const page = await getTranscriptWindow(agentId, { limit: TRANSCRIPT_PAGE, beforeSeq: before });
      setEarlier((prev) => ({
        anchor,
        events: [...page.events, ...(prev?.events ?? [])],
        pages: (prev?.pages ?? 0) + 1,
        hasMore: page.has_more,
      }));
    } catch (err) {
      setError(errorText(err));
    } finally {
      setLoadingEarlier(false);
    }
  };

  if (!agent) return <p className="phone-empty">This agent is not on the Mac any more.</p>;

  // Every refused action shows the desktop's reason and keeps the typed text
  // (FS-20.R27); only a delivered message clears the composer.
  const act = async (fn: () => Promise<unknown>, clears = false, after?: (result: unknown) => void) => {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const result = await fn();
      if (clears) setText("");
      after?.(result);
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
      refresh();
      void client.invalidateQueries({ queryKey: ["held", agentId] });
    }
  };
  const isBusy = agent.state === "busy" || agent.state === "waiting_input";
  const annotate = (draft: AnnotationDraft) => {
    if (!addAnnotation(agentId, draft)) setError("At most 20 annotations can wait at once. Send or discard them first.");
  };
  const heldText = held.data && typeof held.data === "object" && "text" in held.data ? String((held.data as { text?: string }).text ?? "") : "";

  return (
    <div className="phone-agent">
      <header className="phone-agent-header">
        <h1>{agentTitle(agent)}</h1>
        <p className="phone-meta">
          {agent.running ? agent.state.replace("_", " ") : "stopped"}
          {agent.detail ? ` · ${clip(agent.detail, 80)}` : ""}
        </p>
      </header>
      {pending && <PermissionCard agent={agent} event={pending} latest={latest} disabled={offline} onSettled={refresh} />}
      {!chat ? (
        <p className="phone-empty">This is a terminal agent. Its terminal is on the Mac; the phone shows its status only.</p>
      ) : (
        <>
          {olderAvailable &&
            ((earlier?.pages ?? 0) < EARLIER_PAGES ? (
              <button type="button" className="phone-link" disabled={offline || loadingEarlier} onClick={() => void showEarlier()}>
                Show earlier
              </button>
            ) : (
              <p className="phone-meta">Earlier messages are not loaded on the phone.</p>
            ))}
          <ol className="phone-transcript" aria-label="Conversation">
            {events.map((event) => (
              <li key={`${event.seq}:${event.kind}`}>
                <EventRow event={event} onAnnotate={annotate} />
              </li>
            ))}
          </ol>
          <PhoneAnnotationForm agent={agent} />
          {heldText && (
            <div className="phone-card">
              <p className="phone-card-kicker">Held until the current turn ends</p>
              <p>{heldText}</p>
              <div className="phone-actions">
                {agent.steering_available && isBusy && (
                  <button type="button" disabled={offline || busy} onClick={() => void act(() => steerPrompt(agentId, ""))}>
                    Send now
                  </button>
                )}
                <button type="button" disabled={offline || busy} onClick={() => void act(() => withdrawPrompt(agentId))}>
                  Withdraw
                </button>
              </div>
            </div>
          )}
          {agent.running && (
            <form
              className="phone-composer"
              onSubmit={(event) => {
                event.preventDefault();
                if (!text.trim()) return;
                void act(() => sendPrompt(agentId, text), true, (result) => {
                  if ((result as { delivery?: string })?.delivery === "held") setNotice("Held — it will be sent when the current turn ends.");
                });
              }}
            >
              <label className="phone-field">
                {agent.state === "waiting_input" && !pending ? "Reply" : "Message"}
                <textarea rows={3} value={text} onChange={(event) => setText(event.target.value)} />
              </label>
              {error && <p className="phone-error">{error}</p>}
              {notice && <p className="phone-meta">{notice}</p>}
              <div className="phone-actions">
                <button type="submit" className="phone-primary" disabled={offline || busy || !text.trim()}>
                  Send
                </button>
                {agent.steering_available && isBusy && (
                  <button type="button" disabled={offline || busy || !text.trim()} onClick={() => void act(() => steerPrompt(agentId, text), true)}>
                    Steer
                  </button>
                )}
              </div>
            </form>
          )}
        </>
      )}
      {!agent.running && error && <p className="phone-error">{error}</p>}
      <div className="phone-actions phone-controls">
        {agent.running && isBusy && (
          <button type="button" disabled={offline || busy} onClick={() => void act(() => cancelTurn(agentId))}>
            Cancel turn
          </button>
        )}
        {agent.running && (
          <button type="button" className="phone-danger" disabled={offline || busy} onClick={() => void act(() => stopAgent(agentId))}>
            Stop
          </button>
        )}
        {!agent.running && !agent.archived && (
          <button type="button" disabled={offline || busy} onClick={() => void act(() => resumeAgent(agentId))}>
            Resume
          </button>
        )}
      </div>
    </div>
  );
}
