import { useState } from "react";
import type { Model } from "../../schemas/backends";

interface ModelRowProps {
  modelId: string;
  model: Model;
  isDefault: boolean;
  radioGroup: string;
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

export function ModelRow({ modelId, model, isDefault, radioGroup, onSetDefault, onChange, onRemove }: ModelRowProps) {
  const [expanded, setExpanded] = useState(false);
  const pairs = toPairs(model.env);

  const updateEnv = (next: Pair[]) => onChange({ ...model, env: fromPairs(next) });

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
          {expanded ? "▴ env" : `▾ env (${pairs.length})`}
        </button>
        <button type="button" className="btn-danger btn-sm" onClick={onRemove}>Remove</button>
      </div>
      {expanded && (
        <div className="model-env-editor">
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
              <button type="button" onClick={() => updateEnv(pairs.filter((_, j) => j !== i))}>×</button>
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
