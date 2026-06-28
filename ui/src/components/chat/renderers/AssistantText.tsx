import ReactMarkdown from "react-markdown";
import rehypeSanitize from "rehype-sanitize";
import remarkGfm from "remark-gfm";
import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import type { TranscriptEvent } from "../../../api/types";

export function AssistantText({ event }: { event: TranscriptEvent }) {
  const text = String(event.text ?? event.delta ?? "");
  return (
    <article className="message assistant-message">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeSanitize]}
        components={{
          code({ className, children }) {
            const match = /language-(\w+)/.exec(className ?? "");
            return match ? (
              <SyntaxHighlighter language={match[1]} PreTag="div">
                {String(children).replace(/\n$/, "")}
              </SyntaxHighlighter>
            ) : (
              <code className={className}>{children}</code>
            );
          },
        }}
      >
        {text}
      </ReactMarkdown>
    </article>
  );
}
