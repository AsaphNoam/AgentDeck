import { useQueryClient } from "@tanstack/react-query";
import { configErrorMessage, QUERY_KEYS, useConfig, usePutConfig } from "../../api/config";
import type { Config } from "../../schemas/config";
import { SKY_GROVE_SKIN, STUDIO_SKIN, effectiveAppearance } from "../appearance/appearance";
import { useUiStore } from "../../store/uiStore";
import { SettingsHeader } from "./SettingsHeader";

const appearances = [
  {
    id: "core" as const,
    value: "",
    name: "Chuck Core",
    description: "The product-native neutral canvas with crisp technical structure.",
  },
  {
    id: SKY_GROVE_SKIN,
    value: SKY_GROVE_SKIN,
    name: "Sky & Grove",
    description: "Calm sky-blue layers, evergreen structure, and quiet contour linework.",
  },
  {
    id: STUDIO_SKIN,
    value: STUDIO_SKIN,
    name: "Studio",
    description: "Pale blue-green studio light, forest accents, and a quiet dotted canvas.",
  },
];

export function AppearanceEditor() {
  const configQuery = useConfig();
  const putConfig = usePutConfig();
  const queryClient = useQueryClient();
  const pushError = useUiStore((state) => state.pushError);
  const active = effectiveAppearance(configQuery.isError ? undefined : configQuery.data?.appearance_skin);

  const save = (appearanceSkin: string) => {
    if (putConfig.isPending) return;
    const previous = queryClient.getQueryData<Config>(QUERY_KEYS.config);

    queryClient.setQueryData<Config>(QUERY_KEYS.config, (current) => {
      if (!current) return current;
      return {
        ...current,
        appearance_skin: appearanceSkin,
        appearance_skin_warning: undefined,
      };
    });

    putConfig.mutate(
      { appearance_skin: appearanceSkin },
      {
        onError: (error: unknown) => {
          queryClient.setQueryData(QUERY_KEYS.config, previous);
          pushError("Saving appearance failed", configErrorMessage(error));
        },
        onSettled: () => {
          void queryClient.invalidateQueries({ queryKey: QUERY_KEYS.config });
        },
      },
    );
  };

  const warning = configQuery.error
    ? "Appearance could not be loaded. Chuck Core is active."
    : configQuery.data?.appearance_skin_warning === "config_unreadable"
      ? "The saved configuration could not be read. Chuck Core is active; choose an appearance to repair it."
      : configQuery.data?.appearance_skin_warning === "unsupported"
        ? `The saved appearance “${configQuery.data.appearance_skin}” is unavailable. Chuck Core is active.`
        : undefined;

  return (
    <div className="config-editor" data-ui="config-editor" data-variant="appearance">
      <SettingsHeader eyebrow="Appearance" title="Theme" description="Choose one look for every project, agent, and browser session.">
        {putConfig.isPending && <span className="appearance-saving">Saving…</span>}
      </SettingsHeader>

      {warning && <p className="appearance-warning" role="status">{warning}</p>}

      <fieldset className="appearance-options" disabled={putConfig.isPending || configQuery.isLoading} data-slot="list">
        <legend className="ad-visually-hidden">Chuck appearance</legend>
        {appearances.map((appearance) => (
          <label className="appearance-option" data-slot="item" key={appearance.id}>
            <span className="appearance-preview" data-slot="preview" data-preview-skin={appearance.id} aria-hidden="true">
              <span className="appearance-preview-sky" data-slot="preview-sky">
                <span className="appearance-preview-action" data-slot="preview-action" />
                <span className="appearance-preview-signal" data-slot="preview-signal" />
              </span>
              <span className="appearance-preview-surface" data-slot="preview-surface" />
              <span className="appearance-preview-surface" data-slot="preview-surface" />
              <span className="appearance-preview-surface" data-slot="preview-surface" />
            </span>
            <span className="appearance-choice">
              <input
                // When a warning is present the durable selection is not a clean
                // valid appearance (unsupported/unreadable/read error), so leave
                // every radio unchecked — clicking Core then saves "" to repair it
                // rather than being inert because Core is only the effective
                // fallback (FS-12.R32/A11).
                checked={!warning && active === appearance.id}
                name="appearance"
                onChange={() => save(appearance.value)}
                type="radio"
                value={appearance.value}
              />
              <span className="appearance-copy">
                <strong>{appearance.name}</strong>
                <span>{appearance.description}</span>
              </span>
            </span>
          </label>
        ))}
      </fieldset>
    </div>
  );
}
