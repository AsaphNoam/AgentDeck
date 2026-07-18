import { forwardRef, type HTMLAttributes } from "react";

export const Surface = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement> & {
  variant?: "default" | "raised" | "technical";
}>(function Surface({ variant = "default", className = "", ...props }, ref) {
  return (
    <div
      ref={ref}
      className={["ad-surface", `ad-surface-${variant}`, className].filter(Boolean).join(" ")}
      data-ui="surface"
      data-variant={variant}
      {...props}
    />
  );
});
