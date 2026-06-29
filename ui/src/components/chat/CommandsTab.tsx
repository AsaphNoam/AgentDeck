import { useEffect, useRef, useState } from "react";
import { getTrackedCommands } from "../../api/client";
import type { TrackedCommand } from "../../api/types";

function copyToClipboard(text: string) {
  void navigator.clipboard.writeText(text);
}

function CommandRow({ cmd }: { cmd: TrackedCommand }) {
  const failed = cmd.exit_status === "failed";
  return (
    <li className={`tracked-row ${failed ? "tracked-failed" : ""}`}>
      <div className="tracked-row-top">
        <code className="tracked-command">{cmd.command}</code>
        <button type="button" title="Copy command" onClick={() => copyToClipboard(cmd.command)}>
          Copy
        </button>
      </div>
      <div className="tracked-row-meta">
        <span className={`tracked-exit ${cmd.exit_status}`}>{cmd.exit_status}</span>
        {cmd.exit_error && <span className="tracked-exit-error">{cmd.exit_error}</span>}
        <span className="tracked-ts">{new Date(cmd.ts).toLocaleTimeString()}</span>
      </div>
    </li>
  );
}

export function CommandsTab({ agentId }: { agentId: string }) {
  const [commands, setCommands] = useState<TrackedCommand[]>([]);
  const [filter, setFilter] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const mountedRef = useRef(true);

  useEffect(() => {
    mountedRef.current = true;
    setLoading(true);
    setError(null);
    getTrackedCommands(agentId)
      .then((r) => { if (mountedRef.current) setCommands(r.commands); })
      .catch((e: unknown) => { if (mountedRef.current) setError(e instanceof Error ? e.message : "Failed"); })
      .finally(() => { if (mountedRef.current) setLoading(false); });
    return () => { mountedRef.current = false; };
  }, [agentId]);

  const filtered = filter
    ? commands.filter((c) => c.command.toLowerCase().includes(filter.toLowerCase()))
    : commands;

  if (loading) return <p className="tab-placeholder">Loading…</p>;
  if (error) return <p className="tab-error">{error}</p>;

  return (
    <div className="tracked-tab">
      <div className="tracked-filter">
        <input
          type="search"
          placeholder="Filter commands…"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          aria-label="Filter commands"
        />
        <span className="tracked-count">{filtered.length} command{filtered.length !== 1 ? "s" : ""}</span>
      </div>
      {filtered.length === 0 ? (
        <p className="tab-placeholder">{filter ? "No matches." : "No commands tracked yet."}</p>
      ) : (
        <ul className="tracked-list">
          {filtered.map((c) => (
            <CommandRow key={`${c.seq}-${c.tool_call_id}`} cmd={c} />
          ))}
        </ul>
      )}
    </div>
  );
}
