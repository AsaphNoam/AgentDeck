export const BUILT_IN_SKINS = ["sky-grove", "studio"] as const;
export const SKY_GROVE_SKIN = BUILT_IN_SKINS[0];
export const STUDIO_SKIN = BUILT_IN_SKINS[1];

export type BuiltInSkin = (typeof BUILT_IN_SKINS)[number];
export type EffectiveAppearance = "core" | BuiltInSkin;

function isBuiltInSkin(value?: string): value is BuiltInSkin {
  return (BUILT_IN_SKINS as readonly string[]).includes(value ?? "");
}

export function effectiveAppearance(value?: string): EffectiveAppearance {
  return isBuiltInSkin(value) ? value : "core";
}

export function applyAppearance(value?: string): EffectiveAppearance {
  const effective = effectiveAppearance(value);
  if (effective === "core") {
    document.documentElement.removeAttribute("data-skin");
  } else {
    document.documentElement.dataset.skin = effective;
  }
  return effective;
}
