import { useMemo, useRef, useState, type MouseEvent } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router-dom";
import {
  newCommandID,
  sourceFileURL,
  useDeleteThinkTank,
  useThinkTank,
  useThinkTankActivity,
  useThinkTankCommands,
  useThinkTankControl,
  useThinkTankEntries,
  useThinkTankFiles,
  useThinkTankMessage,
  useThinkTankRetry,
  type ThinkTankLaunch,
} from "../../api/thinkTanks";
import { useProjects } from "../../api/config";
import type { AnnotationDraft, FileContent, TranscriptEvent } from "../../api/types";
import { Badge, Button, ConfirmDialog, PageHeader } from "../../components/ui";
import { AnnotationContextMenu, type AnnotationMenuState } from "../../components/chat/AnnotationContextMenu";
import { FileViewer } from "../../components/chat/FileViewer";
import { SanitizedMarkdown } from "../../components/chat/renderers/SanitizedMarkdown";
import type { FileLink } from "../../components/chat/renderers/filePath";
import { canAnnotate, eventDraft, eventRenderer, selectionWithin } from "../../components/chat/TranscriptView";
import { markBackgrounded, nestActivities } from "../../components/chat/runtimeActivity";
import { TurnList, useTurnChoices } from "../../components/chat/TurnList";
import { useAutocomplete } from "../../components/chat/autocomplete";
import { foldTranscript } from "../../store/transcriptStore";
import { useAnnotationStore } from "../../store/annotationStore";
import { useUiStore } from "../../store/uiStore";
import { clipAnnotationExcerpt } from "../../lib/annotations";
import { copyText } from "../../lib/copyText";
import { agentConversationPath } from "../../lib/agentConversation";
import { NewAgentModal } from "../launch/NewAgentModal";
import type { ThinkTankActivity, ThinkTankDetail, ThinkTankEntry } from "../../schemas/thinkTank";
import {
  entryAuthor,
  entryLabel,
  memberState,
  phaseLabel,
  roomAnnotationSource,
  roomStatus,
} from "./roomText";
import { RoomAnnotationTray } from "./RoomAnnotationTray";

/** ThinkTankPage is the room's full conversation page (FS-21.R27): goal and
 *  phase first, then the attributed discussion, then the current action and
 *  the composer, with compact participants beside it (TS-08.R87). */
export function ThinkTankPage() {
  const { id = "" } = useParams();
  const room = useThinkTank(id);
  if (room.isLoading) return <section className="project-route-state"><p>Loading Think Tank…</p></section>;
  if (room.isError || !room.data) {
    return (
      <section className="project-route-state">
        <h1>Think Tank unavailable</h1>
        <p>It may have been deleted.</p>
        <Link to="/">Back to projects</Link> · <Link to="/archive">Open Archive</Link>
      </section>
    );
  }
  // Keyed by room so drafts, menus and panels reset when the route changes (INV §1).
  return <Room key={id} room={room.data} />;
}

function Room({ room }: { room: ThinkTankDetail }) {
  const id = room.room_id;
  const navigate = useNavigate();
  const projects = useProjects();
  const entries = useThinkTankEntries(id);
  const activity = useThinkTankActivity(id);
  const [panel, setPanel] = useState<"" | "files" | "commands">("");
  const files = useThinkTankFiles(id, true);
  const commands = useThinkTankCommands(id, panel === "commands");
  const control = useThinkTankControl(id);
  const retry = useThinkTankRetry(id);
  const remove = useDeleteThinkTank(id);
  const [confirm, setConfirm] = useState<"" | "end" | "delete">("");
  const [judgeRepair, setJudgeRepair] = useState(false);
  const [actionError, setActionError] = useState("");
  const [menu, setMenu] = useState<AnnotationMenuState | null>(null);
  const [search, setSearch] = useSearchParams();
  const addAnnotation = useAnnotationStore((state) => state.add);
  const discardDrafts = useAnnotationStore((state) => state.discard);
  const pushError = useUiStore((state) => state.pushError);
  const sourceKey = roomAnnotationSource(id);

  const openFile = search.get("file");
  const openSource = search.get("source") ?? "";
  const openLine = Number(search.get("line") ?? "") || undefined;
  const setFile = (link: FileLink | null, source = openSource) => {
    const next = new URLSearchParams(search);
    if (!link) {
      next.delete("file");
      next.delete("source");
      next.delete("line");
    } else {
      next.set("file", link.path);
      next.set("source", source);
      if (link.line) next.set("line", String(link.line));
      else next.delete("line");
    }
    setSearch(next);
  };

  // A participant's links open from that participant's retained workspace, so
  // equal relative paths in different workspaces never collide (FS-21.R26).
  const sourceFor = (attemptID: string) => (files.data?.sources ?? []).find((s) => s.attempt_ids.includes(attemptID))?.source_id;
  const openFrom = (attemptID: string) => (link: FileLink | null) => {
    const source = sourceFor(attemptID);
    if (!source) { pushError("The retained workspace source is unavailable. Try again after the room finishes loading."); return; }
    setFile(link, source);
  };

  const act = (run: () => Promise<unknown>) => {
    setActionError("");
    run().catch((err: unknown) => setActionError(err instanceof Error ? err.message : "That action failed."));
  };

  const byAttempt = useMemo(() => groupActivity(activity.data?.activity ?? []), [activity.data]);
  // Attempt activity reads in the same quiet turns as chat, scoped per attempt;
  // canonical contributions and synthesis are room entries, never hidden here (FS-03.R77).
  const turnChoices = useTurnChoices(room.room_id);
  const status = roomStatus(room);
  const ended = room.phase === "ended";
  const originTitle = projects.data?.[room.origin_project]?.title ?? room.origin_project;
  const participants = room.members.filter((m) => m.role === "participant");
  const judges = room.members.filter((m) => m.role === "judge");
  const failedTurn = room.failed.length > 0 && room.hold !== "";

  const annotateEntry = (mouse: MouseEvent<HTMLElement>, entry: ThinkTankEntry) => {
    if (entry.kind === "missing_opening") return;
    const selected = selectionWithin(mouse.currentTarget);
    mouse.preventDefault();
    const draft: AnnotationDraft = {
      room_anchor: "entry",
      seq: entry.seq,
      excerpt: clipAnnotationExcerpt((selected ?? entry.body).trim()),
      instruction: "",
    };
    setMenu({
      x: mouse.clientX,
      y: mouse.clientY,
      label: selected ? "Annotate selection" : "Annotate whole entry",
      copy: selected ? () => copyText(selected, pushError) : undefined,
      annotate: () => addAnnotation(sourceKey, draft),
    });
  };

  const renderActivity = (agentID: string, events: TranscriptEvent[], attemptID: string) => {
    const toRoomDraft = (draft: AnnotationDraft): AnnotationDraft => ({ ...draft, room_anchor: "activity" } as AnnotationDraft);
    const render = eventRenderer({
      agentId: agentID,
      onAnnotate: (draft) => addAnnotation(sourceKey, toRoomDraft(draft)),
      onContextMenu: (mouse, event) => {
        if (!canAnnotate(event)) return;
        const selected = selectionWithin(mouse.currentTarget);
        mouse.preventDefault();
        const draft = selected ? { seq: Number(event.seq), excerpt: clipAnnotationExcerpt(selected.trim()), instruction: "" } : eventDraft(event);
        setMenu({
          x: mouse.clientX,
          y: mouse.clientY,
          label: selected ? "Annotate selection" : "Annotate whole event",
          copy: selected ? () => copyText(selected, pushError) : undefined,
          annotate: () => addAnnotation(sourceKey, toRoomDraft(draft)),
        });
      },
      onOpenFile: openFrom(attemptID),
    });
    return <TurnList agentId={agentID} events={nestActivities(markBackgrounded(foldTranscript(events)))} scope={`${attemptID}:`} choices={turnChoices} renderEvents={(list) => render(list, [], 1)} />;
  };

  const shownEntries = entries.data?.entries ?? [];
  const placed = new Set(shownEntries.map((e) => e.attempt_id).filter(Boolean));
  const loose = [...byAttempt.keys()].filter((attempt) => !placed.has(attempt));

  return (
    <section className="think-tank" data-ui="think-tank" data-state={room.phase}>
      <PageHeader
        eyebrow={<>Think Tank · <Link to={`/project/${encodeURIComponent(room.origin_project)}`}>{originTitle}</Link></>}
        title={<span className="think-tank-goal">{room.goal}</span>}
        description={<span className="think-tank-phase"><Badge variant={ended ? "neutral" : "info"}>{phaseLabel(room.phase)}</Badge>{room.openings && <span>Independent openings</span>}{room.judge.enabled && <span>Final synthesis</span>}</span>}
        actions={
          <>
            <Button type="button" variant="ghost" onClick={() => setPanel(panel === "files" ? "" : "files")} aria-pressed={panel === "files"}>Files</Button>
            <Button type="button" variant="ghost" onClick={() => setPanel(panel === "commands" ? "" : "commands")} aria-pressed={panel === "commands"}>Commands</Button>
            {!ended && room.control === "running" && <Button type="button" onClick={() => act(() => control.mutateAsync("pause"))} busy={control.isPending}>Pause</Button>}
            {!ended && room.control === "pause_requested" && <Button type="button" onClick={() => act(() => control.mutateAsync("resume"))}>Keep going</Button>}
            {!ended && room.control === "paused" && <Button type="button" variant="primary" onClick={() => act(() => control.mutateAsync("resume"))} busy={control.isPending}>Resume</Button>}
            {!ended && room.control !== "end_requested" && <Button type="button" onClick={() => setConfirm("end")}>End discussion</Button>}
            {room.deletable && <Button type="button" variant="danger" onClick={() => setConfirm("delete")}>Delete</Button>}
          </>
        }
        data-slot="header"
      />
      <div className="think-tank-body" data-slot="body">
        <div className="think-tank-discussion" data-slot="discussion">
          <ol className="think-tank-entries" aria-label="Discussion">
            {shownEntries.length === 0 && !entries.isLoading && (
              <li className="think-tank-empty">{room.phase === "openings" ? "Openings stay hidden until every participant has answered." : "No contributions yet."}</li>
            )}
            {entries.data?.clipped && <li className="think-tank-empty">Showing the newest part of a long discussion.</li>}
            {activity.data?.clipped && <li className="think-tank-empty">Showing the newest 5,000 activity records. Earlier activity remains in retained room history.</li>}
            {shownEntries.map((entry) => (
              <li key={entry.seq}>
                <article
                  className="think-tank-entry"
                  data-slot="entry"
                  data-variant={entry.input_id ? "user" : entry.kind}
                  onContextMenu={(mouse) => annotateEntry(mouse, entry)}
                >
                  <header className="think-tank-entry-meta">
                    {entry.agent_id && room.members.find((m) => m.agent_id === entry.agent_id)?.exists
                      ? <Link className="think-tank-author" to={agentConversationPath(entry.agent_id)}>{entryAuthor(entry)}</Link>
                      : <strong className="think-tank-author">{entryAuthor(entry)}</strong>}
                    {entry.project && <span>{projects.data?.[entry.project]?.title ?? entry.project}</span>}
                    {entryLabel(entry) && <span className="think-tank-entry-kind">{entryLabel(entry)}</span>}
                    {entry.undiscussed && <span className="think-tank-entry-kind">Not discussed before the end</span>}
                    <time dateTime={entry.created_at}>{new Date(entry.created_at).toLocaleTimeString()}</time>
                  </header>
                  {entry.body && (
                    <div className="think-tank-entry-body">
                      <SanitizedMarkdown text={entry.body} onOpenFile={entry.attempt_id ? openFrom(entry.attempt_id) : undefined} />
                    </div>
                  )}
                </article>
                {entry.attempt_id && byAttempt.has(entry.attempt_id) && (
                  <AttemptActivity
                    label={`${entryAuthor(entry)}'s tools and changes`}
                    live={room.active?.attempt_id === entry.attempt_id}
                    events={byAttempt.get(entry.attempt_id)!}
                    render={renderActivity}
                  />
                )}
              </li>
            ))}
            {loose.map((attempt) => {
              const rows = byAttempt.get(attempt)!;
              const live = room.active?.attempt_id === attempt;
              return (
                <li key={attempt}>
                  <AttemptActivity
                    label={live ? `${rows[0].agent_name} is working` : `${rows[0].agent_name}'s unfinished turn`}
                    live={live}
                    events={rows}
                    render={renderActivity}
                  />
                </li>
              );
            })}
          </ol>
          <div className="think-tank-status" data-slot="status" data-state={status.tone} role="status" aria-live="polite">
            <p>{status.text}</p>
            {room.pending.length > 0 && <p className="think-tank-pending">{room.pending.length === 1 ? "1 message is" : `${room.pending.length} messages are`} waiting for the turn to finish.</p>}
            <div className="think-tank-status-actions">
              {room.phase === "setup" && room.hold && <Button type="button" onClick={() => act(() => retry.mutateAsync({ target: "setup" }))}>Retry launch</Button>}
              {room.phase !== "setup" && room.hold && !ended && (
                <Button type="button" variant="primary" onClick={() => act(() => retry.mutateAsync({ target: "turn" }))}>{failedTurn ? "Retry turn" : "Resume"}</Button>
              )}
              {room.judge_status === "failed" && (
                <>
                  <Button type="button" variant="primary" onClick={() => act(() => retry.mutateAsync({ target: "judge" }))}>Retry synthesis</Button>
                  <Button type="button" onClick={() => setJudgeRepair(true)}>Change judge settings</Button>
                </>
              )}
            </div>
            {actionError && <p className="form-error" role="alert">{actionError}</p>}
          </div>
          <RoomComposer room={room} />
        </div>
        <aside className="think-tank-side" data-slot="participants" aria-label="Participants">
          <h2>Participants</h2>
          <ul className="think-tank-members">
            {[...participants, ...judges].map((m) => (
              <li key={m.agent_id} className="think-tank-member" data-state={room.active?.agent_id === m.agent_id ? "speaking" : m.state}>
                <div>
                  {m.exists ? <Link to={agentConversationPath(m.agent_id)}>{m.name}</Link> : <strong>{m.name}</strong>}
                  <span>{projects.data?.[m.project]?.title ?? m.project}</span>
                </div>
                <div className="think-tank-member-meta">
                  {m.role === "participant" && <span>{m.completed} of {m.limit} turns</span>}
                  {memberState(room, m) && <span>{memberState(room, m)}</span>}
                  {!m.exists && <span>Agent deleted</span>}
                  {m.exists && m.archived && <span>Archived</span>}
                  {m.setup_error && <span className="form-error">{m.setup_error}</span>}
                </div>
              </li>
            ))}
          </ul>
          {panel === "files" && (
            <section className="think-tank-panel" data-slot="files">
              <h2>Files</h2>
              {files.data?.clipped && <p>Files from the newest 10,000 activity records are shown. Earlier changes remain in retained room history.</p>}
              {(files.data?.files ?? []).length === 0 && <p>No files changed in room turns.</p>}
              <ul>
                {(files.data?.files ?? []).map((f) => (
                  <li key={`${f.source_id}-${f.path}`}>
                    <button type="button" className="annotation-link" onClick={() => setFile({ path: f.path }, f.source_id)}>{f.path}</button>
                    <span>{f.agent_name} · {f.project}</span>
                  </li>
                ))}
              </ul>
            </section>
          )}
          {panel === "commands" && (
            <section className="think-tank-panel" data-slot="commands">
              <h2>Commands</h2>
              {commands.data?.clipped && <p>Commands from the newest 10,000 activity records are shown. Earlier commands remain in retained room history.</p>}
              {(commands.data?.commands ?? []).length === 0 && <p>No commands ran in room turns.</p>}
              <ul>
                {(commands.data?.commands ?? []).map((c) => (
                  <li key={`${c.seq}`}><code>{c.command}</code><span>{c.agent_name} · {c.status || "running"}</span></li>
                ))}
              </ul>
            </section>
          )}
        </aside>
      </div>
      {openFile && (
        <FileViewer
          agentId={openSource}
          link={{ path: openFile, line: openLine }}
          load={(path) => loadSourceFile(id, openSource, path)}
          onClose={() => setFile(null)}
          onOpenFile={(link) => setFile(link)}
          onSelectionMenu={({ x, y, text, draft }) => setMenu({
            x,
            y,
            label: "Annotate selection",
            copy: () => copyText(text, pushError),
            annotate: () => addAnnotation(sourceKey, { ...draft, room_anchor: "file", source_id: openSource } as AnnotationDraft),
          })}
        />
      )}
      <RoomAnnotationTray room={room} />
      <AnnotationContextMenu menu={menu} onClose={() => setMenu(null)} />
      <ConfirmDialog
        open={confirm === "end"}
        title="End this discussion?"
        confirmLabel="End discussion"
        onCancel={() => setConfirm("")}
        onConfirm={() => { setConfirm(""); act(() => control.mutateAsync("end")); }}
      >
        <p>{room.active ? "The current turn finishes first. " : ""}No further participant turn starts{room.judge.enabled ? ", then the judge writes the synthesis" : ""}. Participants keep running and any private work continues.</p>
      </ConfirmDialog>
      <ConfirmDialog
        open={confirm === "delete"}
        title="Delete this Think Tank?"
        confirmLabel="Delete room"
        destructive
        onCancel={() => setConfirm("")}
        onConfirm={() => {
          setConfirm("");
          act(async () => {
            await remove.mutateAsync(undefined);
            discardDrafts(sourceKey);
            navigate(`/project/${encodeURIComponent(room.origin_project)}`);
          });
        }}
      >
        <p>This deletes the room's discussion and retained activity. Participant agents and their own conversations stay.{room.judge.enabled && room.judge_status !== "completed" ? " The pending synthesis will not run." : ""}</p>
      </ConfirmDialog>
      <NewAgentModal
        open={judgeRepair}
        title="Judge settings"
        initialProject={room.origin_project}
        onClose={() => setJudgeRepair(false)}
        onConfigure={(params) => act(() => retry.mutateAsync({ target: "judge", judge: params as ThinkTankLaunch }))}
      />
    </section>
  );
}

function groupActivity(rows: ThinkTankActivity[]) {
  const out = new Map<string, ThinkTankActivity[]>();
  for (const row of rows) {
    const list = out.get(row.attempt_id) ?? [];
    list.push(row);
    out.set(row.attempt_id, list);
  }
  return out;
}

/** AttemptActivity shows one attempt's retained tools, diffs and approvals in a
 *  disclosure scoped to that attempt, so equal tool ids never merge across
 *  actors (TS-14.R15). Only the live attempt's controls are actionable. */
function AttemptActivity({ label, live, events, render }: {
  label: string;
  live: boolean;
  events: ThinkTankActivity[];
  render: (agentID: string, events: TranscriptEvent[], attemptID: string) => React.ReactNode;
}) {
  const pending = live && events.some((row) => row.event?.type === "permission_request")
    && !events.some((row) => row.event?.type === "permission_resolved");
  const transcript: TranscriptEvent[] = events
    .filter((row) => row.event)
    .map((row) => ({ ...row.event, seq: row.seq, agent_id: row.agent_id,
      ...(!live && row.event?.type === "permission_request" ? { data: { ...(row.event.data as object), resolved: "cancelled" } } : {}),
    } as TranscriptEvent));
  const tools = events.filter((row) => row.event?.type === "tool_call").length;
  return (
    <details className="think-tank-activity" data-slot="activity" data-state={live ? "live" : "settled"} open={pending || undefined}>
      <summary>{label}{tools > 0 ? ` · ${tools} tool ${tools === 1 ? "call" : "calls"}` : ""}</summary>
      <fieldset className="think-tank-activity-body">
        {events.some((row) => row.truncated) && <p>Some activity details exceed the display limit. Their retained sequence anchors are preserved.</p>}
        {render(events[0].agent_id, transcript, events[0].attempt_id)}
      </fieldset>
    </details>
  );
}

async function loadSourceFile(roomID: string, sourceID: string, path: string): Promise<FileContent> {
  const url = sourceFileURL(roomID, sourceID, path);
  const response = await fetch(url);
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error((body as { error?: { message?: string } }).error?.message ?? "That file could not be read.");
  return body as FileContent;
}

/** RoomComposer writes shared room input. `@` file and `#` command suggestions
 *  come from one explicitly chosen participant, and each inserted token names
 *  that participant so paths and commands stay attributed (FS-21.R35). */
function RoomComposer({ room }: { room: ThinkTankDetail }) {
  const [text, setText] = useState("");
  const [commandID, setCommandID] = useState(newCommandID);
  const [error, setError] = useState("");
  const send = useThinkTankMessage(room.room_id);
  const ended = room.phase === "ended";
  const held = room.active || room.phase === "openings" || room.phase === "setup";
  const sources = room.members.filter((m) => m.exists);
  const [sourceId, setSourceId] = useState(sources[0]?.agent_id ?? "");
  const sourceName = sources.find((m) => m.agent_id === sourceId)?.name ?? "";
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const autocomplete = useAutocomplete({
    sourceId,
    text,
    textareaRef,
    onText: (next) => { setText(next); setCommandID(newCommandID()); },
    qualify: (item) => `${item.insert.trimEnd()} (${sourceName}) `,
  });
  const submit = () => {
    if (!text.trim()) return;
    setError("");
    send.mutate({ command_id: commandID, body: text }, {
      onSuccess: () => {
        setText("");
        setCommandID(newCommandID());
      },
      onError: (err) => setError(err instanceof Error ? err.message : "The message was not sent."),
    });
  };
  return (
    <form className="think-tank-composer" data-ui="composer" onSubmit={(event) => { event.preventDefault(); submit(); }}>
      <label htmlFor="think-tank-message">Message the room</label>
      <div className="composer-input">
        <textarea
          id="think-tank-message"
          ref={textareaRef}
          value={text}
          disabled={ended || send.isPending}
          placeholder={ended ? "The discussion has ended. Annotate an entry to follow up with an agent." : "Shared with every participant between turns. Type @ or # for a participant's files or commands."}
          onChange={(event) => { setText(event.target.value); setCommandID(newCommandID()); autocomplete.syncTrigger(event.target); }}
          onKeyUp={(event) => autocomplete.syncTrigger(event.currentTarget)}
          onClick={(event) => autocomplete.syncTrigger(event.currentTarget)}
          onKeyDown={(event) => {
            if (autocomplete.onKeyDown(event)) return;
            if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
              event.preventDefault();
              submit();
            }
          }}
        />
        {autocomplete.picker}
      </div>
      <div className="think-tank-composer-row">
        {sources.length > 0 && !ended && (
          <label className="think-tank-check">
            Suggestions from
            <select value={sourceId} onChange={(event) => { setSourceId(event.target.value); autocomplete.reset(); }}>
              {sources.map((m) => <option key={m.agent_id} value={m.agent_id}>{m.name}</option>)}
            </select>
          </label>
        )}
        <span>{held && !ended ? "Held until the current turn finishes." : ""}</span>
        <Button type="submit" variant="primary" disabled={ended || !text.trim()} busy={send.isPending}>Send to room</Button>
      </div>
      {error && <p className="form-error" role="alert">{error}</p>}
    </form>
  );
}
