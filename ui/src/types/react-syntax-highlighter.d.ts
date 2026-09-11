type LineProps = Record<string, string | number | undefined>;

declare module "react-syntax-highlighter" {
  import type { ComponentType, CSSProperties, ReactNode } from "react";

  export const Prism: ComponentType<{
    children?: ReactNode;
    language?: string;
    PreTag?: string;
    style?: Record<string, CSSProperties>;
    // Line numbering and per-line props back the file viewer's line-numbered,
    // line-marked body (FS-03.R52). wrapLines must be on for lineProps to apply.
    showLineNumbers?: boolean;
    wrapLines?: boolean;
    // The library spreads these onto each line's span, so data-* attributes are
    // the accurate shape here — React's HTMLProps would reject them.
    lineProps?: ((lineNumber: number) => LineProps) | LineProps;
  }>;
}
