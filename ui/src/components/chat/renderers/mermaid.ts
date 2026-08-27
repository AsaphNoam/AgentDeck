import { mermaidTheme } from "../../../presentation/integrations";
import type { PresentationColors } from "../../../presentation/resolveColors";

// FS-03.R38 / TS-08.R40: a fixed host-owned source bound, checked before the library is loaded.
// Mermaid renders on the main thread and cannot be interrupted there, so the input is bounded
// instead of the elapsed time.
export const DIAGRAM_SOURCE_LIMIT = 50_000;

// Mermaid's flowchart image-node grammar (`A@{ img: "…" }`) loads its URL while the library builds
// the SVG, before any returned markup can be sanitized. The grammar is refused at the preflight, so
// `parse` and `render` never see it. This is a deliberately unsupported Mermaid feature.
const IMAGE_NODE = /@\{[^}]*\bimg\s*:/;

export const DIAGRAM_TOO_LARGE = "This diagram is too large to render, so its source is shown instead.";
export const DIAGRAM_UNSUPPORTED = "Diagrams that load an external image are not supported, so the source is shown instead.";
export const DIAGRAM_UNRENDERABLE = "This diagram could not be rendered, so its source is shown instead.";

export type DiagramResult = { svg: string } | { note: string };

type Purify = (typeof import("dompurify"))["default"];

let purify: Purify | null = null;

// Mermaid emits one `<style>` element carrying the theme CSS. `classDef` puts author-controlled
// declarations into it, so any remote reference is stripped to keep rendering request-free
// (FS-03.R38); the core theme never emits one.
function stripRemoteStyleReferences(node: Node) {
  if (node.nodeName.toLowerCase() !== "style") return;
  node.textContent = (node.textContent ?? "").replace(/@import[^;]*;?|url\([^)]*\)?/gi, "");
}

async function loadPurify(): Promise<Purify> {
  if (!purify) {
    const module = await import("dompurify");
    module.default.addHook("afterSanitizeElements", stripRemoteStyleReferences);
    purify = module.default;
  }
  return purify;
}

function sanitizeDiagram(svg: string, sanitizer: Purify): string {
  return sanitizer.sanitize(svg, {
    USE_PROFILES: { svg: true, svgFilters: true, html: true },
    FORBID_TAGS: ["script", "image", "foreignObject", "iframe", "use", "a"],
    FORBID_ATTR: ["href", "xlink:href", "src", "srcset", "xlink:show", "xlink:actuate"],
  });
}

/**
 * Renders untrusted diagram source with the library's interactivity disabled and sanitizes the
 * markup it returns (FS-03.R38). The library is loaded on demand so it stays out of the initial
 * bundle (TS-08.R40). A refusal returns the note the caller shows beside the ordinary code block.
 */
export async function renderDiagram(source: string, colors: PresentationColors, id: string): Promise<DiagramResult> {
  if (source.length > DIAGRAM_SOURCE_LIMIT) return { note: DIAGRAM_TOO_LARGE };
  if (IMAGE_NODE.test(source)) return { note: DIAGRAM_UNSUPPORTED };

  const [{ default: mermaid }, sanitizer] = await Promise.all([import("mermaid"), loadPurify()]);

  try {
    // Initialization is host-owned and inside the guard: unresolvable presentation values or a
    // library failure must leave the code block standing, never break the surrounding transcript.
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: "strict",
      htmlLabels: false,
      flowchart: { htmlLabels: false },
      theme: "base",
      themeVariables: mermaidTheme(colors),
      fontFamily: colors.fontFamily,
    });
    if (!(await mermaid.parse(source, { suppressErrors: true }))) return { note: DIAGRAM_UNRENDERABLE };
    const { svg } = await mermaid.render(id, source);
    return { svg: sanitizeDiagram(svg, sanitizer) };
  } catch {
    return { note: DIAGRAM_UNRENDERABLE };
  }
}
