import { useEffect, useState } from "react";
import { useRoles, useProjects, useBackends, useLaunchAgent, usePutConfig, configErrorMessage } from "../../../api/config";
import { useSuggestedName } from "../../launch/useSuggestedName";
import { useUiStore } from "../../../store/uiStore";

interface LaunchStepProps {
  onDone: () => void;
  /** Slug of the project created in ProjectStep. Preferred over the seeded
   * default so onboarding launches a project whose cwd actually exists. */
  initialProject?: string;
}

export function LaunchStep({ onDone, initialProject }: LaunchStepProps) {
  const { data: rolesData } = useRoles();
  const { data: projectsData } = useProjects();
  const { data: backendsData } = useBackends();
  const launch = useLaunchAgent();
  const putConfig = usePutConfig();
  const pushError = useUiStore((state) => state.pushError);

  const roleEntries = Object.entries(rolesData ?? {});
  const projectEntries = Object.entries(projectsData ?? {});

  const defaultBackendId =
    Object.entries(backendsData?.backends ?? {}).find(([, b]) => b.default)?.[0] ??
    Object.keys(backendsData?.backends ?? {})[0] ??
    "";

  const defaultRole = roleEntries.find(([id]) => id === "implementer")?.[0] ?? roleEntries[0]?.[0] ?? "";
  const defaultProject = initialProject ?? projectEntries[0]?.[0] ?? "";

  const [role, setRole] = useState(defaultRole);
  const [project, setProject] = useState(defaultProject);
  const [error, setError] = useState<string | null>(null);
  const [launchSucceeded, setLaunchSucceeded] = useState(false);

  const [name, setName] = useSuggestedName(role, project);

  // Apply defaults once data loads.
  useEffect(() => {
    if (!role && roleEntries.length > 0) setRole(roleEntries.find(([id]) => id === "implementer")?.[0] ?? roleEntries[0][0]);
  }, [roleEntries.length]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (project) return;
    if (initialProject) setProject(initialProject);
    else if (projectEntries.length > 0) setProject(projectEntries[0][0]);
  }, [projectEntries.length, initialProject]); // eslint-disable-line react-hooks/exhaustive-deps

  const handleLaunch = () => {
    setError(null);
    launch.mutate(
      { name: name || undefined, role, project, backend: defaultBackendId || undefined, interface: "chat" },
      {
        onSuccess: () => {
          setLaunchSucceeded(true);
          putConfig.mutate({ onboarding_complete: true }, {
            onSuccess: onDone,
            onError: (e) => {
              pushError("Failed to mark onboarding complete", configErrorMessage(e));
              // Agent launched successfully; keep the wizard visible but show the error.
              // User can manually dismiss or the service will eventually reconcile the state.
            },
          });
        },
        onError: (e) => setError(configErrorMessage(e)),
      },
    );
  };

  return (
    <div className="wizard-step">
      <h3>Launch your first agent</h3>
      <p className="wizard-step-desc">
        You're all set! Launch an agent to complete setup.
      </p>

      <div className="form-field">
        <label>Agent name</label>
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="e.g. Atlas"
        />
      </div>

      <div className="form-field">
        <label>Role</label>
        <select value={role} onChange={(e) => setRole(e.target.value)}>
          {roleEntries.map(([id, r]) => (
            <option key={id} value={id}>{r.title} ({id})</option>
          ))}
        </select>
      </div>

      <div className="form-field">
        <label>Project</label>
        <select value={project} onChange={(e) => setProject(e.target.value)}>
          {projectEntries.map(([id, p]) => (
            <option key={id} value={id}>{p.title} ({id})</option>
          ))}
        </select>
      </div>

      {error && <p className="form-error">{error}</p>}

      <div className="form-actions">
        <button type="button" onClick={handleLaunch} disabled={launch.isPending || putConfig.isPending || !role || !project}>
          {launch.isPending ? "Launching…" : putConfig.isPending ? "Completing setup…" : "Launch"}
        </button>
        {launchSucceeded && !putConfig.isSuccess && (
          <p className="form-info">Agent launched successfully. Completing setup…</p>
        )}
      </div>
    </div>
  );
}
