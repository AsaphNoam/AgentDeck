export function ContextBar({ value }: { value: number }) {
  const pct = Math.max(0, Math.min(1, value || 0));
  const label = Math.round(pct * 100);
  const tone = pct > 0.85 ? "high" : pct >= 0.6 ? "medium" : "low";
  return (
    <div className={`context-bar ${tone}`} aria-label={`${label}% context used`}>
      <span style={{ width: `${label}%` }} />
      {label > 0 && <em>{label}%</em>}
    </div>
  );
}
