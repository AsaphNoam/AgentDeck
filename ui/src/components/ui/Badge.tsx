import type { HTMLAttributes, ReactNode } from "react";

type BadgeVariant = "neutral" | "info" | "success" | "warning" | "danger" | "technical";
type BadgeState = "busy" | "idle" | "waiting_input" | "done" | "error" | "unknown";

export function Badge({
  variant = "neutral",
  state,
  indicator = false,
  className = "",
  children,
  ...props
}: HTMLAttributes<HTMLSpanElement> & {
  variant?: BadgeVariant;
  state?: BadgeState;
  indicator?: boolean;
  children: ReactNode;
}) {
  return (
    <span
      className={["ad-badge", `ad-badge-${variant}`, className].filter(Boolean).join(" ")}
      data-ui="badge"
      data-variant={variant}
      data-state={state}
      {...props}
    >
      {indicator && <span className="ad-badge-indicator" data-slot="indicator" />}
      <span data-slot="label">{children}</span>
    </span>
  );
}
