import { useEffect, useState, type CSSProperties } from "react";
import { useQuery } from "@tanstack/react-query";
import { getHome, type AttentionItem } from "./api";
import { useConnection } from "./connection";
import { navigate } from "./router";
import { phoneFetch } from "./api";
import { deriveDashboardProjects } from "../features/dashboard/projectDashboardData";
import { PhoneIcon } from "./PhoneIcon";
const DEBOUNCE_MS = 400;

/** itemPath is where a Home row opens. */
export function itemPath(item: AttentionItem): string {
  if (item.kind === "run") return `/run/${encodeURIComponent(item.id)}`;
  return `/agent/${encodeURIComponent(item.id)}`;
}

function relative(iso: string): string {
  const minutes = Math.round((Date.now() - new Date(iso).getTime()) / 60000);
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes} min ago`;
  const hours = Math.round(minutes / 60);
  return hours < 24 ? `${hours} h ago` : new Date(iso).toLocaleDateString();
}

function Row({ item }: { item: AttentionItem }) {
  const stage = item.stage_number && item.stage_count ? `Stage ${item.stage_number} of ${item.stage_count}` : "";
  return (
    <li>
      <button type="button" className="phone-row phone-attention" onClick={() => navigate(itemPath(item))}>
        <span className="phone-attention-meta"><span><PhoneIcon name={item.kind === "run" ? "pipeline" : "agent"} size={15} />{item.kind === "run" ? "Pipeline" : "Agent"} · {item.project}</span><small>{relative(item.since)}</small></span>
        <span className="phone-attention-heading"><span className="phone-row-title">{item.title}</span><PhoneIcon name="arrow" size={17} /></span>
        <span className="phone-row-reason">
          {item.reason}
          {stage ? ` · ${stage}` : ""}
          {item.outcome ? ` · ${item.outcome}` : ""}
        </span>
        <span className="phone-attention-tag">{item.kind === "run" ? "Run needs you" : "Agent needs you"}</span>
      </button>
    </li>
  );
}

function Section({ title, items, empty }: { title: string; items: AttentionItem[]; empty: string }) {
  return (
    <section className="phone-section" aria-label={title}>
      <div className="phone-section-title"><h2>{title}</h2><span className="phone-count">{items.length} items</span></div>
      {items.length === 0 ? <p className="phone-empty">{empty}</p> : <ul className="phone-list phone-attention-list">{items.map((item) => <Row key={`${item.kind}:${item.id}`} item={item} />)}</ul>}
    </section>
  );
}

export function HomeScreen() {
  const revision = useConnection((state) => state.revision);
  const link = useConnection((state) => state.link);
  const [debounced, setDebounced] = useState(revision);
  useEffect(() => {
    const timer = window.setTimeout(() => setDebounced(revision), DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [revision]);

  const home = useQuery({ queryKey: ["home", debounced], queryFn: getHome, placeholderData: (prev) => prev });

  if (!home.data) {
    return <p className="phone-empty">{home.isError ? "Could not load what needs you." : "Loading…"}</p>;
  }
  return (
    <>
      <header className="phone-page-heading"><span className="phone-connection"><i />{link === "connected" ? "Connected to your Mac" : "Last known workspace"}</span><h1>Away, not out of the loop.</h1><p>Here’s where you can move things forward.</p></header>
      {home.data.needs_you.length > 0 && <Section title="Needs you" items={home.data.needs_you} empty="" />}
      <Projects />
      {home.data.needs_you.length === 0 && <p className="phone-home-note"><PhoneIcon name="check" size={14} />Nothing needs your attention.</p>}
    </>
  );
}

function Projects() {
  const agents = useConnection((state) => state.agents);
  const revision = useConnection((state) => state.revision);
  const projects = useQuery({ queryKey: ["projects", revision], queryFn: () => phoneFetch<Record<string, { title: string; color: [number, number, number]; archived?: boolean }>>("/api/projects"), placeholderData: (previous) => previous });
  if (!projects.data) return <p className="phone-empty">Loading projects…</p>;
  const entries = deriveDashboardProjects(projects.data, agents);
  return <section className="phone-section" aria-label="Projects"><div className="phone-section-title"><h2>Projects</h2><span className="phone-count">{entries.filter((project) => !project.unavailable).length} active</span></div><ul className="phone-list phone-project-list">{entries.map((project) => <li key={project.id}><button type="button" className={`phone-row phone-project-row${project.unavailable ? " phone-project-unavailable" : ""}`} onClick={() => navigate(`/project/${encodeURIComponent(project.id)}`)}><span className="phone-project-mark" style={project.color ? { "--ad-project-accent": `rgb(${project.color.join(",")})` } as CSSProperties : undefined}><PhoneIcon name="folder" /></span><span className="phone-row-text"><span className="phone-row-title">{project.title}</span><span className="phone-row-reason">{project.unavailable ? "Project unavailable" : `${project.agents.length} agents · ${project.stateSummary}`}</span></span><span className="phone-project-count">{project.agents.length}</span><PhoneIcon name="arrow" size={16} /></button></li>)}</ul>{entries.length === 0 && <p className="phone-empty">No active projects. Configure a project on your Mac.</p>}</section>;
}
