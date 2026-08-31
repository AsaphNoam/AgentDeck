import React, { useEffect, useMemo, useRef, useState } from "react";
import { NavLink, useMatch } from "react-router-dom";
import { useProjects } from "../../api/config";
import type { AgentState } from "../../api/types";
import type { ProjectResponse } from "../../schemas/project";
import { useAgentStore } from "../../store/agentStore";

type ProjectCatalog = Record<string, Omit<ProjectResponse, "project">>;

export interface ActiveProjectEntry {
  id: string;
  title: string;
  color: readonly [number, number, number];
}

export function deriveActiveProjects(
  projects: ProjectCatalog | undefined,
  agents: Record<string, AgentState>,
  currentProjectID: string | undefined,
) {
  if (!projects) return { visible: [], overflow: [] };

  const eligible = new Set(
    Object.values(agents)
      .filter((agent) => agent.running && !agent.archived)
      .map((agent) => agent.project),
  );
  if (currentProjectID && projects[currentProjectID] && !projects[currentProjectID].archived) {
    eligible.add(currentProjectID);
  }

  const entries = [...eligible]
    .filter((id) => projects[id] && !projects[id].archived)
    .map((id): ActiveProjectEntry => ({ id, title: projects[id].title, color: projects[id].color }))
    .sort((a, b) => a.title.localeCompare(b.title) || a.id.localeCompare(b.id));

  let visible = entries.slice(0, 5);
  if (currentProjectID && !visible.some(({ id }) => id === currentProjectID)) {
    const current = entries.find(({ id }) => id === currentProjectID);
    if (current) visible = [...visible.slice(0, 4), current].sort(entryOrder);
  }
  const visibleIDs = new Set(visible.map(({ id }) => id));
  return { visible, overflow: entries.filter(({ id }) => !visibleIDs.has(id)) };
}

function entryOrder(a: ActiveProjectEntry, b: ActiveProjectEntry) {
  return a.title.localeCompare(b.title) || a.id.localeCompare(b.id);
}

export function ActiveProjectNav() {
  const { data: projects } = useProjects();
  const agents = useAgentStore((state) => state.agents);
  const projectMatch = useMatch("/project/:projectId");
  const agentMatch = useMatch("/agent/:agentId");
  const currentProjectID = projectMatch?.params.projectId
    ?? (agentMatch?.params.agentId ? agents[agentMatch.params.agentId]?.project : undefined);
  const { visible, overflow } = useMemo(
    () => deriveActiveProjects(projects, agents, currentProjectID),
    [projects, agents, currentProjectID],
  );

  return <ProjectNav visible={visible} overflow={overflow} currentProjectID={currentProjectID} />;
}

export function ProjectNav({ visible, overflow, currentProjectID }: {
  visible: ActiveProjectEntry[];
  overflow: ActiveProjectEntry[];
  currentProjectID?: string;
}) {
  const [open, setOpen] = useState(false);
  const disclosureRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const closeOutside = (event: PointerEvent) => {
      if (!disclosureRef.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener("pointerdown", closeOutside);
    return () => document.removeEventListener("pointerdown", closeOutside);
  }, [open]);

  useEffect(() => setOpen(false), [currentProjectID]);

  if (!visible.length) return null;

  return (
    <nav className="project-nav" aria-label="Active projects">
      {visible.map((project) => <ProjectLink key={project.id} project={project} selected={project.id === currentProjectID} />)}
      {overflow.length > 0 && (
        <div className="project-nav-overflow" ref={disclosureRef}>
          <button
            type="button"
            aria-expanded={open}
            aria-haspopup="menu"
            aria-label={`${overflow.length} more active ${overflow.length === 1 ? "project" : "projects"}`}
            onClick={() => setOpen((value) => !value)}
            onKeyDown={(event) => { if (event.key === "Escape") setOpen(false); }}
          >+{overflow.length}</button>
          {open && (
            <div className="project-nav-menu" role="menu">
              {overflow.map((project) => <ProjectLink key={project.id} project={project} menu />)}
            </div>
          )}
        </div>
      )}
    </nav>
  );
}

function ProjectLink({ project, menu = false, selected = false }: { project: ActiveProjectEntry; menu?: boolean; selected?: boolean }) {
  return (
    <NavLink
      to={`/project/${encodeURIComponent(project.id)}`}
      title={project.title}
      aria-label={project.title}
      role={menu ? "menuitem" : undefined}
      className={({ isActive }) => isActive || selected ? "active" : undefined}
    >
      <span style={{ "--ad-project-accent": `rgb(${project.color.join(",")})` } as React.CSSProperties}>{project.title}</span>
    </NavLink>
  );
}
