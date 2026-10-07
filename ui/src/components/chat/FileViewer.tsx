import { useEffect, useRef, useState, type MouseEvent } from "react";
import { getFileContent } from "../../api/client";
import type { FileContent } from "../../api/types";
import { CodeBlock } from "./renderers/CodeBlock";
import { SanitizedMarkdown } from "./renderers/SanitizedMarkdown";
import { resolveFromFile, type FileLink } from "./renderers/filePath";
import type { AnnotationDraft } from "../../api/types";
import { clipAnnotationExcerpt } from "../../lib/annotations";
import { claimWebLink } from "../../lib/linkActions";

// The read-only, one-file viewer that opens beside the transcript (FS-03.R52).
// It holds no durable state: `?file=`/`?fileLine=` on the route are the open
// file's single source of truth (FS-03.R54), so this component is told which file
// to show and reports Close upward rather than owning either.

const MARKDOWN_LANGUAGES = new Set(["markdown"]);

type ViewState =
  | { status: "loading" }
  | { status: "error"; reason: string }
  | { status: "loaded"; file: FileContent; readAt: Date };

export function FileViewer({ agentId, link, onClose, onOpenFile, onPathResolved, onSelectionMenu, load }: {
  agentId: string;
  link: FileLink;
  /** Reads the file from another retained context, such as a Think Tank
   *  participant source; agentId then names that context (TS-14.R12). */
  load?: (path: string) => Promise<FileContent>;
  onClose: () => void;
  onOpenFile?: (link: FileLink) => void;
  onPathResolved?: (path: string) => void;
  onSelectionMenu?: (selection: { x: number; y: number; text: string; draft: AnnotationDraft; link?: string }) => void;
}) {
  const [view, setView] = useState<ViewState>({ status: "loading" });
  const [rendered, setRendered] = useState(true);
  const [reloadKey, setReloadKey] = useState(0);
  const bodyRef = useRef<HTMLDivElement>(null);
  // Per-read request token: opening another file, switching agents, or reloading
  // must not be overwritten by a slower earlier read landing afterwards — the
  // same guard FilesTab/CommandsTab already carry (TS-08.R57, INV §1).
  const readToken = useRef(0);
  const loadRef = useRef(load);
  loadRef.current = load;

  // Content is read when the file is opened and when Reload is chosen. There is
  // deliberately no watching and no polling, so the panel keeps showing the text
  // it read until someone reloads it (FS-03.R52).
  // The server returns the normalized path it actually read. Publishing that
  // spelling back to the route makes header, URL, reload, and annotation anchors
  // name the same file (FS-03.R64); the matching route change is not a new read.
  const republished = useRef<string | null>(null);
  useEffect(() => {
    if (republished.current === link.path) {
      republished.current = null;
      return;
    }
    republished.current = null;
    const token = ++readToken.current;
    setView({ status: "loading" });
    (loadRef.current ? loadRef.current(link.path) : getFileContent(agentId, link.path))
      .then((file) => {
        if (readToken.current !== token) return;
        setView({ status: "loaded", file, readAt: new Date() });
        if (file.path !== link.path && onPathResolved) {
          republished.current = file.path;
          onPathResolved(file.path);
        }
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

  const shownPath = view.status === "loaded" ? view.file.path : link.path;
  const isMarkdown = view.status === "loaded" && MARKDOWN_LANGUAGES.has(view.file.language);
  const showRendered = isMarkdown && rendered;
  // A rendered file's own links are relative to that file, not to the working
  // directory the server resolves against (FS-03.R52).
  const openFromFile = onOpenFile && view.status === "loaded"
    ? (target: FileLink) => onOpenFile({ ...target, path: resolveFromFile(view.file.path, target.path) })
    : onOpenFile;

  const openSelectionMenu = (event: MouseEvent<HTMLDivElement>) => {
    if (!onSelectionMenu || view.status !== "loaded") return;
    const selection = window.getSelection();
    const host = bodyRef.current;
    if (!selection || selection.rangeCount === 0 || !host || !selection.anchorNode || !selection.focusNode ||
      !host.contains(selection.anchorNode) || !host.contains(selection.focusNode)) return;
    const text = selection.toString();
    if (!text.trim()) return;
    const draft: AnnotationDraft = {
      anchor_kind: "file",
      path: view.file.path,
      excerpt: clipAnnotationExcerpt(text.trim()),
      instruction: "",
    };
    if (!showRendered) {
      const start = fileLineForNode(selection.getRangeAt(0).startContainer, host);
      const end = fileLineForNode(selection.getRangeAt(0).endContainer, host);
      if (!start || !end) return;
      draft.start_line = Math.min(start, end);
      draft.end_line = Math.max(start, end);
    }
    event.preventDefault();
    onSelectionMenu({ x: event.clientX, y: event.clientY, text, draft, link: claimWebLink(event) });
  };

  return (
    <aside className="file-viewer" data-ui="file-viewer" data-state={view.status}>
      <header className="file-viewer-header" data-slot="header">
        <span className="file-viewer-path" title={shownPath}>{shownPath}</span>
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
      <div className="file-viewer-body" data-slot="content" ref={bodyRef} onContextMenu={openSelectionMenu}>
        {view.status === "loading" && <p className="tab-placeholder">Reading…</p>}
        {view.status === "error" && <p className="file-viewer-error" role="alert">{view.reason}</p>}
        {view.status === "loaded" && (showRendered ? (
          <div className="file-viewer-rendered">
            <SanitizedMarkdown text={view.file.content} onOpenFile={openFromFile} />
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

function fileLineForNode(node: Node, host: HTMLElement): number | null {
  const element = node.nodeType === Node.ELEMENT_NODE ? node as Element : node.parentElement;
  const line = element?.closest<HTMLElement>("[data-file-line]");
  if (!line || !host.contains(line)) return null;
  const value = Number(line.dataset.fileLine);
  return value > 0 ? value : null;
}
