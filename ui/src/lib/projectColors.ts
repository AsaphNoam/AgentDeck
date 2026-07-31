export type ProjectColor = readonly [number, number, number];

export const PROJECT_COLOR_PRESETS = [
  { name: "Slate", color: [100, 116, 139] },
  { name: "Blue", color: [59, 130, 246] },
  { name: "Green", color: [34, 197, 94] },
  { name: "Amber", color: [245, 158, 11] },
  { name: "Rose", color: [244, 63, 94] },
  { name: "Violet", color: [139, 92, 246] },
] as const satisfies readonly { name: string; color: ProjectColor }[];

export const DEFAULT_PROJECT_COLOR: ProjectColor = PROJECT_COLOR_PRESETS[0].color;

export function sameProjectColor(left: readonly number[], right: readonly number[]) {
  return left.length === 3 && right.length === 3 && left.every((channel, index) => channel === right[index]);
}
