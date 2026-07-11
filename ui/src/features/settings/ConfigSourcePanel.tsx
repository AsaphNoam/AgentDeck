import { useEffect, useMemo, useState } from "react";
import { useConfig, useProjects, configErrorMessage } from "../../api/config";
import {
  useBindConfigSource,
  useConfigSources,
  useDeleteConfigSource,
  usePreviewConfigSource,
  useRefreshConfigSource,
} from "../../api/configSources";
import type { BackendType } from "../../schemas/backends";
import type { Effective, SourceProvider } from "../../schemas/configSources";

const PROVIDER_FOR_TYPE: Partial<Record<BackendType, SourceProvider>> = {
  "claude-acp": "claude-code",
  "codex-acp": "codex",
};

function providerLabel(p: SourceProvider): string {
  return p === "claude-code" ? "Claude Code" : "Codex";
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

const INVENTORY_GROUPS: { label: string; kinds: string[] }[] = [
  { label: "Instructions", kinds: ["instruction", "instruction_import"] },
  { label: "Skills", kinds: ["skill"] },
  { label: "Agents", kinds: ["agent"] },
  { label: "Rules", kinds: ["rule"] },
  { label: "MCP", kinds: ["mcp"] },
  { label: "Hooks", kinds: ["hooks"] },
  { label: "Plugins", kinds: ["plugins"] },
];

function EffectiveView({ effective }: { effective: Effective }) {
  const assets = effective.assets ?? [];
  return (
    <div className="source-effective">
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
      <div className="source-inventory">
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
  initialProjectId,
  defaultOpen,
}: {
  backendId: string;
  backendType: BackendType;
  initialProjectId?: string;
  defaultOpen?: boolean;
}) {
  const provider = PROVIDER_FOR_TYPE[backendType];
  const { data: projects } = useProjects();
  const { data: config } = useConfig();

  const projectIds = useMemo(() => Object.keys(projects ?? {}).sort(), [projects]);
  const [projectId, setProjectId] = useState<string>(initialProjectId ?? "");
  useEffect(() => {
    if (projectId || projectIds.length === 0) return;
    const preferred =
      initialProjectId && projectIds.includes(initialProjectId)
        ? initialProjectId
        : config?.default_project && projectIds.includes(config.default_project)
          ? config.default_project
          : projectIds[0];
    setProjectId(preferred);
  }, [projectId, projectIds, config?.default_project, initialProjectId]);

  const { data: sources } = useConfigSources(projectId || undefined);
  const preview = usePreviewConfigSource();
  const bind = useBindConfigSource(projectId || undefined);
  const refresh = useRefreshConfigSource(projectId || undefined);
  const del = useDeleteConfigSource(projectId || undefined);

  // The effective view is loaded on demand (link preview or refresh), never
  // rendered from a cached secret — the server only ever sends redacted fields.
  const [effective, setEffective] = useState<Effective | null>(null);
  const [error, setError] = useState<string | null>(null);
  // The mode the current preview token was minted for. The server derives the bound
  // mode SOLELY from the token, so binding must use a token minted for the requested
  // mode — otherwise "Link (Mirrored)" silently persists Linked (no mirror cache).
  const [previewMode, setPreviewMode] = useState<"linked" | "mirrored" | null>(null);

  // Federation applies to Claude/Codex only.
  if (!provider) return null;

  const binding = (sources?.bindings ?? []).find((b) => b.backend_id === backendId);
  const claims = ["launch_defaults", "model_catalog", "setup"];

  const runPreview = (mode: "linked" | "mirrored") => {
    setError(null);
    setEffective(null);
    setPreviewMode(null);
    preview.mutate(
      { provider, root: "auto", mode, claims, project: projectId },
      {
        onSuccess: (res) => {
          setEffective(res.effective);
          setPreviewMode(mode);
        },
        onError: (e) => setError(configErrorMessage(e)),
      },
    );
  };

  const bindWithToken = (token: string) => {
    bind.mutate(
      { backendId, previewToken: token, overrides: {} },
      {
        onSuccess: () => {
          setEffective(null);
          setPreviewMode(null);
          preview.reset();
        },
        onError: (e) => setError(configErrorMessage(e)),
      },
    );
  };

  const runBind = (mode: "linked" | "mirrored") => {
    setError(null);
    const token = preview.data?.preview_token;
    // Bind only with a token minted for THIS mode. If none exists yet (first click)
    // or the discovery preview was minted for a different mode, re-preview for the
    // requested mode and bind with that fresh token, so the persisted mode matches
    // the button the user clicked.
    if (token && previewMode === mode) {
      bindWithToken(token);
      return;
    }
    preview.mutate(
      { provider, root: "auto", mode, claims, project: projectId },
      {
        onSuccess: (res) => {
          setEffective(res.effective);
          setPreviewMode(mode);
          bindWithToken(res.preview_token);
        },
        onError: (e) => setError(configErrorMessage(e)),
      },
    );
  };

  const runRefresh = () => {
    setError(null);
    refresh.mutate(backendId, {
      onSuccess: (res) => setEffective((res as { effective: Effective }).effective),
      onError: (e) => setError(configErrorMessage(e)),
    });
  };

  const runUnlink = () => {
    setError(null);
    setEffective(null);
    del.mutate({ backendId, detach: false }, { onError: (e) => setError(configErrorMessage(e)) });
  };

  return (
    <details className="backend-source-section" open={defaultOpen}>
      <summary>Configuration source ({providerLabel(provider)})</summary>

      <div className="source-panel">
        <label className="source-project-select">
          Project&nbsp;
          <select value={projectId} onChange={(e) => setProjectId(e.target.value)}>
            {projectIds.length === 0 && <option value="">No projects</option>}
            {projectIds.map((id) => (
              <option key={id} value={id}>
                {id}
              </option>
            ))}
          </select>
        </label>

        {!binding && (
          <div className="source-unbound">
            <p className="source-hint">
              Link {providerLabel(provider)}'s native configuration so this backend reads its real model,
              instructions and tooling. Nothing is copied or modified.
            </p>
            {preview.data && effective && (
              <div className="source-preview">
                <p className="source-hint">Discovered at {preview.data.report.source_digest ? "the native root" : "—"}:</p>
                <EffectiveView effective={effective} />
              </div>
            )}
            <div className="source-actions">
              {!preview.data ? (
                <button type="button" disabled={!projectId || preview.isPending} onClick={() => runPreview("linked")}>
                  {preview.isPending ? "Discovering…" : "Discover native config"}
                </button>
              ) : (
                <>
                  <button type="button" disabled={bind.isPending} onClick={() => runBind("linked")}>
                    Link (Linked — recommended)
                  </button>
                  <button type="button" className="btn-link" disabled={bind.isPending} onClick={() => runBind("mirrored")}>
                    Link (Mirrored — compatibility)
                  </button>
                </>
              )}
              <button type="button" className="btn-link" disabled title="Detached import is not available yet">
                Import detached copy (unavailable)
              </button>
            </div>
          </div>
        )}

        {binding && (
          <div className="source-bound">
            <div className="source-status-row">
              <span className={`source-health source-health-${binding.health ?? "unknown"}`}>
                {binding.stale ? "stale" : binding.health ?? "unknown"}
              </span>
              <span className="source-mode">{binding.mode}</span>
              <code className="source-root" title={binding.root}>
                {binding.root}
              </code>
            </div>
            {(binding.stale || binding.health === "source_invalid" || binding.health === "approval_required") && (
              <p className="source-warning">
                This source needs attention ({binding.health}). Refresh after fixing it, or unlink.
              </p>
            )}
            {effective && <EffectiveView effective={effective} />}
            <div className="source-actions">
              <button type="button" disabled={refresh.isPending} onClick={runRefresh}>
                {refresh.isPending ? "Refreshing…" : effective ? "Refresh" : "Load effective view"}
              </button>
              <button type="button" className="btn-danger btn-sm" disabled={del.isPending} onClick={runUnlink}>
                Unlink
              </button>
            </div>
          </div>
        )}

        {error && <p className="form-error">{error}</p>}
      </div>
    </details>
  );
}
