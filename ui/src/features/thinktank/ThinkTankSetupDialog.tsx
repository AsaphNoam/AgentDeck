import { AutoGrowTextarea } from "../../components/ui";
import { useMemo, useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { useNavigate } from "react-router-dom";
import { newCommandID, useCreateThinkTank, type ThinkTankLaunch } from "../../api/thinkTanks";
import { useProjects } from "../../api/config";
import { useAgentStore } from "../../store/agentStore";
import { NewAgentModal } from "../launch/NewAgentModal";
import { thinkTankPath } from "./roomText";

const DEFAULT_LIMIT = 3;
const MAX_PARTICIPANTS = 32;
const MAX_GOAL = 8000;

type Row = { key: string; agentID?: string; launch?: ThinkTankLaunch; label: string; project: string; limit: number; mayLeave: boolean };

/** ThinkTankSetupDialog starts a room from a non-archived project (FS-21.R27,
 *  R34): a goal, two or more existing or new chat agents from any non-archived
 *  project in a fixed order, each with its own turn limit and permission to
 *  leave (on by default), and optional independent openings and end-only
 *  synthesis (both off by default). Goal and membership are fixed once started. */
export function ThinkTankSetupDialog({ open, onClose, originProject }: { open: boolean; onClose: () => void; originProject: string }) {
  const navigate = useNavigate();
  const projects = useProjects();
  const agents = useAgentStore((state) => state.agents);
  const create = useCreateThinkTank();
  const [goal, setGoal] = useState("");
  const [rows, setRows] = useState<Row[]>([]);
  const [openings, setOpenings] = useState(false);
  const [judge, setJudge] = useState<ThinkTankLaunch | null>(null);
  const [launchFor, setLaunchFor] = useState<"" | "participant" | "judge">("");
  const [pick, setPick] = useState("");
  const [error, setError] = useState("");
  const [commandID, setCommandID] = useState(newCommandID);

  const projectTitle = (id: string) => projects.data?.[id]?.title ?? id;
  const candidates = useMemo(() => Object.values(agents)
    .filter((a) => a.interface === "chat" && !a.archived && !projects.data?.[a.project]?.archived)
    .filter((a) => !rows.some((r) => r.agentID === a.agent_id))
    .sort((a, b) => (a.project + a.name).localeCompare(b.project + b.name)), [agents, projects.data, rows]);

  const update = (key: string, patch: Partial<Row>) => setRows((current) => current.map((r) => r.key === key ? { ...r, ...patch } : r));
  const move = (index: number, delta: number) => setRows((current) => {
    const next = [...current];
    const [row] = next.splice(index, 1);
    next.splice(index + delta, 0, row);
    return next;
  });
  const reset = () => {
    setGoal(""); setRows([]); setOpenings(false); setJudge(null); setError(""); setPick(""); setCommandID(newCommandID());
  };

  const addExisting = () => {
    const agent = agents[pick];
    if (!agent) return;
    setRows((current) => [...current, { key: agent.agent_id, agentID: agent.agent_id, label: agent.name, project: agent.project, limit: DEFAULT_LIMIT, mayLeave: true }]);
    setPick("");
  };

  const problems = [
    !goal.trim() && "Describe the goal.",
    rows.length < 2 && "Add at least two participants.",
    rows.some((r) => !Number.isInteger(r.limit) || r.limit < 1 || r.limit > 1000) && "Turn limits are whole numbers from 1 to 1000.",
  ].filter(Boolean) as string[];

  const submit = () => {
    setError("");
    if (problems.length) {
      setError(problems[0]);
      return;
    }
    create.mutate({
      command_id: commandID,
      goal,
      origin_project: originProject,
      openings,
      judge: judge ?? undefined,
      participants: rows.map((r) => ({ agent_id: r.agentID, new: r.launch, limit: r.limit, may_leave: r.mayLeave })),
    }, {
      onSuccess: (room) => {
        reset();
        onClose();
        navigate(thinkTankPath(room.room_id));
      },
      onError: (err) => setError(err instanceof Error ? err.message : "The Think Tank could not be started."),
    });
  };

  return (
    <>
      <Dialog.Root open={open && !launchFor} onOpenChange={(o) => { if (!o) onClose(); }}>
        <Dialog.Portal>
          <Dialog.Overlay className="dialog-overlay" data-ui="dialog" data-slot="overlay" />
          <Dialog.Content className="dialog-content think-tank-setup" data-ui="dialog" data-slot="content" data-variant="default">
            <Dialog.Title>New Think Tank</Dialog.Title>
            <Dialog.Description>Independent agents discuss one goal in turn. Each keeps its own conversation.</Dialog.Description>
            <form className="config-form" onSubmit={(event) => { event.preventDefault(); submit(); }}>
              <div className="form-field">
                <label htmlFor="think-tank-goal">Goal</label>
                <AutoGrowTextarea id="think-tank-goal" value={goal} maxLength={MAX_GOAL} rows={3} onChange={(e) => { setGoal(e.target.value); setCommandID(newCommandID()); }} placeholder="What should the participants work out?" />
              </div>
              <fieldset className="think-tank-setup-participants">
                <legend>Participants, in speaking order</legend>
                {rows.length === 0 && <p className="form-hint">Add existing agents from any active project, or new ones.</p>}
                <ol>
                  {rows.map((r, index) => (
                    <li key={r.key} className="think-tank-setup-row">
                      <div className="think-tank-setup-name"><strong>{r.label}</strong><span>{projectTitle(r.project)}{r.launch ? " · new agent" : ""}</span></div>
                      <label>Turn limit<input type="number" min={1} max={1000} value={r.limit} onChange={(e) => update(r.key, { limit: Number(e.target.value) })} /></label>
                      <label className="think-tank-check"><input type="checkbox" checked={r.mayLeave} onChange={(e) => update(r.key, { mayLeave: e.target.checked })} /> May leave</label>
                      <div className="think-tank-setup-order">
                        <button type="button" className="annotation-link" disabled={index === 0} onClick={() => move(index, -1)} aria-label={`Move ${r.label} earlier`}>↑</button>
                        <button type="button" className="annotation-link" disabled={index === rows.length - 1} onClick={() => move(index, 1)} aria-label={`Move ${r.label} later`}>↓</button>
                        <button type="button" className="annotation-link" onClick={() => setRows((c) => c.filter((x) => x.key !== r.key))}>Remove</button>
                      </div>
                    </li>
                  ))}
                </ol>
                {rows.length < MAX_PARTICIPANTS && (
                  <div className="think-tank-setup-add">
                    <select value={pick} onChange={(e) => setPick(e.target.value)} aria-label="Existing agent">
                      <option value="">Choose an existing agent</option>
                      {candidates.map((a) => <option key={a.agent_id} value={a.agent_id}>{a.name} ({projectTitle(a.project)}){a.state === "busy" ? " · busy" : ""}</option>)}
                    </select>
                    <button type="button" onClick={addExisting} disabled={!pick}>Add</button>
                    <button type="button" onClick={() => setLaunchFor("participant")}>New agent…</button>
                  </div>
                )}
                <p className="form-hint">A limit is a ceiling, not a target. Busy agents join without being interrupted.</p>
              </fieldset>
              <label className="think-tank-check"><input type="checkbox" checked={openings} onChange={(e) => setOpenings(e.target.checked)} /> Independent openings — each participant answers before seeing the others</label>
              <div className="think-tank-check">
                <label><input type="checkbox" checked={judge !== null} onChange={(e) => e.target.checked ? setLaunchFor("judge") : setJudge(null)} /> Final synthesis by a fresh judge after the discussion ends</label>
                {judge && <span className="form-hint">{judge.name || judge.role} in {projectTitle(judge.project)} · <button type="button" className="annotation-link" onClick={() => setLaunchFor("judge")}>Change</button></span>}
              </div>
              {error && <p className="form-error" role="alert">{error}</p>}
              <div className="form-actions">
                <button type="button" onClick={onClose} disabled={create.isPending}>Cancel</button>
                <button type="submit" disabled={create.isPending}>{create.isPending ? "Starting…" : "Start Think Tank"}</button>
              </div>
            </form>
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>
      <NewAgentModal
        open={launchFor !== ""}
        title={launchFor === "judge" ? "Judge settings" : "New participant"}
        initialProject={originProject}
        onClose={() => setLaunchFor("")}
        onConfigure={(params) => {
          const launch: ThinkTankLaunch = { role: params.role, project: params.project, backend: params.backend, model: params.model, effort: params.effort, fast: params.fast, name: params.name };
          if (launchFor === "judge") setJudge(launch);
          else setRows((current) => [...current, { key: newCommandID(), launch, label: params.name || "New agent", project: params.project, limit: DEFAULT_LIMIT, mayLeave: true }]);
          setCommandID(newCommandID());
        }}
      />
    </>
  );
}
