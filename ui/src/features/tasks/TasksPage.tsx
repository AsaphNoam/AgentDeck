import { useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { Badge, Button, PageHeader, Surface } from "../../components/ui";
import { useProjects, useRoles } from "../../api/config";
import {
  useCancelTask,
  useCreateTask,
  useDeleteTask,
  useFireSignal,
  useRearmTask,
  useRecordTaskResult,
  useRetryTask,
  useTasks,
  TaskAPIError,
  type TaskArmInput,
} from "../../api/tasks";
import type { Task, TaskArm } from "../../schemas/task";
import { TASK_ATTENTION_STATES } from "../../schemas/task";

/** needsAttention is the one definition the page and the dashboard count share:
 *  parked work and work whose agent went away without a result (FS-02.R44). */
export function needsAttention(task: Pick<Task, "state">): boolean {
  return (TASK_ATTENTION_STATES as readonly string[]).includes(task.state);
}

const STATE_TONE: Record<string, "info" | "success" | "warning" | "danger" | "neutral"> = {
  armed: "neutral",
  ready: "info",
  starting: "info",
  running: "info",
  interrupted: "warning",
  dependency_failed: "danger",
  finished: "success",
};

/** waitingOn says what an armed task is still waiting for, which is the question
 *  the Tasks view exists to answer (FS-16.R14). */
export function waitingOn(arms: TaskArm[]): string[] {
  return arms
    .filter((arm) => arm.state === "unsatisfied")
    .map((arm) =>
      arm.kind === "signal"
        ? `signal ${arm.signal_name}`
        : `${arm.source_kind === "pipeline_run" ? "run" : "task"} ${arm.source_id} → ${(arm.satisfying_outcomes ?? []).join(" or ")}`,
    );
}

/** RearmForm replaces a task's whole arm set, which is the only repair for work
 *  parked by a prerequisite that can never be satisfied (FS-16.R23). */
function RearmForm({ task, project, onError }: { task: Task; project: string; onError: (message: string) => void }) {
  const rearm = useRearmTask(project);
  const [sourceID, setSourceID] = useState("");
  const [signal, setSignal] = useState("");

  return (
    <form
      className="task-rearm"
      onSubmit={(event) => {
        event.preventDefault();
        const arms: TaskArmInput[] = [];
        if (sourceID.trim()) {
          arms.push({
            kind: "work_result", source_kind: "task", source_id: sourceID.trim(),
            satisfying_outcomes: ["success"],
          });
        }
        if (signal.trim()) arms.push({ kind: "signal", signal_name: signal.trim() });
        rearm
          .mutateAsync({ taskID: task.task_id, arms })
          .then(() => {
            setSourceID("");
            setSignal("");
          })
          .catch((err: unknown) => onError(err instanceof TaskAPIError ? err.message : "That did not work."));
      }}
    >
      <label>
        Wait for task
        <input value={sourceID} onChange={(e) => setSourceID(e.target.value)} placeholder="tk_…" />
      </label>
      <label>
        Wait for signal
        <input value={signal} onChange={(e) => setSignal(e.target.value)} placeholder="ci-green" />
      </label>
      <Button size="small" type="submit" busy={rearm.isPending}>Re-arm</Button>
    </form>
  );
}

function TaskRow({ task, project }: { task: Task; project: string }) {
  const cancel = useCancelTask(project);
  const retry = useRetryTask(project);
  const remove = useDeleteTask(project);
  const record = useRecordTaskResult(project);
  const [error, setError] = useState("");
  const repairable = task.state === "armed" || task.state === "ready" || task.state === "dependency_failed";

  const arms = task.arms ?? [];
  const waiting = waitingOn(arms);
  const act = (run: () => Promise<unknown>) => {
    setError("");
    run().catch((err: unknown) => {
      setError(err instanceof TaskAPIError ? err.message : "That did not work.");
    });
  };

  return (
    <li className="task-row" data-slot="task" data-state={task.state}>
      <div className="task-row-top" data-slot="metadata">
        <span className="task-name">{task.display_name}</span>
        <Badge className="task-state" variant={STATE_TONE[task.state] ?? "neutral"} indicator>
          {task.state.replace("_", " ")}
        </Badge>
        {task.outcome && (
          <Badge className="task-outcome" variant={task.outcome === "success" ? "success" : "neutral"}>
            {task.outcome}{task.outcome_source ? ` · by ${task.outcome_source}` : ""}
          </Badge>
        )}
      </div>
      <p className="task-instruction" data-slot="instruction">{task.instruction}</p>
      {waiting.length > 0 && (
        <p className="task-waiting" data-slot="waiting">Waiting on: {waiting.join("; ")}</p>
      )}
      {task.attention_reason && (
        <p className="task-attention" data-slot="attention">{task.attention_reason}</p>
      )}
      <div className="task-row-meta" data-slot="metadata">
        <span>{task.target_kind === "agent" ? `assigned to ${task.assigned_agent_id || task.target_agent_id}` : `launches ${task.role}`}</span>
        <span>created by {task.created_by_kind}</span>
      </div>
      <div className="task-row-actions" data-slot="actions">
        {task.state !== "finished" && (
          <Button size="small" onClick={() => act(() => cancel.mutateAsync(task.task_id))}>Cancel</Button>
        )}
        {(task.state === "interrupted" || task.state === "dependency_failed") && (
          <Button size="small" onClick={() => act(() => retry.mutateAsync(task.task_id))}>Retry</Button>
        )}
        {(task.state === "running" || task.state === "interrupted") && (
          <Button
            size="small"
            onClick={() => act(() => record.mutateAsync({
              taskID: task.task_id, outcome: "blocked", summary: "recorded by a person",
            }))}
          >
            Record blocked
          </Button>
        )}
        <Button size="small" variant="ghost" onClick={() => act(() => remove.mutateAsync(task.task_id))}>Delete</Button>
      </div>
      {repairable && <RearmForm task={task} project={project} onError={setError} />}
      {error && <p className="form-error" role="alert">{error}</p>}
    </li>
  );
}

function CreateTaskForm({ project }: { project: string }) {
  const create = useCreateTask(project);
  const { data: roles } = useRoles();
  const [name, setName] = useState("");
  const [instruction, setInstruction] = useState("");
  const [role, setRole] = useState("");
  const [signal, setSignal] = useState("");
  const [error, setError] = useState("");

  const roleNames = Object.keys(roles ?? {});
  const chosenRole = role || roleNames[0] || "";

  return (
    <Surface className="task-create" data-slot="create">
      <h2>New task</h2>
      <form
        onSubmit={(event) => {
          event.preventDefault();
          setError("");
          create
            .mutateAsync({
              project,
              display_name: name,
              instruction,
              target_kind: "launch",
              role: chosenRole,
              arms: signal.trim() ? [{ kind: "signal", signal_name: signal.trim() }] : [],
            })
            .then(() => {
              setName("");
              setInstruction("");
              setSignal("");
            })
            .catch((err: unknown) => setError(err instanceof TaskAPIError ? err.message : "That did not work."));
        }}
      >
        <label>
          Name
          <input value={name} onChange={(e) => setName(e.target.value)} required />
        </label>
        <label>
          Instruction
          <textarea value={instruction} onChange={(e) => setInstruction(e.target.value)} required />
        </label>
        <label>
          Role to launch
          <select value={chosenRole} onChange={(e) => setRole(e.target.value)}>
            {roleNames.map((name) => <option key={name} value={name}>{name}</option>)}
          </select>
        </label>
        <label>
          Wait for signal (optional)
          <input value={signal} onChange={(e) => setSignal(e.target.value)} placeholder="ci-green" />
        </label>
        <Button type="submit" variant="primary" busy={create.isPending}>Create task</Button>
        {error && <p className="form-error" role="alert">{error}</p>}
      </form>
    </Surface>
  );
}

function FireSignalForm({ project }: { project: string }) {
  const fire = useFireSignal(project);
  const [name, setName] = useState("");
  return (
    <Surface className="task-signal" data-slot="signal">
      <h2>Fire a signal</h2>
      <form
        onSubmit={(event) => {
          event.preventDefault();
          fire.mutateAsync(name.trim()).then(() => setName("")).catch(() => undefined);
        }}
      >
        <label>
          Signal name
          <input value={name} onChange={(e) => setName(e.target.value)} required />
        </label>
        <Button type="submit" busy={fire.isPending}>Fire</Button>
      </form>
    </Surface>
  );
}

export function TasksPage() {
  const [search, setSearch] = useSearchParams();
  const { data: projects } = useProjects();
  const projectNames = useMemo(() => Object.keys(projects ?? {}), [projects]);
  const project = search.get("project") || projectNames[0] || "";
  const { data: tasks, isLoading } = useTasks(project || undefined);
  const attention = (tasks ?? []).filter(needsAttention).length;

  return (
    <div className="tasks-page" data-ui="tasks">
      <PageHeader
        eyebrow="Dependent work"
        title="Tasks"
        description="Work that waits for other work and starts itself. Nothing here polls or asks another agent whether it is done."
      />
      <div className="tasks-toolbar" data-slot="toolbar">
        <label>
          Project
          <select value={project} onChange={(e) => setSearch({ project: e.target.value })}>
            {projectNames.map((name) => <option key={name} value={name}>{name}</option>)}
          </select>
        </label>
        <span className="tasks-attention-count" data-slot="attention">{attention} need attention</span>
      </div>
      <div className="tasks-authoring-grid">
        <CreateTaskForm project={project} />
        <FireSignalForm project={project} />
      </div>
      {isLoading ? (
        <p className="tasks-empty">Loading…</p>
      ) : (tasks ?? []).length === 0 ? (
        <p className="tasks-empty">No dependent work in this project yet.</p>
      ) : (
        <ul className="task-list" data-slot="list">
          {(tasks ?? []).map((task) => <TaskRow key={task.task_id} task={task} project={project} />)}
        </ul>
      )}
    </div>
  );
}
