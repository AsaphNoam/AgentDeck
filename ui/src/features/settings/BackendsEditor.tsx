import { useEffect, useState } from "react";
import { useBackends, usePutBackends } from "../../api/config";
import type { BackendsConfig, Backend, Model, CredResult } from "../../schemas/backends";
import { BACKEND_TYPE_LABELS, BACKEND_TYPE_OPTIONS } from "../../lib/backendTypes";
import { ModelRow } from "./ModelRow";

type Pair = { key: string; value: string };

function toPairs(env: Record<string, string> | undefined): Pair[] {
  return Object.entries(env ?? {}).map(([key, value]) => ({ key, value }));
}

function fromPairs(pairs: Pair[]): Record<string, string> {
  const out: Record<string, string> = {};
  for (const { key, value } of pairs) {
    if (key.trim()) out[key.trim()] = value;
  }
  return out;
}

function SensitiveInput({ value, fieldKey, onChange }: {
  value: string;
  fieldKey: string;
  onChange: (v: string) => void;
}) {
  const sensitive = /KEY|TOKEN|SECRET/i.test(fieldKey);
  const [revealed, setRevealed] = useState(false);
  return (
    <span className="sensitive-wrap">
      <input
        type={sensitive && !revealed ? "password" : "text"}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="env-value"
      />
      {sensitive && (
        <button type="button" className="btn-link" onClick={() => setRevealed((r) => !r)}>
          {revealed ? "hide" : "show"}
        </button>
      )}
    </span>
  );
}

function EnvEditor({ pairs, onChange }: { pairs: Pair[]; onChange: (p: Pair[]) => void }) {
  return (
    <div className="env-editor">
      {pairs.map((pair, i) => (
        <div key={i} className="env-row">
          <input
            value={pair.key}
            placeholder="KEY"
            onChange={(e) => {
              const next = [...pairs];
              next[i] = { ...pair, key: e.target.value };
              onChange(next);
            }}
            className="env-key"
          />
          <SensitiveInput
            fieldKey={pair.key}
            value={pair.value}
            onChange={(v) => {
              const next = [...pairs];
              next[i] = { ...pair, value: v };
              onChange(next);
            }}
          />
          <button type="button" onClick={() => onChange(pairs.filter((_, j) => j !== i))}>×</button>
        </div>
      ))}
      <button type="button" className="btn-link" onClick={() => onChange([...pairs, { key: "", value: "" }])}>
        + Env var
      </button>
    </div>
  );
}

interface BackendEntry {
  id: string;
  backend: Backend;
  envPairs: Pair[];
}

function backendEntries(cfg: BackendsConfig): BackendEntry[] {
  return Object.entries(cfg.backends)
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([id, backend]) => ({ id, backend, envPairs: toPairs(backend.env) }));
}

function entriesToConfig(entries: BackendEntry[], defaultId: string): BackendsConfig {
  const backends: Record<string, Backend> = {};
  for (const { id, backend, envPairs } of entries) {
    backends[id] = {
      ...backend,
      default: id === defaultId,
      env: fromPairs(envPairs),
    };
  }
  return { version: 2, backends };
}

function credChip(result: CredResult) {
  const cls = result.status === "ok" ? "cred-ok" : result.status === "failed" ? "cred-failed" : "cred-skipped";
  return (
    <span className={`cred-chip ${cls}`} title={result.detail ?? ""}>
      {result.status}
    </span>
  );
}

export function BackendsEditor() {
  const { data, isLoading } = useBackends();
  const putBackends = usePutBackends();

  const [entries, setEntries] = useState<BackendEntry[]>([]);
  const [defaultId, setDefaultId] = useState<string>("");
  const [credentials, setCredentials] = useState<Record<string, CredResult>>({});
  const [error, setError] = useState<string | null>(null);

  // Initialize draft from query data (once on load; also after successful save).
  useEffect(() => {
    if (!data) return;
    const es = backendEntries(data);
    setEntries(es);
    const def = es.find((e) => e.backend.default)?.id ?? es[0]?.id ?? "";
    setDefaultId(def);
  }, [data]);

  const updateEntry = (id: string, patch: Partial<BackendEntry>) => {
    setEntries((prev) => prev.map((e) => (e.id === id ? { ...e, ...patch } : e)));
  };

  const updateBackend = (id: string, patch: Partial<Backend>) => {
    setEntries((prev) =>
      prev.map((e) => (e.id === id ? { ...e, backend: { ...e.backend, ...patch } } : e)),
    );
  };

  const addBackend = () => {
    const id = `backend-${Date.now()}`;
    const newBackend: Backend = {
      name: "New backend",
      type: "claude-acp",
      default: entries.length === 0,
      default_model: "default",
      models: {
        default: { name: "Default", model: "" },
      },
    };
    const newEntry: BackendEntry = { id, backend: newBackend, envPairs: [] };
    setEntries((prev) => [...prev, newEntry]);
    if (entries.length === 0) setDefaultId(id);
  };

  const removeBackend = (id: string) => {
    setEntries((prev) => {
      const next = prev.filter((e) => e.id !== id);
      if (defaultId === id && next.length > 0) setDefaultId(next[0].id);
      return next;
    });
  };

  const handleSave = () => {
    setError(null);
    const config = entriesToConfig(entries, defaultId);
    putBackends.mutate(config, {
      onSuccess: (resp) => {
        setCredentials(resp.credentials ?? {});
        // Re-sync draft from normalized response.
        const es = backendEntries(resp);
        setEntries(es);
        const def = es.find((e) => e.backend.default)?.id ?? es[0]?.id ?? "";
        setDefaultId(def);
      },
      onError: (e) => setError(String(e)),
    });
  };

  if (isLoading) return <p>Loading backends…</p>;

  return (
    <div className="config-editor backends-editor">
      <div className="config-editor-header">
        <h2>Backends</h2>
        <button type="button" onClick={addBackend}>Add backend</button>
      </div>

      {entries.length === 0 && (
        <p className="config-empty">No backends configured. Add one to get started.</p>
      )}

      {entries.map(({ id, backend, envPairs }) => (
        <div key={id} className="backend-card">
          <div className="backend-card-header">
            <label className="backend-default-label">
              <input
                type="radio"
                name="default-backend"
                checked={id === defaultId}
                onChange={() => setDefaultId(id)}
              />
              Default
            </label>
            <input
              value={backend.name}
              placeholder="Backend name"
              onChange={(e) => updateBackend(id, { name: e.target.value })}
              className="backend-name-input"
            />
            <select
              value={backend.type}
              onChange={(e) => updateBackend(id, { type: e.target.value as Backend["type"] })}
              className="backend-type-select"
            >
              {BACKEND_TYPE_OPTIONS.map((t) => (
                <option key={t} value={t}>
                  {BACKEND_TYPE_LABELS[t]} ({t})
                </option>
              ))}
            </select>
            {credentials[id] && credChip(credentials[id])}
            <button type="button" className="btn-danger btn-sm" onClick={() => removeBackend(id)}>
              Remove
            </button>
          </div>

          <details className="backend-env-section">
            <summary>Backend env ({envPairs.length})</summary>
            <EnvEditor
              pairs={envPairs}
              onChange={(next) => updateEntry(id, { envPairs: next })}
            />
          </details>

          <div className="backend-models-section">
            <div className="backend-models-header">
              <strong>Models</strong>
            </div>
            {Object.entries(backend.models).map(([modelId, model]) => (
              <ModelRow
                key={modelId}
                modelId={modelId}
                model={model}
                isDefault={backend.default_model === modelId}
                radioGroup={`default-model-${id}`}
                onSetDefault={() => updateBackend(id, { default_model: modelId })}
                onChange={(updatedModel) => {
                  updateBackend(id, {
                    models: { ...backend.models, [modelId]: updatedModel },
                  });
                }}
                onRemove={() => {
                  const next = { ...backend.models };
                  delete next[modelId];
                  updateBackend(id, { models: next });
                }}
              />
            ))}
            <button
              type="button"
              className="btn-link"
              onClick={() => {
                const newId = `model-${Date.now()}`;
                const newModel: Model = { name: "New model", model: "" };
                updateBackend(id, {
                  models: { ...backend.models, [newId]: newModel },
                  default_model: Object.keys(backend.models).length === 0 ? newId : backend.default_model,
                });
              }}
            >
              + Add model
            </button>
          </div>
        </div>
      ))}

      {error && <p className="form-error">{error}</p>}

      <div className="backends-footer">
        <button type="button" onClick={handleSave} disabled={putBackends.isPending}>
          {putBackends.isPending ? "Saving…" : "Save"}
        </button>
      </div>
    </div>
  );
}
