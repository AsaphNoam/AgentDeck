import type { ReactNode } from "react";

// The FS-12.R62 section heading: a mono eyebrow naming the tab, the section
// title, an optional one-line description, and the section's action at the right.
export function SettingsHeader({ eyebrow, title, description, children }: {
  eyebrow: string;
  title: string;
  description?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <div className="config-editor-header" data-slot="header">
      <div className="config-editor-heading">
        <p className="config-eyebrow">{eyebrow}</p>
        <h2>{title}</h2>
        {description && <p className="config-description">{description}</p>}
      </div>
      {children}
    </div>
  );
}
