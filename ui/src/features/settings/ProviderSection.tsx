import type { Backend, ProviderRuntime } from "../../schemas/backends";
import {
  describeProviderRuntime,
  providerExecutableKey,
  providerInstallURL,
  providerName,
} from "../../lib/providerRuntime";

export interface ProviderSectionProps {
  backendId: string;
  backend: Backend;
  // The backend-level executable override, edited in place inside the
  // backend env so every other env entry is preserved (FS-09.R76).
  executable: string;
  onModeChange: (mode: "installed" | "bundled") => void;
  onExecutableChange: (value: string) => void;
  // Saved next-start metadata for the backend's default model; undefined for
  // an unsaved backend.
  runtime: ProviderRuntime | undefined;
  // True when the Settings draft differs from the saved catalog.
  dirty: boolean;
  refreshing: boolean;
  refreshMessage: string | null;
  onRefresh: () => void;
}

// ProviderSection is a Claude/Codex backend's provider choice (FS-09.R72,
// R75–R77): Installed by default or the AgentDeck bundle, the advanced
// executable path, the saved next-start provider, and Refresh provider.
export function ProviderSection({
  backendId,
  backend,
  executable,
  onModeChange,
  onExecutableChange,
  runtime,
  dirty,
  refreshing,
  refreshMessage,
  onRefresh,
}: ProviderSectionProps) {
  const name = providerName(backend.type);
  const bundled = backend.provider_mode === "bundled";
  const key = providerExecutableKey(backend.type);
  const modelOverride = Object.values(backend.models ?? {}).some((m) => (m.env?.[key] ?? "") !== "");
  const described = runtime && describeProviderRuntime(backend.type, runtime);
  return (
    <fieldset className="backend-provider-section">
      <legend>Provider</legend>
      <div className="backend-provider-modes" role="radiogroup" aria-label={`${name} provider`}>
        <label className="backend-autosync-label">
          <input type="radio" name={`provider-mode-${backendId}`} checked={!bundled} onChange={() => onModeChange("installed")} />
          Installed provider (default)
        </label>
        <label className="backend-autosync-label">
          <input type="radio" name={`provider-mode-${backendId}`} checked={bundled} onChange={() => onModeChange("bundled")} />
          AgentDeck bundle
        </label>
      </div>
      <p className="backend-provider-hint">
        Installed follows your own {name} updates; the AgentDeck bundle changes only when AgentDeck updates.
        Neither promises new models or account access, or that a session can move back to an older version.
        A change applies to the next process start, not running agents.
      </p>
      {bundled ? (
        (executable !== "" || modelOverride) && (
          <p className="backend-provider-hint">
            Executable paths ({key}) are inactive while the AgentDeck bundle is selected. They are kept and apply again under Installed provider.
          </p>
        )
      ) : (
        <label className="backend-provider-path">
          <span>Executable path (advanced)</span>
          <input
            value={executable}
            placeholder={`Found on PATH (${key})`}
            onChange={(e) => onExecutableChange(e.target.value)}
          />
        </label>
      )}
      <div className="backend-provider-status" aria-live="polite">
        {dirty || !runtime ? (
          <span>Save to check provider.</span>
        ) : (
          described && (
            <>
              <span className="backend-provider-runtime">Next start: {described.summary}</span>
              {described.problem && (
                <span className="form-warning">
                  {described.problem}{" "}
                  {!bundled && runtime.state === "missing" && runtime.source === "detected" && (
                    <a href={providerInstallURL(backend.type)} target="_blank" rel="noreferrer">Install {name}</a>
                  )}
                </span>
              )}
            </>
          )
        )}
        <button
          type="button"
          className="btn-sm"
          onClick={onRefresh}
          disabled={dirty || !runtime || refreshing}
          title={dirty ? "Save your Settings changes first" : undefined}
        >
          {refreshing ? "Checking…" : "Refresh provider"}
        </button>
        {dirty && <span className="backend-provider-hint">Save your Settings changes to refresh.</span>}
        {refreshMessage && !dirty && <span role="status">{refreshMessage}</span>}
      </div>
    </fieldset>
  );
}
