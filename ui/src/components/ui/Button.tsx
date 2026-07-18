import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from "react";

type ButtonVariant = "primary" | "secondary" | "ghost" | "danger" | "icon";

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: "small" | "medium";
  busy?: boolean;
  children: ReactNode;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant = "secondary", size = "medium", busy = false, className = "", children, disabled, ...props },
  ref,
) {
  return (
    <button
      ref={ref}
      className={["ad-button", `ad-button-${variant}`, `ad-button-${size}`, className].filter(Boolean).join(" ")}
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

export const IconButton = forwardRef<HTMLButtonElement, Omit<ButtonProps, "variant">>(function IconButton(
  props,
  ref,
) {
  return <Button ref={ref} variant="icon" {...props} />;
});
