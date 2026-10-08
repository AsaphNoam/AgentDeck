import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
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
import {
  needsAttention, projectWork, rowVisibility, taskReason, taskStatus,
  type DescendantSummary, type HiddenBy, type ProjectWork, type RowVisibility, type TaskLink, type WorkGroup, type WorkRow,
} from "./taskWork";
import { collapsedIdsForProject, pruneProject, setCollapsed } from "./collapseStore";

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

/** LinkItem renders a recorded relationship, and — when it crosses a collapsed
 *  branch — the boundary the row's source sits behind plus a way to reveal
 *  just that ancestor path, never the whole tree (FS-16.R47, TS-08.R92). */
function LinkItem({ link, hiddenBy, onReveal }: { link: TaskLink; hiddenBy: Map<string, HiddenBy>; onReveal: (path: string[]) => void }) {
  const hidden = link.kind === "prerequisite" ? hiddenBy.get(link.sourceID) : undefined;
  return (
    <span data-variant={link.kind}>
      {linkText(link)}
      {hidden && (
        <span className="task-link-boundary" data-slot="boundary">
          {" "}· hidden under a collapsed stage{" "}
          <Button size="small" variant="ghost" onClick={() => onReveal(hidden.path)}>Reveal</Button>
        </span>
      )}
    </span>
  );
}

function descendantSummaryText(summary: DescendantSummary): string {
  const parts = [plural(summary.hidden, "task")];
  if (summary.unfinished > 0) parts.push(`${summary.unfinished} unfinished`);
  if (summary.attention > 0) parts.push(`${summary.attention} need${summary.attention === 1 ? "s" : ""} attention`);
  if (summary.cleanup > 0) parts.push(`${summary.cleanup} in cleanup`);
  return parts.join(" · ");
}

/** CollapseToggle is a sibling of the row's own detail disclosure, never
 *  nested inside it, so expanding details and collapsing descendants stay
 *  independent interactions (FS-16.R46, TS-08.R92). */
function CollapseToggle({ taskID, collapsed, summary, childIDs, onToggle }: {
  taskID: string; collapsed: boolean; summary: DescendantSummary; childIDs: string[]; onToggle: () => void;
}) {
  return (
    <Button
      id={`task-collapse-toggle-${taskID}`}
      size="small"
      variant="ghost"
      className="task-collapse-toggle"
      data-slot="collapse-toggle"
      aria-expanded={!collapsed}
      aria-controls={childIDs.map((id) => `task-row-${id}`).join(" ")}
      onClick={onToggle}
    >
      {collapsed ? "Expand tasks" : "Collapse tasks"} · {descendantSummaryText(summary)}
    </Button>
  );
}

function TaskRowView({ row, open, onToggle, onSignal, hidden, collapseInfo, hiddenBy, onReveal, onBusyChange }: {
  row: WorkRow;
  open: boolean;
  onToggle: () => void;
  onSignal: (project: string, name: string) => void;
  hidden: boolean;
  collapseInfo?: { collapsed: boolean; summary: DescendantSummary; childIDs: string[]; onToggleCollapse: () => void };
  hiddenBy: Map<string, HiddenBy>;
  onReveal: (path: string[]) => void;
  onBusyChange: (taskID: string, busy: boolean) => void;
}) {
  const { task } = row;
  const status = taskStatus(task);
  const reason = taskReason(row);
  const assigned = assigneeID(task);
  const detailID = `task-detail-${task.task_id}`;
  return (
    <li id={`task-row-${task.task_id}`} className={`task-row task-depth-${row.depth}`} data-slot="task" data-state={task.state} hidden={hidden}>
      <div className="task-row-controls" data-slot="controls">
        <button type="button" className="task-row-summary" aria-expanded={open} aria-controls={detailID} onClick={onToggle}>
          <span className="task-row-top" data-slot="metadata">
            <span className="task-name">{task.display_name}</span>
            <Badge className="task-state" variant={status.tone} indicator>{status.label}</Badge>
          </span>
          {reason && <span className={needsAttention(task) ? "task-attention" : "task-waiting"} data-slot={needsAttention(task) ? "attention" : "waiting"}>{reason}</span>}
        </button>
        {collapseInfo && (
          <CollapseToggle
            taskID={task.task_id}
            collapsed={collapseInfo.collapsed}
            summary={collapseInfo.summary}
            childIDs={collapseInfo.childIDs}
            onToggle={collapseInfo.onToggleCollapse}
          />
        )}
      </div>
      {(row.links.length > 0 || row.next.length > 0) && (
        <p className="task-links" data-slot="links">
          {row.links.map((link, index) => <LinkItem key={`${link.kind}:${link.sourceID}:${index}`} link={link} hiddenBy={hiddenBy} onReveal={onReveal} />)}
          {row.next.length > 0 && <span data-variant="next">leads to {row.next.map((item) => item.display_name).join(", ")}</span>}
        </p>
      )}
      <p className="task-row-meta" data-slot="metadata">
        <span>created by <Creator task={task} /></span>
        {task.room
          ? <span>runs as Think Tank · <Link to={`/think-tank/${task.room.room_id}`}>room</Link> · <Link to={`/pipelines/runs/${encodeURIComponent(task.room.run_id)}`}>run</Link></span>
          : assigned ? <span>assigned to <AgentLabel id={assigned} /></span> : task.target_kind === "launch" ? <span>launches {task.role}</span> : null}
      </p>
      {open && <TaskDetail id={detailID} row={row} onSignal={onSignal} onBusyChange={onBusyChange} hiddenBy={hiddenBy} onReveal={onReveal} />}
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

function TaskDetail({ id, row, onSignal, onBusyChange, hiddenBy, onReveal }: {
  id: string;
  row: WorkRow;
  onSignal: (project: string, name: string) => void;
  onBusyChange: (taskID: string, busy: boolean) => void;
  hiddenBy: Map<string, HiddenBy>;
  onReveal: (path: string[]) => void;
}) {
  const { task } = row;
  const cancel = useCancelTask(task.project);
  const retry = useRetryTask(task.project);
  const remove = useDeleteTask(task.project);
  const [error, setError] = useState("");
  const stage = useStageOwnership(task);
  const actions = taskActions(task);
  const restricted = stage.owned || !stage.known;
  // A mutation in flight pins the row visible until it settles, success or
  // not (FS-16.R47) — the collapse view never hides work a person is acting on.
  const act = (run: () => Promise<unknown>) => {
    setError("");
    onBusyChange(task.task_id, true);
    run().catch((err: unknown) => setError(errorMessage(err))).finally(() => onBusyChange(task.task_id, false));
  };
  const outputs = Object.entries(task.outputs);

  return (
    <div className="task-detail" id={id} data-slot="detail">
      <p className="task-instruction" data-slot="instruction">{task.instruction}</p>
      {(row.links.length > 0 || row.external.length > 0) && (
        <div className="task-detail-section">
          <strong>Prerequisites and lineage</strong>
          <ul>
            {row.links.map((link, index) => <li key={`l${index}`}><LinkItem link={link} hiddenBy={hiddenBy} onReveal={onReveal} /></li>)}
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

function GroupView({ group, open, onToggle, onSignal, collapsed, pinned, onToggleCollapse, onReveal, onBusyChange }: {
  group: WorkGroup;
  open: Set<string>;
  onToggle: (id: string) => void;
  onSignal: (project: string, name: string) => void;
  collapsed: ReadonlySet<string>;
  pinned: ReadonlySet<string>;
  onToggleCollapse: (id: string) => void;
  onReveal: (path: string[]) => void;
  onBusyChange: (taskID: string, busy: boolean) => void;
}) {
  // Pure projection per group (TS-08.R92); recomputed only when its inputs move.
  const visibility: RowVisibility = useMemo(() => rowVisibility(group.rows, collapsed, pinned), [group.rows, collapsed, pinned]);
  // After a pin releases and its row actually hides, return focus to the
  // ancestor's toggle rather than letting it fall back to <body> (TS-08.R92).
  const previouslyHidden = useRef<Set<string>>(new Set());
  useEffect(() => {
    const active = document.activeElement;
    for (const [taskID, info] of visibility.hiddenBy) {
      if (previouslyHidden.current.has(taskID)) continue;
      const rowEl = document.getElementById(`task-row-${taskID}`);
      if (rowEl && active && rowEl.contains(active)) {
        document.getElementById(`task-collapse-toggle-${info.ancestorID}`)?.focus();
        break;
      }
    }
    previouslyHidden.current = new Set(visibility.hiddenBy.keys());
  }, [visibility]);

  return (
    <ol className="task-group" data-slot="group" aria-label={group.rows.length > 1 ? `${group.rows.length} related tasks` : undefined}>
      {group.rows.map((row) => {
        const taskID = row.task.task_id;
        const children = visibility.directChildren.get(taskID);
        const summary = visibility.descendants.get(taskID);
        return (
          <TaskRowView
            key={taskID}
            row={row}
            open={open.has(taskID)}
            onToggle={() => onToggle(taskID)}
            onSignal={onSignal}
            hidden={visibility.hiddenBy.has(taskID)}
            hiddenBy={visibility.hiddenBy}
            onReveal={onReveal}
            onBusyChange={onBusyChange}
            collapseInfo={children && summary ? {
              collapsed: collapsed.has(taskID),
              summary,
              childIDs: children.map((child) => child.task_id),
              onToggleCollapse: () => onToggleCollapse(taskID),
            } : undefined}
          />
        );
      })}
    </ol>
  );
}

function ProjectSection({ project, title, query, focused, open, onToggle, onSignal, busy, onBusyChange }: {
  project: string;
  title: string;
  query: { data?: Task[]; isLoading: boolean; isError: boolean; error: unknown; refetch: () => unknown };
  focused: boolean;
  open: Set<string>;
  onToggle: (id: string) => void;
  onSignal: (project: string, name: string) => void;
  busy: Set<string>;
  onBusyChange: (taskID: string, busy: boolean) => void;
}) {
  const work: ProjectWork | null = useMemo(
    () => query.data ? projectWork(project, query.data, new Set(query.data.filter((task) => open.has(task.task_id)).map((task) => task.task_id))) : null,
    [project, query.data, open],
  );
  // Collapse choices live in the feature-owned sessionStorage map, keyed by
  // project/task identity (TS-08.R93); this state mirrors it for rendering.
  const [collapsed, setCollapsedState] = useState<Set<string>>(() => collapsedIdsForProject(project));
  const toggleCollapse = (taskID: string) => {
    setCollapsedState((current) => {
      const next = new Set(current);
      const nowCollapsed = !next.has(taskID);
      if (nowCollapsed) next.add(taskID); else next.delete(taskID);
      setCollapsed(project, taskID, nowCollapsed);
      return next;
    });
  };
  const reveal = (path: string[]) => {
    if (path.length === 0) return;
    setCollapsedState((current) => {
      const next = new Set(current);
      for (const id of path) {
        next.delete(id);
        setCollapsed(project, id, false);
      }
      return next;
    });
  };
  // A complete project read is the only authority that prunes stale choices —
  // a loading or filtered view never reads as deletion (TS-08.R93).
  useEffect(() => {
    if (!query.data) return;
    const liveIDs = new Set(query.data.map((task) => task.task_id));
    pruneProject(project, liveIDs);
    setCollapsedState((current) => {
      let changed = false;
      const next = new Set<string>();
      for (const id of current) {
        if (liveIDs.has(id)) next.add(id);
        else changed = true;
      }
      return changed ? next : current;
    });
  }, [project, query.data]);
  const pinned = useMemo(() => new Set([...open, ...busy]), [open, busy]);
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
          {work.active.map((group) => (
            <GroupView key={group.id} group={group} open={open} onToggle={onToggle} onSignal={onSignal}
              collapsed={collapsed} pinned={pinned} onToggleCollapse={toggleCollapse} onReveal={reveal} onBusyChange={onBusyChange} />
          ))}
        </div>
      )}
      {work && work.history.length > 0 && (
        <details className="tasks-history" data-slot="history">
          <summary>Completed history · {plural(work.history.length, "group")}, {plural(work.history.reduce((total, group) => total + group.rows.length, 0), "task")}</summary>
          <div className="tasks-groups">
            {work.history.map((group) => (
              <GroupView key={group.id} group={group} open={open} onToggle={onToggle} onSignal={onSignal}
                collapsed={collapsed} pinned={pinned} onToggleCollapse={toggleCollapse} onReveal={reveal} onBusyChange={onBusyChange} />
            ))}
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
  const [busy, setBusy] = useState<Set<string>>(new Set());
  const [signalOpen, setSignalOpen] = useState(false);
  const [signalDraft, setSignalDraft] = useState({ project: focus, name: "" });
  const toggle = (id: string) => setOpen((current) => {
    const next = new Set(current);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    return next;
  });
  // A mutation in flight pins its task visible through the collapse view
  // regardless of ancestor state (FS-16.R47).
  const setBusyState = (taskID: string, isBusy: boolean) => setBusy((current) => {
    if (isBusy === current.has(taskID)) return current;
    const next = new Set(current);
    if (isBusy) next.add(taskID); else next.delete(taskID);
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
          busy={busy}
          onBusyChange={setBusyState}
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
