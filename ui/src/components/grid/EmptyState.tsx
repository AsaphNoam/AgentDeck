interface EmptyStateProps {
  onNewAgent: () => void;
}

export function EmptyState({ onNewAgent }: EmptyStateProps) {
  return (
    <section className="empty-state" data-ui="dashboard" data-slot="empty">
      <h1>No running agents</h1>
      <button type="button" onClick={onNewAgent}>
        New Agent
      </button>
    </section>
  );
}
