export function ContextBar({ value }: { value: number }) {
  const pct = Math.max(0, Math.min(1, value || 0));
  const label = Math.round(pct * 100);
  const tone = pct > 0.85 ? "high" : pct >= 0.6 ? "medium" : "low";
  return (
    <div className={`context-bar ${tone}`} data-ui="context-meter" data-slot="track" data-variant={tone} aria-label={`${label}% context used`}>
      <span data-slot="fill" style={{ width: `${label}%` }} />
      {label > 0 && <em data-slot="label">{label}%</em>}
    </div>
  );
}
