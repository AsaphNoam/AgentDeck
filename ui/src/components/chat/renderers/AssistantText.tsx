import { useMemo, useRef } from "react";
import ReactMarkdown from "react-markdown";
import type { Components } from "react-markdown";
import rehypeSanitize from "rehype-sanitize";
import remarkGfm from "remark-gfm";
import type { Element } from "hast";
import type { TranscriptEvent } from "../../../api/types";
import { CodeBlock } from "./CodeBlock";
import { MermaidDiagram } from "./MermaidDiagram";

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

export function AssistantText({ event }: { event: TranscriptEvent }) {
  const text = String(event.text ?? event.delta ?? "");
  // The component map must stay referentially stable for the life of the message: a new map
  // remounts every block under it, so rebuilding it per streamed delta drops a settled diagram
  // back to source and repeats main-thread Mermaid work (FS-03.R37, TS-08.R40). The fence check
  // needs the current text, which the map reads through a ref written on each render instead.
  const textRef = useRef(text);
  textRef.current = text;
  const components = useMemo<Components>(() => ({
    code({ className, children, node }) {
      const match = /language-(\w+)/.exec(className ?? "");
      if (!match) return <code className={className}>{children}</code>;
      const value = String(children).replace(/\n$/, "");
      if (match[1] === "mermaid" && fenceIsClosed(textRef.current, node)) return <MermaidDiagram source={value} />;
      return <CodeBlock language={match[1]}>{value}</CodeBlock>;
    },
  }), []);
  return (
    <article className="message assistant-message" data-ui="transcript" data-variant="assistant">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeSanitize]}
        components={components}
      >
        {text}
      </ReactMarkdown>
    </article>
  );
}
