import { useEffect, useRef, useState } from "react";
import { getFileContent } from "../../api/client";
import type { FileContent } from "../../api/types";
import { CodeBlock } from "./renderers/CodeBlock";
import { SanitizedMarkdown } from "./renderers/SanitizedMarkdown";
import type { FileLink } from "./renderers/filePath";

// The read-only, one-file viewer that opens beside the transcript (FS-03.R52).
// It holds no durable state: `?file=`/`?fileLine=` on the route are the open
// file's single source of truth (FS-03.R54), so this component is told which file
// to show and reports Close upward rather than owning either.

const MARKDOWN_LANGUAGES = new Set(["markdown"]);

type ViewState =
  | { status: "loading" }
  | { status: "error"; reason: string }
  | { status: "loaded"; file: FileContent; readAt: Date };

export function FileViewer({ agentId, link, onClose, onOpenFile }: {
  agentId: string;
  link: FileLink;
  onClose: () => void;
  onOpenFile?: (link: FileLink) => void;
}) {
  const [view, setView] = useState<ViewState>({ status: "loading" });
  const [rendered, setRendered] = useState(true);
  const [reloadKey, setReloadKey] = useState(0);
  const bodyRef = useRef<HTMLDivElement>(null);
  // Per-read request token: opening another file, switching agents, or reloading
  // must not be overwritten by a slower earlier read landing afterwards — the
  // same guard FilesTab/CommandsTab already carry (TS-08.R57, INV §1).
  const readToken = useRef(0);

  // Content is read when the file is opened and when Reload is chosen. There is
  // deliberately no watching and no polling, so the panel keeps showing the text
  // it read until someone reloads it (FS-03.R52).
  useEffect(() => {
    const token = ++readToken.current;
    setView({ status: "loading" });
    getFileContent(agentId, link.path)
      .then((file) => {
        if (readToken.current !== token) return;
        setView({ status: "loaded", file, readAt: new Date() });
      })
      .catch((error: unknown) => {
        if (readToken.current !== token) return;
        setView({ status: "error", reason: error instanceof Error ? error.message : "That file could not be read." });
      });
  }, [agentId, link.path, reloadKey]);

  // A cited line is found rather than hunted for: scroll it into view and mark it
  // once the highlighted body for this read has mounted (FS-03.R52).
  useEffect(() => {
    if (view.status !== "loaded" || !link.line) return;
    const host = bodyRef.current;
    if (!host) return;
    const target = host.querySelector(`[data-file-line="${link.line}"]`);
    if (target) target.scrollIntoView({ block: "center" });
    else host.scrollTop = 0;
  }, [view, link.line, rendered]);

  const isMarkdown = view.status === "loaded" && MARKDOWN_LANGUAGES.has(view.file.language);
  const showRendered = isMarkdown && rendered;

  return (
    <aside className="file-viewer" data-ui="file-viewer" data-state={view.status}>
      <header className="file-viewer-header" data-slot="header">
        <span className="file-viewer-path" title={link.path}>{link.path}</span>
        <div className="file-viewer-actions" data-slot="actions">
          {isMarkdown && (
            <div className="file-viewer-form" role="group" aria-label="Markdown display">
              <button type="button" aria-pressed={rendered} onClick={() => setRendered(true)}>Rendered</button>
              <button type="button" aria-pressed={!rendered} onClick={() => setRendered(false)}>Source</button>
            </div>
          )}
          <button type="button" onClick={() => setReloadKey((key) => key + 1)}>Reload</button>
          <button type="button" onClick={onClose}>Close</button>
        </div>
      </header>
      {view.status === "loaded" && (
        <p className="file-viewer-meta" data-slot="metadata">
          {view.file.truncated
            ? `Showing the first ${view.file.line_count} lines — this file is larger than the viewer reads.`
            : `${view.file.line_count} line${view.file.line_count === 1 ? "" : "s"}`}
          {" · read "}
          <time dateTime={view.readAt.toISOString()}>{view.readAt.toLocaleTimeString()}</time>
        </p>
      )}
      <div className="file-viewer-body" data-slot="content" ref={bodyRef}>
        {view.status === "loading" && <p className="tab-placeholder">Reading…</p>}
        {view.status === "error" && <p className="file-viewer-error" role="alert">{view.reason}</p>}
        {view.status === "loaded" && (showRendered ? (
          <div className="file-viewer-rendered">
            <SanitizedMarkdown text={view.file.content} onOpenFile={onOpenFile} />
          </div>
        ) : (
          <CodeBlock language={view.file.language || "text"} showLineNumbers markedLine={link.line}>
            {view.file.content}
          </CodeBlock>
        ))}
      </div>
    </aside>
  );
}
