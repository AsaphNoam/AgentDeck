import { useEffect, useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { getCapabilities } from "../../api/client";
import { useRoles } from "../../api/config";
import { useProjects } from "../../api/config";
import { useBackends } from "../../api/config";
import { useConfig } from "../../api/config";
import { useLaunchAgent } from "../../api/config";
import { useConfigSources } from "../../api/configSources";
import { terminalSupported } from "../../lib/backendTypes";
import { useSuggestedName } from "./useSuggestedName";

interface NewAgentModalProps {
  open: boolean;
  onClose: () => void;
  /** Pre-select role (e.g. from onboarding wizard). */
  initialRole?: string;
  /** Pre-select project (e.g. from onboarding wizard). */
  initialProject?: string;
}

export function NewAgentModal({ open, onClose, initialRole, initialProject }: NewAgentModalProps) {
  const { data: rolesData } = useRoles();
  const { data: projectsData } = useProjects();
  const { data: backendsData } = useBackends();
  const { data: configData } = useConfig();
  const launch = useLaunchAgent();

  const roleEntries = Object.entries(rolesData ?? {});
  const projectEntries = Object.entries(projectsData ?? {});

  const defaultBackendId =
    Object.entries(backendsData?.backends ?? {}).find(([, b]) => b.default)?.[0] ??
    Object.keys(backendsData?.backends ?? {})[0] ??
    "";

  const [role, setRole] = useState(initialRole ?? "");
  const [project, setProject] = useState(initialProject ?? "");
  const [backendId, setBackendId] = useState(defaultBackendId);
  const [modelId, setModelId] = useState("");
  const [agentInterface, setAgentInterface] = useState<"chat" | "terminal">("chat");
  const [terminalAvailable, setTerminalAvailable] = useState(true);
  const [launchError, setLaunchError] = useState<string | null>(null);

  const [name, setName] = useSuggestedName(role, project);

  // Set defaults once data loads: prefer the configured default_role/
  // default_project, falling back to the first available entry only when the
  // configured id is absent (or unset).
  useEffect(() => {
    if (role || roleEntries.length === 0) return;
    const configured = configData?.default_role;
    if (configured && roleEntries.some(([id]) => id === configured)) {
      setRole(configured);
    } else {
      setRole(roleEntries[0][0]);
    }
  }, [roleEntries.length, configData?.default_role]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (project || projectEntries.length === 0) return;
    const configured = configData?.default_project;
    if (configured && projectEntries.some(([id]) => id === configured)) {
      setProject(configured);
    } else {
      setProject(projectEntries[0][0]);
    }
  }, [projectEntries.length, configData?.default_project]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!backendId && defaultBackendId) setBackendId(defaultBackendId);
  }, [defaultBackendId]); // eslint-disable-line react-hooks/exhaustive-deps

  // When backend changes, reset model to that backend's default_model.
  useEffect(() => {
    const backend = backendsData?.backends[backendId];
    setModelId(backend?.default_model ?? Object.keys(backend?.models ?? {})[0] ?? "");
  }, [backendId, backendsData]);

  useEffect(() => {
    if (!open) return;
    void getCapabilities().then((caps) => setTerminalAvailable(caps.terminal.available)).catch(() => setTerminalAvailable(false));
  }, [open]);

  const selectedBackend = backendsData?.backends[backendId];
  const modelEntries = Object.entries(selectedBackend?.models ?? {});

  // Federation preflight: if the chosen backend has a linked configuration source
  // that is stale or broken, launch will be blocked server-side (422/409). Warn the
  // user up front so they can refresh/fix it in Settings instead of only hitting a
  // late error after clicking Launch.
  const { data: sources } = useConfigSources(project || undefined);
  const sourceBinding = (sources?.bindings ?? []).find((b) => b.backend_id === backendId);
  const sourceNeedsAttention =
    !!sourceBinding &&
    (sourceBinding.stale ||
      ["source_invalid", "approval_required", "source_conflict"].includes(sourceBinding.health ?? ""));

  // Terminal is offered only when the host advertises it AND the selected backend
  // type supports it (only claude-acp — mirrors the server terminalSupported gate).
  const backendTerminalOK = !selectedBackend || terminalSupported(selectedBackend.type);
  const canTerminal = terminalAvailable && backendTerminalOK;

  // A backend that can't run terminal must not leave a stale terminal selection.
  useEffect(() => {
    if (!canTerminal && agentInterface === "terminal") setAgentInterface("chat");
  }, [canTerminal, agentInterface]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setLaunchError(null);
    launch.mutate(
      { name: name || undefined, role, project, backend: backendId || undefined, model: modelId || undefined, interface: agentInterface },
      {
        onSuccess: () => onClose(),
        onError: (err) => {
          // Surface the server's actual reason (e.g. a nonexistent project cwd
          // → runtime launch failure) instead of an opaque "HTTP 502".
          const e = err as { body?: { error?: { message?: string } } };
          setLaunchError(e?.body?.error?.message ?? String(err));
        },
      },
    );
  };

  return (
    <Dialog.Root open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <Dialog.Portal>
        <Dialog.Overlay className="dialog-overlay" />
        <Dialog.Content className="dialog-content">
          <Dialog.Title>New agent</Dialog.Title>
          <form onSubmit={handleSubmit} className="config-form">
            <div className="form-field">
              <label>Name</label>
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. Atlas"
              />
            </div>

            <div className="form-field">
              <label>Role</label>
              <select value={role} onChange={(e) => setRole(e.target.value)}>
                {roleEntries.length === 0 && <option value="">No roles</option>}
                {roleEntries.map(([id, r]) => (
                  <option key={id} value={id}>{r.title} ({id})</option>
                ))}
              </select>
            </div>

            <div className="form-field">
              <label>Project</label>
              <select value={project} onChange={(e) => setProject(e.target.value)}>
                {projectEntries.length === 0 && <option value="">No projects</option>}
                {projectEntries.map(([id, p]) => (
                  <option key={id} value={id}>{p.title}</option>
                ))}
              </select>
            </div>

            <div className="form-field">
              <label>Backend</label>
              <select value={backendId} onChange={(e) => setBackendId(e.target.value)}>
                {Object.entries(backendsData?.backends ?? {}).map(([id, b]) => (
                  <option key={id} value={id}>{b.name} ({id})</option>
                ))}
              </select>
            </div>

            <div className="form-field">
              <label>Model</label>
              <select
                value={modelId}
                onChange={(e) => setModelId(e.target.value)}
              >
                {modelEntries.map(([id, m]) => (
                  <option key={id} value={id}>{m.name} ({id})</option>
                ))}
              </select>
            </div>

            <div className="form-field">
              <label>Interface</label>
              <div className="interface-controls">
                <label className="interface-option">
                  <input type="radio" name="interface" value="chat" checked={agentInterface === "chat"} onChange={() => setAgentInterface("chat")} />
                  Chat
                </label>
                <label className={canTerminal ? "interface-option" : "interface-option interface-disabled"} title={canTerminal ? "Terminal runtime" : !backendTerminalOK ? "Terminal is only supported by the Claude backend" : "Terminal unavailable"}>
                  <input type="radio" name="interface" value="terminal" checked={agentInterface === "terminal"} disabled={!canTerminal} onChange={() => setAgentInterface("terminal")} />
                  Terminal
                </label>
              </div>
            </div>

            {sourceNeedsAttention && (
              <p className="source-warning">
                This backend's linked configuration needs attention
                {sourceBinding!.health ? ` (${sourceBinding!.health}${sourceBinding!.stale ? ", stale" : ""})` : ""}.
                Launch may be blocked — refresh or fix it in Settings → Backends → Configuration source first.
              </p>
            )}
            {launchError && <p className="form-error">{launchError}</p>}

            <div className="form-actions">
              <button type="button" onClick={onClose} disabled={launch.isPending}>Cancel</button>
              <button
                type="submit"
                disabled={launch.isPending || !role || !project}
              >
                {launch.isPending ? "Launching…" : "Launch"}
              </button>
            </div>
          </form>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
