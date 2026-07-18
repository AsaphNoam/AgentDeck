declare module "react-syntax-highlighter" {
  import type { ComponentType, CSSProperties, ReactNode } from "react";

  export const Prism: ComponentType<{
    children?: ReactNode;
    language?: string;
    PreTag?: string;
    style?: Record<string, CSSProperties>;
  }>;
}
