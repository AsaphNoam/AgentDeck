import { useEffect, useState } from "react";
import { configErrorMessage } from "../../api/config";
import {
  useBindConfigSource,
  useConfigSources,
  useDeleteConfigSource,
  usePreviewConfigSource,
  useRefreshConfigSource,
} from "../../api/configSources";
import type { BackendType } from "../../schemas/backends";
import type { BindingView, Effective, SourceProvider } from "../../schemas/configSources";

const PROVIDER_FOR_TYPE: Partial<Record<BackendType, SourceProvider>> = {
  "claude-acp": "claude-code",
  "codex-acp": "codex",
};

function providerLabel(p: SourceProvider): string {
  return p === "claude-code" ? "Claude Code" : "Codex";
}

type ConfigSourceVisualState = "unbound" | "ok" | "source_invalid" | "approval_required" | "source_conflict" | "stale" | "unknown";

function configSourceVisualState(binding?: BindingView): ConfigSourceVisualState {
  if (!binding) return "unbound";
  if (binding.stale) return "stale";
  if (["ok", "source_invalid", "approval_required", "source_conflict"].includes(binding.health ?? "")) {
    return binding.health as ConfigSourceVisualState;
  }
  return "unknown";
}

// modelProvenanceLabel turns the redacted per-field provenance into the honest
// source label the spec (§2.8) requires: an AgentDeck override, an inherited
// native value naming its scope/path, or "inherit the CLI default".
function provenanceLabel(effective: Effective, field: "model" | "effort", value: string | null | undefined): string {
  if (value == null || value === "") return "Inherit CLI default";
  const prov = effective.provenance?.[field];
  if (!prov) return value;
  if (prov.scope === "agentdeck_override") return `${value} — AgentDeck override`;
  const where = prov.path ? `${prov.scope} (${prov.path})` : prov.scope;
  return `${value} — inherited from ${where}`;
}

// Asset kinds differ per provider: Claude emits singular kinds (instruction, rule,
// mcp) while Codex emits plural ones (instructions, rules, mcp_servers). Each group
// lists both so a Codex user actually sees AGENTS.md instructions and MCP servers.
const INVENTORY_GROUPS: { label: string; kinds: string[] }[] = [
  { label: "Instructions", kinds: ["instruction", "instruction_import", "instructions"] },
  { label: "Skills", kinds: ["skill"] },
  { label: "Agents", kinds: ["agent"] },
  { label: "Rules", kinds: ["rule", "rules"] },
  { label: "MCP", kinds: ["mcp", "mcp_servers"] },
  { label: "Hooks", kinds: ["hooks"] },
  { label: "Plugins", kinds: ["plugins"] },
];

function EffectiveView({ effective }: { effective: Effective }) {
  const assets = effective.assets ?? [];
  return (
    <div className="source-effective" data-slot="effective">
      <div className="source-field-row">
        <span className="source-field-label">Model</span>
        <span className="source-field-value">{provenanceLabel(effective, "model", effective.model)}</span>
      </div>
      <div className="source-field-row">
        <span className="source-field-label">Effort</span>
        <span className="source-field-value">{provenanceLabel(effective, "effort", effective.effort)}</span>
      </div>
      {effective.provider && (
        <div className="source-field-row">
          <span className="source-field-label">Provider</span>
          <span className="source-field-value">{effective.provider}</span>
        </div>
      )}
      {(effective.models?.length ?? 0) > 0 && (
        <div className="source-field-row">
          <span className="source-field-label">Configured models</span>
          <span className="source-field-value">
            {effective.models!.map((m) => m.id).join(", ")}
            <em className="source-note"> (not an entitlement check)</em>
          </span>
        </div>
      )}
      <div className="source-inventory" data-slot="inventory">
        {INVENTORY_GROUPS.map((group) => {
          const items = assets.filter((a) => group.kinds.includes(a.kind));
          if (items.length === 0) return null;
          return (
            <div key={group.label} className="source-inventory-group">
              <strong>{group.label}</strong>
              <ul>
                {items.map((a, i) => (
                  <li key={i} title={a.path}>
                    {a.name ?? a.path.split("/").pop()} <span className="source-scope">[{a.scope}]</span>{" "}
                    <span className="source-status">{a.status}</span>
                  </li>
                ))}
              </ul>
            </div>
          );
        })}
        {(effective.environment_keys?.length ?? 0) > 0 && (
          <div className="source-inventory-group">
            <strong>Env keys</strong>
            <ul>
              {effective.environment_keys!.map((k, i) => (
                <li key={i}>
                  {k.name} <span className="source-scope">[{k.scope}]</span>
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
    </div>
  );
}

export function ConfigSourcePanel({
  backendId,
  backendType,
  persisted = true,
  defaultOpen,
  claimMutation,
  releaseMutation,
  onConnected,
}: {
  backendId: string;
  backendType: BackendType;
  // A Settings draft has a client-only id until the enclosing backend catalog
  // is saved. Binding against it would fail with "unknown backend".
  persisted?: boolean;
  defaultOpen?: boolean;
  claimMutation?: () => boolean;
  releaseMutation?: () => void;
  // onConnected lets the enclosing editor refresh just this backend's card after
  // a connection imported models into it.
  onConnected?: () => void;
}) {
  const provider = PROVIDER_FOR_TYPE[backendType];

  // A binding is backend-global (FS-08.R34): the normal connection chooses no
  // project, root, profile, claim set, or mode, and works with no project
  // configured at all. The launched agent's actual project still resolves later.
  const { data: sources } = useConfigSources();
  const preview = usePreviewConfigSource();
  const bind = useBindConfigSource();
  const refresh = useRefreshConfigSource();
  const del = useDeleteConfigSource();

  // The effective view is loaded on demand (link preview or refresh), never
  // rendered from a cached secret — the server only ever sends redacted fields.
  const [effective, setEffective] = useState<Effective | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [importNote, setImportNote] = useState<string | null>(null);
  // AgentDeck override inputs for a bound source (empty = inherit the native value).
  const [overrideModel, setOverrideModel] = useState("");
  const [overrideEffort, setOverrideEffort] = useState("");

  const binding = (sources?.bindings ?? []).find((b) => b.backend_id === backendId);
  const visualState = configSourceVisualState(binding);

  // Seed the override inputs from the persisted binding so the fields reflect the
  // current override and "Reset to inherit" is meaningful.
  useEffect(() => {
    setOverrideModel(binding?.overrides?.model ?? "");
    setOverrideEffort(binding?.overrides?.effort ?? "");
  }, [binding?.overrides?.model, binding?.overrides?.effort]);

  // Federation applies to Claude/Codex only.
  if (!provider) return null;

  if (!persisted) {
    return (
      <details className="backend-source-section" data-ui="config-source" data-state="unbound" open={defaultOpen}>
        <summary>Configuration source ({providerLabel(provider)})</summary>
        <div className="source-panel">
          <p className="source-hint">Save this backend before linking a configuration source.</p>
        </div>
      </details>
    );
  }

  const claims = ["launch_defaults", "model_catalog", "setup"];

  // runConnect is the one visible action of the normal flow (FS-08.R34): a
  // standard auto-root, user-level, read-only Linked preview followed
  // immediately by its matching bind with the model import enabled. Mirrored is
  // never offered as recovery — it uses the same resolver and only adds a
  // post-success cache — so a failure leaves the backend unbound and retryable.
  const runConnect = () => {
    if (claimMutation && !claimMutation()) return;
    setError(null);
    setImportNote(null);
    preview.mutate(
      { provider, root: "auto", mode: "linked", claims },
      {
        onSuccess: (res) =>
          bind.mutate(
            { backendId, previewToken: res.preview_token, overrides: {}, enableModelSync: true },
            {
              onSuccess: (bound) => {
                setEffective(null);
                preview.reset();
                const added = bound.models_added ?? 0;
                setImportNote(
                  added > 0
                    ? `Imported ${added} configured model${added === 1 ? "" : "s"}. This lists what your configuration names; it does not check availability or entitlement.`
                    : "No new configured models to import. Your existing models are unchanged.",
                );
                onConnected?.();
              },
              onError: (e) => setError(configErrorMessage(e)),
              onSettled: releaseMutation,
            },
          ),
        onError: (e) => {
          setError(configErrorMessage(e));
          releaseMutation?.();
        },
      },
    );
  };

  const runRefresh = () => {
    if (claimMutation && !claimMutation()) return;
    setError(null);
    refresh.mutate(backendId, {
      onSuccess: (res) => setEffective((res as { effective: Effective }).effective),
      onError: (e) => setError(configErrorMessage(e)),
      onSettled: releaseMutation,
    });
  };

  const runUnlink = () => {
    if (claimMutation && !claimMutation()) return;
    setError(null);
    setEffective(null);
    del.mutate({ backendId, detach: false }, { onError: (e) => setError(configErrorMessage(e)), onSettled: releaseMutation });
  };

  // applyOverrides changes the AgentDeck model/effort overrides on a bound source by
  // re-previewing the SAME source (its root/profile/mode) for a fresh consent token,
  // then re-binding with the new overrides. Passing null for both resets to native
  // inheritance. The server derives the mode from the token, so the token is minted
  // for the binding's current mode.
  const applyOverrides = (overrides: { model: string | null; effort: string | null }) => {
    if (!binding) return;
    if (claimMutation && !claimMutation()) return;
    setError(null);
    preview.mutate(
      {
        provider,
        root: binding.root,
        profile: binding.profile,
        mode: binding.mode as "linked" | "mirrored",
        claims,
      },
      {
        onSuccess: (res) =>
          bind.mutate(
            { backendId, previewToken: res.preview_token, overrides },
            {
              onSuccess: () => {
                setEffective(null);
                preview.reset();
              },
              onError: (e) => setError(configErrorMessage(e)),
              onSettled: releaseMutation,
            },
          ),
        onError: (e) => {
          setError(configErrorMessage(e));
          releaseMutation?.();
        },
      },
    );
  };

  return (
    <details className="backend-source-section" data-ui="config-source" data-state={visualState} open={defaultOpen}>
      <summary>Configuration source ({providerLabel(provider)})</summary>

      <div className="source-panel">
        {!binding && (
          <div className="source-unbound" data-slot="status">
            <p className="source-hint">
              AgentDeck reads {providerLabel(provider)}'s existing setup — its model, instructions and
              tooling — without copying or modifying it.
            </p>
            <div className="source-actions" data-slot="actions">
              <button type="button" disabled={preview.isPending || bind.isPending} onClick={runConnect}>
                {preview.isPending || bind.isPending
                  ? "Connecting…"
                  : `Use my ${providerLabel(provider)} configuration`}
              </button>
              <button type="button" className="btn-link" disabled title="Detached import is not available yet">
                Import detached copy (unavailable)
              </button>
            </div>
          </div>
        )}

        {binding && (
          <div className="source-bound">
            <div className="source-status-row" data-slot="status">
              <span className={`source-health source-health-${visualState}`}>
                {binding.stale ? "stale" : binding.health ?? "unknown"}
              </span>
              <span className="source-mode">{binding.mode}</span>
              <code className="source-root" data-slot="root" title={binding.root}>
                {binding.root}
              </code>
            </div>
            {(binding.stale || binding.health === "source_invalid" || binding.health === "approval_required") && (
              <p className="source-warning" data-slot="warning">
                This source needs attention ({binding.health}). Refresh after fixing it, or unlink.
              </p>
            )}
            {effective && <EffectiveView effective={effective} />}
            <div className="source-overrides" data-slot="overrides">
              <div className="source-field-row">
                <label className="source-field-label" htmlFor={`src-override-model-${backendId}`}>Model override</label>
                <input
                  id={`src-override-model-${backendId}`}
                  value={overrideModel}
                  placeholder="inherit native"
                  onChange={(e) => setOverrideModel(e.target.value)}
                />
              </div>
              <div className="source-field-row">
                <label className="source-field-label" htmlFor={`src-override-effort-${backendId}`}>Effort override</label>
                <input
                  id={`src-override-effort-${backendId}`}
                  value={overrideEffort}
                  placeholder="inherit native"
                  onChange={(e) => setOverrideEffort(e.target.value)}
                />
              </div>
              <div className="source-actions" data-slot="actions">
                <button
                  type="button"
                  disabled={bind.isPending || preview.isPending}
                  onClick={() => applyOverrides({ model: overrideModel.trim() || null, effort: overrideEffort.trim() || null })}
                >
                  Apply override
                </button>
                <button
                  type="button"
                  className="btn-link"
                  disabled={bind.isPending || preview.isPending}
                  onClick={() => {
                    setOverrideModel("");
                    setOverrideEffort("");
                    applyOverrides({ model: null, effort: null });
                  }}
                >
                  Reset to inherit
                </button>
              </div>
            </div>
            <div className="source-actions" data-slot="actions">
              <button type="button" disabled={refresh.isPending} onClick={runRefresh}>
                {refresh.isPending ? "Refreshing…" : effective ? "Refresh" : "Load effective view"}
              </button>
              <button
                type="button"
                className="btn-link"
                disabled
                title="Detached import (materializing an AgentDeck-owned copy) is not available yet — deferred until a verified launch-injection path exists"
              >
                Detach copy (unavailable)
              </button>
              <button type="button" className="btn-danger btn-sm" disabled={del.isPending} onClick={runUnlink}>
                Unlink
              </button>
            </div>
          </div>
        )}

        {/* The import result stands on its own: it stays readable while the
            invalidated binding query refetches, and states plainly that it
            lists what the configuration names rather than what is available. */}
        {importNote && <p className="source-hint">{importNote}</p>}
        {error && <p className="form-error">{error}</p>}
      </div>
    </details>
  );
}
