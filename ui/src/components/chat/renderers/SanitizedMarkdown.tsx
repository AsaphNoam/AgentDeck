import { useMemo, useRef } from "react";
import ReactMarkdown, { defaultUrlTransform } from "react-markdown";
import type { Components } from "react-markdown";
import rehypeSanitize, { defaultSchema } from "rehype-sanitize";
import remarkGfm from "remark-gfm";
import type { Element } from "hast";
import { CodeBlock } from "./CodeBlock";
import { MermaidDiagram } from "./MermaidDiagram";
import { classifyFileLink, type FileLink } from "./filePath";

// The one sanitized Markdown renderer in the product. An assistant message and the
// file viewer's rendered Markdown form both go through it, so `rehypeSanitize`,
// the fenced-code override, the diagram rules, and file-link handling stay
// single-sourced and no second raw-markup insertion path exists
// (FS-03.R20/R37/R38/R51, TS-08.R57, INV §2).

// A `file://` link is one of the three forms an agent writes (FS-03.R51), and two
// separate gates drop it before the `a` override below can classify it: the
// sanitizer's allowed link protocols, and react-markdown's own URL transform.
// Both are widened for `file:` alone — `javascript:` and `data:` stay blocked
// exactly as before, and every other URL still goes through the shipped
// transform. The widening has no rendered effect on its own: a local path never
// reaches an `href`, because the override turns it into a viewer control or, on a
// surface with no viewer, into inert text (TS-05.R21).
const LINK_SCHEMA = {
  ...defaultSchema,
  protocols: { ...defaultSchema.protocols, href: [...(defaultSchema.protocols?.href ?? []), "file"] },
};

function transformUrl(url: string): string {
  return classifyFileLink(url) ? url : defaultUrlTransform(url);
}

// A fence still being streamed is not valid diagram source, so a `mermaid` block becomes a diagram
// only once its closing fence has arrived (FS-03.R37). The block's source span carries that
// closing line; an open fence ends on its last content line.
const CLOSING_FENCE = /\n[ \t]{0,3}(?:`{3,}|~{3,})[ \t]*$/;

function fenceIsClosed(source: string, node: Element | undefined): boolean {
  const start = node?.position?.start.offset;
  const end = node?.position?.end.offset;
  if (start === undefined || end === undefined) return false;
  return CLOSING_FENCE.test(source.slice(start, end));
}

export function SanitizedMarkdown({ text, onOpenFile }: { text: string; onOpenFile?: (link: FileLink) => void }) {
  // The component map must stay referentially stable for the life of the message: a new map
  // remounts every block under it, so rebuilding it per streamed delta drops a settled diagram
  // back to source and repeats main-thread Mermaid work (FS-03.R37, TS-08.R40). The fence check
  // and the current open-file handler are read through refs written on each render instead.
  const textRef = useRef(text);
  textRef.current = text;
  const openRef = useRef(onOpenFile);
  openRef.current = onOpenFile;
  const components = useMemo<Components>(() => ({
    code({ className, children, node }) {
      const match = /language-(\w+)/.exec(className ?? "");
      if (!match) return <code className={className}>{children}</code>;
      const value = String(children).replace(/\n$/, "");
      if (match[1] === "mermaid" && fenceIsClosed(textRef.current, node)) return <MermaidDiagram source={value} />;
      return <CodeBlock language={match[1]}>{value}</CodeBlock>;
    },
    // A link whose target is a local path opens the file viewer instead of
    // navigating the browser, which today leaves the conversation entirely
    // (FS-03.R51). Everything else — `http`, `https`, `mailto` — is untouched.
    a({ href, children, node, ...rest }) {
      void node;
      const link = classifyFileLink(href);
      if (!link) return <a href={href} {...rest}>{children}</a>;
      if (!openRef.current) return <span className="file-link">{children}</span>;
      return (
        <button
          type="button"
          className="file-link"
          data-file-path={link.path}
          onClick={() => openRef.current?.(link)}
        >
          {children}
        </button>
      );
    },
  }), []);
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm]}
      rehypePlugins={[[rehypeSanitize, LINK_SCHEMA]]}
      urlTransform={transformUrl}
      components={components}
    >
      {text}
    </ReactMarkdown>
  );
}
