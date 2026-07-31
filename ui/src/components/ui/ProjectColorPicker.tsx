import { PROJECT_COLOR_PRESETS, sameProjectColor, type ProjectColor } from "../../lib/projectColors";
import { VisuallyHidden } from "./VisuallyHidden";

interface ProjectColorPickerProps {
  value: readonly number[];
  onChange: (color: ProjectColor) => void;
  disabled?: boolean;
}

export function ProjectColorPicker({ value, onChange, disabled }: ProjectColorPickerProps) {
  return (
    <div className="project-color-picker" role="group" aria-label="Project color">
      {PROJECT_COLOR_PRESETS.map((preset) => {
        const selected = sameProjectColor(value, preset.color);
        return (
          <button
            key={preset.name}
            type="button"
            className="project-color-preset"
            aria-label={`${preset.name} project color`}
            aria-pressed={selected}
            disabled={disabled}
            onClick={() => onChange(preset.color)}
            style={{ background: `rgb(${preset.color.join(",")})` }}
          >
            <VisuallyHidden>{preset.name}</VisuallyHidden>
          </button>
        );
      })}
    </div>
  );
}
