import type { TranscriptEvent } from "../../../api/types";
import { SanitizedMarkdown } from "./SanitizedMarkdown";
import type { FileLink } from "./filePath";

export function AssistantText({ event, onOpenFile }: { event: TranscriptEvent; onOpenFile?: (link: FileLink) => void }) {
  const text = String(event.text ?? event.delta ?? "");
  return (
    <article className="message assistant-message" data-ui="transcript" data-variant="assistant">
      <SanitizedMarkdown text={text} onOpenFile={onOpenFile} />
    </article>
  );
}
