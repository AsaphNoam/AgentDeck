import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { restoreProject, searchArchive, searchArchiveProject } from "../../api/client";
import { useUiStore } from "../../store/uiStore";
import type { ArchiveProjectGroup, ArchiveResult } from "../../api/types";
import { Badge } from "../../components/ui";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { QUERY_KEYS } from "../../api/config";
import type { ProjectResponse } from "../../schemas/project";

function useDebounce<T>(value: T, delay: number): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const t = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(t);
  }, [value, delay]);
  return debounced;
}

function StateBadge({ active }: { active: boolean }) {
  return (
    <Badge className={`state-badge ${active ? "idle" : "done"}`} variant={active ? "info" : "success"} indicator>
      {active ? "active" : "inactive"}
    </Badge>
  );
}

function ArchiveRow({ result, onClick }: { result: ArchiveResult; onClick: () => void }) {
  return (
    <li className="archive-row" data-slot="result" data-state={result.active ? "active" : "inactive"} role="button" tabIndex={0} onClick={onClick} onKeyDown={(e) => e.key === "Enter" && onClick()}>
      <div className="archive-row-top" data-slot="metadata">
        <span className="archive-name">{result.name}</span>
        <StateBadge active={result.active} />
      </div>
      <div className="archive-row-sub" data-slot="metadata">
        <span>{result.role} · {result.project}</span>
        <span>{result.archived ? "archived" : result.active ? "running" : "stopped"}</span>
        <span>{[result.backend, result.model, result.effort].filter(Boolean).join(" · ")}</span>
      </div>
      {result.snippet && (
        <p className="archive-snippet" data-slot="snippet">…{result.snippet}…</p>
      )}
      {result.matched_in && result.matched_in.length > 0 && (
        <div className="archive-match-tags" data-slot="tags">
          {result.matched_in.map((m) => (
            <span key={m} className="archive-match-tag">{m}</span>
          ))}
        </div>
      )}
      <div className="archive-row-meta" data-slot="metadata">
        <span>{result.turn_count} turns</span>
        <span>{result.files_touched} files</span>
        <span>{result.commands_run} cmds</span>
        <span className="archive-updated">updated {new Date(result.updated_at).toLocaleString()}</span>
      </div>
    </li>
  );
}

export function ArchivePage() {
  const navigate = useNavigate();
  const pushError = useUiStore((state) => state.pushError);
  const queryClient = useQueryClient();
  const [q, setQ] = useState("");
  const debouncedQ = useDebounce(q, 250);
  const [results, setResults] = useState<ArchiveProjectGroup[]>([]);
  const [total, setTotal] = useState(0);
  const [searchMode, setSearchMode] = useState<"full_text" | "metadata" | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const handleProjectError = useCallback((message: string) => {
    pushError("Load project archive failed", message);
  }, [pushError]);

  const load = useCallback(async (query: string, offset: number, append: boolean, rendered: ArchiveProjectGroup[] = []) => {
    if (abortRef.current) abortRef.current.abort();
    const ac = new AbortController();
    abortRef.current = ac;
    setLoading(true);
    setError(null);
    try {
      const resp = await searchArchive(query, 50, offset, ac.signal);
      const raw = resp.results ?? [];
      // Retain a defensive compatibility projection for an older server during
      // a rolling upgrade; the grouped API is authoritative for new responses.
      let page = normalizeGroups(raw);
      if (append) {
        const seen = new Set(rendered.map((group) => group.project));
        const duplicate = page.some((group) => seen.has(group.project));
        if (duplicate) {
          const head = await searchArchive(query, 50, 0, ac.signal);
          page = mergeGroups(normalizeGroups(head.results ?? []), page);
        }
      }
      if (!ac.signal.aborted) {
        setResults((current) => append ? mergeGroups(current, page) : page);
        setTotal(resp.total);
        setSearchMode(resp.search_mode === "full_text" ? "full_text" : "metadata");
      }
    } catch (err: unknown) {
      if (!ac.signal.aborted) {
        setError(err instanceof Error ? err.message : "Failed to load archive");
      }
    } finally {
      if (!ac.signal.aborted) setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load(debouncedQ, 0, false);
  }, [debouncedQ, load]);

  const restore = useMutation({
    mutationFn: restoreProject,
    onSuccess: (project) => {
      queryClient.setQueryData<Record<string, Omit<ProjectResponse, "project">>>(QUERY_KEYS.projects, (current) => (
        current ? { ...current, [project.project]: { ...current[project.project], ...project, archived: false } } : current
      ));
      void queryClient.invalidateQueries({ queryKey: QUERY_KEYS.projects });
      setResults((current) => current.map((group) => (
        group.project === project.project ? { ...group, project_status: "active" } : group
      )));
      void load(debouncedQ, 0, false);
    },
    onError: (err) => pushError("Restore project failed", err instanceof Error ? err.message : String(err)),
  });

  const handleClick = (result: ArchiveResult) => {
    navigate(result.archived ? `/archive/${result.agent_id}` : `/agent/${result.agent_id}`);
  };

  const searchPlaceholder = searchMode === "full_text"
    ? "Search agents, roles, projects, transcript…"
    : searchMode === "metadata"
      ? "Search agents, roles, projects, backends…"
      : "Search archive…";

  return (
    <section className="archive-page" data-ui="archive" data-state={error ? "error" : loading ? "loading" : results.length === 0 ? "empty" : undefined}>
      <div className="archive-header" data-slot="header">
        <h1>Archive</h1>
        <Link to="/">Back to Dashboard</Link>
      </div>
      <div className="archive-search" data-slot="search">
        <input
          type="search"
          placeholder={searchPlaceholder}
          value={q}
          onChange={(e) => setQ(e.target.value)}
          aria-label="Search archive"
          aria-describedby="archive-search-scope"
        />
        {total > 0 && <span className="archive-count">{total} result{total !== 1 ? "s" : ""}</span>}
      </div>
      <p className="form-info" id="archive-search-scope">
        {searchMode === "full_text"
          ? "All terms must match within one metadata record, transcript turn, or annotation; search does not combine text across turns."
          : searchMode === "metadata"
            ? "This build searches session metadata only; transcript text and snippets are unavailable."
            : "Loading search capabilities…"}
      </p>
      {error && <p className="archive-error">{error}</p>}
      {loading && results.length === 0 && <p className="archive-loading">Loading…</p>}
      {!loading && !error && results.length === 0 && (
        <p className="archive-empty">{q ? `No results for "${q}"` : "No archived agents yet. Stopped agents remain on their project dashboard."}</p>
      )}
      <ul className="archive-list" data-slot="results">
        {results.map((group) => (
          <ArchiveProjectRows
            key={group.project}
            group={group}
            query={debouncedQ}
            onClick={handleClick}
            onError={handleProjectError}
            onRestore={() => restore.mutate(group.project)}
            restoring={restore.isPending && restore.variables === group.project}
          />
        ))}
      </ul>
      {results.length < total && (
        <div className="form-actions">
          <button type="button" disabled={loading} onClick={() => void load(debouncedQ, results.length, true, results)}>
            {loading ? "Loading…" : "Load more"}
          </button>
        </div>
      )}
    </section>
  );
}

function ArchiveProjectRows({
  group, query, onClick, onError, onRestore, restoring,
}: {
  group: ArchiveProjectGroup;
  query: string;
  onClick: (result: ArchiveResult) => void;
  onError: (message: string) => void;
  onRestore: () => void;
  restoring: boolean;
}) {
  const [results, setResults] = useState<ArchiveResult[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const resultsRef = useRef(results);
  const abortRef = useRef<AbortController | null>(null);
  const requestGeneration = useRef(0);

  useEffect(() => {
    resultsRef.current = results;
  }, [results]);

  const load = useCallback(async (offset: number, append: boolean) => {
    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;
    const generation = ++requestGeneration.current;
    const isCurrent = () => generation === requestGeneration.current && !ac.signal.aborted;
    setLoading(true);
    setError(null);
    try {
      const page = await searchArchiveProject(group.project, query, 50, offset, ac.signal);
      let incoming = page.results ?? [];
      if (append && incoming.some((result) => resultsRef.current.some((current) => current.agent_id === result.agent_id))) {
        const head = await searchArchiveProject(group.project, query, 50, 0, ac.signal);
        incoming = mergeAgentRows(head.results ?? [], incoming);
      }
      if (isCurrent()) {
        setResults((current) => append ? mergeAgentRows(current, incoming) : incoming);
        setTotal(page.total);
      }
    } catch (err: unknown) {
      if (isCurrent()) {
        const message = err instanceof Error ? err.message : "Failed to load project archive";
        setError(message);
        onError(message);
      }
    } finally {
      if (isCurrent()) setLoading(false);
    }
  }, [group.project, onError, query]);

  useEffect(() => {
    setResults([]);
    setTotal(0);
    setError(null);
    void load(0, false);
    return () => {
      requestGeneration.current += 1;
      abortRef.current?.abort();
    };
  }, [group.project, load, query]);

  return (
    <li className="archive-project-group">
      <h2>
        {group.title} <small>{group.project_status} · {group.archived_agent_count} archived</small>
        {group.project_status === "archived" && <button type="button" disabled={restoring} onClick={onRestore}>{restoring ? "Restoring…" : "Restore project"}</button>}
      </h2>
      <ul>{results.map((result) => <ArchiveRow key={result.agent_id} result={result} onClick={() => onClick(result)} />)}</ul>
      {error && (
        <div className="form-error" role="alert">
          <p>Could not load this project's archived agents.</p>
          <button type="button" disabled={loading} onClick={() => void load(results.length, results.length > 0)}>
            {loading ? "Retrying…" : "Retry"}
          </button>
        </div>
      )}
      {results.length < total && (
        <div className="form-actions">
          <button type="button" disabled={loading} onClick={() => void load(results.length, true)}>{loading ? "Loading…" : "Load more agents"}</button>
        </div>
      )}
    </li>
  );
}

function normalizeGroups(raw: ArchiveProjectGroup[]): ArchiveProjectGroup[] {
  if (raw.length === 0 || Array.isArray(raw[0]?.results)) return raw.map((group) => ({ ...group, results: group.results ?? [] }));
  const groups = new Map<string, ArchiveProjectGroup>();
  for (const result of raw as unknown as ArchiveResult[]) {
    const group = groups.get(result.project) ?? { project: result.project, title: result.project, color: [128, 128, 128], project_status: "active" as const, archived_agent_count: 0, results: [] };
    group.results.push(result);
    groups.set(result.project, group);
  }
  return [...groups.values()];
}

function mergeGroups(current: ArchiveProjectGroup[], page: ArchiveProjectGroup[]): ArchiveProjectGroup[] {
  const merged = new Map(current.map((group) => [group.project, { ...group, results: [...group.results] }]));
  for (const incoming of page) {
    const group = merged.get(incoming.project) ?? { ...incoming, results: [] };
    const ids = new Set(group.results.map((result) => result.agent_id));
    group.results.push(...incoming.results.filter((result) => !ids.has(result.agent_id)));
    merged.set(incoming.project, group);
  }
  return [...merged.values()];
}

function mergeAgentRows(current: ArchiveResult[], page: ArchiveResult[]) {
  const known = new Set(current.map((result) => result.agent_id));
  return [...current, ...page.filter((result) => !known.has(result.agent_id))];
}
