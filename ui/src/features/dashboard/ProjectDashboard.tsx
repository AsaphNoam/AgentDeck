import { useNavigate, useParams, Link } from "react-router-dom";
import { useMemo, useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { QUERY_KEYS, useProjects, useUpdateProject } from "../../api/config";
import { useAgentStore } from "../../store/agentStore";
import { CardGrid } from "../../components/grid/CardGrid";
import { archiveProject } from "../../api/client";
import { useUiStore } from "../../store/uiStore";
import { ConfirmDialog } from "../../components/ui";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { ProjectResponse } from "../../schemas/project";
import type { MouseEvent } from "react";

type ProjectEdit = { id: string; project: Omit<ProjectResponse, "project">; field: "title" | "color" };

export function ProjectDashboard() {
  const navigate = useNavigate();
  const projects = useProjects();
  const agents = useAgentStore((state) => state.agents);
  const pushError = useUiStore((state) => state.pushError);
  const queryClient = useQueryClient();
  const updateProject = useUpdateProject();
  const [contextProject, setContextProject] = useState<string | null>(null);
  const [edit, setEdit] = useState<ProjectEdit | null>(null);
  const [archiveID, setArchiveID] = useState<string | null>(null);
  const [archiveError, setArchiveError] = useState("");
  const archive = useMutation({
    mutationFn: archiveProject,
    onSuccess: ({ project }) => {
      queryClient.setQueryData<Record<string, Omit<ProjectResponse, "project">>>(QUERY_KEYS.projects, (current) => (
        current ? { ...current, [project.project]: { ...current[project.project], ...project, archived: true } } : current
      ));
      void queryClient.invalidateQueries({ queryKey: QUERY_KEYS.projects });
      setArchiveID(null);
    },
    onError: (err) => { const message = err instanceof Error ? err.message : String(err); setArchiveError(message); pushError("Archive project failed", message); },
  });
  const active = useMemo(() => Object.entries(projects.data ?? {}).filter(([, project]) => !project.archived), [projects.data]);
  const unavailable = useMemo(() => [...new Set(Object.values(agents).filter((agent) => !agent.archived && !projects.data?.[agent.project]).map((agent) => agent.project))], [agents, projects.data]);
  if (projects.isLoading) return <section className="project-dashboard" data-ui="project-dashboard"><p>Loading projects…</p></section>;
  if (projects.isError) return <section className="project-dashboard" data-ui="project-dashboard"><p className="form-error">Could not load projects. Please retry.</p></section>;

  const archiveProjectEntry = archiveID ? projects.data?.[archiveID] : null;
  return (
    <section className="project-dashboard" data-ui="project-dashboard">
      <h1>Projects</h1>
      <div className="project-card-grid">
        {active.map(([id, project]) => {
          const projectAgents = Object.values(agents).filter((agent) => agent.project === id && !agent.archived);
          const stateSummary = Object.entries(projectAgents.reduce<Record<string, number>>((counts, agent) => ({ ...counts, [agent.state]: (counts[agent.state] ?? 0) + 1 }), {})).map(([state, count]) => `${count} ${state}`).join(" · ") || "No agents";
          return <article className="project-card" key={id} onClick={() => navigate(`/project/${id}`)} onContextMenu={(event: MouseEvent) => { event.preventDefault(); setContextProject(id); }}>
            <span className="project-card-color" style={{ background: `rgb(${project.color[0]}, ${project.color[1]}, ${project.color[2]})` }} aria-label="Project color" />
            <strong>{project.title}</strong><span>{projectAgents.length} agents</span><small>{stateSummary}</small>
            {contextProject === id && <div className="project-card-actions" onClick={(event) => event.stopPropagation()}>
              <button type="button" onClick={() => { setEdit({ id, project, field: "title" }); setContextProject(null); }}>Rename</button>
              <button type="button" onClick={() => { setEdit({ id, project, field: "color" }); setContextProject(null); }}>Change color</button>
              <button type="button" onClick={() => { setArchiveError(""); setArchiveID(id); setContextProject(null); }}>Archive</button>
            </div>}
          </article>;
        })}
        {unavailable.map((id) => <article className="project-card unavailable" key={id} onClick={() => navigate(`/project/${id}`)}><strong>{id}</strong><span>Project unavailable</span></article>)}
      </div>
      {edit && <ProjectEditDialog key={`${edit.id}-${edit.field}`} edit={edit} onCancel={() => setEdit(null)} onSave={(data, setError) => updateProject.mutate({ id: edit.id, data }, { onSuccess: () => setEdit(null), onError: (err) => { const message = err instanceof Error ? err.message : String(err); setError(message); pushError(edit.field === "title" ? "Rename project failed" : "Change color failed", message); } })} />}
      {archiveProjectEntry && (
        <ConfirmDialog open title={`Archive ${archiveProjectEntry.title}?`} confirmLabel="Archive project" destructive onCancel={() => { setArchiveError(""); setArchiveID(null); }} onConfirm={() => archive.mutate(archiveID!)} pending={archive.isPending}>
          <p>Running agents will be stopped and every agent in this project will be archived.</p>
          {archiveError && <p className="form-error">{archiveError}</p>}
        </ConfirmDialog>
      )}
    </section>
  );
}

function ProjectEditDialog({ edit, onCancel, onSave }: { edit: ProjectEdit; onCancel: () => void; onSave: (data: Omit<ProjectResponse, "project">, setError: (message: string) => void) => void }) {
  const [title, setTitle] = useState(edit.project.title);
  const [color, setColor] = useState<[number, number, number]>(edit.project.color);
  const [error, setError] = useState("");
  const submit = (event: React.FormEvent) => {
    event.preventDefault();
    if (edit.field === "title") {
      if (!title.trim()) { setError("Project name is required."); return; }
      onSave({ ...edit.project, title: title.trim() }, setError);
      return;
    }
    if (color.some((channel) => !Number.isInteger(channel) || channel < 0 || channel > 255)) {
      setError("Each channel must be a whole number from 0 to 255.");
      return;
    }
    onSave({ ...edit.project, color }, setError);
  };
  return (
    <Dialog.Root open onOpenChange={(open) => { if (!open) onCancel(); }}>
      <Dialog.Portal>
        <Dialog.Overlay className="dialog-overlay" data-ui="dialog" data-slot="overlay" />
        <Dialog.Content className="dialog-content" data-ui="dialog" data-slot="content" data-variant="default">
          <Dialog.Title data-slot="title">{edit.field === "title" ? "Rename project" : "Change project color"}</Dialog.Title>
          <form className="config-form" onSubmit={submit}>
            {edit.field === "title" ? (
              <div className="form-field"><label htmlFor="project-title">Name</label><input id="project-title" value={title} onChange={(event) => setTitle(event.target.value)} /></div>
            ) : (
              <div className="form-field"><label>Color (RGB)</label>{(["R", "G", "B"] as const).map((channel, index) => <label key={channel}>{channel}<input aria-label={`${channel} channel`} type="number" min={0} max={255} value={color[index]} onChange={(event) => setColor((current) => current.map((value, item) => item === index ? Number(event.target.value) : value) as [number, number, number])} /></label>)}</div>
            )}
            {error && <p className="form-error">{error}</p>}
            <div className="form-actions" data-slot="actions"><button type="button" onClick={onCancel}>Cancel</button><button type="submit">Save</button></div>
          </form>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

export function ScopedProjectDashboard() {
  const { project: id = "" } = useParams();
  const projects = useProjects();
  const hasAgents = useAgentStore((state) => Object.values(state.agents).some((agent) => agent.project === id && !agent.archived));
  const project = projects.data?.[id];
  if (projects.isLoading) return <section className="project-route-state"><p>Loading project…</p></section>;
  if (projects.isError) return <section className="project-route-state"><p className="form-error">Could not load this project. Please retry.</p><Link to="/">Back to projects</Link></section>;
  if (project?.archived) return <section className="project-route-state"><h1>{project.title} is archived</h1><Link to="/">Back to projects</Link> · <Link to="/archive">Open Archive</Link></section>;
  if (!project && !hasAgents) return <section className="project-route-state"><h1>Project unavailable</h1><Link to="/">Back to projects</Link></section>;
  return <section><Link to="/">Back to projects</Link><CardGrid projectID={id} projectTitle={project?.title ?? id} /></section>;
}
