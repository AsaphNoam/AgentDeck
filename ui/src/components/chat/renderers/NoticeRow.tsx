import type { TranscriptEvent } from "../../../api/types";
import { Badge } from "../../ui";

// A runtime advisory (FS-03.R68, TS-08.R86): one compact row, never a message
// bubble. The server already bounds the text and maps unknown severity to info.
export function NoticeRow({ event }: { event: TranscriptEvent }) {
  const warning = event.severity === "warning";
  const description = String(event.description ?? "");
  return (
    <div className="notice-row" role="note">
      <Badge variant={warning ? "warning" : "neutral"}>{warning ? "warning" : "info"}</Badge>
      <span className="notice-title">{String(event.title ?? "")}</span>
      {description ? <span className="notice-description">{description}</span> : null}
    </div>
  );
}
