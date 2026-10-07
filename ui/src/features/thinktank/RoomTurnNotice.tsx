import { Link } from "react-router-dom";
import { useThinkTanks } from "../../api/thinkTanks";
import { roomTitle, thinkTankPath } from "./roomText";

/** RoomTurnNotice identifies a running Think Tank turn in the agent's own
 *  conversation, so ordinary Send and Steer stay understandable: Steer shapes
 *  the room contribution being formed and stays private, while Send queues a
 *  private follow-up (FS-03.R69, FS-21.R33). The room list is only read while
 *  the agent is busy. */
export function RoomTurnNotice({ agentId, busy }: { agentId: string; busy: boolean }) {
  const rooms = useThinkTanks(undefined, busy);
  if (!busy) return null;
  const room = rooms.data?.find((r) => r.active_agent_id === agentId);
  if (!room) return null;
  return (
    <p className="room-turn-notice" data-ui="think-tank" data-slot="turn-notice" role="status">
      Taking a turn in the Think Tank <Link to={thinkTankPath(room.room_id)}>{roomTitle(room)}</Link>. Steer shapes this
      contribution and stays in this private conversation; Send waits until the turn ends.
    </p>
  );
}
