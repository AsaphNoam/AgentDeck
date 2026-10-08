import type { PipelineTemplate } from "../../schemas/pipeline";

type Stage = PipelineTemplate["stages"][number];
type RoomConfig = NonNullable<Stage["think_tank"]>;

/** defaultThinkTank seeds a model-neutral two-participant room (FS-14.R81). */
export function defaultThinkTank(role: string): RoomConfig {
  return {
    participants: [
      { id: "first", role, limit: 3, may_leave: false },
      { id: "second", role, limit: 3, may_leave: false },
    ],
    openings: false,
    judge_role: role,
  };
}

/** ThinkTankStageFields edits a think_tank stage's participants, openings and
 *  judge role. Runtimes are chosen per run at Setup → Review, never here. */
export function ThinkTankStageFields({ stage, stageIndex, roleOptions, mutate }: {
  stage: Stage;
  stageIndex: number;
  roleOptions: Array<[string, string]>;
  mutate: (change: (next: PipelineTemplate) => void) => void;
}) {
  const room = stage.think_tank;
  if (!room) return null;
  const edit = (change: (room: RoomConfig) => void) => mutate((next) => {
    const target = next.stages[stageIndex].think_tank;
    if (target) change(target);
  });
  const roleSelect = (value: string, onChange: (role: string) => void, label: string) => (
    <select aria-label={label} value={value} onChange={(event) => onChange(event.target.value)}>
      {roleOptions.map(([roleID, title]) => <option key={roleID} value={roleID}>{title}</option>)}
      <option value="">Select role</option>
    </select>
  );
  return (
    <div className="pipeline-think-tank" data-slot="think-tank">
      <p className="pipeline-think-tank-note">Fresh participants deliberate in a room; a fresh judge synthesizes after discussion ends. The judge's published synthesis becomes this stage's one output.</p>
      <ol className="pipeline-think-tank-participants">
        {room.participants.map((participant, index) => (
          <li key={`participant-${index}`}>
            <label className="form-field"><span>Participant id</span><input value={participant.id} onChange={(event) => edit((r) => { r.participants[index].id = event.target.value; })} /></label>
            <label className="form-field"><span>Role</span>{roleSelect(participant.role, (role) => edit((r) => { r.participants[index].role = role; }), `Participant ${index + 1} role`)}</label>
            <label className="form-field"><span>Contribution limit</span><input type="number" min={1} max={1000} value={participant.limit} onChange={(event) => edit((r) => { r.participants[index].limit = Number(event.target.value); })} /></label>
            <label className="pipeline-check"><input type="checkbox" checked={participant.may_leave} onChange={(event) => edit((r) => { r.participants[index].may_leave = event.target.checked; })} /> May leave</label>
            <button type="button" disabled={room.participants.length <= 2} onClick={() => edit((r) => { r.participants.splice(index, 1); })}>Remove</button>
          </li>
        ))}
      </ol>
      <div className="pipeline-think-tank-actions">
        <button type="button" disabled={room.participants.length >= 32} onClick={() => edit((r) => { r.participants.push({ id: `participant-${r.participants.length + 1}`, role: r.participants[0]?.role ?? "", limit: 3, may_leave: false }); })}>Add participant</button>
        <label className="pipeline-check"><input type="checkbox" checked={room.openings} onChange={(event) => edit((r) => { r.openings = event.target.checked; })} /> Independent openings</label>
        <label className="form-field"><span>Judge role</span>{roleSelect(room.judge_role, (role) => edit((r) => { r.judge_role = role; }), "Judge role")}</label>
      </div>
    </div>
  );
}
