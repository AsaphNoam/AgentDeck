import { useEffect, useState } from "react";
import { useBackends, usePutBackends } from "../../../api/config";
import type { BackendsConfig } from "../../../schemas/backends";

interface BackendStepProps {
  onDone: () => void;
}

export function BackendStep({ onDone }: BackendStepProps) {
  const { data: existing } = useBackends();
  const putBackends = usePutBackends();

  const [type, setType] = useState<"claude-acp" | "codex-acp">("claude-acp");
  const [modelKey, setModelKey] = useState("default");
  const [modelName, setModelName] = useState("Default");
  const [modelStr, setModelStr] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [credStatus, setCredStatus] = useState<string | null>(null);
  const [credDetail, setCredDetail] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Pre-fill from the seeded backend entry for the selected type (once loaded), so an
  // untouched submit reuses the seeded default model (name + model string) instead of a
  // synthesized placeholder.
  useEffect(() => {
    if (!existing) return;
    const backendId = type === "claude-acp" ? "claude" : "codex";
    const seeded = existing.backends[backendId];
    const seededModel = seeded?.default_model ? seeded.models[seeded.default_model] : undefined;
    if (seeded && seeded.default_model && seededModel) {
      setModelKey(seeded.default_model);
      setModelName(seededModel.name);
      setModelStr(seededModel.model);
    }
  }, [existing, type]);

  // If existing backends already have a default with ok creds shown — just offer to continue.
  // (The wizard calls onDone when the cred check comes back ok.)

  const handleValidate = () => {
    setError(null);
    setCredStatus(null);
    const backendId = type === "claude-acp" ? "claude" : "codex";
    const env: Record<string, string> = {};
    if (apiKey) env["OPENAI_API_KEY"] = apiKey;

    const backends: BackendsConfig["backends"] = existing?.backends ?? {};
    const seeded = backends[backendId];
    // Merge-preserve: keep the seeded backend's full `models` map (and other fields) and
    // only overlay the one model the user is editing here. This avoids clobbering seeded
    // models with a synthesized single-model map (which previously could persist the
    // literal placeholder model string "default" when the form was left untouched).
    const config: BackendsConfig = {
      version: 2,
      backends: {
        ...backends,
        [backendId]: {
          name: seeded?.name ?? (type === "claude-acp" ? "Claude" : "Codex"),
          type,
          default: true,
          default_model: modelKey,
          models: {
            ...(seeded?.models ?? {}),
            [modelKey]: { name: modelName, model: modelStr || modelKey, env },
          },
          env: seeded?.env ?? {},
        },
      },
    };

    // Clear default flag from all other backends.
    for (const id of Object.keys(config.backends)) {
      if (id !== backendId) config.backends[id] = { ...config.backends[id], default: false };
    }

    putBackends.mutate(config, {
      onSuccess: (resp) => {
        const cred = resp.credentials?.[backendId];
        const status = cred?.status ?? null;
        setCredStatus(status);
        setCredDetail(cred?.detail ?? null);
        if (status === "ok") onDone();
      },
      onError: (e) => setError(String(e)),
    });
  };

  return (
    <div className="wizard-step">
      <h3>Configure your AI backend</h3>
      <p className="wizard-step-desc">
        Choose a backend and validate your credentials before continuing.
      </p>

      <div className="form-field">
        <label>Backend type</label>
        <select value={type} onChange={(e) => setType(e.target.value as typeof type)}>
          <option value="claude-acp">Claude (claude-acp)</option>
          <option value="codex-acp">Codex / OpenAI (codex-acp)</option>
        </select>
      </div>

      {type === "codex-acp" && (
        <div className="form-field">
          <label>OpenAI API key</label>
          <input
            type="password"
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            placeholder="sk-..."
            autoComplete="off"
          />
        </div>
      )}

      <div className="form-field">
        <label>Default model ID</label>
        <input
          value={modelKey}
          onChange={(e) => setModelKey(e.target.value)}
          placeholder="e.g. sonnet"
        />
      </div>

      <div className="form-field">
        <label>Provider model string</label>
        <input
          value={modelStr}
          onChange={(e) => setModelStr(e.target.value)}
          placeholder={type === "claude-acp" ? "e.g. claude-sonnet-4-6" : "e.g. gpt-4o"}
        />
      </div>

      {credStatus && credStatus !== "ok" && (
        <p className="form-error">
          Credential check: <strong>{credStatus}</strong>
          {credDetail ? ` — ${credDetail}` : ""}. Please check your settings and try again.
        </p>
      )}
      {error && <p className="form-error">{error}</p>}

      <div className="form-actions">
        <button type="button" onClick={handleValidate} disabled={putBackends.isPending}>
          {putBackends.isPending ? "Validating…" : "Validate & Continue"}
        </button>
      </div>
    </div>
  );
}
