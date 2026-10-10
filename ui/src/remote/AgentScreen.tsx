import { AutoGrowTextarea, ConfirmDialog, VisuallyHidden } from "../components/ui";
import { useEffect, useLayoutEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  cancelTurn,
  archiveAgent,
  cloneAgent,
  decidePermission,
  getFileContent,
  getHeldPrompt,
  getTrackedFiles,
  renameAgent,
  getTranscriptWindow,
  resumeAgent,
  sendPrompt,
  setSessionConfig,
  steerPrompt,
  stopAgent,
  switchRuntime,
  withdrawPrompt,
  updateAgentIdentity,
} from "../api/client";
import type { AgentState, AnnotationDraft, TranscriptEvent } from "../api/types";
import { useAnnotationStore } from "../store/annotationStore";
import { useReasoningStore } from "../store/reasoningStore";
import { foldTranscript, normalizeEvent } from "../store/transcriptStore";
import { WorkingIndicator } from "../components/chat/WorkingIndicator";
import { AssistantText } from "../components/chat/renderers/AssistantText";
import { DiffBlock } from "../components/chat/renderers/DiffBlock";
import { NoticeRow } from "../components/chat/renderers/NoticeRow";
import { ToolCall } from "../components/chat/renderers/ToolCall";
import { ToolResult } from "../components/chat/renderers/ToolResult";
import { groupTranscriptRows, ToolRun } from "../components/chat/toolRun";
import { markBackgrounded, nestActivities, withReasoning, type ChildNode } from "../components/chat/runtimeActivity";
import { carryLeadKey } from "../components/chat/turnActivity";
import { ChildActivity } from "../components/chat/renderers/ChildActivity";
import { ThinkingDisclosure } from "../components/chat/renderers/ThinkingDisclosure";
import { TurnList, useFocusReturn, useTurnChoices } from "../components/chat/TurnList";
import { PhoneAnnotationForm } from "./AnnotationForm";
import { useConnection, watchReasoning, type OpenTranscript } from "./connection";
import { getRuntimeOptions } from "./api";
import { navigate } from "./router";
import { PhoneIcon } from "./PhoneIcon";
import { GroupPicker } from "../components/ui/GroupPicker";

// The phone reads a bounded window and keeps at most EARLIER_PAGES older ones
// (FS-20.R13).
const TRANSCRIPT_PAGE = 750;
const EARLIER_PAGES = 3;
const EARLIER_EVENTS = TRANSCRIPT_PAGE * EARLIER_PAGES;
const EARLIER_BYTES = 3 * 1024 * 1024;
const errorText = (error: unknown) => (error instanceof Error ? error.message : String(error));

function retainEarlier(events: TranscriptEvent[]) {
  const unique = [...new Map(events.map((event) => [event.seq, event])).values()].sort((a, b) => (a.seq ?? 0) - (b.seq ?? 0));
  const encoder = new TextEncoder();
  let bytes = 0;
  let start = unique.length;
  while (start > 0 && unique.length - start < EARLIER_EVENTS) {
    const size = encoder.encode(JSON.stringify(unique[start - 1])).byteLength;
    if (bytes + size > EARLIER_BYTES && start < unique.length) break;
    bytes += size;
    start--;
  }
  return unique.slice(start);
}

export function agentTitle(agent: Pick<AgentState, "name" | "role" | "project">) {
  return agent.name || `${agent.role}@${agent.project}`;
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
      <div className="phone-actions phone-permission-actions">
        <button type="button" disabled={disabled || busy} onClick={() => void decide("deny")}>
          Deny
        </button>
        <button type="button" className="ad-button-primary" disabled={disabled || busy} onClick={() => void decide("approve")}>
          Approve
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
      return <ToolResult event={event} />;
    case "diff":
      return <DiffBlock event={event} onAnnotate={onAnnotate} selectHint="Tap line numbers to select a range." />;
    case "permission_request":
      return event.resolved ? <p className="phone-meta">{String(event.name ?? "Permission")} · {String(event.resolved)}</p> : null;
    case "error":
      return <p className="phone-error">{String(event.message ?? event.text ?? "The turn failed.")}</p>;
    case "notice":
      return <NoticeRow event={event} />;
    case "reasoning":
      return <ThinkingDisclosure text={String(event.text ?? "")} activityId={event.activity_id} />;
    default:
      return null;
  }
}

// renderRows is the phone's row renderer inside the shared turn list: tool runs,
// nested native children and ordinary rows, each one list item (FS-03.R77).
function renderRows(list: TranscriptEvent[], onAnnotate: (draft: AnnotationDraft) => void, ancestry: string[], depth: number): ReactNode {
  const nested = (events: TranscriptEvent[], path: string[], level: number) => renderRows(events, onAnnotate, path, level);
  return groupTranscriptRows(list).map((row, index) => {
    if (row.kind === "tool-run") {
      return (
        <div className="phone-message" role="listitem" key={`run:${row.events[0].seq ?? index}`}>
          <ToolRun events={row.events} renderEvent={(event) => <EventRow key={`${event.seq}:${event.kind}`} event={event} onAnnotate={onAnnotate} />} />
        </div>
      );
    }
    const event = row.event;
    return (
      <div className="phone-message" role="listitem" key={`${event.seq ?? event.message_id ?? index}:${event.kind}`}>
        {event.kind === "activity" ? (
          <ChildActivity node={event.node as ChildNode} ancestry={ancestry} depth={depth} renderEvents={nested} />
        ) : (
          <EventRow event={event} onAnnotate={onAnnotate} />
        )}
      </div>
    );
  });
}

// The phone stores a span's anchor as the last seq it had seen; place it before
// the first later row of the current window.
function slotAfter(events: TranscriptEvent[], seq: number) {
  const index = events.findIndex((event) => typeof event.seq === "number" && event.seq > seq);
  return index < 0 ? events.length : index;
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
  const [stopOpen, setStopOpen] = useState(false);
  const [tab, setTab] = useState<"chat" | "files" | "manage">("chat");
  const [rename, setRename] = useState("");
  const [filePath, setFilePath] = useState<string | null>(null);
  const [diffSeq, setDiffSeq] = useState<number | null>(null);
  const chat = agent?.interface !== "terminal";
  const files = useQuery({ queryKey: ["tracked-files", agentId, rev], queryFn: () => getTrackedFiles(agentId), enabled: tab === "files" });
  const file = useQuery({ queryKey: ["tracked-file", agentId, filePath], queryFn: () => getFileContent(agentId, filePath!), enabled: !!filePath });
  // Open diff reads exactly the requested event, wherever it falls (FS-20.R37).
  const focusedDiff = useQuery({
    queryKey: ["focused-diff", agentId, diffSeq],
    queryFn: async () => (await getTranscriptWindow(agentId, { limit: 1, beforeSeq: diffSeq! + 1 })).events.find((event) => event.seq === diffSeq) ?? null,
    enabled: diffSeq !== null,
  });
  const diffCard = useRef<HTMLElement | null>(null);
  useEffect(() => {
    if (tab === "chat" && focusedDiff.data) diffCard.current?.scrollIntoView?.({ block: "start" });
  }, [tab, focusedDiff.data]);
  const addAnnotation = useAnnotationStore((state) => state.add);

  // Older windows cover seqs before `anchor`; while they are shown, the live
  // window reads from the anchor so the two stay contiguous.
  const [earlier, setEarlier] = useState<{ anchor: number; events: TranscriptEvent[]; pages: number; hasMore: boolean; recoveredLiveStart?: number; recoveryPages?: number } | null>(null);
  const [loadingEarlier, setLoadingEarlier] = useState(false);
  const [gapRetry, setGapRetry] = useState(0);
  const loadingEarlierRef = useRef(false);
  const scrollAnchor = useRef<{ height: number; top: number } | null>(null);
  const generation = useRef(0);

  const transcript = useQuery({
    queryKey: ["transcript", agentId, rev, earlier?.anchor],
    queryFn: () => getTranscriptWindow(agentId, { limit: TRANSCRIPT_PAGE, sinceSeq: earlier?.anchor }),
    enabled: chat,
    placeholderData: (prev) => prev?.agent_id === agentId ? prev : undefined,
  });
  const liveStart = transcript.data?.events[0]?.seq;
  const gap = !!earlier && !transcript.isPlaceholderData && !!transcript.data?.has_more && earlier.recoveredLiveStart !== liveStart;
  const held = useQuery({
    queryKey: ["held", agentId, rev, agent?.state],
    queryFn: () => getHeldPrompt(agentId).catch(() => null),
    enabled: chat && !!agent?.running,
  });

  // Fold the joined windows as the desktop does, so streamed deltas coalesce
  // into one message even across a window edge (TS-08.R73).
  const events = useMemo(() => {
    const joined = [...(earlier?.events ?? []), ...(gap ? [] : (transcript.data?.events ?? []))];
    return foldTranscript([...new Map(joined.map((event) => [event.seq, event])).values()].sort((a, b) => (a.seq ?? 0) - (b.seq ?? 0)));
  }, [earlier, gap, transcript.data]);
  // Live reasoning arrives on the same authenticated stream as the desktop's and
  // shares its bounded store (TS-08.R104). The phone's window slides as the
  // conversation grows, so a span anchors after the last seq it saw rather than
  // at a list position. Only an exact window read owns a thought: none before the
  // first read, nor while a newer read replaces placeholder data (TS-08.R103).
  // The leading turn keeps its identity as the window slides (TS-08.R102).
  const reasoning = useReasoningStore((state) => state.byAgent[agentId]?.spans);
  const knownKeys = useRef(new Map<number, string>());
  const atStart = earlier ? !earlier.hasMore : !transcript.data?.has_more;
  const lead = useMemo(() => {
    const carried = carryLeadKey(events, atStart, knownKeys.current);
    knownKeys.current = carried.known;
    return carried.lead;
  }, [events, atStart]);
  const viewRef = useRef<OpenTranscript | null>(null);
  viewRef.current = transcript.data && !transcript.isPlaceholderData ? { events, lead, rev } : null;
  useEffect(() => watchReasoning(agentId, () => viewRef.current), [agentId]);
  const rows = useMemo(() => {
    const spans = reasoning?.map((span) => ({ ...span, anchor: slotAfter(events, span.anchor) }));
    return nestActivities(withReasoning(markBackgrounded(events), spans, lead));
  }, [events, reasoning, lead]);
  const choices = useTurnChoices(agentId);
  const listRef = useRef<HTMLDivElement>(null);
  const trackFocus = useFocusReturn(listRef);
  // The server derives both from the whole session, so they hold when the
  // request or the reply falls before the window.
  const pendingEvent = transcript.data?.pending_permission;
  const pending = pendingEvent ? normalizeEvent(pendingEvent) : undefined;
  const latest = transcript.data?.latest_assistant ?? "";
  const refresh = () => void client.invalidateQueries({ queryKey: ["transcript", agentId] });
  const olderAvailable = earlier ? earlier.hasMore : !!transcript.data?.has_more;

  const showEarlier = async () => {
    if (loadingEarlierRef.current) return;
    if (gap) {
      setGapRetry((value) => value + 1);
      return;
    }
    if ((earlier?.pages ?? 0) >= EARLIER_PAGES) return;
    const anchor = earlier?.anchor ?? transcript.data?.events[0]?.seq;
    const before = earlier?.events[0]?.seq ?? anchor;
    if (!anchor || !before) return;
    const requestGeneration = generation.current;
    loadingEarlierRef.current = true;
    setLoadingEarlier(true);
    scrollAnchor.current = { height: document.documentElement.scrollHeight, top: window.scrollY };
    try {
      const page = await getTranscriptWindow(agentId, { limit: TRANSCRIPT_PAGE, beforeSeq: before });
      if (requestGeneration !== generation.current) return;
      setEarlier((prev) => ({
        anchor,
        events: retainEarlier([...page.events, ...(prev?.events ?? []), ...(!prev ? transcript.data?.events.slice(0, 1) ?? [] : [])]),
        pages: (prev?.pages ?? 0) + 1,
        hasMore: page.has_more,
      }));
      setError(null);
    } catch (err) {
      if (requestGeneration !== generation.current) return;
      scrollAnchor.current = null;
      setError(errorText(err));
    } finally {
      if (requestGeneration !== generation.current) return;
      loadingEarlierRef.current = false;
      setLoadingEarlier(false);
    }
  };

  useLayoutEffect(() => {
    if (!scrollAnchor.current) return;
    const { height, top } = scrollAnchor.current;
    scrollAnchor.current = null;
    window.scrollTo(0, top + document.documentElement.scrollHeight - height);
  }, [earlier?.events]);

  useEffect(() => {
    let lastY = window.scrollY;
    const onScroll = () => {
      const y = window.scrollY;
      if (y < lastY && y <= 200 && (gap || olderAvailable)) void showEarlier();
      lastY = y;
    };
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, [gap, olderAvailable, earlier, transcript.data]);

  useEffect(() => {
    if (!gap || loadingEarlierRef.current || !earlier || !liveStart) return;
    if (earlier.recoveredLiveStart && earlier.recoveredLiveStart !== liveStart) {
      setEarlier({ ...earlier, events: earlier.events.filter((event) => (event.seq ?? 0) <= earlier.anchor), recoveredLiveStart: undefined, recoveryPages: 0 });
      return;
    }
    if ((earlier.recoveryPages ?? 0) >= EARLIER_PAGES) {
      setEarlier(null);
      setError("New activity exceeded the phone's retained history. The phone returned to the latest messages.");
      return;
    }
    const recover = async () => {
      const requestGeneration = generation.current;
      loadingEarlierRef.current = true;
      setLoadingEarlier(true);
      try {
        const bridge = earlier.events.filter((event) => (event.seq ?? 0) > earlier.anchor);
        const beforeSeq = bridge[0]?.seq ?? liveStart;
        const page = await getTranscriptWindow(agentId, { limit: TRANSCRIPT_PAGE, beforeSeq });
        if (requestGeneration !== generation.current) return;
        setEarlier((prev) => prev && ({
          ...prev,
          events: retainEarlier([...prev.events, ...page.events.filter((event) => (event.seq ?? 0) >= prev.anchor)]),
          recoveryPages: (prev.recoveryPages ?? 0) + 1,
          recoveredLiveStart: (page.events[0]?.seq ?? 0) <= prev.anchor ? liveStart : prev.recoveredLiveStart,
        }));
        setError(null);
      } catch (err) {
        if (requestGeneration !== generation.current) return;
        setError(errorText(err));
      } finally {
        if (requestGeneration !== generation.current) return;
        loadingEarlierRef.current = false;
        setLoadingEarlier(false);
      }
    };
    void recover();
  }, [agentId, gap, gapRetry, earlier, liveStart]);

  useEffect(() => {
    generation.current++;
    setEarlier(null);
    setGapRetry(0);
    setLoadingEarlier(false);
    setError(null);
    loadingEarlierRef.current = false;
    scrollAnchor.current = null;
  }, [agentId]);

  if (!agent) return <p className="phone-empty">This agent is not on the Mac any more. <button type="button" className="phone-link ad-button-secondary" onClick={() => navigate("/")}>Home</button></p>;
  // Archived from either device: the screen stops offering work (FS-20.R39).
  if (agent.archived) {
    return (
      <div className="phone-agent">
        <header className="phone-agent-header"><h1>{agentTitle(agent)}</h1></header>
        <p className="phone-empty">Archived on the Mac. Restore is available on the desktop.</p>
        <div className="phone-actions">
          <button type="button" onClick={() => navigate(`/project/${encodeURIComponent(agent.project)}`)}>Project</button>
          <button type="button" onClick={() => navigate("/")}>Home</button>
        </div>
      </div>
    );
  }

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
  const stateLabel = agent.running ? agent.state.replace("_", " ") : "Stopped";
  const runtimeLabel = [agent.backend, agent.model].filter(Boolean).join(" · ");
  const roleBackend = [agent.role, agent.backend].filter(Boolean).join(" · ");

  return (
    <div className="phone-agent">
      <div className="phone-agent-top">
        <button type="button" className="phone-agent-back ad-button-secondary" aria-label={`Back to ${agent.project}`} onClick={() => navigate(`/project/${encodeURIComponent(agent.project)}`)}>
          <PhoneIcon name="back" size={19} />{agent.project}
        </button>
        {agent.running ? (
          <button type="button" className="ad-button-danger" disabled={offline || busy} onClick={() => setStopOpen(true)}>Stop</button>
        ) : (
          <button type="button" className="ad-button-secondary" disabled={offline || busy} onClick={() => void act(() => resumeAgent(agentId))}>Resume</button>
        )}
      </div>
      <header className="phone-agent-header phone-agent-identity">
        <div className="phone-agent-title">
          <span className="phone-eyebrow">{roleBackend}</span>
          <h1>{agentTitle(agent)}</h1>
          <p className="phone-agent-runtime">{agent.model}{agent.effort ? ` · ${agent.effort} effort` : ""}</p>
        </div>
        <span className="phone-status" data-tone={agent.running ? agent.state : "stopped"}>
          {stateLabel}
        </span>
      </header>
      <div className="phone-actions phone-tabs" role="tablist" aria-label="Agent views">
        {(["chat", "files", "manage"] as const).map((name) => <button key={name} type="button" role="tab" aria-selected={tab === name} className={tab === name ? "active" : undefined} onClick={() => setTab(name)}>{name[0].toUpperCase() + name.slice(1)}{name === "files" && files.data?.files.length ? <span className="phone-tab-count">{files.data.files.length}</span> : null}</button>)}
      </div>
      {!agent.running && tab !== "manage" && error && <p className="phone-error" role="alert">{error}</p>}
      {tab === "files" && <section className="phone-section phone-files" aria-label="Files">
        <h2>Files</h2>
        {files.isError ? <p className="phone-error">{errorText(files.error)}</p> : <ul className="phone-list">
          {(files.data?.files ?? []).map((tracked) => <li key={tracked.path}>
            <div className="phone-row">
              <span className="phone-row-title">{tracked.path}</span>
              <span className="phone-row-meta">{tracked.edit_count} edits · {new Date(tracked.last_ts).toLocaleString()}</span>
              <div className="phone-actions phone-file-actions">
                {tracked.has_diff && tracked.diff_refs[0] && <button type="button" onClick={() => { setDiffSeq(tracked.diff_refs[0].seq); setTab("chat"); }}>Open diff</button>}
                <button type="button" aria-expanded={filePath === tracked.path} onClick={() => setFilePath(filePath === tracked.path ? null : tracked.path)}>{filePath === tracked.path ? "Close" : "Open file"}</button>
              </div>
              {filePath === tracked.path && <section className="phone-file-preview" aria-label="File content">
                {file.isError ? <p className="phone-error">{errorText(file.error)}</p> : file.isPending ? <p className="phone-meta">Loading…</p> : <pre className="phone-pre">{file.data?.content}</pre>}
              </section>}
            </div>
          </li>)}
        </ul>}
      </section>}
      {tab === "manage" && <AgentManagement agent={agent} offline={offline} busy={busy} act={act} rename={rename} setRename={setRename} error={error} />}
      {tab === "chat" && <div className="phone-conversation">
      {diffSeq !== null && (
        <section ref={diffCard} className="phone-card" aria-label="Requested diff">
          <div className="phone-actions">
            <strong>Diff</strong>
            <button type="button" onClick={() => setDiffSeq(null)}>Close</button>
          </div>
          {focusedDiff.isError ? <p className="phone-error">{errorText(focusedDiff.error)}</p> : focusedDiff.data ? <EventRow event={normalizeEvent(focusedDiff.data)} onAnnotate={annotate} /> : focusedDiff.data === null ? <p className="phone-meta">This diff is no longer in the conversation.</p> : <p className="phone-meta">Loading…</p>}
        </section>
      )}
      {!chat ? (
        <p className="phone-empty">This is a terminal agent. Its terminal is on the Mac; the phone shows its status only.</p>
      ) : (
        <>
          {loadingEarlier && <p className="phone-meta" role="status">Loading earlier messages…</p>}
          {olderAvailable && (earlier?.pages ?? 0) >= EARLIER_PAGES && (
            <p className="phone-meta">Earlier messages are outside the phone's retained history.</p>
          )}
          <div className="phone-transcript phone-transcript-bubbles" role="list" aria-label="Conversation" ref={listRef} onFocus={trackFocus}>
            <TurnList agentId={agentId} events={rows} lead={lead} choices={choices} renderEvents={(list) => renderRows(list, annotate, [], 1)} />
            {!offline && agent.running && agent.state === "busy" && <WorkingIndicator />}
          </div>
          {pending && <PermissionCard agent={agent} event={pending} latest={latest} disabled={offline} onSettled={refresh} />}
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
              className="phone-composer phone-composer-box"
              onSubmit={(event) => {
                event.preventDefault();
                if (!text.trim()) return;
                void act(() => sendPrompt(agentId, text), true, (result) => {
                  if ((result as { delivery?: string })?.delivery === "held") setNotice("Held — it will be sent when the current turn ends.");
                });
              }}
            >
              <label className="phone-field">
                <VisuallyHidden>{agent.state === "waiting_input" && !pending ? "Reply" : "Message"}</VisuallyHidden>
                <AutoGrowTextarea rows={3} maxHeight="40vh" value={text} placeholder={offline ? "Reconnect to send a message" : "Ask for changes…"} onChange={(event) => setText(event.target.value)} />
              </label>
              {error && <p className="phone-error">{error}</p>}
              {notice && <p className="phone-meta">{notice}</p>}
              <div className="phone-actions phone-composer-actions">
                <span className="phone-composer-meta">{runtimeLabel}{agent.effort ? ` · ${agent.effort}` : ""}</span>
                {agent.steering_available && isBusy && (
                  <button type="button" disabled={offline || busy || !text.trim()} onClick={() => void act(() => steerPrompt(agentId, text), true)}>
                    Steer
                  </button>
                )}
                <button type="submit" className="phone-send-icon ad-button-icon-primary" aria-label="Send" title="Send" disabled={offline || busy || !text.trim()}>
                  <PhoneIcon name="up" size={14} />
                </button>
              </div>
            </form>
          )}
        </>
      )}
      <div className="phone-actions phone-controls">
        {agent.running && isBusy && (
          <button type="button" disabled={offline || busy} onClick={() => void act(() => cancelTurn(agentId))}>
            Cancel turn
          </button>
        )}
      </div>
      </div>}
      <ConfirmDialog
        open={stopOpen}
        title="Stop this agent?"
        confirmLabel="Stop"
        destructive
        pending={busy}
        confirmDisabled={offline}
        onCancel={() => setStopOpen(false)}
        onConfirm={() => void act(() => stopAgent(agentId), false, () => setStopOpen(false))}
      >
        <p>Stop this agent on your Mac? You can still view its transcript and recorded output.</p>
        {error && <p className="phone-error" role="alert">{error}</p>}
      </ConfirmDialog>
    </div>
  );
}

function AgentManagement({ agent, offline, busy, act, rename, setRename, error }: { agent: AgentState; offline: boolean; busy: boolean; act: (fn: () => Promise<unknown>, clears?: boolean, after?: (result: unknown) => void) => void; rename: string; setRename: (value: string) => void; error: string | null }) {
  const options = useQuery({ queryKey: ["runtime-options"], queryFn: getRuntimeOptions });
  const [runtime, setRuntime] = useState({ backend: agent.backend, model: agent.model, effort: agent.effort ?? "" });
  const allAgents = useConnection((state) => state.agents);
  const groups = useMemo(() => Object.values(allAgents).filter((item) => !item.archived).map((item) => item.group?.trim()).filter((label): label is string => !!label), [allAgents]);
  const [group, setGroup] = useState(agent.group ?? "");
  const [archiveConfirm, setArchiveConfirm] = useState(false);
  // A runtime change from either device restarts the switch draft from the
  // agent's live runtime (FS-20.R17).
  useEffect(() => setRuntime({ backend: agent.backend, model: agent.model, effort: agent.effort ?? "" }), [agent.backend, agent.model, agent.effort]);
  useEffect(() => setGroup(agent.group ?? ""), [agent.agent_id, agent.group]);
  const findModel = (backendID: string, modelID: string) => options.data?.backends.find((item) => item.id === backendID)?.models.find((item) => item.id === modelID);
  const backend = options.data?.backends.find((item) => item.id === runtime.backend);
  const model = findModel(runtime.backend, runtime.model);
  const liveModel = findModel(agent.backend, agent.model);
  const disable = offline || busy || !agent.running || agent.interface !== "chat";
  // Choosing a model always starts from that model's default effort.
  const selectModel = (backendID: string, modelID: string) => setRuntime({ backend: backendID, model: modelID, effort: findModel(backendID, modelID)?.default_effort ?? "" });
  const selectBackend = (backendID: string) => {
    const next = options.data?.backends.find((item) => item.id === backendID);
    selectModel(backendID, (next?.models.find((item) => item.id === next.default_model) ?? next?.models[0])?.id ?? "");
  };
  return (
    <section className="phone-section phone-manage" aria-label="Agent management">
      <form className="phone-card phone-form phone-manage-rename" onSubmit={(event) => { event.preventDefault(); if (rename.trim()) act(() => renameAgent(agent.agent_id, rename.trim()), false, () => setRename("")); }}>
        <label className="phone-field">Name<input value={rename} placeholder={agent.name || agent.role} onChange={(event) => setRename(event.target.value)} /></label>
        <button type="submit" disabled={offline || busy || !rename.trim()}>Rename</button>
      </form>
      <form className="phone-card phone-form phone-manage-group" onSubmit={(event) => { event.preventDefault(); act(() => updateAgentIdentity(agent.agent_id, { group: group.trim() }), false, () => setGroup(group.trim())); }}>
        <label className="phone-field">Group<GroupPicker id="phone-manage-group" value={group} groups={groups} onChange={setGroup} disabled={offline || busy} /></label>
        <button type="submit" disabled={offline || busy}>Save group</button>
      </form>
      {agent.running && agent.interface === "chat" && (
        <form className="phone-card phone-form" onSubmit={(event) => { event.preventDefault(); act(() => switchRuntime(agent.agent_id, runtime)); }}>
          <p className="phone-card-kicker">Runtime</p>
          <label className="phone-field">Backend<select value={runtime.backend} onChange={(event) => selectBackend(event.target.value)}><option value="">Choose…</option>{(options.data?.backends ?? []).map((item) => <option key={item.id} value={item.id}>{item.name || item.id}</option>)}</select></label>
          <label className="phone-field">Model<select value={runtime.model} onChange={(event) => selectModel(runtime.backend, event.target.value)}>{(backend?.models ?? []).map((item) => <option key={item.id} value={item.id}>{item.name || item.id}</option>)}</select></label>
          {(model?.efforts.length ?? 0) > 0 && <label className="phone-field">Effort<select value={runtime.effort} onChange={(event) => setRuntime({ ...runtime, effort: event.target.value })}><option value="">Model default</option>{model!.efforts.map((effort) => <option key={effort} value={effort}>{effort}</option>)}</select></label>}
          <button type="submit" disabled={disable || !runtime.backend || !runtime.model}>Switch runtime</button>
        </form>
      )}
      {agent.fast_available && <label className="phone-card phone-check"><input type="checkbox" checked={agent.fast} disabled={disable} onChange={(event) => act(() => setSessionConfig(agent.agent_id, { fast: event.target.checked }))} /> Fast mode</label>}
      {agent.running && agent.interface === "chat" && (liveModel?.efforts.length ?? 0) > 0 && <label className="phone-card phone-field">Effort<select value={agent.effort ?? ""} disabled={disable} onChange={(event) => act(() => setSessionConfig(agent.agent_id, { effort: event.target.value }))}><option value="">Model default</option>{liveModel!.efforts.map((effort) => <option key={effort} value={effort}>{effort}</option>)}</select></label>}
      <div className="phone-card phone-manage-lifecycle">
        <button type="button" disabled={offline || busy || !agent.clone?.available} title={agent.clone?.reason} onClick={() => act(() => cloneAgent(agent.agent_id), false, (result) => navigate(`/agent/${encodeURIComponent((result as { agent: { agent_id: string } }).agent.agent_id)}`))}>Clone</button>
        {agent.clone && !agent.clone.available && <p className="phone-meta">{agent.clone.reason}</p>}
        <button type="button" className="ad-button-danger" disabled={offline || busy} onClick={() => setArchiveConfirm(true)}>Archive</button>
      </div>
      {!archiveConfirm && error && <p className="phone-error" role="alert">{error}</p>}
      <ConfirmDialog
        open={archiveConfirm}
        title="Archive this agent?"
        confirmLabel="Archive agent"
        destructive
        pending={busy}
        confirmDisabled={offline}
        onCancel={() => setArchiveConfirm(false)}
        onConfirm={() => act(() => archiveAgent(agent.agent_id), false, () => navigate(`/project/${encodeURIComponent(agent.project)}`))}
      >
        <p>Restore is available on the desktop.</p>
        {error && <p className="phone-error" role="alert">{error}</p>}
      </ConfirmDialog>
    </section>
  );
}
