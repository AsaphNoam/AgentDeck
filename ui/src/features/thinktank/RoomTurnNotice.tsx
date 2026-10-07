import { Link } from "react-router-dom";
import { useAgentThinkTanks } from "../../api/thinkTanks";
import { agentConversationPath } from "../../lib/agentConversation";
import { useProjects } from "../../api/config";
import type { ThinkTankSummary } from "../../schemas/thinkTank";
import { judgeText, phaseLabel, roomTitle, thinkTankPath } from "./roomText";

/** RoomCue is the compact Think Tank identification under an agent's
 *  ordinary header: the room title and whether this agent is taking a room
 *  turn now. Full goal, membership and the Send/Steer explanation live in the
 *  Think Tank tab (FS-03.R69, R71, FS-21.R45). It renders nothing for an
 *  agent in no room. */
export function RoomCue({ agentId }: { agentId: string }) {
  const rooms = useAgentThinkTanks(agentId);
  const list = rooms.data ?? [];
  if (list.length === 0) return null;
  const turnIn = list.find((room) => room.active_attempts.some((a) => a.agent_id === agentId));
  const shown = turnIn ?? list.find((room) => room.phase !== "ended") ?? list[0];
  const more = list.length - 1;
  return (
    <p className="room-turn-notice" data-ui="think-tank" data-slot="turn-notice" data-state={turnIn ? "active" : "settled"} role="status">
      <span>Think Tank</span>
      <Link to={thinkTankPath(shown.room_id)}>{roomTitle(shown)}</Link>
      <span>{turnIn ? "Taking a room turn — Steer shapes it privately; Send waits for it to end" : phaseLabel(shown.phase)}</span>
      {more > 0 && <span>+{more} more in the Think Tank tab</span>}
    </p>
  );
}

/** ThinkTankTab lists every room this agent belongs to with its goal, state,
 *  members and allowances, linking to the whole room and each surviving
 *  member's ordinary chat (FS-21.R45, FS-03.R71). */
export function ThinkTankTab({ agentId }: { agentId: string }) {
  const rooms = useAgentThinkTanks(agentId);
  if (rooms.isError) return <p className="form-error">Could not load this agent's Think Tanks.</p>;
  if (rooms.isLoading) return <p>Loading Think Tanks…</p>;
  const list = rooms.data ?? [];
  if (list.length === 0) return <p>This agent is not in a Think Tank.</p>;
  return (
    <div className="think-tank-tab" data-ui="think-tank" data-slot="membership">
      <p className="think-tank-tab-note">
        Room turns use this agent&rsquo;s own conversation. During a room turn, Steer shapes the contribution and
        stays private; Send waits until the turn ends. Nothing private here is copied into the room.
      </p>
      {list.map((room) => <Membership key={room.room_id} room={room} agentId={agentId} />)}
    </div>
  );
}

function Membership({ room, agentId }: { room: ThinkTankSummary; agentId: string }) {
  const projects = useProjects();
  const me = room.roster.find((m) => m.agent_id === agentId);
  const speaking = new Set(room.active_attempts.map((a) => a.agent_id));
  return (
    <section className="think-tank-membership" aria-label={roomTitle(room)}>
      <h3><Link to={thinkTankPath(room.room_id)}>{roomTitle(room)}</Link></h3>
      <p>
        {phaseLabel(room.phase)}
        {me?.role === "judge" ? " · You are the judge" : me ? ` · ${me.state === "departed" ? "Left" : `${me.remaining} of ${me.limit} turns left`}` : ""}
        {room.hold ? ` · Needs attention: ${room.hold}` : ""}
      </p>
      <details>
        <summary>Goal</summary>
        <p className="think-tank-membership-goal">{room.goal}</p>
      </details>
      <ul className="think-tank-card-roster" aria-label="Members">
        {room.roster.map((m) => (
          <li key={m.agent_id} data-state={speaking.has(m.agent_id) ? "speaking" : m.state === "departed" ? "departed" : m.state === "exhausted" ? "exhausted" : "active"}>
            {m.agent_id === agentId ? <strong>{m.name} (this agent)</strong> : m.exists ? <Link to={agentConversationPath(m.agent_id)}>{m.name}</Link> : <span>{m.name} (deleted)</span>}
            <span>{projects.data?.[m.project]?.title ?? m.project}</span>
            <span>{m.role === "judge" ? "Judge" : m.state === "departed" ? "Left" : `${m.remaining} of ${m.limit} left`}</span>
            {speaking.has(m.agent_id) && <span>Speaking</span>}
          </li>
        ))}
      </ul>
      {room.judge_enabled && <p>Judge: {judgeText(room.judge_status) || "waiting"}</p>}
    </section>
  );
}
