export const presentationColorTokens = {
  background: "--ad-technical-background",
  surface: "--ad-technical-surface",
  foreground: "--ad-technical-text",
  muted: "--ad-technical-muted",
  accent: "--ad-action-primary",
  selection: "--ad-action-secondary",
  success: "--ad-state-done",
  error: "--ad-state-error",
  line: "--ad-border-default",
  fontFamily: "--ad-font-mono",
} as const;

export type PresentationColors = Record<keyof typeof presentationColorTokens, string>;

export function resolvePresentationColors(element: Element = document.documentElement): PresentationColors {
  const style = getComputedStyle(element);
  return Object.fromEntries(
    Object.entries(presentationColorTokens).map(([name, token]) => [name, style.getPropertyValue(token).trim()]),
  ) as PresentationColors;
}
