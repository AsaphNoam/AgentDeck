import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { getHome, type AttentionItem } from "./api";
import { useConnection } from "./connection";
import { navigate } from "./router";

const SEEN_KEY = "agentdeck.homeSeenAt";
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
  // "Since you last looked" compares with the previous time this phone opened
  // Home; that time lives only in the phone's storage (TS-13.R10).
  const [since] = useState(() => {
    const previous = localStorage.getItem(SEEN_KEY) ?? new Date(Date.now() - 24 * 3600_000).toISOString();
    localStorage.setItem(SEEN_KEY, new Date().toISOString());
    return previous;
  });
  const revision = useConnection((state) => state.revision);
  const [debounced, setDebounced] = useState(revision);
  useEffect(() => {
    const timer = window.setTimeout(() => setDebounced(revision), DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [revision]);

  const home = useQuery({ queryKey: ["home", since, debounced], queryFn: () => getHome(since), placeholderData: (prev) => prev });

  const moving = useMemo(() => {
    const groups = new Map<string, AttentionItem[]>();
    for (const item of home.data?.moving ?? []) groups.set(item.project, [...(groups.get(item.project) ?? []), item]);
    return [...groups.entries()];
  }, [home.data]);

  if (!home.data) {
    return <p className="phone-empty">{home.isError ? "Could not load what needs you." : "Loading…"}</p>;
  }
  return (
    <>
      <Section title="Needs you" items={home.data.needs_you} empty="Nothing needs you right now." />
      <section className="phone-section" aria-label="Moving">
        <h2>
          Moving <span className="phone-count">{home.data.moving.length}</span>
        </h2>
        {moving.length === 0 && <p className="phone-empty">Nothing is running.</p>}
        {moving.map(([project, items]) => (
          <div key={project} className="phone-group">
            <h3>{project}</h3>
            <ul className="phone-list">
              {items.map((item) => (
                <Row key={`${item.kind}:${item.id}`} item={item} />
              ))}
            </ul>
          </div>
        ))}
      </section>
      <Section title="Since you last looked" items={home.data.since_last} empty="Nothing finished since you last looked." />
    </>
  );
}
