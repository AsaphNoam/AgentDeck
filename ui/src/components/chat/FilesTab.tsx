import { useEffect, useRef, useState } from "react";
import { getTrackedFiles } from "../../api/client";
import type { TrackedFile } from "../../api/types";

function copyToClipboard(text: string) {
  void navigator.clipboard.writeText(text);
}

function FileRow({ file, onDiffClick }: { file: TrackedFile; onDiffClick: (seq: number) => void }) {
  return (
    <li className="tracked-row" data-ui="tracked-list" data-slot="row" data-variant="files">
      <div className="tracked-row-top" data-slot="metadata">
        <span className="tracked-path">{file.path}</span>
        <div className="tracked-row-actions" data-slot="actions">
          <button
            type="button"
            title="Copy path"
            onClick={() => copyToClipboard(file.path)}
          >
            Copy
          </button>
          {file.has_diff && file.diff_refs.length > 0 && (
            <button
              type="button"
              title="View diff in transcript"
              onClick={() => onDiffClick(file.diff_refs[0].seq)}
            >
              Diff
            </button>
          )}
        </div>
      </div>
      <div className="tracked-row-meta">
        <span>{file.edit_count} edit{file.edit_count !== 1 ? "s" : ""}</span>
        {file.has_diff && <span className="tracked-has-diff">has diff</span>}
      </div>
    </li>
  );
}

export function FilesTab({ agentId, onReveal }: { agentId: string; onReveal?: (seq: number) => void }) {
  const [files, setFiles] = useState<TrackedFile[]>([]);
  const [filter, setFilter] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const mountedRef = useRef(true);

  useEffect(() => {
    mountedRef.current = true;
    setLoading(true);
    setError(null);
    getTrackedFiles(agentId)
      .then((r) => { if (mountedRef.current) setFiles(r.files); })
      .catch((e: unknown) => { if (mountedRef.current) setError(e instanceof Error ? e.message : "Failed"); })
      .finally(() => { if (mountedRef.current) setLoading(false); });
    return () => { mountedRef.current = false; };
  }, [agentId]);

  const filtered = filter
    ? files.filter((f) => f.path.toLowerCase().includes(filter.toLowerCase()))
    : files;

  // Prefer the parent's reveal (switches to the transcript tab first, since its
  // content is unmounted while the Files tab is active); fall back to an in-place
  // scroll if used standalone.
  const handleDiff = (seq: number) => {
    if (onReveal) {
      onReveal(seq);
      return;
    }
    const el = document.querySelector(`[data-seq="${seq}"]`);
    if (el) el.scrollIntoView({ behavior: "smooth", block: "center" });
  };

  if (loading) return <p className="tab-placeholder" data-ui="tracked-list" data-state="loading" data-variant="files">Loading…</p>;
  if (error) return <p className="tab-error" data-ui="tracked-list" data-state="error" data-variant="files">{error}</p>;

  return (
    <div className="tracked-tab" data-ui="tracked-list" data-state={filtered.length === 0 ? "empty" : undefined} data-variant="files">
      <div className="tracked-filter" data-slot="filter">
        <input
          type="search"
          placeholder="Filter files…"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          aria-label="Filter files"
        />
        <span className="tracked-count" data-slot="count">{filtered.length} file{filtered.length !== 1 ? "s" : ""}</span>
      </div>
      {filtered.length === 0 ? (
        <p className="tab-placeholder">{filter ? "No matches." : "No files tracked yet."}</p>
      ) : (
        <ul className="tracked-list" data-slot="items">
          {filtered.map((f) => (
            <FileRow key={f.path} file={f} onDiffClick={handleDiff} />
          ))}
        </ul>
      )}
    </div>
  );
}
