import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import { syntaxTheme } from "../../../presentation/integrations";

export function CodeBlock({ language, children }: { language: string; children: string }) {
  return (
    <SyntaxHighlighter language={language} PreTag="div" style={syntaxTheme}>
      {children}
    </SyntaxHighlighter>
  );
}
