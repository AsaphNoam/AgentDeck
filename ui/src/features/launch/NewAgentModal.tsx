import { useEffect, useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { getCapabilities } from "../../api/client";
import { useRoles } from "../../api/config";
import { useProjects } from "../../api/config";
import { useBackends } from "../../api/config";
import { useConfig } from "../../api/config";
import { useLaunchAgent } from "../../api/config";
import { useConfigSources } from "../../api/configSources";
import { launchSupportFor, type LaunchSupport } from "../../schemas/backends";
import { resetRuntimeForBackend, resetRuntimeForModel } from "../../lib/runtimeSelection";
import { describeProviderRuntime, hasProviderSource } from "../../lib/providerRuntime";
import { useSuggestedName } from "./useSuggestedName";

interface NewAgentModalProps {
  open: boolean;
  onClose: () => void;
  /** Pre-select role (e.g. from onboarding wizard). */
  initialRole?: string;
  /** Pre-select project (e.g. from onboarding wizard). */
  initialProject?: string;
  /** Fix the launch to a project and omit the project chooser. */
  fixedProject?: string;
  /** Called after a successful launch, before this modal closes. */
  onLaunched?: (agentId: string) => void;
}

export function NewAgentModal({ open, onClose, initialRole, initialProject, fixedProject, onLaunched }: NewAgentModalProps) {
  const { data: rolesData } = useRoles();
  const { data: projectsData } = useProjects();
  const { data: backendsData, refetch: refetchBackends, isFetching: backendsFetching } = useBackends();
  const { data: configData } = useConfig();
  const launch = useLaunchAgent();

  const roleEntries = Object.entries(rolesData ?? {});
  const projectEntries = Object.entries(projectsData ?? {}).filter(([, project]) => !project.archived);
  const roleLabels = roleEntries.map(([id, role]) => [id, role.title] as [string, string]);
  const projectLabels = projectEntries.map(([id, project]) => [id, project.title] as [string, string]);

  const defaultBackendId =
    Object.entries(backendsData?.backends ?? {}).find(([, b]) => b.default)?.[0] ??
    Object.keys(backendsData?.backends ?? {})[0] ??
    "";

  const [role, setRole] = useState(initialRole ?? "");
  const [project, setProject] = useState(fixedProject ?? initialProject ?? "");
  const [backendId, setBackendId] = useState(defaultBackendId);
  const [modelId, setModelId] = useState("");
  const [effort, setEffort] = useState("");
  const [fast, setFast] = useState(false);
  const [agentInterface, setAgentInterface] = useState<"chat" | "terminal">("chat");
  const [optionsOpen, setOptionsOpen] = useState(false);
  const [terminalAvailable, setTerminalAvailable] = useState(true);
  const [launchError, setLaunchError] = useState<string | null>(null);

  const [name, setName] = useSuggestedName(role);

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
    if (fixedProject) setProject(fixedProject);
  }, [fixedProject]);

  useEffect(() => {
    if (!fixedProject && project && projectsData && (!projectsData[project] || projectsData[project].archived)) setProject("");
  }, [fixedProject, project, projectsData]);

  useEffect(() => {
    if (!backendId && defaultBackendId) setBackendId(defaultBackendId);
  }, [defaultBackendId]); // eslint-disable-line react-hooks/exhaustive-deps

  // When backend changes, reset model to that backend's default_model. Keyed on
  // the backend entry (kept by reference across an unchanged refetch) so a
  // launch-options retry does not overwrite the person's choices.
  const backendEntry = backendsData?.backends[backendId];
  useEffect(() => {
    const runtime = resetRuntimeForBackend(backendsData, backendId);
    setModelId(runtime.model);
    setEffort(runtime.effort);
    setFast(false);
  }, [backendId, backendEntry]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!open) return;
    void getCapabilities().then((caps) => setTerminalAvailable(caps.terminal.available)).catch(() => setTerminalAvailable(false));
  }, [open]);

  const selectedBackend = backendsData?.backends[backendId];
  const backendEntries = Object.entries(backendsData?.backends ?? {});
  const backendLabels = backendEntries.map(([id, backend]) => [id, backend.name] as [string, string]);
  const modelEntries = Object.entries(selectedBackend?.models ?? {});
  const modelLabels = modelEntries.map(([id, model]) => [id, model.name] as [string, string]);
  const selectedModel = selectedBackend?.models[modelId];
  const effortLevels = selectedModel?.efforts ?? [];
  // The saved next-start provider for this backend/model (FS-09.R72). New
  // Agent reports it; the choice itself lives in Settings.
  const providerRuntime = selectedBackend && hasProviderSource(selectedBackend.type)
    ? backendsData?.provider_runtimes?.[backendId]?.[modelId]
    : undefined;
  const providerDescription = selectedBackend && providerRuntime && describeProviderRuntime(selectedBackend.type, providerRuntime);

  useEffect(() => {
    setEffort(resetRuntimeForModel(backendsData, backendId, modelId).effort);
    setFast(false);
  }, [backendId, modelId]); // eslint-disable-line react-hooks/exhaustive-deps

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

  // Optional launch choices come from the server's adapter launch support
  // (FS-09.R61): Terminal also needs the host, effort/fast also need the model's
  // declaration. Missing metadata offers none of them and never infers support
  // from the backend type.
  const support = backendsData?.backend_support;
  const chatSupport = launchSupportFor(support, selectedBackend?.type, "chat");
  const terminalSupport = launchSupportFor(support, selectedBackend?.type, "terminal");
  const interfaceSupport = agentInterface === "terminal" ? terminalSupport : chatSupport;
  const chatSupportMissing = !!selectedBackend && !chatSupport;
  const terminalSupportMissing = !!selectedBackend && !terminalSupport;
  const supportMissing = chatSupportMissing || terminalSupportMissing;
  const canTerminal = terminalAvailable && !!terminalSupport?.available;
  const offerEffort = effortLevels.length > 0 && !!interfaceSupport?.effort;
  const offerFast = !!selectedModel?.fast && !!interfaceSupport?.fast;
  const defaultEffort = selectedModel?.default_effort ?? "";
  // A non-default choice made before support went missing must be re-verified
  // or cleared; an ordinary chat launch with defaults is never gated.
  const unverifiedSelection =
    (agentInterface === "terminal" && terminalSupportMissing) ||
    (agentInterface === "chat" && chatSupportMissing && (fast || effort !== defaultEffort));

  // A known-unsupported Terminal must not leave a stale selection; unknown
  // support keeps it for re-verification instead of silently changing it.
  useEffect(() => {
    const knownNoTerminal = !terminalAvailable || (!!terminalSupport && !terminalSupport.available);
    if (knownNoTerminal && agentInterface === "terminal") setAgentInterface("chat");
  }, [terminalAvailable, terminalSupport, agentInterface]);
  useEffect(() => {
    if (interfaceSupport && !interfaceSupport.fast) setFast(false);
  }, [interfaceSupport]);

  const clearUnverified = () => {
    setAgentInterface("chat");
    setFast(false);
    setEffort(defaultEffort);
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setLaunchError(null);
    if (unverifiedSelection) return;
    launch.mutate(
      {
        name: name || undefined, role, project, backend: backendId || undefined, model: modelId || undefined,
        effort: (offerEffort && effort) || undefined, fast: offerFast && fast, interface: agentInterface,
      },
      {
        onSuccess: (result) => {
          onLaunched?.(result.agent.agent_id);
          onClose();
        },
        onError: (err) => {
          // Surface the server's actual reason (e.g. a nonexistent project cwd
          // → runtime launch failure) instead of an opaque "HTTP 502".
          const e = err as { body?: { error?: { message?: string } } };
          setOptionsOpen(true);
          setLaunchError(e?.body?.error?.message ?? String(err));
        },
      },
    );
  };

  return (
    <Dialog.Root open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <Dialog.Portal>
        <Dialog.Overlay className="dialog-overlay" data-ui="dialog" data-slot="overlay" />
        <Dialog.Content className="dialog-content" data-ui="dialog" data-slot="content" data-variant="default">
          <Dialog.Title>New agent</Dialog.Title>
          <form onSubmit={handleSubmit} className="config-form">
            <div className="form-field">
              <label htmlFor="new-agent-role">Role</label>
              <select id="new-agent-role" value={role} onChange={(e) => setRole(e.target.value)}>
                {roleEntries.length === 0 && <option value="">No roles</option>}
                {roleEntries.map(([id]) => <option key={id} value={id}>{displayLabel(roleLabels, id)}</option>)}
              </select>
            </div>

            {fixedProject ? (
              <div className="form-field">
                <label htmlFor="new-agent-project">Project</label>
                <output id="new-agent-project">{projectsData?.[fixedProject]?.title || fixedProject}</output>
              </div>
            ) : (
              <div className="form-field">
                <label htmlFor="new-agent-project">Project</label>
                <select id="new-agent-project" value={project} onChange={(e) => setProject(e.target.value)}>
                  {projectEntries.length === 0 && <option value="">No projects</option>}
                  {projectEntries.map(([id]) => <option key={id} value={id}>{displayLabel(projectLabels, id)}</option>)}
                </select>
              </div>
            )}

            <div className="new-agent-runtime" aria-live="polite">
              <span>Runs with</span>
              <strong>{displayLabel(backendLabels, backendId) || "Configured backend"}</strong>
              <span>{displayLabel(modelLabels, modelId) || "Default model"}</span>
              {effort && <span>{effort} effort</span>}
              {fast && <span>Fast mode</span>}
              <span>{agentInterface === "terminal" ? "Terminal" : "Chat"}</span>
            </div>
            {providerDescription && (
              <p className={providerDescription.problem ? "form-warning" : "new-agent-provider"}>
                {providerDescription.problem ?? providerDescription.summary}
                {providerDescription.problem && " Change it in Settings → Backends."}
              </p>
            )}

            <details className="new-agent-options" open={optionsOpen} onToggle={(event) => setOptionsOpen(event.currentTarget.open)}>
              <summary>Options</summary>
              <div className="config-form">
                <div className="form-field">
                  <label htmlFor="new-agent-name">Name</label>
                  <input id="new-agent-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Atlas" />
                </div>

                <div className="form-field">
                  <label htmlFor="new-agent-backend">Backend</label>
                  <select id="new-agent-backend" value={backendId} onChange={(e) => setBackendId(e.target.value)}>
                    {backendEntries.map(([id]) => <option key={id} value={id}>{displayLabel(backendLabels, id)}</option>)}
                  </select>
                </div>

                <div className="form-field">
                  <label htmlFor="new-agent-model">Model</label>
                  <select id="new-agent-model" value={modelId} onChange={(e) => {
                    const runtime = resetRuntimeForModel(backendsData, backendId, e.target.value);
                    setModelId(runtime.model);
                    setEffort(runtime.effort);
                  }}>
                    {modelEntries.map(([id]) => <option key={id} value={id}>{displayLabel(modelLabels, id)}</option>)}
                  </select>
                </div>

                {offerEffort && (
                  <div className="form-field">
                    <label htmlFor="new-agent-effort">Effort</label>
                    <select id="new-agent-effort" value={effort} onChange={(e) => setEffort(e.target.value)}>
                      {effortLevels.map((level) => <option key={level} value={level}>{level}</option>)}
                    </select>
                  </div>
                )}

                {offerFast && (
                  <label className="form-field">
                    <span>Speed</span>
                    <span><input type="checkbox" checked={fast} onChange={(e) => setFast(e.target.checked)} /> Fast mode — faster responses with higher provider usage</span>
                  </label>
                )}

                <div className="form-field">
                  <label>Interface</label>
                  <div className="interface-controls">
                    <label className="interface-option">
                      <input type="radio" name="interface" value="chat" checked={agentInterface === "chat"} onChange={() => setAgentInterface("chat")} />
                      Chat
                    </label>
                    <label className={canTerminal ? "interface-option" : "interface-option interface-disabled"} title={terminalOptionTitle(canTerminal, terminalAvailable, terminalSupport)}>
                      <input type="radio" name="interface" value="terminal" checked={agentInterface === "terminal"} disabled={!canTerminal} onChange={() => setAgentInterface("terminal")} />
                      Terminal
                    </label>
                  </div>
                </div>
              </div>
            </details>

            {sourceNeedsAttention && (
              <p className="source-warning">
                This backend's linked configuration needs attention
                {sourceBinding!.health ? ` (${sourceBinding!.health}${sourceBinding!.stale ? ", stale" : ""})` : ""}.
                Launch may be blocked — refresh or fix it in Settings → Backends → Configuration source first.
              </p>
            )}
            {supportMissing && (
              <div className="form-warning" role="status">
                <p>
                  Launch options could not be loaded for {[
                    chatSupportMissing ? "Chat" : "",
                    terminalSupportMissing ? "Terminal" : "",
                  ].filter(Boolean).join(" and ")}. Optional choices for those interfaces are unavailable.
                  {unverifiedSelection && " Retry to verify your selected options, or clear them to launch with defaults."}
                </p>
                <button type="button" onClick={() => void refetchBackends()} disabled={backendsFetching}>
                  {backendsFetching ? "Retrying…" : "Retry"}
                </button>
                {unverifiedSelection && <button type="button" onClick={clearUnverified}>Clear options</button>}
              </div>
            )}
            {launchError && <p className="form-error">{launchError}</p>}

            <div className="form-actions">
              <button type="button" onClick={onClose} disabled={launch.isPending}>Cancel</button>
              <button
                type="submit"
                disabled={launch.isPending || !role || !project || unverifiedSelection}
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

function terminalOptionTitle(canTerminal: boolean, hostAvailable: boolean, support: LaunchSupport | undefined): string {
  if (canTerminal) return "Terminal runtime";
  if (!hostAvailable) return "Terminal unavailable";
  if (!support) return "Launch options could not be loaded";
  return "This backend does not support Terminal";
}

function displayLabel(entries: [string, string][], id: string): string {
  const entry = entries.find(([entryId]) => entryId === id);
  if (!entry) return "";
  const duplicates = entries.filter(([, label]) => label === entry[1]).length > 1;
  return duplicates ? `${entry[1]} (${entry[0]})` : entry[1];
}
