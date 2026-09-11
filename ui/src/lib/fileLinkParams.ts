import type { FileLink } from "../components/chat/renderers/filePath";

// `?file=` and `?fileLine=` are the open file's single source of truth on the
// agent and archived-agent routes (FS-03.R54). Reading and writing them lives
// here so both screens spell the address the same way, and so no store, context,
// or persisted browser key is added for state the URL already carries.

// fileLinkFromParams reads the open file out of the current query string. A
// `fileLine` that is not a positive integer is ignored rather than rejected: the
// file still opens, just without a line anchor.
export function fileLinkFromParams(params: URLSearchParams): FileLink | null {
  const path = params.get("file");
  if (!path) return null;
  const raw = Number(params.get("fileLine"));
  return Number.isInteger(raw) && raw > 0 ? { path, line: raw } : { path };
}

// writeFileLinkParams returns the next query string for an open file, or for
// Close when link is null. Every other parameter — `?tab=` above all — is
// preserved, and the two keys are always written together so a stale `fileLine`
// cannot outlive the file it cited.
export function writeFileLinkParams(params: URLSearchParams, link: FileLink | null): URLSearchParams {
  const next = new URLSearchParams(params);
  next.delete("file");
  next.delete("fileLine");
  if (link?.path) {
    next.set("file", link.path);
    if (link.line) next.set("fileLine", String(link.line));
  }
  return next;
}
