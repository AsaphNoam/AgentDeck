import { Link } from "react-router-dom";
import { useThinkTanks } from "../../api/thinkTanks";
import { useProjects } from "../../api/config";
import { endReasonText, phaseLabel, roomTitle, thinkTankPath } from "./roomText";

/** RoomList discovers rooms on their originating project and in Archive,
 *  including after the project is removed (FS-21.R39, FS-05.R39). Rooms are
 *  not agent cards and not archived agent sessions. */
export function RoomList({ project, title = "Think Tanks", emptyText }: { project?: string; title?: string; emptyText?: string }) {
  const rooms = useThinkTanks(project);
  const projects = useProjects();
  if (rooms.isError) return <section className="think-tank-list" data-ui="think-tank" data-slot="list"><h2>{title}</h2><p className="form-error">Could not load Think Tanks.</p></section>;
  const list = rooms.data ?? [];
  if (list.length === 0 && !emptyText) return null;
  return (
    <section className="think-tank-list" data-ui="think-tank" data-slot="list">
      <h2>{title}</h2>
      {list.length === 0 && <p>{emptyText}</p>}
      <ul>
        {list.map((room) => {
          const origin = projects.data?.[room.origin_project];
          return (
            <li key={room.room_id} data-state={room.phase}>
              <Link to={thinkTankPath(room.room_id)}>{roomTitle(room)}</Link>
              <span>
                {phaseLabel(room.phase)}
                {room.phase === "ended" && room.end_reason ? ` — ${endReasonText(room.end_reason)}` : ""}
                {room.hold ? " — needs attention" : room.control === "paused" ? " — paused" : ""}
              </span>
              <span>{room.participants.join(", ")}</span>
              {!project && <span>{origin ? origin.title : `${room.origin_project} (removed project)`}</span>}
            </li>
          );
        })}
      </ul>
    </section>
  );
}
