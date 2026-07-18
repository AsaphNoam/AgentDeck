import ReactMarkdown from "react-markdown";
import rehypeSanitize from "rehype-sanitize";
import remarkGfm from "remark-gfm";
import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import type { TranscriptEvent } from "../../../api/types";
import { syntaxTheme } from "../../../presentation/integrations";

export function AssistantText({ event }: { event: TranscriptEvent }) {
  const text = String(event.text ?? event.delta ?? "");
  return (
    <article className="message assistant-message" data-ui="transcript" data-variant="assistant">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeSanitize]}
        components={{
          code({ className, children }) {
            const match = /language-(\w+)/.exec(className ?? "");
            return match ? (
              <SyntaxHighlighter language={match[1]} PreTag="div" style={syntaxTheme}>
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
