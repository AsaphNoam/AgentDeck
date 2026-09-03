export function ContextBar({ value, compact = false }: { value: number; compact?: boolean }) {
  const pct = Math.max(0, Math.min(1, value || 0));
  const label = Math.round(pct * 100);
  const tone = pct > 0.85 ? "high" : pct >= 0.6 ? "medium" : "low";
  return (
    // Tone and density are orthogonal, so they take separate contract dimensions: folding the
    // compact form into `data-variant` would leave the compact meter with no tone a skin can
    // read (TS-08.R14/R48).
    <div className={`context-bar ${tone}`} data-ui="context-meter" data-slot="track" data-variant={tone} data-state={compact ? "compact" : undefined} aria-label={`${label}% context used`}>
      <span data-slot="fill" style={{ width: `${label}%` }} />
      <em data-slot="label">{label}% context used</em>
    </div>
  );
}
