import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { searchArchive } from "../../api/client";
import type { ArchiveResult } from "../../api/types";
import { Badge } from "../../components/ui";

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
        <span>{result.backend} · {result.model}</span>
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
  const [q, setQ] = useState("");
  const debouncedQ = useDebounce(q, 250);
  const [results, setResults] = useState<ArchiveResult[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  const load = useCallback(async (query: string) => {
    if (abortRef.current) abortRef.current.abort();
    const ac = new AbortController();
    abortRef.current = ac;
    setLoading(true);
    setError(null);
    try {
      const resp = await searchArchive(query, 50, 0, ac.signal);
      if (!ac.signal.aborted) {
        setResults(resp.results ?? []);
        setTotal(resp.total);
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
    void load(debouncedQ);
  }, [debouncedQ, load]);

  const handleClick = (result: ArchiveResult) => {
    if (result.active) {
      navigate(`/agent/${result.agent_id}`);
    } else {
      navigate(`/archive/${result.agent_id}`);
    }
  };

  return (
    <section className="archive-page" data-ui="archive" data-state={error ? "error" : loading ? "loading" : results.length === 0 ? "empty" : undefined}>
      <div className="archive-header" data-slot="header">
        <h1>Archive</h1>
        <Link to="/">Back to Dashboard</Link>
      </div>
      <div className="archive-search" data-slot="search">
        <input
          type="search"
          placeholder="Search agents, roles, projects, transcript…"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          aria-label="Search archive"
        />
        {total > 0 && <span className="archive-count">{total} result{total !== 1 ? "s" : ""}</span>}
      </div>
      {error && <p className="archive-error">{error}</p>}
      {loading && results.length === 0 && <p className="archive-loading">Loading…</p>}
      {!loading && !error && results.length === 0 && (
        <p className="archive-empty">{q ? `No results for "${q}"` : "No sessions yet."}</p>
      )}
      <ul className="archive-list" data-slot="results">
        {results.map((r) => (
          <ArchiveRow key={r.agent_id} result={r} onClick={() => handleClick(r)} />
        ))}
      </ul>
    </section>
  );
}
