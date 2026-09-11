import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import { syntaxTheme } from "../../../presentation/integrations";

// The one highlighter call in the product. A transcript fence uses its bare form;
// the file viewer adds line numbers and marks a cited line through the same
// component rather than a second highlighter (FS-03.R52, TS-08.R57, INV §2).
export function CodeBlock({ language, children, showLineNumbers = false, markedLine }: {
  language: string;
  children: string;
  showLineNumbers?: boolean;
  markedLine?: number;
}) {
  return (
    <SyntaxHighlighter
      language={language}
      PreTag="div"
      style={syntaxTheme}
      showLineNumbers={showLineNumbers}
      wrapLines={showLineNumbers}
      // The mark is a data attribute, not a className: the highlighter overwrites
      // a line's className with its own token classes, so a class here would be
      // silently dropped (FS-03.R52).
      lineProps={showLineNumbers ? (line: number) => ({
        "data-file-line": String(line),
        "data-file-marked": line === markedLine ? "true" : undefined,
      }) : undefined}
    >
      {children}
    </SyntaxHighlighter>
  );
}
