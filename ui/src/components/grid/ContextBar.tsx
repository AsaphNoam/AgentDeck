export function ContextBar({ value, used, size, compact = false }: { value: number; used?: number; size?: number; compact?: boolean }) {
  const pct = Math.max(0, Math.min(1, value || 0));
  const label = Math.round(pct * 100);
  const tone = pct > 0.85 ? "high" : pct >= 0.6 ? "medium" : "low";
  // The exact pair renders only when both raw numbers are known (FS-02.R62,
  // TS-08.R81); a lone value or a missing pair keeps the percentage-only label
  // rather than inventing a fraction from the rounded percentage (INV §2).
  const hasExactPair = typeof used === "number" && typeof size === "number";
  const text = hasExactPair
    ? `${used.toLocaleString("en-US")} / ${size.toLocaleString("en-US")} tokens · ${label}% context used`
    : `${label}% context used`;
  return (
    // Tone and density are orthogonal, so they take separate contract dimensions: folding the
    // compact form into `data-variant` would leave the compact meter with no tone a skin can
    // read (TS-08.R14/R48).
    <div className={`context-bar ${tone}`} data-ui="context-meter" data-slot="track" data-variant={tone} data-state={compact ? "compact" : undefined} aria-label={text}>
      <span data-slot="fill" style={{ width: `${label}%` }} />
      <em data-slot="label">{text}</em>
    </div>
  );
}
