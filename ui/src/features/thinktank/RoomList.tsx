import { Link } from "react-router-dom";
import { useThinkTanks } from "../../api/thinkTanks";
import { useProjects } from "../../api/config";
import { agentConversationPath } from "../../lib/agentConversation";
import type { ThinkTankSummary } from "../../schemas/thinkTank";
import { endReasonText, judgeText, phaseLabel, roomTitle, thinkTankPath } from "./roomText";

/** RoomList discovers rooms on their originating project and in Archive,
 *  including after the project is removed (FS-21.R39, FS-05.R39). Each room is
 *  one wide navigation card per row, independent of agent-grid density
 *  (FS-02.R71, FS-21.R44), in the Figma room-summary composition
 *  (FS-12.R63, TS-08.R114). Rooms are not agent cards. */
export function RoomList({ project, title = "Thinking together", emptyText }: { project?: string; title?: string; emptyText?: string }) {
  const rooms = useThinkTanks(project);
  if (rooms.isLoading) return null;
  const list = rooms.data ?? [];
  if (!rooms.isError && list.length === 0 && !emptyText) return null;
  return (
    <section className="think-tank-list" data-ui="think-tank" data-slot="list" aria-label="Think Tank rooms">
      <div className="room-list-heading">
        <div>
          <p className="room-list-eyebrow">Shared rooms <span>/ Think Tank</span></p>
          <h2>{title}</h2>
        </div>
        {list.length > 0 && <span className="room-list-note">{list.length} {list.length === 1 ? "room" : "rooms"} · separate from agent sessions</span>}
      </div>
      {rooms.isError && <p className="form-error">Could not load Think Tanks.</p>}
      {!rooms.isError && list.length === 0 && <p>{emptyText}</p>}
      <ul>
        {list.map((room) => <RoomCard key={room.room_id} room={room} showOrigin={!project} />)}
      </ul>
    </section>
  );
}

type Tone = "active" | "attention" | "paused" | "ended" | "waiting";

/** cardStatus is the card's one attention/activity line: who is speaking,
 *  why the room waits, or how it ended. Judge state has its own footer slot. */
function cardStatus(room: ThinkTankSummary): { tone: Tone; text: string } {
  const name = (id: string) => room.roster.find((m) => m.agent_id === id)?.name ?? "A participant";
  if (room.phase === "ended") {
    return { tone: room.judge_status === "failed" ? "attention" : "ended", text: endReasonText(room.end_reason) || "Discussion ended." };
  }
  if (room.hold) return { tone: "attention", text: room.hold };
  if (room.control === "paused") return { tone: "paused", text: "Paused." };
  if (room.active_attempts.length > 1) return { tone: "active", text: `Writing openings: ${room.active_attempts.map((a) => name(a.agent_id)).join(", ")}` };
  if (room.active_attempts.length === 1) return { tone: "active", text: `${name(room.active_attempts[0].agent_id)} is speaking.` };
  if (room.control === "end_requested") return { tone: "waiting", text: "Ending." };
  return { tone: "waiting", text: phaseLabel(room.phase) };
}

const STATE_CHIP: Record<Tone, { symbol: string; label: string }> = {
  active: { symbol: "●", label: "Active" },
  waiting: { symbol: "●", label: "Active" },
  paused: { symbol: "Ⅱ", label: "Paused" },
  attention: { symbol: "!", label: "Needs attention" },
  ended: { symbol: "■", label: "Ended" },
};

function initials(name: string) {
  const words = name.trim().split(/\s+/).filter(Boolean);
  const letters = words.length > 1 ? words[0][0] + words[1][0] : (words[0] ?? "?").slice(0, 2);
  return letters.toUpperCase();
}

const turnsLeft = (n: number) => `${n} ${n === 1 ? "turn" : "turns"} left`;

function RoomSymbol() {
  return (
    <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M4 5h12v9H9l-5 4V5Z" />
      <path d="M19 9h2v12l-5-4h-4" />
      <path d="M8 9h4" />
    </svg>
  );
}

function RoomCard({ room, showOrigin }: { room: ThinkTankSummary; showOrigin: boolean }) {
  const projects = useProjects();
  const projectTitle = (id: string) => projects.data?.[id]?.title ?? id;
  const status = cardStatus(room);
  const chip = STATE_CHIP[status.tone];
  const speaking = new Set(room.active_attempts.map((a) => a.agent_id));
  const participants = room.roster.filter((m) => m.role === "participant");
  const ended = room.phase === "ended";
  const origin = projects.data?.[room.origin_project];
  const judge = room.judge_enabled ? `Judge: ${judgeText(room.judge_status) || "waiting"}` : "";
  return (
    <li className="room-card" data-ui="think-tank" data-slot="card" data-state={status.tone}>
      <div className="room-card-icon"><RoomSymbol /></div>
      <div className="room-card-content">
        <div className="room-card-top">
          <h3 className="room-card-title">
            <Link to={thinkTankPath(room.room_id)}>{roomTitle(room)}<span aria-hidden="true">↗</span></Link>
            <span className="room-card-phase">{phaseLabel(room.phase)}</span>
          </h3>
          <span className="room-card-state"><span aria-hidden="true">{chip.symbol}</span>{chip.label}</span>
        </div>
        <p className="room-card-status" role="status">{status.text}</p>
        {room.title && room.title !== room.goal && <p className="room-card-goal"><span>Goal</span>{room.goal}</p>}
        <ul className="room-card-roster" data-slot="roster" aria-label="Participants">
          {participants.map((m) => {
            const state = speaking.has(m.agent_id) ? "speaking" : m.state === "departed" ? "departed" : m.state === "exhausted" ? "exhausted" : "active";
            return (
              <li key={m.agent_id} data-state={state}>
                <span className="room-card-avatar" aria-hidden="true">{initials(m.name)}</span>
                <div className="room-card-person">
                  <span className="room-card-person-name">
                    {m.exists ? <Link to={agentConversationPath(m.agent_id)}>{m.name}<span aria-hidden="true"> ↗</span></Link> : m.name}
                  </span>
                  <span className="room-card-person-meta">{projectTitle(m.project)}{!m.exists && " · agent deleted"} · {turnsLeft(m.remaining)}</span>
                  <span className="room-card-person-state">
                    {state === "speaking" && <span aria-hidden="true">● </span>}
                    {state === "speaking" ? "Speaking" : state === "departed" ? "Departed" : state === "exhausted" ? "Exhausted" : "Ready"}
                  </span>
                </div>
              </li>
            );
          })}
        </ul>
        <div className="room-card-footer">
          <span>
            {showOrigin
              ? <>Origin: <strong>{origin ? origin.title : room.origin_project}</strong>{!origin && projects.data && " · project removed"}</>
              : <>In this project <span>· shared participant conversations stay separate</span></>}
          </span>
          {judge && <span>{judge}</span>}
        </div>
      </div>
      {/* The collective allowance is a ceiling; on an ended card it stays as
          the unused remainder (FS-21.R44). */}
      <div className="room-card-budget">
        <span className="room-card-budget-number">{room.total_remaining}</span>
        <span>{ended ? "unused turns" : "turns remaining"}</span>
        <small>{ended ? "Not a success signal" : "Combined ceiling, not a target"}</small>
      </div>
    </li>
  );
}
