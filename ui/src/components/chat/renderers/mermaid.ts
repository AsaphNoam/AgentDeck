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
// declarations into it, and the same author text can reach a `style` attribute, so any remote
// reference must be gone before the markup is inserted (FS-03.R38).
//
// CSS tokenization resolves identifier escapes, so `u\72 l(…)` is the same declaration as
// `url(…)`: the text is decoded before it is judged, never after. A carrier that still names a
// URL-bearing token once decoded is dropped whole rather than patched — patching a value the
// browser re-tokenizes is how the escape got through in the first place, and the core theme emits
// no such token, so nothing the product renders is lost (INV §8).
const URL_BEARING = /url\(|@import|image-set\(|src\(/i;

function decodeCSSEscapes(css: string): string {
  return css.replace(/\\(?:([0-9a-fA-F]{1,6})[ \t\n\r\f]?|([\s\S]))/g, (_match, hex: string | undefined, literal: string | undefined) => {
    if (hex === undefined) return literal ?? "";
    const code = parseInt(hex, 16);
    // CSS maps null, out-of-range, and surrogate escapes to U+FFFD, which carries
    // no token meaning — decoding them to anything else would be inventing text.
    return code === 0 || code > 0x10ffff || (code >= 0xd800 && code <= 0xdfff) ? "\ufffd" : String.fromCodePoint(code);
  });
}

function namesRemoteReference(css: string): boolean {
  return URL_BEARING.test(decodeCSSEscapes(css));
}

function stripRemoteStyleReferences(node: Node) {
  if (node.nodeName.toLowerCase() !== "style") return;
  if (namesRemoteReference(node.textContent ?? "")) node.textContent = "";
}

function stripRemoteInlineStyleReferences(node: Node) {
  if (!(node instanceof Element) || !node.hasAttribute("style")) return;
  if (namesRemoteReference(node.getAttribute("style") ?? "")) node.removeAttribute("style");
}

async function loadPurify(): Promise<Purify> {
  if (!purify) {
    const module = await import("dompurify");
    module.default.addHook("afterSanitizeElements", stripRemoteStyleReferences);
    module.default.addHook("afterSanitizeAttributes", stripRemoteInlineStyleReferences);
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
      // A draw-stage failure inside mermaid.render still throws (caught below), but without this
      // flag Mermaid also draws its own error SVG into the scratch node it appends to
      // document.body and leaves that node behind — a leak outside React's tree on every such
      // failure (FS-03.R38, INV §4).
      suppressErrorRendering: true,
    });
    if (!(await mermaid.parse(source, { suppressErrors: true }))) return { note: DIAGRAM_UNRENDERABLE };
    const { svg } = await mermaid.render(id, source);
    return { svg: sanitizeDiagram(svg, sanitizer) };
  } catch {
    return { note: DIAGRAM_UNRENDERABLE };
  }
}
