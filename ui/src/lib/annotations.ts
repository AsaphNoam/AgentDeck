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

// The first line of the machine annotation block `runtime.FormatAnnotationBlock`
// writes (`internal/runtime/event.go`). It is the only part of that format the
// client borrows: the transcript store recognizes a self-targeted send's prompt
// by this prefix (FS-13.R23) instead of respelling the block's layout, which
// would drift the moment either side changed. `internal/server` pins the pair by
// reading this constant and asserting the emitted block still starts with it.
export const annotationBlockSentinel = "[AgentDeck annotations]";
