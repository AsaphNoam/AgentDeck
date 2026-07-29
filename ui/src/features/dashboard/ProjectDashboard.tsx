import { useNavigate, useParams, Link } from "react-router-dom";
import { useMemo, useState } from "react";
import { QUERY_KEYS, useProjects, useUpdateProject } from "../../api/config";
import { useAgentStore } from "../../store/agentStore";
import { CardGrid } from "../../components/grid/CardGrid";
import { archiveProject } from "../../api/client";
import { useUiStore } from "../../store/uiStore";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { ProjectResponse } from "../../schemas/project";
import type { MouseEvent } from "react";

export function ProjectDashboard() {
  const navigate = useNavigate();
  const projects = useProjects();
  const agents = useAgentStore((state) => state.agents);
  const pushError = useUiStore((state) => state.pushError);
  const queryClient = useQueryClient();
  const updateProject = useUpdateProject();
  const [contextProject, setContextProject] = useState<string | null>(null);
  const archive = useMutation({
    mutationFn: archiveProject,
    onSuccess: ({ project }) => {
      queryClient.setQueryData<Record<string, Omit<ProjectResponse, "project">>>(QUERY_KEYS.projects, (current) => (
        current ? { ...current, [project.project]: { ...current[project.project], ...project, archived: true } } : current
      ));
      void queryClient.invalidateQueries({ queryKey: QUERY_KEYS.projects });
    },
    onError: (err) => pushError("Archive project failed", err instanceof Error ? err.message : String(err)),
  });
  const active = useMemo(() => Object.entries(projects.data ?? {}).filter(([, project]) => !project.archived), [projects.data]);
  const unavailable = useMemo(() => [...new Set(Object.values(agents).filter((agent) => !agent.archived && !projects.data?.[agent.project]).map((agent) => agent.project))], [agents, projects.data]);
  if (projects.isLoading) return <section className="project-dashboard" data-ui="project-dashboard"><p>Loading projects…</p></section>;
  if (projects.isError) return <section className="project-dashboard" data-ui="project-dashboard"><p className="form-error">Could not load projects. Please retry.</p></section>;

  const editProject = (id: string, project: Omit<ProjectResponse, "project">, field: "title" | "color") => {
    const next = field === "title"
      ? window.prompt("Project name", project.title)
      : window.prompt("Project color as R,G,B", project.color.join(","));
    if (next === null) return;
    if (field === "title") {
      if (!next.trim()) return;
      updateProject.mutate({ id, data: { ...project, title: next.trim() } }, {
        onError: (err) => pushError("Rename project failed", err instanceof Error ? err.message : String(err)),
      });
      return;
    }
    const channels = next.split(",").map((value) => Number(value.trim()));
    if (channels.length !== 3 || channels.some((value) => !Number.isInteger(value) || value < 0 || value > 255)) {
      pushError("Change color failed", "Use three whole RGB values from 0 to 255.");
      return;
    }
    updateProject.mutate({ id, data: { ...project, color: channels as [number, number, number] } }, {
      onError: (err) => pushError("Change color failed", err instanceof Error ? err.message : String(err)),
    });
  };
  return <section className="project-dashboard" data-ui="project-dashboard"><h1>Projects</h1><div className="project-card-grid">
    {active.map(([id, project]) => {
      const projectAgents = Object.values(agents).filter((agent) => agent.project === id && !agent.archived);
      const stateSummary = Object.entries(projectAgents.reduce<Record<string, number>>((counts, agent) => ({ ...counts, [agent.state]: (counts[agent.state] ?? 0) + 1 }), {})).map(([state, count]) => `${count} ${state}`).join(" · ") || "No agents";
      return <article className="project-card" key={id} onClick={() => navigate(`/project/${id}`)} onContextMenu={(event: MouseEvent) => { event.preventDefault(); setContextProject(id); }}>
        <span className="project-card-color" style={{ background: `rgb(${project.color[0]}, ${project.color[1]}, ${project.color[2]})` }} aria-label="Project color" />
        <strong>{project.title}</strong><span>{projectAgents.length} agents</span><small>{stateSummary}</small>
        {contextProject === id && <div className="project-card-actions" onClick={(event) => event.stopPropagation()}>
          <button type="button" onClick={() => editProject(id, project, "title")}>Rename</button>
          <button type="button" onClick={() => editProject(id, project, "color")}>Change color</button>
          <button type="button" onClick={() => { if (window.confirm(`Archive ${project.title}? Running agents will be stopped and all project agents archived.`)) archive.mutate(id); }}>Archive</button>
        </div>}
      </article>;
    })}
    {unavailable.map((id) => <article className="project-card unavailable" key={id} onClick={() => navigate(`/project/${id}`)}><strong>{id}</strong><span>Project unavailable</span></article>)}
  </div></section>;
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
