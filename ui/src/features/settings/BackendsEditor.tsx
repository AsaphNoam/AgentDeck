import { useEffect, useRef, useState } from "react";
import { useBackends, usePutBackends, configErrorMessage, type CatalogETagged } from "../../api/config";
import type {
  BackendsConfig,
  Backend,
  Model,
  CredResult,
  CreateBackendResponse,
} from "../../schemas/backends";
import { BACKEND_TYPE_LABELS, BACKEND_TYPE_OPTIONS } from "../../lib/backendTypes";
import { ModelRow } from "./ModelRow";
import { ConfigSourcePanel } from "./ConfigSourcePanel";
import { AddBackendDialog } from "./AddBackendDialog";

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
  return Object.entries(cfg.backends ?? {})
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
  const { data, isLoading, refetch } = useBackends();
  const putBackends = usePutBackends();

  const [entries, setEntries] = useState<BackendEntry[]>([]);
  const [defaultId, setDefaultId] = useState<string>("");
  const [catalogEtag, setCatalogEtag] = useState<string | undefined>();
  const [savedBackendIds, setSavedBackendIds] = useState<Set<string>>(new Set());
  const [credentials, setCredentials] = useState<Record<string, CredResult>>({});
  const [error, setError] = useState<string | null>(null);
  const [addOpen, setAddOpen] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);

  const seedDraft = (cfg: CatalogETagged<BackendsConfig>) => {
    const es = backendEntries(cfg);
    setEntries(es);
    setSavedBackendIds(new Set(Object.keys(cfg.backends ?? {})));
    setDefaultId(es.find((e) => e.backend.default)?.id ?? es[0]?.id ?? "");
    setCatalogEtag(cfg.catalogEtag);
  };

  // Seed the draft from the server exactly once. Re-seeding on every query
  // change would let a background refetch — including the one an item create or
  // a source connection triggers — silently discard unrelated unsaved edits
  // (FS-04.R40). Save and create merge their own results explicitly.
  const seeded = useRef(false);
  useEffect(() => {
    if (!data || seeded.current) return;
    seeded.current = true;
    seedDraft(data);
  }, [data]);

  const updateEntry = (id: string, patch: Partial<BackendEntry>) => {
    setEntries((prev) => prev.map((e) => (e.id === id ? { ...e, ...patch } : e)));
  };

  const updateBackend = (id: string, patch: Partial<Backend>) => {
    setEntries((prev) =>
      prev.map((e) => (e.id === id ? { ...e, backend: { ...e.backend, ...patch } } : e)),
    );
  };

  // mergeBackend inserts or replaces exactly one entry, leaving every other
  // unsaved edit in the draft untouched.
  const mergeBackend = (id: string, backend: Backend) => {
    setEntries((prev) => {
      const entry: BackendEntry = { id, backend, envPairs: toPairs(backend.env) };
      const at = prev.findIndex((e) => e.id === id);
      if (at === -1) return [...prev, entry];
      const next = [...prev];
      next[at] = entry;
      return next;
    });
    setSavedBackendIds((prev) => new Set(prev).add(id));
    if (backend.default) setDefaultId(id);
  };

  const handleCreated = ({ backend_id: id, backend, connection, catalogEtag: nextCatalogEtag }: CatalogETagged<CreateBackendResponse>) => {
    setError(null);
    mergeBackend(id, backend);
    setCatalogEtag(nextCatalogEtag);
    setAddOpen(false);
    if (!connection) {
      setNotice(null);
    } else if (connection.status === "connected") {
      const added = connection.models_added ?? 0;
      setNotice(
        `Connected ${backend.name} to your native configuration` +
          (added > 0 ? ` and imported ${added} configured model${added === 1 ? "" : "s"}.` : ".") +
          " Imported models are what your configuration names, not a check of availability or entitlement.",
      );
    } else {
      setNotice(
        `${backend.name} was created but is not connected: ${connection.error?.message ?? "the connection failed"}.` +
          " Use its Configuration source panel to try again.",
      );
    }
  };

  // syncBackendFromServer refreshes one card from the authoritative catalog after
  // a connection imported models into it, without re-seeding the whole draft.
  const syncBackendFromServer = async (id: string) => {
    const fresh = await refetch();
    const backend = fresh.data?.backends?.[id];
    if (backend) mergeBackend(id, backend);
    setCatalogEtag(fresh.data?.catalogEtag);
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
    const config = { ...entriesToConfig(entries, defaultId), catalogEtag };
    putBackends.mutate(config, {
      onSuccess: (resp) => {
        setCredentials(resp.credentials ?? {});
        seedDraft(resp); // re-sync the draft from the normalized response
      },
      onError: (e) => setError(configErrorMessage(e)),
    });
  };

  if (isLoading) return <p data-ui="config-editor" data-state="loading" data-variant="backends">Loading backends…</p>;

  return (
    <div className="config-editor backends-editor" data-ui="config-editor" data-state={error ? "error" : entries.length === 0 ? "empty" : undefined} data-variant="backends">
      <div className="config-editor-header" data-slot="header">
        <h2>Backends</h2>
        <button type="button" onClick={() => setAddOpen(true)}>Add backend</button>
      </div>

      <AddBackendDialog
        open={addOpen}
        existingIds={entries.map((e) => e.id)}
        onCancel={() => setAddOpen(false)}
        onCreated={handleCreated}
      />

      {notice && <p className="config-notice" role="status">{notice}</p>}

      {entries.length === 0 && (
        <p className="config-empty">No backends configured. Add one to get started.</p>
      )}

      {entries.map(({ id, backend, envPairs }) => (
        <div key={id} className="backend-card" data-slot="item">
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

          {backend.type === "codex-acp" && (
            <label className="backend-autosync-label" title="On dashboard startup, add newly available Codex models from ~/.codex/models_cache.json. Existing entries and the default are never changed.">
              <input
                type="checkbox"
                checked={backend.autosync_models ?? false}
                onChange={(e) => updateBackend(id, { autosync_models: e.target.checked })}
              />
              Auto-sync models from Codex on startup
            </label>
          )}

          {backend.type === "claude-acp" && (
            <label className="backend-autosync-label" title="At the next dashboard start, add the models named in your user-level Claude settings (~/.claude/settings.json). Existing entries and the default are never changed.">
              <input
                type="checkbox"
                checked={backend.autosync_models ?? false}
                onChange={(e) => updateBackend(id, { autosync_models: e.target.checked })}
              />
              Import configured models from Claude on startup
            </label>
          )}

          {(backend.type === "claude-acp" || backend.type === "codex-acp") && (
            <ConfigSourcePanel
              backendId={id}
              backendType={backend.type}
              persisted={savedBackendIds.has(id)}
              onConnected={() => void syncBackendFromServer(id)}
            />
          )}

          <div className="backend-models-section">
            <div className="backend-models-header">
              <strong>Models</strong>
            </div>
            {Object.entries(backend.models ?? {}).map(([modelId, model]) => (
              <ModelRow
                key={modelId}
                modelId={modelId}
                model={model}
                isDefault={backend.default_model === modelId}
                radioGroup={`default-model-${id}`}
                onSetDefault={() => updateBackend(id, { default_model: modelId })}
                onChange={(updatedModel) => {
                  updateBackend(id, {
                    models: { ...(backend.models ?? {}), [modelId]: updatedModel },
                  });
                }}
                onRemove={() => {
                  const next = { ...(backend.models ?? {}) };
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
                  models: { ...(backend.models ?? {}), [newId]: newModel },
                  default_model: Object.keys(backend.models ?? {}).length === 0 ? newId : backend.default_model,
                });
              }}
            >
              + Add model
            </button>
          </div>
        </div>
      ))}

      {error && <p className="form-error">{error}</p>}

      <div className="backends-footer" data-slot="actions">
        <button type="button" onClick={handleSave} disabled={putBackends.isPending}>
          {putBackends.isPending ? "Saving…" : "Save"}
        </button>
      </div>
    </div>
  );
}
