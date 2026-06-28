import ReactDiffViewer from "react-diff-viewer-continued";
import type { TranscriptEvent } from "../../../api/types";

export function DiffBlock({ event }: { event: TranscriptEvent }) {
  return (
    <article className="diff-block">
      <strong>{String(event.path ?? "diff")}</strong>
      <ReactDiffViewer oldValue={String(event.old_text ?? event.old ?? "")} newValue={String(event.new_text ?? event.new ?? "")} splitView={false} />
    </article>
  );
}
