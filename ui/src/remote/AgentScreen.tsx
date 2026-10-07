import { AutoGrowTextarea } from "../components/ui";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  cancelTurn,
  archiveAgent,
  cloneAgent,
  decidePermission,
  getFileContent,
  getHeldPrompt,
  getTrackedCommands,
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
} from "../api/client";
import type { AgentState, AnnotationDraft, TranscriptEvent } from "../api/types";
import { useAnnotationStore } from "../store/annotationStore";
import { useReasoningStore } from "../store/reasoningStore";
import { foldTranscript, normalizeEvent } from "../store/transcriptStore";
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

// The phone reads a bounded window and keeps at most EARLIER_PAGES older ones
// (FS-20.R13).
const TRANSCRIPT_PAGE = 150;
const EARLIER_PAGES = 3;
const errorText = (error: unknown) => (error instanceof Error ? error.message : String(error));

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
  const [tab, setTab] = useState<"chat" | "files" | "commands" | "manage">("chat");
  const [rename, setRename] = useState("");
  const [filePath, setFilePath] = useState<string | null>(null);
  const [diffSeq, setDiffSeq] = useState<number | null>(null);
  const chat = agent?.interface !== "terminal";
  const files = useQuery({ queryKey: ["tracked-files", agentId, rev], queryFn: () => getTrackedFiles(agentId), enabled: tab === "files" });
  const commands = useQuery({ queryKey: ["tracked-commands", agentId, rev], queryFn: () => getTrackedCommands(agentId), enabled: tab === "commands" });
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

  // Fold the joined windows as the desktop does, so streamed deltas coalesce
  // into one message even across a window edge (TS-08.R73).
  const events = useMemo(
    () => foldTranscript([...(earlier?.events ?? []), ...(transcript.data?.events ?? [])]),
    [earlier, transcript.data],
  );
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

  if (!agent) return <p className="phone-empty">This agent is not on the Mac any more. <button type="button" className="phone-link" onClick={() => navigate("/")}>Home</button></p>;
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

  return (
    <div className="phone-agent">
      <header className="phone-agent-header">
        <h1>{agentTitle(agent)}</h1>
        <p className="phone-meta">
          {agent.running ? agent.state.replace("_", " ") : "stopped"}
          {agent.detail ? ` · ${clip(agent.detail, 80)}` : ""}
        </p>
      </header>
      <div className="phone-actions phone-tabs" role="tablist" aria-label="Agent views">
        {(["chat", "files", "commands", "manage"] as const).map((name) => <button key={name} type="button" role="tab" aria-selected={tab === name} onClick={() => setTab(name)}>{name[0].toUpperCase() + name.slice(1)}</button>)}
      </div>
      {tab === "files" && <section className="phone-section" aria-label="Files"><h2>Files</h2>{files.isError ? <p className="phone-error">{errorText(files.error)}</p> : <ul className="phone-list">{(files.data?.files ?? []).map((tracked) => <li key={tracked.path}><div className="phone-row"><span className="phone-row-title">{tracked.path}</span><span className="phone-row-meta">{tracked.edit_count} edits · {new Date(tracked.last_ts).toLocaleString()}</span><div className="phone-actions">{tracked.has_diff && tracked.diff_refs[0] && <button type="button" onClick={() => { setDiffSeq(tracked.diff_refs[0].seq); setTab("chat"); }}>Open diff</button>}<button type="button" onClick={() => setFilePath(tracked.path)}>Open file</button></div></div></li>)}</ul>}{filePath && <section className="phone-card" aria-label="File content"><div className="phone-actions"><strong>{filePath}</strong><button type="button" onClick={() => setFilePath(null)}>Close</button></div>{file.isError ? <p className="phone-error">{errorText(file.error)}</p> : <pre className="phone-pre">{file.data?.content}</pre>}</section>}</section>}
      {tab === "commands" && <section className="phone-section" aria-label="Commands"><h2>Commands</h2>{commands.isError ? <p className="phone-error">{errorText(commands.error)}</p> : <ul className="phone-list">{(commands.data?.commands ?? []).map((command) => <li key={`${command.seq}:${command.command}`}><div className="phone-row"><span className="phone-row-title">{command.command}</span><span className="phone-row-meta">{command.exit_status || "Running"}{command.exit_error ? ` · ${command.exit_error}` : ""}</span></div></li>)}</ul>}</section>}
      {tab === "manage" && <AgentManagement agent={agent} offline={offline} busy={busy} act={act} rename={rename} setRename={setRename} />}
      {tab === "manage" && error && <p className="phone-error">{error}</p>}
      {tab === "chat" && <>
      {pending && <PermissionCard agent={agent} event={pending} latest={latest} disabled={offline} onSettled={refresh} />}
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
          {olderAvailable &&
            ((earlier?.pages ?? 0) < EARLIER_PAGES ? (
              <button type="button" className="phone-link" disabled={offline || loadingEarlier} onClick={() => void showEarlier()}>
                Show earlier
              </button>
            ) : (
              <p className="phone-meta">Earlier messages are not loaded on the phone.</p>
            ))}
          <div className="phone-transcript" role="list" aria-label="Conversation" ref={listRef} onFocus={trackFocus}>
            <TurnList agentId={agentId} events={rows} lead={lead} choices={choices} renderEvents={(list) => renderRows(list, annotate, [], 1)} />
          </div>
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
                <AutoGrowTextarea rows={3} maxHeight="40vh" value={text} onChange={(event) => setText(event.target.value)} />
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
      </>}
    </div>
  );
}

function AgentManagement({ agent, offline, busy, act, rename, setRename }: { agent: AgentState; offline: boolean; busy: boolean; act: (fn: () => Promise<unknown>, clears?: boolean, after?: (result: unknown) => void) => void; rename: string; setRename: (value: string) => void }) {
  const options = useQuery({ queryKey: ["runtime-options"], queryFn: getRuntimeOptions });
  const [runtime, setRuntime] = useState({ backend: agent.backend, model: agent.model, effort: agent.effort ?? "" });
  const [archiveConfirm, setArchiveConfirm] = useState(false);
  // A runtime change from either device restarts the switch draft from the
  // agent's live runtime (FS-20.R17).
  useEffect(() => setRuntime({ backend: agent.backend, model: agent.model, effort: agent.effort ?? "" }), [agent.backend, agent.model, agent.effort]);
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
  return <section className="phone-section" aria-label="Agent management"><form className="phone-card phone-form" onSubmit={(event) => { event.preventDefault(); if (rename.trim()) act(() => renameAgent(agent.agent_id, rename.trim()), false, () => setRename("")); }}><label className="phone-field">Name<input value={rename} placeholder={agent.name || agent.role} onChange={(event) => setRename(event.target.value)} /></label><button type="submit" disabled={offline || busy || !rename.trim()}>Rename</button></form>{agent.running && agent.interface === "chat" && <form className="phone-card phone-form" onSubmit={(event) => { event.preventDefault(); act(() => switchRuntime(agent.agent_id, runtime)); }}><p className="phone-card-kicker">Runtime</p><label className="phone-field">Backend<select value={runtime.backend} onChange={(event) => selectBackend(event.target.value)}><option value="">Choose…</option>{(options.data?.backends ?? []).map((item) => <option key={item.id} value={item.id}>{item.name || item.id}</option>)}</select></label><label className="phone-field">Model<select value={runtime.model} onChange={(event) => selectModel(runtime.backend, event.target.value)}>{(backend?.models ?? []).map((item) => <option key={item.id} value={item.id}>{item.name || item.id}</option>)}</select></label>{(model?.efforts.length ?? 0) > 0 && <label className="phone-field">Effort<select value={runtime.effort} onChange={(event) => setRuntime({ ...runtime, effort: event.target.value })}><option value="">Model default</option>{model!.efforts.map((effort) => <option key={effort} value={effort}>{effort}</option>)}</select></label>}<button type="submit" disabled={disable || !runtime.backend || !runtime.model}>Switch runtime</button></form>}{agent.fast_available && <label className="phone-card phone-check"><input type="checkbox" checked={agent.fast} disabled={disable} onChange={(event) => act(() => setSessionConfig(agent.agent_id, { fast: event.target.checked }))} /> Fast mode</label>}{agent.running && agent.interface === "chat" && (liveModel?.efforts.length ?? 0) > 0 && <label className="phone-card phone-field">Effort<select value={agent.effort ?? ""} disabled={disable} onChange={(event) => act(() => setSessionConfig(agent.agent_id, { effort: event.target.value }))}><option value="">Model default</option>{liveModel!.efforts.map((effort) => <option key={effort} value={effort}>{effort}</option>)}</select></label>}<div className="phone-card"><button type="button" disabled={offline || busy || !agent.clone?.available} title={agent.clone?.reason} onClick={() => act(() => cloneAgent(agent.agent_id), false, (result) => navigate(`/agent/${encodeURIComponent((result as { agent: { agent_id: string } }).agent.agent_id)}`))}>Clone</button>{agent.clone && !agent.clone.available && <p className="phone-meta">{agent.clone.reason}</p>}{archiveConfirm ? <><p>Archive this agent? Restore is available on the desktop.</p><div className="phone-actions"><button type="button" disabled={offline || busy} onClick={() => setArchiveConfirm(false)}>Cancel</button><button type="button" className="phone-danger" disabled={offline || busy} onClick={() => act(() => archiveAgent(agent.agent_id), false, () => navigate(`/project/${encodeURIComponent(agent.project)}`))}>Archive agent</button></div></> : <button type="button" className="phone-danger" disabled={offline || busy} onClick={() => setArchiveConfirm(true)}>Archive</button>}</div></section>;
}
