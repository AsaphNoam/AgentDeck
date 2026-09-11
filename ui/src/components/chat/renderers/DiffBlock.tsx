import { useState } from "react";
import ReactDiffViewer from "react-diff-viewer-continued";
import type { AnnotationDraft, TranscriptEvent } from "../../../api/types";
import { clipAnnotationExcerpt } from "../../../lib/annotations";
import { diffTheme } from "../../../presentation/integrations";
import type { FileLink } from "./filePath";

export function DiffBlock({ event, onAnnotate, onOpenFile }: { event: TranscriptEvent; onAnnotate: (draft: AnnotationDraft) => void; onOpenFile?: (link: FileLink) => void }) {
  const [selection, setSelection] = useState<{ side: "old" | "new"; start: number; end: number } | null>(null);
  const chooseLine = (lineId: string) => {
    const match = /^([LR])-(\d+)$/.exec(lineId);
    if (!match) return;
    const side = match[1] === "L" ? "old" : "new";
    const line = Number(match[2]);
    setSelection((current) => current?.side === side ? { ...current, end: line } : { side, start: line, end: line });
  };
  const addSelection = () => {
    if (!selection || event.seq == null) return;
    const start = Math.min(selection.start, selection.end);
    const end = Math.max(selection.start, selection.end);
    const text = selection.side === "old" ? String(event.old_text ?? event.old ?? "") : String(event.new_text ?? event.new ?? "");
    const excerpt = text.split("\n").slice(start - 1, end).join("\n");
    onAnnotate({ seq: Number(event.seq), path: String(event.path ?? "diff"), side: selection.side, start_line: start, end_line: end, excerpt: clipAnnotationExcerpt(excerpt), instruction: "" });
    setSelection(null);
  };
  return (
    <article className="diff-block" data-ui="transcript" data-variant="diff">
      <div className="diff-heading">{/* The heading path opens the same viewer a chat file link does, while the
          diff keeps its own content and line-selection behavior (FS-05.R37). */}
        {onOpenFile && event.path ? (
          <button type="button" className="file-link" data-file-path={String(event.path)} onClick={() => onOpenFile({ path: String(event.path) })}><strong>{String(event.path)}</strong></button>
        ) : (
          <strong>{String(event.path ?? "diff")}</strong>
        )}<small>Click line numbers to select a range.</small>{selection && <button type="button" className="annotation-event-trigger" onClick={addSelection}>Annotate lines {Math.min(selection.start, selection.end)}–{Math.max(selection.start, selection.end)}</button>}</div>
      <ReactDiffViewer oldValue={String(event.old_text ?? event.old ?? "")} newValue={String(event.new_text ?? event.new ?? "")} splitView={false} styles={diffTheme} onLineNumberClick={chooseLine} />
    </article>
  );
}
