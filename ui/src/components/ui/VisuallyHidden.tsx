import type { HTMLAttributes } from "react";

export function VisuallyHidden({ className = "", ...props }: HTMLAttributes<HTMLSpanElement>) {
  return <span className={["ad-visually-hidden", className].filter(Boolean).join(" ")} {...props} />;
}
