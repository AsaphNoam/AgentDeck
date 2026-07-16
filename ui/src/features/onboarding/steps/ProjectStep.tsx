import { useState } from "react";
import { useCreateProject } from "../../../api/config";

interface ProjectStepProps {
  /** Receives the slug of the project just created, so the launch step can
   * default to it instead of the seeded my-app (whose cwd may not exist). */
  onDone: (projectId: string) => void;
}

export function ProjectStep({ onDone }: ProjectStepProps) {
  const createProject = useCreateProject();
  const [title, setTitle] = useState("");
  const [cwd, setCwd] = useState("");
  const [context, setContext] = useState("");
  const [warning, setWarning] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const handleCreate = () => {
    if (!title.trim() || !cwd.trim()) {
      setError("Title and working directory are required.");
      return;
    }
    setError(null);
    setWarning(null);
    createProject.mutate(
      {
        // Empty id: the server derives the project id from the title (R31).
        project: "",
        title: title.trim(),
        color: [128, 128, 128],
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
        onError: (e) => setError(String(e)),
      },
    );
  };

  return (
    <div className="wizard-step">
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

      <div className="form-actions">
        <button type="button" onClick={handleCreate} disabled={createProject.isPending}>
          {createProject.isPending ? "Creating…" : "Create project"}
        </button>
      </div>
    </div>
  );
}
