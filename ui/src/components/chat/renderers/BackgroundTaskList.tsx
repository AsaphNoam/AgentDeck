import { useEffect, useState } from "react";
import { stopBackgroundTask } from "../../../api/client";
import type { BackgroundTask, TaskState } from "../runtimeActivity";

const STATE_LABEL: Record<TaskState, string> = {
  running: "Running",
  completed: "Completed",
  failed: "Failed",
  stopped: "Stopped",
};

// The compact Background tasks list at the transcript tail (FS-03.R59,
// TS-08.R59). It is open while any task runs and a disclosure once all have
// finished. Rows put state first; output stays in the related tool call.
export function BackgroundTaskList({ agentId, tasks, controllable }: { agentId: string; tasks: BackgroundTask[]; controllable: boolean }) {
  const anyRunning = tasks.some((task) => task.state === "running");
  const [chosen, setChosen] = useState<boolean | null>(null);
  const open = chosen ?? anyRunning;
  if (tasks.length === 0) return null;
  return (
    <section className="background-tasks" data-ui="runtime-activity" data-slot="task-list" data-state={anyRunning ? "active" : "completed"}>
      <button type="button" className="tool-toggle" aria-expanded={open} onClick={() => setChosen(!open)}>
        {open ? "▾" : "▸"} Background tasks ({tasks.length})
      </button>
      {open && (
        <ul className="background-task-rows">
          {tasks.map((task) => (
            <TaskRow key={task.taskId} agentId={agentId} task={task} controllable={controllable} />
          ))}
        </ul>
      )}
    </section>
  );
}

function TaskRow({ agentId, task, controllable }: { agentId: string; task: BackgroundTask; controllable: boolean }) {
  const [stopping, setStopping] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Accepted stop waits for the runtime's own terminal update (FS-03.R59).
  useEffect(() => {
    if (task.state !== "running") setStopping(false);
  }, [task.state]);
  const stop = () => {
    setError(null);
    setStopping(true);
    stopBackgroundTask(agentId, task.taskId, task.activityId).catch(() => {
      setStopping(false);
      setError("Could not stop this task. Try again.");
    });
  };
  const label = task.fenced ? "Ended with the previous session" : STATE_LABEL[task.state];
  return (
    <li className="background-task" data-slot="task" data-state={task.state}>
      <span className="background-task-state">{stopping ? "Stopping…" : label}</span>
      <span className="background-task-name">{task.name || task.toolTitle || "Command"}</span>
      {(task.owner || (task.toolTitle && task.toolTitle !== task.name)) && (
        <span className="background-task-meta">
          {[task.owner, task.toolTitle && task.toolTitle !== task.name ? `from ${task.toolTitle}` : ""].filter(Boolean).join(" · ")}
        </span>
      )}
      {controllable && task.state === "running" && task.canStop && !task.fenced && (
        <button type="button" className="background-task-stop" disabled={stopping} onClick={stop}>
          Stop
        </button>
      )}
      {error && <p className="background-task-error" role="alert">{error}</p>}
    </li>
  );
}
