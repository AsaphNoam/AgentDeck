export const BUILT_IN_SKINS = ["sky-grove"] as const;
export const SKY_GROVE_SKIN = BUILT_IN_SKINS[0];

export type EffectiveAppearance = "core" | typeof SKY_GROVE_SKIN;

export function effectiveAppearance(value?: string): EffectiveAppearance {
  return value === SKY_GROVE_SKIN ? SKY_GROVE_SKIN : "core";
}

export function applyAppearance(value?: string): EffectiveAppearance {
  const effective = effectiveAppearance(value);
  if (effective === SKY_GROVE_SKIN) {
    document.documentElement.dataset.skin = SKY_GROVE_SKIN;
  } else {
    document.documentElement.removeAttribute("data-skin");
  }
  return effective;
}
