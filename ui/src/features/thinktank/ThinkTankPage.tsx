import { useMemo, useState, type MouseEvent, type ReactNode } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router-dom";
import * as Dialog from "@radix-ui/react-dialog";
import {
  sourceFileURL,
  useDeleteThinkTank,
  useThinkTank,
  useThinkTankActivity,
  useThinkTankCommands,
  useThinkTankControl,
  useThinkTankEntries,
  useThinkTankFiles,
  useThinkTankRetry,
  type ThinkTankLaunch,
} from "../../api/thinkTanks";
import { useProjects } from "../../api/config";
import type { AnnotationDraft, FileContent, TranscriptEvent } from "../../api/types";
import { Button, ConfirmDialog } from "../../components/ui";
import { BackIcon, CommandIcon, FileIcon, PauseIcon, RoomIcon } from "../../components/ui/icons";
import { useAgentStore } from "../../store/agentStore";
import { AnnotationContextMenu, type AnnotationMenuState } from "../../components/chat/AnnotationContextMenu";
import { FileViewer } from "../../components/chat/FileViewer";
import { SanitizedMarkdown } from "../../components/chat/renderers/SanitizedMarkdown";
import type { FileLink } from "../../components/chat/renderers/filePath";
import { canAnnotate, eventDraft, eventRenderer, selectionWithin } from "../../components/chat/TranscriptView";
import { markBackgrounded, nestActivities } from "../../components/chat/runtimeActivity";
import { TurnList, useTurnChoices } from "../../components/chat/TurnList";
import { foldTranscript } from "../../store/transcriptStore";
import { useAnnotationStore } from "../../store/annotationStore";
import { useUiStore } from "../../store/uiStore";
import { clipAnnotationExcerpt } from "../../lib/annotations";
import { copyText } from "../../lib/copyText";
import { claimWebLink } from "../../lib/linkActions";
import { agentConversationPath } from "../../lib/agentConversation";
import { NewAgentModal } from "../launch/NewAgentModal";
import type { ThinkTankActivity, ThinkTankDetail, ThinkTankEntry, ThinkTankMember } from "../../schemas/thinkTank";
import {
  entryAuthor,
  entryAddressees,
  entryLabel,
  judgeText,
  memberName,
  memberState,
  phaseLabel,
  roomAnnotationSource,
  roomStatus,
  roomTitle,
  type RoomStatus,
} from "./roomText";
import { RoomAnnotationTray } from "./RoomAnnotationTray";
import { TurnLimitEditor } from "./TurnLimitEditor";
import { RoomComposer } from "./RoomComposer";
import { speakerSlot } from "./speakerSlot";

const STATE_LABEL: Record<RoomStatus["tone"], string> = {
  active: "Running",
  waiting: "Waiting",
  attention: "Needs attention",
  paused: "Paused",
  ended: "Ended",
};

const STATUS_SYMBOL: Record<RoomStatus["tone"], string> = { active: "", waiting: "", attention: "!", paused: "Ⅱ", ended: "—" };

/** ThinkTankPage is the room's full conversation page (FS-21.R27) in the Figma
 *  think-tank composition (FS-12.R64, TS-08.R115): breadcrumb, header and
 *  context row, the goal card, then the attributed discussion, the current
 *  action and the composer, with participant cards beside it (TS-08.R87). */
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
  const commands = useThinkTankCommands(id, true);
  const agents = useAgentStore((state) => state.agents);
  const runtimeOf = (agentID: string) => {
    const agent = agents[agentID];
    return agent?.model ? [agent.model, agent.effort].filter(Boolean).join(" · ") : "";
  };
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
  // Each failed opening is retried explicitly by its attempt (FS-21.R48).
  const failedOpenings = room.phase === "openings" ? room.failed.filter((a) => a.turn === "opening") : [];

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
      link: claimWebLink(mouse),
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
          link: claimWebLink(mouse),
        });
      },
      onOpenFile: openFrom(attemptID),
    });
    return <TurnList agentId={agentID} events={nestActivities(markBackgrounded(foldTranscript(events)))} scope={`${attemptID}:`} choices={turnChoices} renderEvents={(list) => render(list, [], 1)} />;
  };

  const shownEntries = entries.data?.entries ?? [];
  const placed = new Set(shownEntries.map((e) => e.attempt_id).filter(Boolean));
  for (const row of activity.data?.activity ?? []) {
    if (row.published) placed.add(row.attempt_id);
  }
  const loose = [...byAttempt.keys()].filter((attempt) => !placed.has(attempt));
  const projectTitle = (project: string) => projects.data?.[project]?.title ?? project;
  const fileList = files.data?.files ?? [];
  const commandList = commands.data?.commands ?? [];
  const writing = room.active_attempts.map((a) => memberName(room, a.agent_id));
  // Ruled round labels introduce the first opening and the first discussion reply.
  const firstOpening = shownEntries.find((e) => e.kind === "opening" || e.kind === "missing_opening")?.seq;
  const firstReply = shownEntries.find((e) => e.kind === "reply")?.seq;

  return (
    <section className="think-tank" data-ui="think-tank" data-state={room.phase}>
      <nav className="think-tank-breadcrumb" aria-label="Room location">
        <Link to={`/project/${encodeURIComponent(room.origin_project)}`}><BackIcon />Project</Link>
        <span className="think-tank-breadcrumb-path">{originTitle}<span aria-hidden="true">/</span><strong>Think Tank</strong></span>
        <code>{room.room_id}</code>
      </nav>
      <header className="think-tank-header" data-slot="header">
        <div className="think-tank-heading">
          <p className="think-tank-eyebrow"><span className="think-tank-symbol"><RoomIcon /></span>Think Tank <span>/ {originTitle}</span></p>
          <h1 className="think-tank-title">{roomTitle(room)}</h1>
        </div>
        <div className="think-tank-actions">
          <Button type="button" onClick={() => setPanel("files")}><FileIcon />Files <span className="think-tank-count">{fileList.length}</span></Button>
          <Button type="button" onClick={() => setPanel("commands")}><CommandIcon />Commands <span className="think-tank-count">{commandList.length}</span></Button>
          {(!ended || room.deletable) && <span className="think-tank-action-rule" aria-hidden="true" />}
          {!ended && room.control === "running" && <Button type="button" onClick={() => act(() => control.mutateAsync("pause"))} busy={control.isPending}><PauseIcon />Pause</Button>}
          {!ended && room.control === "pause_requested" && <Button type="button" onClick={() => act(() => control.mutateAsync("resume"))}>Keep going</Button>}
          {!ended && room.control === "paused" && <Button type="button" onClick={() => act(() => control.mutateAsync("resume"))} busy={control.isPending}>Resume</Button>}
          {!ended && room.control !== "end_requested" && <Button type="button" variant="danger" onClick={() => setConfirm("end")}>End discussion</Button>}
          {room.deletable && <Button type="button" variant="danger" onClick={() => setConfirm("delete")}>Delete</Button>}
        </div>
      </header>
      <div className="think-tank-context">
        <span className="think-tank-state" data-state={status.tone}><i aria-hidden="true" />{STATE_LABEL[status.tone]}</span>
        <span>Phase <b>{phaseLabel(room.phase)}</b></span>
        <span className="think-tank-context-rule" aria-hidden="true" />
        <span>Independent openings <b>{room.openings ? "On" : "Off"}</b></span>
        <span>Final synthesis <b>{room.judge.enabled ? "On" : "Off"}</b></span>
        {room.pipeline && <Link className="think-tank-pipeline" to={`/pipelines/runs/${encodeURIComponent(room.pipeline.run_id)}`}>Pipeline run · stage {room.pipeline.stage_id}</Link>}
      </div>
      <details className="think-tank-goal" data-slot="goal">
        <summary>
          <span className="think-tank-goal-chevron" aria-hidden="true">›</span>
          <strong>Room goal</strong>
          <span className="think-tank-goal-preview">{room.goal}</span>
          <span className="think-tank-goal-toggle" aria-hidden="true"><span>View full goal</span><span>Collapse</span></span>
        </summary>
        <p>{room.goal}</p>
      </details>
      <div className="think-tank-body" data-slot="body">
        <div className="think-tank-discussion" data-slot="discussion">
          <div className="think-tank-discussion-heading">
            <h2>Room discussion</h2>
            <span>Shared history only · started {new Date(room.created_at).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" })}</span>
          </div>
          <ol className="think-tank-entries" aria-label="Discussion">
            {shownEntries.length === 0 && !entries.isLoading && room.phase !== "openings" && (
              <li className="think-tank-empty">No contributions yet.</li>
            )}
            {entries.data?.clipped && <li className="think-tank-empty">Showing the newest part of a long discussion.</li>}
            {activity.data?.clipped && <li className="think-tank-empty">Showing the newest 5,000 activity records. Earlier activity remains in retained room history.</li>}
            {shownEntries.map((entry) => {
              const member = room.members.find((m) => m.agent_id === entry.agent_id);
              const event = entry.kind === "departure" || entry.kind === "missing_opening";
              const user = !!entry.input_id;
              return (
                <li key={entry.seq}>
                  {entry.seq === firstOpening && <RoundLabel label="Independent openings" note="revealed together" />}
                  {entry.seq === firstReply && <RoundLabel label="Discussion" />}
                  <article
                    className="think-tank-entry"
                    data-slot="entry"
                    data-variant={user ? "user" : entry.kind}
                    data-speaker-slot={user || event || entry.kind === "stage_context" ? undefined : speakerSlot(member)}
                    onContextMenu={(mouse) => annotateEntry(mouse, entry)}
                  >
                    {user && <span className="think-tank-avatar" aria-hidden="true">Y</span>}
                    {event && <span className="think-tank-event-symbol" aria-hidden="true">{entry.kind === "departure" ? "↗" : "○"}</span>}
                    <div className="think-tank-entry-main">
                      <header className="think-tank-entry-meta">
                        {entry.agent_id && member?.exists
                          ? <Link className="think-tank-author" to={agentConversationPath(entry.agent_id)}>{entryAuthor(entry)}</Link>
                          : <strong className="think-tank-author">{entryAuthor(entry)}</strong>}
                        {entry.project && <span>{projectTitle(entry.project)}</span>}
                        {entryLabel(entry) && <span className="think-tank-entry-kind">{entryLabel(entry)}</span>}
                        {entry.undiscussed && <span className="think-tank-entry-kind">Not discussed before the end</span>}
                        <time dateTime={entry.created_at}>{new Date(entry.created_at).toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })}</time>
                      </header>
                      {entryAddressees(entry) && <p className="think-tank-addressed">Addressed to {entryAddressees(entry)} · shared with the room</p>}
                      {entry.body && (
                        <div className="think-tank-entry-body">
                          <SanitizedMarkdown text={entry.body} onOpenFile={entry.attempt_id ? openFrom(entry.attempt_id) : undefined} />
                        </div>
                      )}
                    </div>
                  </article>
                </li>
              );
            })}
            {room.phase === "openings" && (
              <li className="think-tank-hidden-openings">
                <span className="think-tank-symbol" aria-hidden="true"><RoomIcon /></span>
                <h3>A little space for independent thinking</h3>
                <p>Openings stay hidden until every participant has answered, then appear together.</p>
                {writing.length > 0 && <p className="think-tank-thinking"><span className="think-tank-dots" aria-hidden="true"><i /><i /><i /></span>Still writing: {writing.join(", ")}</p>}
              </li>
            )}
            {loose.map((attempt) => {
              const rows = byAttempt.get(attempt)!;
              const live = room.active_attempts.some((a) => a.attempt_id === attempt);
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
            <span className="think-tank-status-symbol" aria-hidden="true">
              {status.tone === "active" || status.tone === "waiting" ? <span className="think-tank-dots"><i /><i /><i /></span> : STATUS_SYMBOL[status.tone]}
            </span>
            <div className="think-tank-status-main">
            <p className="think-tank-status-text">{status.text}</p>
            {room.pending.length > 0 && <p className="think-tank-pending">{room.pending.length === 1 ? "1 message is" : `${room.pending.length} messages are`} waiting for the turn to finish.</p>}
            <div className="think-tank-status-actions">
              {room.phase === "setup" && room.hold && <Button type="button" onClick={() => act(() => retry.mutateAsync({ target: "setup" }))}>Retry launch</Button>}
              {failedOpenings.map((a) => (
                <Button key={a.attempt_id} type="button" onClick={() => act(() => retry.mutateAsync({ target: "turn", attempt_id: a.attempt_id }))}>
                  Retry {memberName(room, a.agent_id)}&rsquo;s opening
                </Button>
              ))}
              {room.phase !== "setup" && room.hold && !ended && failedOpenings.length === 0 && (
                <Button type="button" onClick={() => act(() => retry.mutateAsync({ target: "turn" }))}>{failedTurn ? "Retry turn" : "Resume"}</Button>
              )}
              {failedOpenings.length > 0 && room.control === "paused" && (
                <Button type="button" onClick={() => act(() => control.mutateAsync("resume"))}>Resume</Button>
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
          </div>
          <RoomComposer room={room} />
        </div>
        <aside className="think-tank-side" data-slot="participants" aria-label="Participants">
          <div className="think-tank-side-heading"><h2>Participants</h2><span>{participants.length}</span></div>
          <p className="think-tank-side-note">Individual agents, shared context.</p>
          <ul className="think-tank-members">
            {participants.map((m) => (
              <MemberCard key={m.agent_id} room={room} member={m} project={projectTitle(m.project)} runtime={runtimeOf(m.agent_id)}>
                <span className="think-tank-member-turns">{m.completed} / {m.limit} turns</span>
              </MemberCard>
            ))}
          </ul>
          {judges.length > 0 && (
            <section className="think-tank-judge">
              <div className="think-tank-side-heading"><h3>Final synthesis</h3><span>Judge</span></div>
              <ul className="think-tank-members">
                {judges.map((m) => (
                  <MemberCard key={m.agent_id} room={room} member={m} project={projectTitle(m.project)} runtime={runtimeOf(m.agent_id)} />
                ))}
              </ul>
              <p className="think-tank-side-note">The judge observes the room and writes the final synthesis. It does not take discussion turns.</p>
            </section>
          )}
          <section className="think-tank-history-note">
            <RoomIcon />
            <h3>A room, not a group assistant</h3>
            <p>Each entry belongs to its author. Agent links open private conversations; only contributions shared here appear in this room.</p>
          </section>
        </aside>
      </div>
      <Dialog.Root open={panel !== ""} onOpenChange={(open) => { if (!open) setPanel(""); }}>
        <Dialog.Portal>
          <Dialog.Overlay className="dialog-overlay" data-ui="dialog" data-slot="overlay" />
          <Dialog.Content className="dialog-content think-tank-sources" data-ui="dialog" data-slot="content" data-variant="default" aria-describedby={undefined}>
            <p className="think-tank-eyebrow">Think Tank / {originTitle}</p>
            <Dialog.Title data-slot="title">{panel === "files" ? "Files" : "Commands"}</Dialog.Title>
            {panel === "files" && (
              <section data-ui="think-tank" data-slot="files">
                <p className="think-tank-sources-note">Files changed in participant turns. Each file opens from its participant's retained workspace.</p>
                {files.data?.clipped && <p className="think-tank-sources-note">Files from the newest 10,000 activity records are shown. Earlier changes remain in retained room history.</p>}
                {fileList.length === 0 && <p className="think-tank-sources-note">No files changed in room turns.</p>}
                <ul className="think-tank-source-rows">
                  {fileList.map((f) => (
                    <li key={`${f.source_id}-${f.path}`}>
                      <FileIcon />
                      <div><code>{f.path}</code><span>{f.agent_name} · {projectTitle(f.project)}</span></div>
                      <Button type="button" aria-label={f.path} onClick={() => { setPanel(""); setFile({ path: f.path }, f.source_id); }}>Open read-only ↗</Button>
                    </li>
                  ))}
                </ul>
              </section>
            )}
            {panel === "commands" && (
              <section data-ui="think-tank" data-slot="commands">
                <p className="think-tank-sources-note">Commands from participant activity, not room-level tool calls.</p>
                {commands.data?.clipped && <p className="think-tank-sources-note">Commands from the newest 10,000 activity records are shown. Earlier commands remain in retained room history.</p>}
                {commandList.length === 0 && <p className="think-tank-sources-note">No commands ran in room turns.</p>}
                <ul className="think-tank-source-rows">
                  {commandList.map((c) => (
                    <li key={`${c.seq}`}>
                      <CommandIcon />
                      <div><code>{c.command}</code><span>{c.agent_name} · {projectTitle(c.project)} · {c.status || "running"}</span></div>
                    </li>
                  ))}
                </ul>
              </section>
            )}
            <div className="form-actions" data-slot="actions">
              <Dialog.Close asChild><button type="button">Close</button></Dialog.Close>
            </div>
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>
      {openFile && (
        <FileViewer
          agentId={openSource}
          link={{ path: openFile, line: openLine }}
          load={(path) => loadSourceFile(id, openSource, path)}
          onClose={() => setFile(null)}
          onOpenFile={(link) => setFile(link)}
          onSelectionMenu={({ x, y, text, draft, link }) => setMenu({
            x,
            y,
            label: "Annotate selection",
            copy: () => copyText(text, pushError),
            annotate: () => addAnnotation(sourceKey, { ...draft, room_anchor: "file", source_id: openSource } as AnnotationDraft),
            link,
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
        groupPicker={false}
        title="Judge settings"
        initialProject={room.origin_project}
        onClose={() => setJudgeRepair(false)}
        onConfigure={(params) => act(() => retry.mutateAsync({ target: "judge", judge: params as ThinkTankLaunch }))}
      />
    </section>
  );
}

function RoundLabel({ label, note }: { label: string; note?: string }) {
  return <p className="think-tank-round" aria-hidden="true"><span />{label}{note && <b>{note}</b>}<span /></p>;
}

/** One participant or judge card: the speaker tint on its leading edge, then
 *  name/project, the live runtime when known, turns, and its current state. */
function MemberCard({ room, member: m, project, runtime, children }: {
  room: ThinkTankDetail;
  member: ThinkTankMember;
  project: string;
  runtime: string;
  children?: ReactNode;
}) {
  const speaking = room.active_attempts.some((a) => a.agent_id === m.agent_id) || room.active?.agent_id === m.agent_id;
  const base = memberState(room, m);
  const state = base === "Judge" ? judgeText(room.judge_status, room.judge.error) || base : base || (room.phase === "ended" ? "Discussion ended" : "Waiting for next turn");
  return (
    <li className="think-tank-member" data-speaker-slot={speakerSlot(m)} data-state={speaking ? "speaking" : m.state}>
      <div className="think-tank-member-top">
        {m.exists ? <Link to={agentConversationPath(m.agent_id)}>{m.name} <span aria-hidden="true">↗</span></Link> : <strong>{m.name}</strong>}
        <span>{project}</span>
      </div>
      {runtime && <p className="think-tank-member-runtime">{runtime}</p>}
      <div className="think-tank-member-meta">
        {children}
        {!m.exists && <span>Agent deleted</span>}
        {m.exists && m.archived && <span>Archived</span>}
      </div>
      <p className="think-tank-member-state"><i aria-hidden="true" />{state}</p>
      {m.setup_error && <p className="form-error">{m.setup_error}</p>}
      <TurnLimitEditor room={room} member={m} />
    </li>
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
