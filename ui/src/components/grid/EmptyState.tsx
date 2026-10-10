import type { ReactNode } from "react";

interface EmptyStateProps {
  onNewAgent: () => void;
  /** Extra creation actions beside New Agent, such as Think Tank (FS-02.R65). */
  actions?: ReactNode;
}

export function EmptyState({ onNewAgent, actions }: EmptyStateProps) {
  return (
    <section className="empty-state" data-ui="dashboard" data-slot="empty">
      <h1>No running agents</h1>
      <button type="button" className="ad-button-primary" onClick={onNewAgent}>
        New Agent
      </button>
      {actions}
    </section>
  );
}
