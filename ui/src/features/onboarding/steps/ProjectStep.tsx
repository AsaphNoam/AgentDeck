import { useState } from "react";
import { configErrorMessage, useCreateProject } from "../../../api/config";
import { ProjectColorPicker } from "../../../components/ui";
import { DEFAULT_PROJECT_COLOR, type ProjectColor } from "../../../lib/projectColors";

interface ProjectStepProps {
  /** Receives the slug of the project just created, so the launch step can
   * default to it instead of the seeded my-app (whose cwd may not exist). */
  onDone: (projectId: string) => void;
  claimMutation?: () => boolean;
  releaseMutation?: () => void;
}

export function ProjectStep({ onDone, claimMutation, releaseMutation }: ProjectStepProps) {
  const createProject = useCreateProject();
  const [title, setTitle] = useState("");
  const [cwd, setCwd] = useState("");
  const [context, setContext] = useState("");
  const [color, setColor] = useState<ProjectColor>(DEFAULT_PROJECT_COLOR);
  const [warning, setWarning] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const handleCreate = () => {
    if (!title.trim() || !cwd.trim()) {
      setError("Title and working directory are required.");
      return;
    }
    if (claimMutation && !claimMutation()) return;
    setError(null);
    setWarning(null);
    createProject.mutate(
      {
        // Empty id: the server derives the project id from the title (R31).
        project: "",
        title: title.trim(),
        color: [...color] as [number, number, number],
        cwd: cwd.trim(),
        add_dirs: [],
        context_prompt: context.trim(),
        // Server-computed and read-only; ignored on create (TS-03.R12).
        resource_dir: "",
      },
      {
        onSuccess: (resp) => {
          const cwdWarn = resp.warnings?.find((w) => w.code === "cwd_not_found");
          if (cwdWarn) {
            setWarning(cwdWarn.message);
          }
          onDone(resp.project);
        },
        onError: (e) => setError(configErrorMessage(e)),
        onSettled: releaseMutation,
      },
    );
  };

  return (
    <div className="wizard-step" data-ui="onboarding" data-slot="step" data-variant="project">
      <h3>Create your first project</h3>
      <p className="wizard-step-desc">
        A project scopes your agents to a codebase directory.
      </p>

      <div className="form-field">
        <label>Title</label>
        <input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="e.g. My App"
        />
      </div>

      <div className="form-field">
        <label>Working directory</label>
        <input
          value={cwd}
          onChange={(e) => setCwd(e.target.value)}
          placeholder="~/Projects/my-app"
        />
      </div>

      <div className="form-field">
        <label>Color</label>
        <ProjectColorPicker value={color} onChange={setColor} disabled={createProject.isPending} />
      </div>

      <div className="form-field">
        <label>Context prompt (optional)</label>
        <textarea
          value={context}
          onChange={(e) => setContext(e.target.value)}
          placeholder="Brief description of this project…"
          rows={3}
        />
      </div>

      {warning && <p className="form-warning">{warning}</p>}
      {error && <p className="form-error">{error}</p>}

      <div className="form-actions" data-slot="actions">
        <button type="button" onClick={handleCreate} disabled={createProject.isPending}>
          {createProject.isPending ? "Creating…" : "Create project"}
        </button>
      </div>
    </div>
  );
}
