import { useMemo, useState, type ReactNode } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { useQueries } from "@tanstack/react-query";
import { Badge, Button, PageHeader } from "../../components/ui";
import { useProjects } from "../../api/config";
import { limitConcurrency, taskListOptions, useCancelTask, useDeleteTask, useRetryTask } from "../../api/tasks";
import { usePipelineRun } from "../../api/pipelines";
import type { Task } from "../../schemas/task";
import { taskActions } from "../../schemas/task";
import { useAgentStore } from "../../store/agentStore";
import { CreateTaskForm, FireSignalForm, RearmForm, RecordResultForm, errorMessage } from "./taskForms";
import { needsAttention, projectWork, taskReason, taskStatus, type ProjectWork, type TaskLink, type WorkGroup, type WorkRow } from "./taskWork";

export { needsAttention };

/** Project task reads this page may have in flight at once (TS-08.R82). */
const projectReads = limitConcurrency(4);

const ALL = "";

function plural(count: number, word: string) {
  return `${count} ${word}${count === 1 ? "" : "s"}`;
}

/** AgentLabel names an agent through the live identity store, falling back to its
 *  stable id; only a live, unarchived agent gets a chat link (FS-16.R44). */
function AgentLabel({ id }: { id: string }) {
  const agent = useAgentStore((state) => state.agents[id]);
  if (!agent) return <span className="task-agent-unavailable">{id} (unavailable)</span>;
  if (agent.archived) return <span>{agent.name} <Badge>archived</Badge></span>;
  return <Link to={`/agent/${encodeURIComponent(id)}`}>{agent.name}</Link>;
}

function Creator({ task }: { task: Task }) {
  if (task.created_by_kind === "agent" && task.created_by_agent_id) return <AgentLabel id={task.created_by_agent_id} />;
  return <span>{task.created_by_kind === "person" ? "a person" : task.created_by_kind}</span>;
}

function assigneeID(task: Task) {
  return task.assigned_agent_id || (task.target_kind === "agent" ? task.target_agent_id : "");
}

function linkText(link: TaskLink): ReactNode {
  const name = link.source ? link.source.display_name : <span className="task-agent-unavailable">unavailable task {link.sourceID}</span>;
  if (link.kind === "delegation") return <>delegated by {name}</>;
  const mark = link.state === "satisfied" ? "met" : link.state === "unsatisfiable" ? "cannot be met" : "pending";
  return <>after {name} → {link.outcomes.join(" or ")} ({mark})</>;
}

function TaskRowView({ row, open, onToggle, onSignal }: { row: WorkRow; open: boolean; onToggle: () => void; onSignal: (project: string, name: string) => void }) {
  const { task } = row;
  const status = taskStatus(task);
  const reason = taskReason(row);
  const assigned = assigneeID(task);
  const detailID = `task-detail-${task.task_id}`;
  return (
    <li className={`task-row task-depth-${row.depth}`} data-slot="task" data-state={task.state}>
      <button type="button" className="task-row-summary" aria-expanded={open} aria-controls={detailID} onClick={onToggle}>
        <span className="task-row-top" data-slot="metadata">
          <span className="task-name">{task.display_name}</span>
          <Badge className="task-state" variant={status.tone} indicator>{status.label}</Badge>
        </span>
        {reason && <span className={needsAttention(task) ? "task-attention" : "task-waiting"} data-slot={needsAttention(task) ? "attention" : "waiting"}>{reason}</span>}
      </button>
      {(row.links.length > 0 || row.next.length > 0) && (
        <p className="task-links" data-slot="links">
          {row.links.map((link, index) => <span key={`${link.kind}:${link.sourceID}:${index}`} data-variant={link.kind}>{linkText(link)}</span>)}
          {row.next.length > 0 && <span data-variant="next">leads to {row.next.map((item) => item.display_name).join(", ")}</span>}
        </p>
      )}
      <p className="task-row-meta" data-slot="metadata">
        <span>created by <Creator task={task} /></span>
        {assigned ? <span>assigned to <AgentLabel id={assigned} /></span> : task.target_kind === "launch" ? <span>launches {task.role}</span> : null}
      </p>
      {open && <TaskDetail id={detailID} row={row} onSignal={onSignal} />}
    </li>
  );
}

/** StageOwnership withholds controls R36 forbids on an authoritative stage task;
 *  run lineage alone does not make a task the stage (TS-08.R83). */
function useStageOwnership(task: Task): { owned: boolean; known: boolean; failed: boolean; runID: string } {
  const runID = task.lineage.pipeline_run_id;
  const run = usePipelineRun(runID || null);
  if (!runID) return { owned: false, known: true, failed: false, runID };
  if (!run.data) return { owned: false, known: false, failed: run.isError, runID };
  return { owned: run.data.stage_tasks.some((stage) => stage.task_id === task.task_id), known: true, failed: false, runID };
}

function TaskDetail({ id, row, onSignal }: { id: string; row: WorkRow; onSignal: (project: string, name: string) => void }) {
  const { task } = row;
  const cancel = useCancelTask(task.project);
  const retry = useRetryTask(task.project);
  const remove = useDeleteTask(task.project);
  const [error, setError] = useState("");
  const stage = useStageOwnership(task);
  const actions = taskActions(task);
  const restricted = stage.owned || !stage.known;
  const act = (run: () => Promise<unknown>) => {
    setError("");
    run().catch((err: unknown) => setError(errorMessage(err)));
  };
  const outputs = Object.entries(task.outputs);

  return (
    <div className="task-detail" id={id} data-slot="detail">
      <p className="task-instruction" data-slot="instruction">{task.instruction}</p>
      {(row.links.length > 0 || row.external.length > 0) && (
        <div className="task-detail-section">
          <strong>Prerequisites and lineage</strong>
          <ul>
            {row.links.map((link, index) => <li key={`l${index}`}>{linkText(link)}</li>)}
            {row.external.map((wait, index) => wait.kind === "signal" ? (
              <li key={`e${index}`}>
                signal {wait.name} ({wait.state === "satisfied" ? "fired" : "waiting"})
                {wait.state === "unsatisfied" && <Button size="small" variant="ghost" onClick={() => onSignal(task.project, wait.name)}>Go to signal control</Button>}
              </li>
            ) : (
              <li key={`e${index}`}>pipeline run <Link to={`/pipelines/runs/${encodeURIComponent(wait.runID)}`}>{wait.runID}</Link> → {wait.outcomes.join(" or ")} ({wait.state})</li>
            ))}
          </ul>
        </div>
      )}
      {task.outcome && (
        <div className="task-detail-section">
          <strong>Result: {task.outcome}{task.outcome_source ? ` · recorded by ${task.outcome_source}` : ""}</strong>
          {task.outcome_summary && <p>{task.outcome_summary}</p>}
          {task.outcome_details && <p className="task-outcome-details">{task.outcome_details}</p>}
          {outputs.length > 0 && <dl className="task-outputs">{outputs.map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{value}</dd></div>)}</dl>}
        </div>
      )}
      {(task.cleanup_phase || task.cleanup_last_error) && (
        <p className="task-waiting">Cleanup {task.cleanup_phase || "pending"}{task.cleanup_failure_count > 0 ? ` · ${plural(task.cleanup_failure_count, "failed attempt")}` : ""}{task.cleanup_unsafe ? " · needs repair from the run or agent" : " · retries automatically"}</p>
      )}
      {stage.runID && (
        <p className="task-row-meta">
          <span>Pipeline run <Link to={`/pipelines/runs/${encodeURIComponent(stage.runID)}`}>{stage.runID}</Link>{task.lineage.pipeline_stage_id ? ` · stage ${task.lineage.pipeline_stage_id}` : ""}</span>
          {stage.owned && <span>Stage work: record, re-arm and delete from its run.</span>}
          {!stage.known && (stage.failed
            ? <span>Stage ownership could not be loaded; record, re-arm and delete are withheld. Use the run.</span>
            : <span>Checking stage ownership; record, re-arm and delete are withheld until it loads.</span>)}
        </p>
      )}
      <div className="task-row-actions" data-slot="actions">
        {actions.cancel && <Button size="small" onClick={() => act(() => cancel.mutateAsync(task.task_id))}>Cancel</Button>}
        {actions.retry && <Button size="small" onClick={() => act(() => retry.mutateAsync(task.task_id))}>Retry</Button>}
        {!restricted && <Button size="small" variant="ghost" onClick={() => act(() => remove.mutateAsync(task.task_id))}>Delete</Button>}
      </div>
      {actions.recordResult && !restricted && <RecordResultForm task={task} onError={setError} />}
      {actions.rearm && !restricted && <RearmForm task={task} onError={setError} />}
      {error && <p className="form-error" role="alert">{error}</p>}
    </div>
  );
}

function GroupView({ group, open, onToggle, onSignal }: { group: WorkGroup; open: Set<string>; onToggle: (id: string) => void; onSignal: (project: string, name: string) => void }) {
  return (
    <ol className="task-group" data-slot="group" aria-label={group.rows.length > 1 ? `${group.rows.length} related tasks` : undefined}>
      {group.rows.map((row) => (
        <TaskRowView key={row.task.task_id} row={row} open={open.has(row.task.task_id)} onToggle={() => onToggle(row.task.task_id)} onSignal={onSignal} />
      ))}
    </ol>
  );
}

function ProjectSection({ project, title, query, focused, open, onToggle, onSignal }: {
  project: string;
  title: string;
  query: { data?: Task[]; isLoading: boolean; isError: boolean; error: unknown; refetch: () => unknown };
  focused: boolean;
  open: Set<string>;
  onToggle: (id: string) => void;
  onSignal: (project: string, name: string) => void;
}) {
  const work: ProjectWork | null = useMemo(
    () => query.data ? projectWork(project, query.data, new Set(query.data.filter((task) => open.has(task.task_id)).map((task) => task.task_id))) : null,
    [project, query.data, open],
  );
  const headingID = `tasks-project-${project}`;
  if (work && query.data!.length === 0 && !focused && !query.isError) return null;

  return (
    <section className="tasks-project" data-slot="project" aria-labelledby={headingID}>
      <header className="tasks-project-header">
        <h2 id={headingID}>{title}</h2>
        {work && (
          <span className="tasks-project-counts">
            {plural(work.unfinished, "unfinished task")}
            {work.attention > 0 && <> · <span className="tasks-attention-count" data-slot="attention">{work.attention} need{work.attention === 1 ? "s" : ""} attention</span></>}
          </span>
        )}
      </header>
      {query.isLoading && <p className="tasks-empty">Loading tasks…</p>}
      {query.isError && (
        <p className="form-error" role="alert">
          {work ? "Refreshing failed; showing the last loaded tasks, which may be stale. " : `Tasks for ${title} could not be loaded. `}
          <Button size="small" variant="ghost" onClick={() => void query.refetch()}>Try again</Button>
        </p>
      )}
      {work && query.data!.length === 0 && <p className="tasks-empty">No tasks in this project. Tasks agents create for dependent work appear here.</p>}
      {work && work.active.length > 0 && (
        <div className="tasks-groups" data-slot="list">
          {work.active.map((group) => <GroupView key={group.id} group={group} open={open} onToggle={onToggle} onSignal={onSignal} />)}
        </div>
      )}
      {work && work.history.length > 0 && (
        <details className="tasks-history" data-slot="history">
          <summary>Completed history · {plural(work.history.length, "group")}, {plural(work.history.reduce((total, group) => total + group.rows.length, 0), "task")}</summary>
          <div className="tasks-groups">
            {work.history.map((group) => <GroupView key={group.id} group={group} open={open} onToggle={onToggle} onSignal={onSignal} />)}
          </div>
        </details>
      )}
    </section>
  );
}

export function TasksPage() {
  const [search, setSearch] = useSearchParams();
  const projectsQuery = useProjects();
  const catalog = projectsQuery.data ?? {};
  // Archived projects leave the all-projects view and filter; an explicit focus still shows them (FS-02.R70).
  const projectNames = useMemo(() => Object.entries(projectsQuery.data ?? {}).filter(([, item]) => !item.archived).map(([name]) => name).sort((a, b) => a.localeCompare(b)), [projectsQuery.data]);
  const focus = search.get("project") ?? ALL;
  const unknownFocus = focus !== ALL && projectsQuery.isSuccess && !(focus in catalog);
  const archivedFocus = focus !== ALL && Boolean(catalog[focus]?.archived);
  const scope = focus === ALL ? projectNames : [focus];
  const queries = useQueries({ queries: scope.map((project) => taskListOptions(project, projectReads)) });
  const [open, setOpen] = useState<Set<string>>(new Set());
  const [signalOpen, setSignalOpen] = useState(false);
  const [signalDraft, setSignalDraft] = useState({ project: focus, name: "" });
  const toggle = (id: string) => setOpen((current) => {
    const next = new Set(current);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    return next;
  });
  const showSignal = (project: string, name: string) => {
    setSignalDraft({ project, name });
    setSignalOpen(true);
    // Focus, not firing: the person confirms the release explicitly (FS-16.R45).
    requestAnimationFrame(() => document.querySelector<HTMLInputElement>("#tasks-signal input")?.focus());
  };

  const loaded = queries.filter((query) => query.data);
  const incomplete = loaded.length < queries.length || queries.some((query) => query.isError);
  const unfinished = loaded.reduce((total, query) => total + query.data!.filter((task) => task.state !== "finished").length, 0);
  const attention = loaded.reduce((total, query) => total + query.data!.filter(needsAttention).length, 0);
  const globallyEmpty = focus === ALL && queries.length > 0 && !incomplete && loaded.every((query) => query.data!.length === 0);

  return (
    <div className="tasks-page" data-ui="tasks">
      <PageHeader
        eyebrow="Dependent work"
        title="Tasks"
        description="What is running, what waits and why, and where a person is needed — grouped by project and by the relationships agents recorded."
      />
      <div className="tasks-toolbar" data-slot="toolbar">
        <label>
          Project
          <select value={focus} onChange={(e) => setSearch(e.target.value ? { project: e.target.value } : {})}>
            <option value={ALL}>All projects</option>
            {projectNames.map((name) => <option key={name} value={name}>{catalog[name]?.title || name}</option>)}
            {archivedFocus && <option value={focus}>{catalog[focus]?.title || focus}</option>}
            {unknownFocus && <option value={focus}>{focus} (unknown)</option>}
          </select>
        </label>
        <span className="tasks-summary" data-slot="attention">
          {plural(unfinished, "unfinished task")} · {attention} need{attention === 1 ? "s" : ""} attention{incomplete ? " (some projects not loaded)" : ""}
        </span>
      </div>

      {projectsQuery.isError && <p className="form-error" role="alert">Projects could not be loaded.</p>}
      {unknownFocus && <p className="form-error" role="alert">No project named “{focus}” exists. Choose another project or All projects.</p>}
      {globallyEmpty && <p className="tasks-empty">No tasks yet. When agents hand work to each other, the tasks they create appear here grouped by project.</p>}

      {!unknownFocus && scope.map((project, index) => (
        <ProjectSection
          key={project}
          project={project}
          title={catalog[project]?.title || project}
          query={queries[index]}
          focused={focus !== ALL}
          open={open}
          onToggle={toggle}
          onSignal={showSignal}
        />
      ))}

      <div className="tasks-authoring" data-slot="authoring">
        <details className="tasks-disclosure">
          <summary>Create task manually</summary>
          <CreateTaskForm projects={projectNames} initialProject={focus === ALL || unknownFocus ? "" : focus} />
        </details>
        <details className="tasks-disclosure" id="tasks-signal" open={signalOpen} onToggle={(event) => setSignalOpen(event.currentTarget.open)}>
          <summary>Fire a signal</summary>
          <FireSignalForm projects={projectNames} project={signalDraft.project} name={signalDraft.name} onChange={setSignalDraft} />
        </details>
      </div>
    </div>
  );
}
