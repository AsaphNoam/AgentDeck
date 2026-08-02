import { useNavigate, useParams, Link } from "react-router-dom";
import { useEffect, useMemo, useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { createPortal } from "react-dom";
import { configErrorMessage, QUERY_KEYS, useCreateProject, useProjects, useUpdateProject } from "../../api/config";
import { useAgentStore } from "../../store/agentStore";
import { CardGrid } from "../../components/grid/CardGrid";
import { archiveProject } from "../../api/client";
import { useUiStore } from "../../store/uiStore";
import { ConfirmDialog, ProjectColorPicker } from "../../components/ui";
import { ProjectForm } from "../settings/ProjectForm";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { ProjectResponse } from "../../schemas/project";
import type { MouseEvent } from "react";

type ProjectEdit = { id: string; project: Omit<ProjectResponse, "project"> };
type ProjectMenu = { id: string; x: number; y: number };
type BackgroundMenu = { x: number; y: number };

export function ProjectDashboard() {
  const navigate = useNavigate();
  const projects = useProjects();
  const agents = useAgentStore((state) => state.agents);
  const pushError = useUiStore((state) => state.pushError);
  const queryClient = useQueryClient();
  const updateProject = useUpdateProject();
  const createProject = useCreateProject();
  const [contextMenu, setContextMenu] = useState<ProjectMenu | null>(null);
  const [bgMenu, setBgMenu] = useState<BackgroundMenu | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [createError, setCreateError] = useState("");
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
  useEffect(() => {
    if (!contextMenu && !bgMenu) return;
    const close = () => { setContextMenu(null); setBgMenu(null); };
    const dismiss = (event: globalThis.MouseEvent) => {
      if (!(event.target as HTMLElement).closest(".context-menu")) close();
    };
    const escape = (event: KeyboardEvent) => { if (event.key === "Escape") close(); };
    window.addEventListener("mousedown", dismiss);
    window.addEventListener("keydown", escape);
    return () => {
      window.removeEventListener("mousedown", dismiss);
      window.removeEventListener("keydown", escape);
    };
  }, [contextMenu, bgMenu]);

  const updateColor = (id: string, project: Omit<ProjectResponse, "project">, color: readonly number[]) => {
    queryClient.setQueryData<Record<string, Omit<ProjectResponse, "project">>>(QUERY_KEYS.projects, (current) => (
      current ? { ...current, [id]: { ...current[id], color: [...color] as [number, number, number] } } : current
    ));
    updateProject.mutate(
      { id, data: { ...project, color: [...color] as [number, number, number] } },
      { onError: (err) => { void queryClient.invalidateQueries({ queryKey: QUERY_KEYS.projects }); pushError("Change project color failed", configErrorMessage(err)); } },
    );
  };
  const openCreate = () => { setCreateError(""); setCreateOpen(true); setContextMenu(null); setBgMenu(null); };
  const createEntry = (data: ProjectResponse) => {
    setCreateError("");
    createProject.mutate(data, {
      onSuccess: () => setCreateOpen(false),
      onError: (err) => setCreateError(configErrorMessage(err)),
    });
  };
  // The projects-home background opens a New project menu, but a right-click that
  // lands on a project card must fall through to that card's own menu (R41).
  const openBackgroundMenu = (event: MouseEvent) => {
    if ((event.target as HTMLElement).closest(".project-card")) return;
    event.preventDefault();
    setContextMenu(null);
    setBgMenu({ x: event.clientX, y: event.clientY });
  };

  if (projects.isLoading) return <section className="project-dashboard" data-ui="project-dashboard"><p>Loading projects…</p></section>;
  if (projects.isError) return <section className="project-dashboard" data-ui="project-dashboard"><p className="form-error">Could not load projects. Please retry.</p></section>;

  const archiveProjectEntry = archiveID ? projects.data?.[archiveID] : null;
  return (
    <section className="project-dashboard" data-ui="project-dashboard" onContextMenu={openBackgroundMenu}>
      <div className="project-dashboard-header">
        <h1>Projects</h1>
        <button type="button" onClick={openCreate}>New project</button>
      </div>
      <div className="project-card-grid">
        {active.map(([id, project]) => {
          const projectAgents = Object.values(agents).filter((agent) => agent.project === id && !agent.archived);
          const stateSummary = Object.entries(projectAgents.reduce<Record<string, number>>((counts, agent) => ({ ...counts, [agent.state]: (counts[agent.state] ?? 0) + 1 }), {})).map(([state, count]) => `${count} ${state}`).join(" · ") || "No agents";
          return <article className="project-card" key={id} style={{ "--ad-project-accent": `rgb(${project.color.join(",")})` } as React.CSSProperties} onClick={() => navigate(`/project/${id}`)} onContextMenu={(event: MouseEvent) => { event.preventDefault(); setBgMenu(null); setContextMenu({ id, x: event.clientX, y: event.clientY }); }}>
            <span className="project-card-color" style={{ background: `rgb(${project.color[0]}, ${project.color[1]}, ${project.color[2]})` }} aria-label="Project color" />
            <strong>{project.title}</strong><span>{projectAgents.length} agents</span><small>{stateSummary}</small>
          </article>;
        })}
        {unavailable.map((id) => <article className="project-card unavailable" key={id} onClick={() => navigate(`/project/${id}`)}><strong>{id}</strong><span>Project unavailable</span></article>)}
      </div>
      {contextMenu && projects.data?.[contextMenu.id] && createPortal(
        <div className="context-menu" data-ui="context-menu" role="menu" style={{ left: contextMenu.x, top: contextMenu.y }}>
          <button type="button" data-slot="item" onClick={() => { setEdit({ id: contextMenu.id, project: projects.data![contextMenu.id] }); setContextMenu(null); }}>Rename</button>
          <div className="context-menu-color" role="menuitem">
            <span>Change color</span>
            <ProjectColorPicker value={projects.data[contextMenu.id].color} onChange={(color) => updateColor(contextMenu.id, projects.data![contextMenu.id], color)} />
          </div>
          <button type="button" data-slot="item" onClick={() => { setArchiveError(""); setArchiveID(contextMenu.id); setContextMenu(null); }}>Archive</button>
        </div>,
        document.body,
      )}
      {bgMenu && createPortal(
        <div className="context-menu" data-ui="context-menu" role="menu" style={{ left: bgMenu.x, top: bgMenu.y }}>
          <button type="button" data-slot="item" onClick={openCreate}>New project</button>
        </div>,
        document.body,
      )}
      {createOpen && (
        <Dialog.Root open onOpenChange={(open) => { if (!open) setCreateOpen(false); }}>
          <Dialog.Portal>
            <Dialog.Overlay className="dialog-overlay" data-ui="dialog" data-slot="overlay" />
            <Dialog.Content className="dialog-content" data-ui="dialog" data-slot="content" data-variant="default">
              <Dialog.Title>New project</Dialog.Title>
              <ProjectForm
                onSubmit={createEntry}
                onCancel={() => setCreateOpen(false)}
                submitting={createProject.isPending}
                error={createError}
              />
            </Dialog.Content>
          </Dialog.Portal>
        </Dialog.Root>
      )}
      {edit && <ProjectEditDialog key={edit.id} edit={edit} onCancel={() => setEdit(null)} onSave={(data, setError) => updateProject.mutate({ id: edit.id, data }, { onSuccess: () => setEdit(null), onError: (err) => { const message = configErrorMessage(err); setError(message); pushError("Rename project failed", message); } })} />}
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
  const [error, setError] = useState("");
  const submit = (event: React.FormEvent) => {
    event.preventDefault();
    if (!title.trim()) { setError("Project name is required."); return; }
    onSave({ ...edit.project, title: title.trim() }, setError);
  };
  return (
    <Dialog.Root open onOpenChange={(open) => { if (!open) onCancel(); }}>
      <Dialog.Portal>
        <Dialog.Overlay className="dialog-overlay" data-ui="dialog" data-slot="overlay" />
        <Dialog.Content className="dialog-content" data-ui="dialog" data-slot="content" data-variant="default">
          <Dialog.Title data-slot="title">Rename project</Dialog.Title>
          <form className="config-form" onSubmit={submit}>
            <div className="form-field"><label htmlFor="project-title">Name</label><input id="project-title" value={title} onChange={(event) => setTitle(event.target.value)} /></div>
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
  // fixedProject only when the route project is a current, non-archived catalog
  // member. On the `!project && hasAgents` fall-through (a live agent naming a
  // removed project) the New Agent picker must stay available so a launch can
  // target a real project instead of the rejected route id (INV §10).
  return <section><Link to="/">Back to projects</Link><CardGrid projectID={id} fixedProject={project ? id : undefined} projectTitle={project?.title ?? id} /></section>;
}
