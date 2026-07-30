import { useEffect } from "react";
import { useConfig } from "../../api/config";
import { applyAppearance } from "./appearance";

// AppearanceRoot is the only bridge from durable config state to presentation.
// Core remains the first paint and fallback because absent/failed config removes
// the optional skin marker instead of blocking the application shell.
export function AppearanceRoot() {
  const config = useConfig();
  const skin = config.isError ? undefined : config.data?.appearance_skin;

  useEffect(() => {
    applyAppearance(skin);
  }, [skin]);

  return null;
}
