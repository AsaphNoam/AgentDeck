import type { AnnotationBatch, AnnotationDraft } from "../api/types";

// Annotation capture limits shared by every surface that builds a draft
// (FS-13.R2). Two components clipped excerpts with their own copy of this
// function and had already drifted from the server's marker by two runes, so
// this is the one helper both use; the server's `clipAnnotationExcerpt`
// (`internal/server/sessions.go`) stays authoritative and re-clips on send.
export const annotationExcerptLimit = 2000;
export const annotationClippedMarker = "\n… [excerpt clipped]";

export function clipAnnotationExcerpt(value: string, limit = annotationExcerptLimit) {
  const chars = [...value];
  if (chars.length <= limit) return value;
  const marker = [...annotationClippedMarker];
  return chars.slice(0, limit - marker.length).join("") + annotationClippedMarker;
}

// annotationAnchor is the label a pending draft shows for where it points
// (FS-13.R22): file and line range for diff lines, otherwise the event.
export function annotationAnchor(draft: AnnotationDraft) {
  if (!draft.path) return `Event ${draft.seq}`;
  const range = draft.end_line && draft.end_line !== draft.start_line ? `–${draft.end_line}` : "";
  return `${draft.path}:${draft.start_line}${range}`;
}

// annotationBatch is the one request body every tray sends (FS-13.R4): the
// desktop tray and the phone form differ only in presentation (FS-20.R32).
export function annotationBatch(drafts: AnnotationDraft[], overall: string, agentId?: string): AnnotationBatch {
  return {
    annotations: drafts,
    overall_instruction: overall || undefined,
    target: agentId ? { kind: "agent", agent_id: agentId } : { kind: "self" },
  };
}

// The first line of the machine annotation block `runtime.FormatAnnotationBlock`
// writes (`internal/runtime/event.go`). It is the only part of that format the
// client borrows: the transcript store recognizes a self-targeted send's prompt
// by this prefix (FS-13.R23) instead of respelling the block's layout, which
// would drift the moment either side changed. `internal/server` pins the pair by
// reading this constant and asserting the emitted block still starts with it.
export const annotationBlockSentinel = "[AgentDeck annotations]";
