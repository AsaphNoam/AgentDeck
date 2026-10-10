import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from "react";

type ButtonVariant = "primary" | "secondary" | "ghost" | "danger" | "icon" | "icon-primary";

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  busy?: boolean;
  children: ReactNode;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant = "secondary", busy = false, className = "", children, disabled, ...props },
  ref,
) {
  return (
    <button
      ref={ref}
      className={["ad-button", `ad-button-${variant}`, className].filter(Boolean).join(" ")}
      data-ui="button"
      data-variant={variant}
      data-state={busy ? "busy" : disabled ? "disabled" : undefined}
      disabled={disabled || busy}
      {...props}
    >
      <span data-slot="label">{children}</span>
    </button>
  );
});

export const IconButton = forwardRef<HTMLButtonElement, Omit<ButtonProps, "variant"> & { variant?: "icon" | "icon-primary" }>(
  function IconButton({ variant = "icon", ...props }, ref) {
    return <Button ref={ref} variant={variant} {...props} />;
  },
);
