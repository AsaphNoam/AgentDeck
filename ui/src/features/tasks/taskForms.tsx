import { useId, useMemo, useState } from "react";
import { AutoGrowTextarea, Button } from "../../components/ui";
import { useBackends, useConfig, useRoles } from "../../api/config";
import {
  useCreateTask,
  useFireSignal,
  useRearmTask,
  useRecordTaskResult,
  useTasks,
  TaskAPIError,
  type TaskArmInput,
} from "../../api/tasks";
import { usePipelineRuns } from "../../api/pipelines";
import type { PipelineRunSummary } from "../../schemas/pipeline";
import type { Task, TaskArm } from "../../schemas/task";
import { WORK_RESULT_OUTCOMES } from "../../schemas/task";
import { useAgentStore } from "../../store/agentStore";

type WorkSourceKind = "task" | "pipeline_run";
type WorkPrerequisiteSelection = { sourceKind: WorkSourceKind; sourceID: string; mode: "named" | "manual"; outcomes: string[] };
type TasksQuery = ReturnType<typeof useTasks>;
type RunsQuery = ReturnType<typeof usePipelineRuns>;

const WORK_OUTCOMES = WORK_RESULT_OUTCOMES;
const EMPTY_SELECTION: WorkPrerequisiteSelection = { sourceKind: "task", sourceID: "", mode: "named", outcomes: ["success"] };

export const errorMessage = (err: unknown) => err instanceof TaskAPIError ? err.message : "That did not work.";

function dependencyArms(selection: WorkPrerequisiteSelection, signal: string): TaskArmInput[] {
  const arms: TaskArmInput[] = [];
  if (selection.sourceID.trim()) {
    arms.push({
      kind: "work_result",
      source_kind: selection.sourceKind,
      source_id: selection.sourceID.trim(),
      satisfying_outcomes: [...selection.outcomes],
    });
  }
  if (signal.trim()) arms.push({ kind: "signal", signal_name: signal.trim() });
  return arms;
}

function prerequisiteError(selection: WorkPrerequisiteSelection): string {
  if (!selection.sourceID.trim()) return "";
  if (selection.outcomes.length === 0) return "Choose at least one outcome for this prerequisite.";
  const valid = new Set(WORK_OUTCOMES[selection.sourceKind].map(({ value }) => value));
  if (selection.outcomes.some((outcome) => !valid.has(outcome))) {
    return `Remove outcomes that are unavailable for ${selection.sourceKind === "task" ? "tasks" : "pipeline runs"}.`;
  }
  return "";
}

function sourceLabel(sourceKind: WorkSourceKind, sourceID: string, tasks: Task[], runs: PipelineRunSummary[]): string {
  if (sourceKind === "task") {
    const task = tasks.find((item) => item.task_id === sourceID);
    return task ? `${task.display_name} · ${task.state} · ${task.task_id}` : `Unavailable task · ${sourceID}`;
  }
  const run = runs.find((item) => item.run_id === sourceID);
  return run ? `${run.display_name || run.template_id} · ${run.state} · ${run.run_id}` : `Unavailable run · ${sourceID}`;
}

function armDescription(arm: TaskArm, tasks: Task[], runs: PipelineRunSummary[]): string {
  if (arm.kind === "signal") return `Signal: ${arm.signal_name}`;
  const kind = arm.source_kind === "pipeline_run" ? "pipeline_run" : "task";
  const outcomes = (arm.satisfying_outcomes ?? []).join(" or ") || "no outcomes recorded";
  return `${kind === "task" ? "Task" : "Pipeline run"}: ${sourceLabel(kind, arm.source_id, tasks, runs)} → ${outcomes} (${arm.state})`;
}

function PrerequisitePicker({
  project,
  selection,
  onChange,
  tasksQuery,
  runsQuery,
  omitTaskID,
  idPrefix,
}: {
  project: string;
  selection: WorkPrerequisiteSelection;
  onChange: (selection: WorkPrerequisiteSelection) => void;
  tasksQuery: TasksQuery;
  runsQuery: RunsQuery;
  omitTaskID?: string;
  idPrefix: string;
}) {
  const modeName = useId();
  const tasks = tasksQuery.data ?? [];
  const runs = useMemo(() => {
    const byID = new Map((runsQuery.data?.pages ?? []).flatMap((page) => page.runs).map((run) => [run.run_id, run]));
    return [...byID.values()].filter((run) => run.project === project);
  }, [project, runsQuery.data]);
  const namedTasks = selection.sourceKind === "task" ? tasks.filter((task) => task.project === project && task.task_id !== omitTaskID) : [];
  const namedRuns = selection.sourceKind === "pipeline_run" ? runs : [];
  const namedSourceCount = selection.sourceKind === "task" ? namedTasks.length : namedRuns.length;
  const knownSelection = selection.sourceKind === "task"
    ? tasks.some((task) => task.task_id === selection.sourceID)
    : runs.some((run) => run.run_id === selection.sourceID);
  const validOutcomes = WORK_OUTCOMES[selection.sourceKind];
  const selectedUnavailableOutcomes = selection.outcomes.filter((value) => !validOutcomes.some((outcome) => outcome.value === value));
  const manualID = `${idPrefix}-manual-source`;
  const rawID = `${idPrefix}-source-id`;
  const outcomeError = prerequisiteError(selection);

  return (
    <section className="task-prerequisite">
      <div className="task-prerequisite-heading">
        <strong>Wait for</strong>
        <span>Choose one task or pipeline run. Success is selected by default.</span>
      </div>
      <label>
        Source type
        <select
          value={selection.sourceKind}
          onChange={(event) => onChange({ ...selection, sourceKind: event.target.value as WorkSourceKind })}
        >
          <option value="task">Task</option>
          <option value="pipeline_run">Pipeline run</option>
        </select>
      </label>
      <label>
        Named source
        <select
          aria-label="Named prerequisite"
          value={selection.mode === "named" ? selection.sourceID : ""}
          onChange={(event) => onChange({ ...selection, mode: "named", sourceID: event.target.value })}
          disabled={selection.mode !== "named"}
        >
          <option value="">Choose a {selection.sourceKind === "task" ? "task" : "pipeline run"}</option>
          {!knownSelection && selection.sourceID && <option value={selection.sourceID}>{sourceLabel(selection.sourceKind, selection.sourceID, tasks, runs)}</option>}
          {namedTasks.map((task) => <option key={task.task_id} value={task.task_id}>{task.display_name} · {task.state} · {task.task_id}</option>)}
          {namedRuns.map((run) => <option key={run.run_id} value={run.run_id}>{run.display_name || run.template_id} · {run.state} · {run.run_id}</option>)}
        </select>
      </label>
      {selection.sourceKind === "task" ? (
        tasksQuery.isLoading ? <p className="task-query-state">Loading tasks…</p>
          : tasksQuery.isError ? <p className="form-error" role="alert">Tasks could not be loaded. The current selection is kept.</p>
            : namedSourceCount === 0 ? <p className="task-query-state">No other tasks in this project yet.</p> : null
      ) : (
        <div className="task-run-choices" aria-live="polite">
          {runsQuery.isLoading && <p className="task-query-state">Loading pipeline runs…</p>}
          {runsQuery.isError && <p className="form-error" role="alert">Pipeline runs could not be loaded. The current selection is kept.</p>}
          {!runsQuery.isLoading && !runsQuery.isError && runs.length === 0 && runsQuery.hasNextPage && <p className="task-query-state">No matching runs in the loaded pages yet.</p>}
          {!runsQuery.isLoading && !runsQuery.isError && runs.length === 0 && !runsQuery.hasNextPage && <p className="task-query-state">No pipeline runs in this project.</p>}
          {runs.length > 0 && <p className="task-query-state">Showing {runs.length} loaded run{runs.length === 1 ? "" : "s"} for this project.</p>}
          {runsQuery.hasNextPage && <Button size="small" type="button" disabled={runsQuery.isFetchingNextPage} onClick={() => void runsQuery.fetchNextPage()}>{runsQuery.isFetchingNextPage ? "Loading runs…" : "Load more runs"}</Button>}
          {runsQuery.isFetchNextPageError && <p className="form-error" role="alert">More runs could not be loaded. The current selection is kept.</p>}
        </div>
      )}
      {selection.sourceID && (
        <fieldset className="task-outcome-choices">
          <legend>Satisfying outcomes</legend>
          {[...selectedUnavailableOutcomes.map((value) => ({ value, label: `${value} (unavailable for ${selection.sourceKind === "task" ? "tasks" : "pipeline runs"})` })), ...validOutcomes].map(({ value, label }) => (
            <label key={value}>
              <input
                type="checkbox"
                checked={selection.outcomes.includes(value)}
                onChange={(event) => onChange({
                  ...selection,
                  outcomes: event.target.checked ? [...selection.outcomes, value] : selection.outcomes.filter((item) => item !== value),
                })}
              />
              {label}
            </label>
          ))}
          {outcomeError && <p className="form-error" role="alert">{outcomeError}</p>}
        </fieldset>
      )}
      <details className="task-advanced">
        <summary>Advanced</summary>
        <div className="task-advanced-content">
          <fieldset>
            <legend>Source entry</legend>
            <label>
              <input type="radio" name={modeName} checked={selection.mode === "named"} onChange={() => onChange({ ...selection, mode: "named" })} />
              Choose a named source
            </label>
            <label htmlFor={manualID}>
              <input id={manualID} type="radio" name={modeName} checked={selection.mode === "manual"} onChange={() => onChange({ ...selection, mode: "manual" })} />
              Enter an ID manually
            </label>
            {selection.mode === "manual" && <label htmlFor={rawID}>Source ID<input id={rawID} value={selection.sourceID} onChange={(event) => onChange({ ...selection, sourceID: event.target.value })} placeholder={selection.sourceKind === "task" ? "tk_…" : "pr_…"} /></label>}
          </fieldset>
        </div>
      </details>
    </section>
  );
}

/** RearmForm replaces a task's whole arm set, which is the only repair for work
 *  parked by a prerequisite that can never be satisfied (FS-16.R23). */
export function RearmForm({ task, onError }: { task: Task; onError: (message: string) => void }) {
  const project = task.project;
  const rearm = useRearmTask(project);
  const tasksQuery = useTasks(project);
  const runsQuery = usePipelineRuns();
  const [selection, setSelection] = useState<WorkPrerequisiteSelection>(EMPTY_SELECTION);
  const [signal, setSignal] = useState("");
  const [validationError, setValidationError] = useState("");
  const tasks = tasksQuery.data ?? [];
  const runs = (runsQuery.data?.pages ?? []).flatMap((page) => page.runs);
  const currentArms = task.arms ?? [];

  return (
    <form
      className="task-rearm"
      onSubmit={(event) => {
        event.preventDefault();
        const error = prerequisiteError(selection);
        if (error) {
          setValidationError(error);
          return;
        }
        setValidationError("");
        onError("");
        rearm
          .mutateAsync({ taskID: task.task_id, arms: dependencyArms(selection, signal) })
          .then(() => {
            setSelection(EMPTY_SELECTION);
            setSignal("");
          })
          .catch((err: unknown) => onError(errorMessage(err)));
      }}
    >
      <div className="task-rearm-replacement">
        <strong>Current waits</strong>
        {currentArms.length === 0 ? <p>None.</p> : <ul>{currentArms.map((arm) => <li key={arm.arm_id}>{armDescription(arm, tasks, runs)}</li>)}</ul>}
        <p>Re-arm replaces this entire wait set. Any wait not listed below will be removed. An empty replacement removes all waits.</p>
        <strong>Proposed replacement</strong>
        {selection.sourceID.trim() || signal.trim() ? (
          <ul>
            {selection.sourceID.trim() && <li>{sourceLabel(selection.sourceKind, selection.sourceID.trim(), tasks, runs)} → {selection.outcomes.join(" or ") || "no outcomes selected"}</li>}
            {signal.trim() && <li>Signal: {signal.trim()}</li>}
          </ul>
        ) : <p>None — this removes all waits.</p>}
      </div>
      <PrerequisitePicker project={project} selection={selection} onChange={setSelection} tasksQuery={tasksQuery} runsQuery={runsQuery} omitTaskID={task.task_id} idPrefix={`rearm-${task.task_id}`} />
      <details className="task-advanced">
        <summary>Advanced signals</summary>
        <label>Wait for signal<input value={signal} onChange={(event) => setSignal(event.target.value)} placeholder="ci-green" /></label>
      </details>
      {validationError && <p className="form-error" role="alert">{validationError}</p>}
      <Button size="small" type="submit" busy={rearm.isPending}>Re-arm</Button>
    </form>
  );
}

/** RecordResultForm is the person's counterpart to an agent's report (FS-16.R22). */
export function RecordResultForm({ task, onError }: { task: Task; onError: (message: string) => void }) {
  const record = useRecordTaskResult(task.project);
  const [outcome, setOutcome] = useState("success");
  const [summary, setSummary] = useState("");
  const [details, setDetails] = useState("");
  return (
    <form
      className="task-record"
      onSubmit={(event) => {
        event.preventDefault();
        onError("");
        record.mutateAsync({ taskID: task.task_id, outcome, summary, details }).catch((err: unknown) => onError(errorMessage(err)));
      }}
    >
      <label>Result<select aria-label="Result outcome" value={outcome} onChange={(e) => setOutcome(e.target.value)}><option value="success">Success</option><option value="failure">Failure</option><option value="blocked">Blocked</option></select></label>
      <label>Summary<input aria-label="Result summary" value={summary} onChange={(e) => setSummary(e.target.value)} required /></label>
      <label>Details<AutoGrowTextarea aria-label="Result details" value={details} onChange={(e) => setDetails(e.target.value)} /></label>
      <Button size="small" type="submit" busy={record.isPending}>Record result</Button>
    </form>
  );
}

/** CreateTaskForm owns its concrete project. Changing the project clears only the
 *  dependency choice that belonged to the old project (FS-16.R40/R45). */
export function CreateTaskForm({ projects, initialProject }: { projects: string[]; initialProject: string }) {
  const [project, setProject] = useState(initialProject);
  const create = useCreateTask(project || undefined);
  const tasksQuery = useTasks(project || undefined);
  const runsQuery = usePipelineRuns();
  const { data: roles } = useRoles();
  const { data: config } = useConfig();
  const { data: backends } = useBackends();
  const [name, setName] = useState("");
  const [instruction, setInstruction] = useState("");
  const [role, setRole] = useState("");
  const [signal, setSignal] = useState("");
  const [targetKind, setTargetKind] = useState<"launch" | "agent">("launch");
  const [targetAgentID, setTargetAgentID] = useState("");
  const [backend, setBackend] = useState("");
  const [model, setModel] = useState("");
  const [effort, setEffort] = useState("");
  const [fast, setFast] = useState(false);
  const [selection, setSelection] = useState<WorkPrerequisiteSelection>(EMPTY_SELECTION);
  const [contextRefID, setContextRefID] = useState("");
  const [contextLabel, setContextLabel] = useState("");
  const [contextDescription, setContextDescription] = useState("");
  const agents = useAgentStore((state) => state.agents);
  const [error, setError] = useState("");
  const [validationError, setValidationError] = useState("");
  const [created, setCreated] = useState("");

  const roleNames = Object.keys(roles ?? {});
  const configuredRole = config?.default_role && roleNames.includes(config.default_role) ? config.default_role : "";
  const chosenRole = role || configuredRole || roleNames[0] || "";
  const defaultBackendID = Object.entries(backends?.backends ?? {}).find(([, item]) => item.default)?.[0] ?? "";
  const effectiveBackend = backends?.backends[backend || defaultBackendID];
  const effectiveModel = effectiveBackend?.models[model || effectiveBackend.default_model];

  return (
    <form
      className="task-create"
      data-slot="create"
      onSubmit={(event) => {
        event.preventDefault();
        setError("");
        setCreated("");
        const dependencyError = prerequisiteError(selection);
        if (dependencyError) {
          setValidationError(dependencyError);
          return;
        }
        setValidationError("");
        create
          .mutateAsync({
            project,
            display_name: name,
            instruction,
            target_kind: targetKind,
            target_agent_id: targetKind === "agent" ? targetAgentID : undefined,
            role: targetKind === "launch" ? chosenRole : undefined,
            backend: targetKind === "launch" ? backend : undefined,
            model: targetKind === "launch" ? model : undefined,
            effort: targetKind === "launch" ? effort : undefined,
            fast: targetKind === "launch" ? fast : undefined,
            arms: dependencyArms(selection, signal),
            attachments: contextRefID.trim() ? [{ context_ref_id: contextRefID.trim(), label: contextLabel, description: contextDescription }] : [],
          })
          .then(() => {
            setCreated(`Created “${name}” in ${project}.`);
            setName("");
            setInstruction("");
            setSignal("");
          })
          .catch((err: unknown) => setError(errorMessage(err)));
      }}
    >
      <label>
        Project
        <select
          value={project}
          required
          onChange={(e) => {
            setProject(e.target.value);
            setSelection(EMPTY_SELECTION);
            setTargetAgentID("");
          }}
        >
          <option value="">Choose a project</option>
          {projects.map((item) => <option key={item} value={item}>{item}</option>)}
        </select>
      </label>
      <label>
        Name
        <input value={name} onChange={(e) => setName(e.target.value)} required />
      </label>
      <label>
        Instruction
        <AutoGrowTextarea value={instruction} onChange={(e) => setInstruction(e.target.value)} required />
      </label>
      <label>Target<select value={targetKind} onChange={(e) => setTargetKind(e.target.value as "launch" | "agent")}><option value="launch">Launch a new agent</option><option value="agent">Use an existing agent</option></select></label>
      {targetKind === "agent" ? <label>Existing agent<select value={targetAgentID} onChange={(e) => setTargetAgentID(e.target.value)} required><option value="">Choose an agent</option>{Object.values(agents).filter((agent) => agent.project === project && agent.interface === "chat" && !agent.archived).map((agent) => <option key={agent.agent_id} value={agent.agent_id}>{agent.name}</option>)}</select></label> : <>
        <label>
          Role to launch
          <select value={chosenRole} onChange={(e) => setRole(e.target.value)}>
            {roleNames.map((item) => <option key={item} value={item}>{item}</option>)}
          </select>
        </label>
        <label>Backend (optional)<input value={backend} onChange={(e) => { setBackend(e.target.value); setFast(false); }} /></label>
        <label>Model (optional)<input value={model} onChange={(e) => { setModel(e.target.value); setFast(false); }} /></label>
        <label>Effort (optional)<input value={effort} onChange={(e) => setEffort(e.target.value)} placeholder="a level the model declares" /></label>
        {effectiveModel?.fast && <label><input type="checkbox" checked={fast} onChange={(e) => setFast(e.target.checked)} /> Fast mode — higher provider usage</label>}
      </>}
      {project && <PrerequisitePicker project={project} selection={selection} onChange={setSelection} tasksQuery={tasksQuery} runsQuery={runsQuery} idPrefix="create" />}
      <details className="task-advanced">
        <summary>Advanced signal and context</summary>
        <div className="task-advanced-content">
          <label>Wait for signal (optional)<input value={signal} onChange={(e) => setSignal(e.target.value)} placeholder="ci-green" /></label>
          <label>Context reference ID<input value={contextRefID} onChange={(e) => setContextRefID(e.target.value)} placeholder="cx_…" /></label>
          <label>Context label<input value={contextLabel} onChange={(e) => setContextLabel(e.target.value)} /></label>
          <label>Context description<AutoGrowTextarea value={contextDescription} onChange={(e) => setContextDescription(e.target.value)} /></label>
        </div>
      </details>
      {validationError && <p className="form-error" role="alert">{validationError}</p>}
      <Button type="submit" variant="primary" busy={create.isPending}>Create task</Button>
      {error && <p className="form-error" role="alert">{error}</p>}
      {created && <p className="task-query-state" role="status">{created}</p>}
    </form>
  );
}

/** FireSignalForm releases a named signal across one concrete project. */
export function FireSignalForm({ projects, project, name, onChange }: {
  projects: string[];
  project: string;
  name: string;
  onChange: (draft: { project: string; name: string }) => void;
}) {
  const fire = useFireSignal(project || undefined);
  const [error, setError] = useState("");
  const [released, setReleased] = useState("");
  return (
    <form
      className="task-signal"
      data-slot="signal"
      onSubmit={(event) => {
        event.preventDefault();
        setError("");
        setReleased("");
        fire.mutateAsync(name.trim())
          .then((result) => {
            setReleased(`Fired ${name.trim()} in ${project}; ${result.released} wait${result.released === 1 ? "" : "s"} released.`);
            onChange({ project, name: "" });
          })
          .catch((err: unknown) => setError(errorMessage(err)));
      }}
    >
      <label>
        Signal project
        <select value={project} required onChange={(e) => onChange({ project: e.target.value, name })}>
          <option value="">Choose a project</option>
          {projects.map((item) => <option key={item} value={item}>{item}</option>)}
        </select>
      </label>
      <label>
        Signal name
        <input value={name} onChange={(e) => onChange({ project, name: e.target.value })} required />
      </label>
      <Button type="submit" busy={fire.isPending}>Fire</Button>
      {error && <p className="form-error" role="alert">{error}</p>}
      {released && <p className="task-query-state" role="status">{released}</p>}
    </form>
  );
}
