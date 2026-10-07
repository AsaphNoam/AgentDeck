import { useState } from "react";
import { newCommandID, useThinkTankTurnLimit } from "../../api/thinkTanks";
import { Button } from "../../components/ui";
import type { ThinkTankDetail, ThinkTankMember } from "../../schemas/thinkTank";

const MAX_LIMIT = 1000;

/** TurnLimitEditor raises one participant's turn ceiling while openings or
 *  discussion are open (FS-21.R49). It shows completed, current and proposed
 *  counts before Save; a refused change keeps the draft. Raising a ceiling
 *  never sends, steers or resumes. */
export function TurnLimitEditor({ room, member }: { room: ThinkTankDetail; member: ThinkTankMember }) {
  const save = useThinkTankTurnLimit(room.room_id);
  const [draft, setDraft] = useState<number | null>(null);
  const [commandID, setCommandID] = useState(newCommandID);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(0);
  const open = (room.phase === "openings" || room.phase === "discussion") && room.control !== "end_requested";
  if (!open || member.role !== "participant" || member.state === "departed" || member.limit >= MAX_LIMIT) return null;

  if (draft === null) {
    return (
      <>
        {saved === member.limit && <span role="status">Turn limit saved.</span>}
        <Button type="button" variant="ghost" onClick={() => { setDraft(member.limit + 1); setError(""); setCommandID(newCommandID()); }}>
          Raise turn limit
        </Button>
      </>
    );
  }
  const valid = Number.isInteger(draft) && draft > member.limit && draft <= MAX_LIMIT;
  const submit = () => {
    if (!valid) {
      setError(`Choose a whole number from ${member.limit + 1} to ${MAX_LIMIT}.`);
      return;
    }
    setError("");
    save.mutate({ agent_id: member.agent_id, command_id: commandID, expected_limit: member.limit, limit: draft }, {
      onSuccess: () => { setSaved(draft); setDraft(null); },
      onError: (err) => setError(err instanceof Error ? err.message : "The turn limit could not be saved."),
    });
  };
  return (
    <form className="think-tank-limit" data-slot="turn-limit" onSubmit={(event) => { event.preventDefault(); submit(); }}>
      <label>
        New turn limit for {member.name}
        <input type="number" min={member.limit + 1} max={MAX_LIMIT} value={draft}
          onChange={(e) => { setDraft(Number(e.target.value)); setCommandID(newCommandID()); }} />
      </label>
      <span>{member.completed} used · limit {member.limit} → {valid ? draft : "?"} · {valid ? draft - member.completed : "?"} left after saving</span>
      {error && <span className="form-error" role="alert">{error}</span>}
      <span className="think-tank-limit-actions">
        <Button type="submit" variant="primary" busy={save.isPending}>Save</Button>
        <Button type="button" variant="ghost" onClick={() => { setDraft(null); setError(""); }}>Cancel</Button>
      </span>
    </form>
  );
}
