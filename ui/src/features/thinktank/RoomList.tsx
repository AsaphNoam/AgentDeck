import { Link } from "react-router-dom";
import { useThinkTanks } from "../../api/thinkTanks";
import { useProjects } from "../../api/config";
import { Badge } from "../../components/ui";
import { agentConversationPath } from "../../lib/agentConversation";
import type { ThinkTankSummary } from "../../schemas/thinkTank";
import { endReasonText, judgeText, phaseLabel, roomTitle, thinkTankPath } from "./roomText";

/** RoomList discovers rooms on their originating project and in Archive,
 *  including after the project is removed (FS-21.R39, FS-05.R39). Each room is
 *  one wide navigation card per row, independent of agent-grid density
 *  (FS-02.R71, FS-21.R44, TS-08.R96). Rooms are not agent cards. */
export function RoomList({ project, title = "Think Tanks", emptyText }: { project?: string; title?: string; emptyText?: string }) {
  const rooms = useThinkTanks(project);
  if (rooms.isError) return <section className="think-tank-list" data-ui="think-tank" data-slot="list"><h2>{title}</h2><p className="form-error">Could not load Think Tanks.</p></section>;
  if (rooms.isLoading) return null;
  const list = rooms.data ?? [];
  if (list.length === 0 && !emptyText) return null;
  return (
    <section className="think-tank-list" data-ui="think-tank" data-slot="list">
      <h2>{title}</h2>
      {list.length === 0 && <p>{emptyText}</p>}
      <ul>
        {list.map((room) => <RoomCard key={room.room_id} room={room} showOrigin={!project} />)}
      </ul>
    </section>
  );
}

/** cardStatus is the card's one attention/activity line: who is speaking,
 *  why the room waits, or how it ended. */
function cardStatus(room: ThinkTankSummary): { tone: "active" | "attention" | "paused" | "ended" | "waiting"; text: string } {
  const name = (id: string) => room.roster.find((m) => m.agent_id === id)?.name ?? "A participant";
  if (room.phase === "ended") {
    const text = [endReasonText(room.end_reason), room.judge_enabled ? judgeText(room.judge_status) : ""].filter(Boolean).join(" ");
    return { tone: room.judge_status === "failed" ? "attention" : "ended", text: text || "Discussion ended." };
  }
  if (room.hold) return { tone: "attention", text: `Needs attention: ${room.hold}` };
  if (room.control === "paused") return { tone: "paused", text: "Paused." };
  if (room.active_attempts.length > 1) return { tone: "active", text: `Writing openings: ${room.active_attempts.map((a) => name(a.agent_id)).join(", ")}` };
  if (room.active_attempts.length === 1) return { tone: "active", text: `${name(room.active_attempts[0].agent_id)} is speaking.` };
  if (room.control === "end_requested") return { tone: "waiting", text: "Ending." };
  return { tone: "waiting", text: phaseLabel(room.phase) };
}

function RoomCard({ room, showOrigin }: { room: ThinkTankSummary; showOrigin: boolean }) {
  const projects = useProjects();
  const projectTitle = (id: string) => projects.data?.[id]?.title ?? id;
  const status = cardStatus(room);
  const speaking = new Set(room.active_attempts.map((a) => a.agent_id));
  const participants = room.roster.filter((m) => m.role === "participant");
  const ended = room.phase === "ended";
  const origin = projects.data?.[room.origin_project];
  return (
    <li className="think-tank-card" data-ui="think-tank" data-slot="card" data-state={status.tone}>
      <div className="think-tank-card-head">
        <h3><Link to={thinkTankPath(room.room_id)}>{roomTitle(room)}</Link></h3>
        <Badge variant={ended ? "neutral" : "info"}>{phaseLabel(room.phase)}</Badge>
        {showOrigin && <span className="think-tank-card-origin">{origin ? origin.title : `${room.origin_project} (removed project)`}</span>}
      </div>
      <p className="think-tank-card-status" role="status">{status.text}</p>
      {room.title && room.title !== room.goal && <p className="think-tank-card-goal">{room.goal}</p>}
      <ul className="think-tank-card-roster" data-slot="roster" aria-label="Participants">
        {participants.map((m) => (
          <li key={m.agent_id} data-state={speaking.has(m.agent_id) ? "speaking" : m.state === "departed" ? "departed" : m.state === "exhausted" ? "exhausted" : "active"}>
            {m.exists ? <Link to={agentConversationPath(m.agent_id)}>{m.name}</Link> : <span>{m.name} (deleted)</span>}
            <span>{projectTitle(m.project)}</span>
            <span>{m.state === "departed" ? "Left" : `${m.remaining} of ${m.limit} left`}</span>
            {speaking.has(m.agent_id) && <span>Speaking</span>}
          </li>
        ))}
      </ul>
      {/* Ended cards keep the collective total as an unused ceiling; their
          judge state is already in the card status (FS-21.R44). */}
      <p className="think-tank-card-totals">
        {ended
          ? `${room.total_remaining} turns went unused`
          : `${room.total_remaining} turns left in total — a ceiling, not a plan`}
        {!ended && room.judge_enabled && <> · Judge: {judgeText(room.judge_status) || "waiting"}</>}
      </p>
    </li>
  );
}
