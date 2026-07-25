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
