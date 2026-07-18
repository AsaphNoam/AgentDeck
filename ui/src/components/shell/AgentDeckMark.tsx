export function AgentDeckMark({ compact = false }: { compact?: boolean }) {
  return (
    <span className="app-mark" data-slot="mark" aria-hidden="true">
      <svg viewBox="0 0 36 36" role="img">
        <path d="M4 4h12v12H4zM20 4h12v12H20zM4 20h12v12H4z" fill="currentColor" />
        <path d="M20 20h12v12H20z" fill="none" stroke="currentColor" strokeWidth="3" />
        <path d="M24 24h4v4h-4z" fill="currentColor" />
      </svg>
      {!compact && <strong data-slot="wordmark">AgentDeck</strong>}
    </span>
  );
}
