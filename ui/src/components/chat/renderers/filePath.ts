// The one place a link target is classified as a local filesystem path and its
// `:line`/`:line:col` suffix parsed (FS-03.R51). Both the assistant renderer and
// any other surface that upgrades a link go through here, so "what counts as a
// file link" has a single spelling.

export type FileLink = { path: string; line?: number };

// A scheme other than `file:` means the browser already knows what to do with the
// link. `mailto:`, `http:`, and `https:` are the ones that actually appear, but
// matching the scheme grammar rather than a list keeps an unforeseen one
// (`vscode:`, `slack:`) out of the viewer instead of feeding it a nonsense path.
const SCHEME = /^[a-z][a-z0-9+.-]*:/i;

// A trailing `:12`, `:12:3`, or Codex's `:12-40` cites a line; the column or range
// end is ignored and the viewer marks the first cited line.
const LINE_SUFFIX = /:(\d+)(?:[:-]\d+)?$/;

// `README.md:12` matches the scheme grammar, but a "scheme" followed only by a line
// suffix is a bare file name citing a line, not a URL.
const LINE_ONLY = /^\d+(?:[:-]\d+)?$/;

// classifyFileLink returns the file the link names, or null when the link is not
// a local path and must keep its ordinary browser behavior.
export function classifyFileLink(href: string | undefined): FileLink | null {
  const raw = (href ?? "").trim();
  if (!raw) return null;
  // In-page anchors and protocol-relative URLs are not paths.
  if (raw.startsWith("#") || raw.startsWith("//")) return null;

  let target = raw;
  const scheme = SCHEME.exec(target);
  if (scheme && !LINE_ONLY.test(target.slice(scheme[0].length))) {
    if (!/^file:/i.test(target)) return null;
    target = fromFileURL(target);
    if (!target) return null;
  }

  // A query string or fragment on a local path is not part of the file's name.
  target = target.split(/[?#]/, 1)[0];
  if (!target) return null;

  const match = LINE_SUFFIX.exec(target);
  if (!match) return { path: target };
  const line = Number(match[1]);
  const path = target.slice(0, match.index);
  if (!path) return { path: target };
  return line > 0 ? { path, line } : { path };
}

// fromFileURL turns `file:///a/b.go` or `file://localhost/a/b.go` into `/a/b.go`,
// decoding percent-escapes. Only an empty or `localhost` authority names this
// machine; any other host, or a malformed URL, yields "" and stays a plain link.
function fromFileURL(url: string): string {
  let path = url.replace(/^file:/i, "");
  if (path.startsWith("//")) {
    const authority = /^\/\/([^/?#]*)(?=\/|$)/.exec(path);
    if (!authority || !/^(localhost)?$/i.test(authority[1])) return "";
    path = path.slice(authority[0].length);
  }
  try {
    return decodeURIComponent(path);
  } catch {
    return "";
  }
}

// resolveFromFile resolves a link found inside a rendered file against that file's
// own directory, as Markdown does (FS-03.R52). `fromFile` is the viewer's
// working-directory-relative path. An absolute target passes through; a target
// that climbs above the working directory keeps its leading `..` so the server
// still refuses it rather than the client silently clamping it.
export function resolveFromFile(fromFile: string, target: string): string {
  if (target.startsWith("/")) return target;
  const parts = fromFile.split("/").slice(0, -1);
  for (const segment of target.split("/")) {
    if (segment === "" || segment === ".") continue;
    if (segment === ".." && parts.length > 0 && parts[parts.length - 1] !== "..") parts.pop();
    else parts.push(segment);
  }
  return parts.join("/");
}

// formatFileLabel renders the path with its cited line, which is how a link and
// the viewer header both name the same file.
export function formatFileLabel(link: FileLink): string {
  return link.line ? `${link.path}:${link.line}` : link.path;
}
