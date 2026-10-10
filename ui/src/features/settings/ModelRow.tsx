import { useState } from "react";
import type { Model } from "../../schemas/backends";

// ModelCapability tells the row whether the owning backend's current type
// supports NEW effort/fast declarations (FS-09.R62), including per-option
// uncertainty when interface metadata is missing.
export interface ModelCapability {
  effortAllowed: boolean;
  fastAllowed: boolean;
  effortUnknown: boolean;
  fastUnknown: boolean;
}

interface ModelRowProps {
  modelId: string;
  model: Model;
  isDefault: boolean;
  radioGroup: string;
  capability: ModelCapability;
  onSetDefault: () => void;
  onChange: (model: Model) => void;
  onRemove: () => void;
}

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

export function ModelRow({ modelId, model, isDefault, radioGroup, capability, onSetDefault, onChange, onRemove }: ModelRowProps) {
  const [expanded, setExpanded] = useState(false);
  const pairs = toPairs(model.env);

  const updateEnv = (next: Pair[]) => onChange({ ...model, env: fromPairs(next) });
  const updateEfforts = (raw: string) => {
    const efforts = raw.split(",").map((level) => level.trim()).filter(Boolean);
    onChange({ ...model, efforts, default_effort: efforts.includes(model.default_effort ?? "") ? model.default_effort : efforts[0] });
  };

  // A declaration already in the draft stays visible with a reason and an
  // explicit clear, even when the current backend type can't offer it as a
  // NEW choice (FS-09.R62). Clearing effort also clears its default_effort;
  // clearing fast just turns it off.
  const hasEffort = (model.efforts ?? []).length > 0;
  const hasFast = model.fast === true;
  const effortReason = capability.effortUnknown
    ? "Effort support could not be loaded."
    : "This backend type doesn't support this.";
  const fastReason = capability.fastUnknown
    ? "Fast support could not be loaded."
    : "This backend type doesn't support this.";
  const clearEfforts = () => onChange({ ...model, efforts: [], default_effort: undefined });
  const clearFast = () => onChange({ ...model, fast: false });
  // Keep an unsupported declaration in view rather than behind a collapsed row.
  const needsRepair = (hasEffort && !capability.effortAllowed) || (hasFast && !capability.fastAllowed);
  const showEditor = expanded || needsRepair;

  return (
    <div className="model-row">
      <div className="model-row-header">
        <label className="model-default-label">
          <input
            type="radio"
            name={radioGroup}
            checked={isDefault}
            onChange={onSetDefault}
          />
        </label>
        <input
          value={model.name}
          placeholder="Display name"
          onChange={(e) => onChange({ ...model, name: e.target.value })}
          className="model-display-name"
        />
        <input
          value={model.model}
          placeholder="Provider model string"
          onChange={(e) => onChange({ ...model, model: e.target.value })}
          className="model-provider-string"
        />
        <code className="config-slug">{modelId}</code>
        {isDefault && <span className="config-badge">default</span>}
        <button type="button" className="btn-link" onClick={() => setExpanded((x) => !x)}>
          {showEditor ? "▴ env" : `▾ env (${pairs.length})`}
        </button>
        <button type="button" className="config-text-action-danger ad-button-danger" onClick={onRemove}>Remove</button>
      </div>
      {showEditor && (
        <div className="model-env-editor">
          {(capability.effortAllowed || hasEffort) && (
            <label className="form-field">
              <span>Effort levels (comma separated)</span>
              {capability.effortAllowed ? (
                <input value={(model.efforts ?? []).join(", ")} placeholder="low, medium, high" onChange={(e) => updateEfforts(e.target.value)} />
              ) : (
                <span className="model-capability-note">
                  <input value={(model.efforts ?? []).join(", ")} disabled />
                  <span className="model-capability-reason">{effortReason}</span>
                  <button type="button" className="btn-link" onClick={clearEfforts}>Clear effort levels</button>
                </span>
              )}
            </label>
          )}
          {(model.efforts ?? []).length > 0 && (
            <label className="form-field">
              <span>Default effort</span>
              <select
                value={model.default_effort ?? ""}
                disabled={!capability.effortAllowed}
                onChange={(e) => onChange({ ...model, default_effort: e.target.value })}
              >
                {(model.efforts ?? []).map((level) => <option key={level} value={level}>{level}</option>)}
              </select>
            </label>
          )}
          {(capability.fastAllowed || hasFast) && (
            <label className="form-field">
              <span>Fast mode capability</span>
              {capability.fastAllowed ? (
                <span><input type="checkbox" checked={model.fast ?? false} onChange={(e) => onChange({ ...model, fast: e.target.checked })} /> This model can use the adapter's faster, higher-usage mode</span>
              ) : (
                <span className="model-capability-note">
                  <input type="checkbox" checked disabled />
                  <span className="model-capability-reason">{fastReason}</span>
                  <button type="button" className="btn-link" onClick={clearFast}>Clear fast mode</button>
                </span>
              )}
            </label>
          )}
          {pairs.map((pair, i) => (
            <div key={i} className="env-row">
              <input
                value={pair.key}
                placeholder="KEY"
                onChange={(e) => {
                  const next = [...pairs];
                  next[i] = { ...pair, key: e.target.value };
                  updateEnv(next);
                }}
                className="env-key"
              />
              <SensitiveInput
                fieldKey={pair.key}
                value={pair.value}
                onChange={(v) => {
                  const next = [...pairs];
                  next[i] = { ...pair, value: v };
                  updateEnv(next);
                }}
              />
              <button type="button" className="ad-button-icon" onClick={() => updateEnv(pairs.filter((_, j) => j !== i))}>×</button>
            </div>
          ))}
          <button type="button" className="btn-link" onClick={() => updateEnv([...pairs, { key: "", value: "" }])}>
            + Env var
          </button>
        </div>
      )}
    </div>
  );
}
