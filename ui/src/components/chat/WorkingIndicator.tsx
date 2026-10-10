export function WorkingIndicator() {
  return (
    <div className="transcript-pending" aria-live="polite">
      <span className="spinner" aria-hidden="true" />
      <span>Working…</span>
    </div>
  );
}
