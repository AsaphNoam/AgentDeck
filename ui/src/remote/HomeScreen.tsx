import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { getHome, type AttentionItem } from "./api";
import { useConnection } from "./connection";
import { navigate } from "./router";
import { phoneFetch } from "./api";
import { deriveDashboardProjects } from "../features/dashboard/projectDashboardData";
const DEBOUNCE_MS = 400;

/** itemPath is where a Home row opens. */
export function itemPath(item: AttentionItem): string {
  if (item.kind === "task") return `/task/${encodeURIComponent(item.id)}`;
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
      <button type="button" className="phone-row" onClick={() => navigate(itemPath(item))}>
        <span className="phone-row-title">{item.title}</span>
        <span className="phone-row-reason">
          {item.reason}
          {stage ? ` · ${stage}` : ""}
          {item.outcome ? ` · ${item.outcome}` : ""}
        </span>
        <span className="phone-row-meta">
          {item.project} · {relative(item.since)}
        </span>
      </button>
    </li>
  );
}

function Section({ title, items, empty }: { title: string; items: AttentionItem[]; empty: string }) {
  return (
    <section className="phone-section" aria-label={title}>
      <h2>
        {title} <span className="phone-count">{items.length}</span>
      </h2>
      {items.length === 0 ? <p className="phone-empty">{empty}</p> : <ul className="phone-list">{items.map((item) => <Row key={`${item.kind}:${item.id}`} item={item} />)}</ul>}
    </section>
  );
}

export function HomeScreen() {
  const revision = useConnection((state) => state.revision);
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
      {home.data.needs_you.length > 0 && <Section title="Needs you" items={home.data.needs_you.filter((item) => item.kind !== "task")} empty="" />}
      <Projects />
    </>
  );
}

function Projects() {
  const agents = useConnection((state) => state.agents);
  const projects = useQuery({ queryKey: ["projects"], queryFn: () => phoneFetch<Record<string, { title: string; color: [number, number, number]; archived?: boolean }>>("/api/projects") });
  if (!projects.data) return <p className="phone-empty">Loading projects…</p>;
  const entries = deriveDashboardProjects(projects.data, agents);
  return <section className="phone-section" aria-label="Projects"><h2>Projects</h2><ul className="phone-list">{entries.map((project) => <li key={project.id}><button type="button" className="phone-row" onClick={() => navigate(`/project/${encodeURIComponent(project.id)}`)}><span className="phone-row-title">{project.title}</span><span className="phone-row-reason">{project.unavailable ? "Project unavailable" : `${project.agents.length} agents · ${project.stateSummary}`}</span></button></li>)}</ul></section>;
}
