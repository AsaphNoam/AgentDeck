import type { HTMLAttributes, ReactNode } from "react";

export function PageHeader({
  eyebrow,
  title,
  description,
  actions,
  compact = false,
  className = "",
  ...props
}: Omit<HTMLAttributes<HTMLElement>, "title"> & {
  eyebrow?: ReactNode;
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  compact?: boolean;
}) {
  return (
    <header
      className={["ad-page-header", compact ? "ad-page-header-compact" : "", className].filter(Boolean).join(" ")}
      data-ui="page-header"
      data-variant={compact ? "compact" : "default"}
      {...props}
    >
      <div className="ad-page-header-copy">
        {eyebrow && <p data-slot="eyebrow">{eyebrow}</p>}
        <h1 data-slot="title">{title}</h1>
        {description && <p data-slot="description">{description}</p>}
      </div>
      {actions && <div className="ad-page-header-actions" data-slot="actions">{actions}</div>}
    </header>
  );
}
