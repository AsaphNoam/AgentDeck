import { useState } from "react";
import { configErrorMessage, useBackends, usePutBackends } from "../../../api/config";
import type { BackendsConfig, BackendType } from "../../../schemas/backends";
import { BACKEND_TYPE_LABELS, BACKEND_TYPE_OPTIONS } from "../../../lib/backendTypes";

interface BackendStepProps {
  // Reports the chosen seeded backend so later steps (e.g. the federation Config
  // step) target the right provider instead of assuming Claude.
  onDone: (backend: { id: string; type: BackendType }) => void;
  claimMutation?: () => boolean;
  releaseMutation?: () => void;
}

// seededIdForType maps a backend type to its seeded backend id (onboarding
// edits the seeded entry for the chosen type).
const seededIdForType: Record<BackendType, string> = {
  "claude-acp": "claude",
  "codex-acp": "codex",
  "opencode-acp": "opencode",
  "openhands-acp": "openhands",
};

// signInProvider maps a backend type to the `agentdeck auth` selector for
// providers that own an interactive sign-in. AgentDeck never runs, proxies, or
// observes that flow — onboarding only tells the person which command to run in
// their own terminal, then re-checks readiness (FS-04.R34, TS-04.R15).
const signInProvider: Partial<Record<BackendType, string>> = {
  "claude-acp": "claude",
  "codex-acp": "codex",
};

function credentialGuidance(status: string, detail: string | null, type: BackendType): string {
  if (detail === "cli_not_installed") {
    return `The ${BACKEND_TYPE_LABELS[type]} adapter is not installed. Install it, then check again.`;
  }
  if (detail === "not_logged_in") {
    return `${BACKEND_TYPE_LABELS[type]} is not signed in. Complete its sign-in below, then check again.`;
  }
  if (detail === "no_api_key") {
    return type === "codex-acp"
      ? "No sign-in and no API key were found. Sign in below, or add an API key, then check again."
      : "No API key was found. Add the API key above, then check again.";
  }
  return status === "failed"
    ? "The credential check failed. Confirm this provider's sign-in or API key, then check again."
    : "The credential check could not run yet. Confirm the provider setup, then check again.";
}

export function BackendStep({ onDone, claimMutation, releaseMutation }: BackendStepProps) {
  const { data: existing, isLoading } = useBackends();
  const putBackends = usePutBackends();

  const [type, setType] = useState<BackendType>("claude-acp");
  const [apiKey, setApiKey] = useState("");
  const [llmApiKey, setLlmApiKey] = useState("");
  const [llmBaseUrl, setLlmBaseUrl] = useState("");
  const [credStatus, setCredStatus] = useState<string | null>(null);
  const [credDetail, setCredDetail] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const backendId = seededIdForType[type];
  const seeded = existing?.backends[backendId];
  const readyToValidate = !!existing && !isLoading;
  // Once a readiness result came back non-ok, the same submit becomes a plain
  // re-check: nothing about the document changed, only the provider's state.
  const hasUnreadyResult = credStatus !== null && credStatus !== "ok";
  const authProvider = signInProvider[type];
  // A hand-edited catalog can leave the chosen backend with no usable default
  // model. Onboarding no longer invents one (FS-04.R33), so say so plainly
  // rather than silently saving a broken default (INV §8, §11).
  const defaultModel = seeded?.default_model ?? "";
  const defaultModelEntry = defaultModel ? seeded?.models?.[defaultModel] : undefined;
  const catalogUnusable = readyToValidate && (!seeded || !defaultModelEntry);

  const handleValidate = () => {
    if (!readyToValidate || catalogUnusable) return;
    if (claimMutation && !claimMutation()) return;
    setError(null);
    setCredStatus(null);
    // Credentials entered here belong to the backend, not to one model entry, so
    // they survive a later model change in Settings.
    const env: Record<string, string> = { ...(seeded?.env ?? {}) };
    if (type === "codex-acp" && apiKey) env["OPENAI_API_KEY"] = apiKey;
    if (type === "openhands-acp") {
      if (llmApiKey) env["LLM_API_KEY"] = llmApiKey;
      if (llmBaseUrl) env["LLM_BASE_URL"] = llmBaseUrl;
    }

    const backends: BackendsConfig["backends"] = existing?.backends ?? {};
    // Merge-preserve (INV §3): the seeded catalog — every model entry and its
    // default — is submitted unchanged. This step only marks the chosen backend
    // default and overlays credentials; it never writes back a partial
    // on-screen view of the model catalog.
    const config: BackendsConfig = {
      version: 2,
      backends: {
        ...backends,
        [backendId]: {
          ...seeded,
          name: seeded?.name ?? BACKEND_TYPE_LABELS[type],
          type,
          default: true,
          default_model: defaultModel,
          models: seeded?.models ?? {},
          env,
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
        if (status === "ok") onDone({ id: backendId, type });
      },
      onError: (e) => setError(configErrorMessage(e)),
      onSettled: releaseMutation,
    });
  };

  return (
    <div className="wizard-step" data-ui="onboarding" data-slot="step" data-variant="backend">
      <h3>Configure your AI backend</h3>
      <p className="wizard-step-desc">
        Choose a backend and confirm it is ready. You can change models and add more backends later
        in Settings.
      </p>

      <div className="form-field">
        <label>Backend type</label>
        <select value={type} onChange={(e) => setType(e.target.value as BackendType)}>
          {BACKEND_TYPE_OPTIONS.map((t) => (
            <option key={t} value={t}>
              {BACKEND_TYPE_LABELS[t]} ({t})
            </option>
          ))}
        </select>
      </div>

      {defaultModelEntry && (
        <p className="form-hint">
          Using this backend's default model: {defaultModelEntry.name}.
        </p>
      )}

      {authProvider && (
        <div className="wizard-guidance" data-slot="guidance">
          <p>
            {BACKEND_TYPE_LABELS[type]} sign-in happens in {BACKEND_TYPE_LABELS[type]}'s own tool, not
            in AgentDeck. In a terminal, run:
          </p>
          <p>
            <code>agentdeck auth {authProvider}</code>
          </p>
          <p className="form-hint">
            {type === "codex-acp"
              ? "Already signed in to Codex? Nothing else is needed — an API key below is only an alternative."
              : "AgentDeck never sees or stores your credentials."}
          </p>
        </div>
      )}

      {type === "codex-acp" && (
        <div className="form-field">
          <label>OpenAI API key (optional)</label>
          <input
            type="password"
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            placeholder="sk-..."
            autoComplete="off"
          />
        </div>
      )}

      {type === "openhands-acp" && (
        <>
          <div className="form-field">
            <label>LLM API key</label>
            <input
              type="password"
              value={llmApiKey}
              onChange={(e) => setLlmApiKey(e.target.value)}
              placeholder="sk-..."
              autoComplete="off"
            />
          </div>
          <div className="form-field">
            <label>LLM base URL (optional)</label>
            <input
              value={llmBaseUrl}
              onChange={(e) => setLlmBaseUrl(e.target.value)}
              placeholder="https://api.anthropic.com"
            />
          </div>
        </>
      )}

      {catalogUnusable && (
        <p className="form-error">
          This backend has no usable default model in <code>backends.json</code>. Fix its catalog in
          Settings → Backends, or choose another backend.
        </p>
      )}
      {credStatus && credStatus !== "ok" && (
        <p className="form-error">
          {credentialGuidance(credStatus, credDetail, type)}
        </p>
      )}
      {error && <p className="form-error">{error}</p>}

      <div className="form-actions" data-slot="actions">
        <button
          type="button"
          onClick={handleValidate}
          disabled={putBackends.isPending || !readyToValidate || catalogUnusable}
        >
          {putBackends.isPending
            ? "Checking…"
            : hasUnreadyResult
              ? "Check again"
              : "Validate & Continue"}
        </button>
      </div>
    </div>
  );
}
