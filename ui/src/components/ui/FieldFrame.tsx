import type { ReactNode } from "react";

export function FieldFrame({
  label,
  htmlFor,
  hint,
  error,
  disabled = false,
  className = "",
  children,
}: {
  label: ReactNode;
  htmlFor?: string;
  hint?: ReactNode;
  error?: ReactNode;
  disabled?: boolean;
  className?: string;
  children: ReactNode;
}) {
  return (
    <div
      className={["ad-field", className].filter(Boolean).join(" ")}
      data-ui="field"
      data-state={error ? "invalid" : disabled ? "disabled" : undefined}
    >
      <label htmlFor={htmlFor} data-slot="label">{label}</label>
      <div data-slot="control">{children}</div>
      {hint && <div className="ad-field-hint" data-slot="hint">{hint}</div>}
      {error && <div className="ad-field-error" data-slot="error">{error}</div>}
    </div>
  );
}
