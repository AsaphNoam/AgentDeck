import { useUiStore } from "../../store/uiStore";

export function DensityControl() {
  const density = useUiStore((state) => state.density);
  const setDensity = useUiStore((state) => state.setDensity);
  return (
    <div className="density-control">
      <label>
        Columns
        <input
          type="number"
          min={1}
          max={8}
          value={density.perRow}
          onChange={(event) => setDensity({ ...density, perRow: Number(event.target.value) })}
        />
      </label>
      <label>
        Gap
        <input
          type="number"
          min={0}
          max={48}
          value={density.gap}
          onChange={(event) => setDensity({ ...density, gap: Number(event.target.value) })}
        />
      </label>
    </div>
  );
}
