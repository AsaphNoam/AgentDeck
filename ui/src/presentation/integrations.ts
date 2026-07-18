import type { CSSProperties } from "react";
import type { PresentationColors } from "./resolveColors";

type SyntaxStyles = Record<string, CSSProperties>;

export const syntaxTheme: SyntaxStyles = {
  'code[class*="language-"]': {
    color: "var(--ad-technical-text)",
    background: "var(--ad-technical-background)",
    fontFamily: "var(--ad-font-mono)",
  },
  'pre[class*="language-"]': {
    color: "var(--ad-technical-text)",
    background: "var(--ad-technical-background)",
    fontFamily: "var(--ad-font-mono)",
  },
  comment: { color: "var(--ad-technical-muted)" },
  punctuation: { color: "var(--ad-technical-muted)" },
  property: { color: "var(--ad-highlight)" },
  tag: { color: "var(--ad-state-error)" },
  boolean: { color: "var(--ad-action-primary)" },
  number: { color: "var(--ad-action-primary)" },
  string: { color: "var(--ad-state-done)" },
  operator: { color: "var(--ad-action-secondary)" },
  keyword: { color: "var(--ad-action-secondary)" },
  function: { color: "var(--ad-highlight)" },
};

export const diffTheme = {
  variables: {
    light: {
      diffViewerBackground: "var(--ad-technical-background)",
      diffViewerColor: "var(--ad-technical-text)",
      addedBackground: "color-mix(in srgb, var(--ad-state-done) 24%, var(--ad-technical-background))",
      addedColor: "var(--ad-technical-text)",
      removedBackground: "color-mix(in srgb, var(--ad-state-error) 24%, var(--ad-technical-background))",
      removedColor: "var(--ad-technical-text)",
      wordAddedBackground: "color-mix(in srgb, var(--ad-state-done) 46%, var(--ad-technical-surface))",
      wordRemovedBackground: "color-mix(in srgb, var(--ad-state-error) 46%, var(--ad-technical-surface))",
      addedGutterBackground: "var(--ad-technical-surface)",
      removedGutterBackground: "var(--ad-technical-surface)",
      gutterBackground: "var(--ad-technical-surface)",
      gutterBackgroundDark: "var(--ad-technical-background)",
      highlightBackground: "var(--ad-technical-surface)",
      highlightGutterBackground: "var(--ad-technical-surface)",
      codeFoldGutterBackground: "var(--ad-technical-surface)",
      codeFoldBackground: "var(--ad-technical-surface)",
      emptyLineBackground: "var(--ad-technical-background)",
      gutterColor: "var(--ad-technical-muted)",
      addedGutterColor: "var(--ad-state-done)",
      removedGutterColor: "var(--ad-state-error)",
      codeFoldContentColor: "var(--ad-technical-muted)",
      diffViewerTitleBackground: "var(--ad-technical-surface)",
      diffViewerTitleColor: "var(--ad-technical-text)",
      diffViewerTitleBorderColor: "var(--ad-border-default)",
    },
  },
  line: { fontFamily: "var(--ad-font-mono)" },
  contentText: { fontFamily: "var(--ad-font-mono)" },
};

export function xtermTheme(colors: PresentationColors) {
  return {
    background: colors.background,
    foreground: colors.foreground,
    cursor: colors.accent,
    cursorAccent: colors.background,
    selectionBackground: colors.selection,
    black: colors.background,
    brightBlack: colors.muted,
    red: colors.error,
    brightRed: colors.error,
    green: colors.success,
    brightGreen: colors.success,
    yellow: colors.accent,
    brightYellow: colors.accent,
    blue: colors.selection,
    brightBlue: colors.selection,
    magenta: colors.accent,
    brightMagenta: colors.accent,
    cyan: colors.selection,
    brightCyan: colors.selection,
    white: colors.foreground,
    brightWhite: colors.foreground,
  };
}
