declare module "react-syntax-highlighter" {
  import type { ComponentType, ReactNode } from "react";

  export const Prism: ComponentType<{
    children?: ReactNode;
    language?: string;
    PreTag?: string;
  }>;
}
