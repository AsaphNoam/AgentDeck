import ReactDiffViewer from "react-diff-viewer-continued";
import type { TranscriptEvent } from "../../../api/types";
import { diffTheme } from "../../../presentation/integrations";

export function DiffBlock({ event }: { event: TranscriptEvent }) {
  return (
    <article className="diff-block" data-ui="transcript" data-variant="diff">
      <strong>{String(event.path ?? "diff")}</strong>
      <ReactDiffViewer oldValue={String(event.old_text ?? event.old ?? "")} newValue={String(event.new_text ?? event.new ?? "")} splitView={false} styles={diffTheme} />
    </article>
  );
}
