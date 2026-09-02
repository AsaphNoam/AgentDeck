import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { BrowseDirectoryButton, ProjectColorPicker } from "../../components/ui";
import { DEFAULT_PROJECT_COLOR, type ProjectColor } from "../../lib/projectColors";
import type { ProjectResponse, FieldWarning } from "../../schemas/project";

const schema = z.object({
  // Project id is server-derived from the title (FS-04.R31); the create form no
  // longer collects it. Edit mode shows the existing id read-only.
  title: z.string().min(1, "title is required").max(120),
  cwd: z.string().min(1, "cwd is required"),
  context_prompt: z.string(),
  // Both optional (FS-04.R45): empty base_branch means "auto-detect the
  // repository's default branch at use time", empty setup_command means none.
  base_branch: z.string(),
  setup_command: z.string(),
});

type FormValues = z.infer<typeof schema>;

interface ProjectFormProps {
  initial?: ProjectResponse;
  onSubmit: (data: ProjectResponse) => void;
  onCancel: () => void;
  submitting?: boolean;
  error?: string;
  warnings?: FieldWarning[];
}

export function ProjectForm({
  initial,
  onSubmit,
  onCancel,
  submitting,
  error,
  warnings,
}: ProjectFormProps) {
  const isEdit = !!initial;
  const [color, setColor] = useState<ProjectColor>(
    initial?.color ?? DEFAULT_PROJECT_COLOR,
  );
  const [addDirs, setAddDirs] = useState<string[]>(initial?.add_dirs ?? []);
  const [addDirsInput, setAddDirsInput] = useState("");
  const [browseError, setBrowseError] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    setValue,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      title: initial?.title ?? "",
      cwd: initial?.cwd ?? "",
      context_prompt: initial?.context_prompt ?? "",
      base_branch: initial?.base_branch ?? "",
      setup_command: initial?.setup_command ?? "",
    },
  });

  const submit = (vals: FormValues) => {
    onSubmit({
      // Empty id on create tells the server to derive it from the title (R31).
      project: isEdit ? initial!.project : "",
      title: vals.title,
      color: [...color] as [number, number, number],
      cwd: vals.cwd,
      add_dirs: addDirs,
      context_prompt: vals.context_prompt,
      base_branch: vals.base_branch,
      setup_command: vals.setup_command,
      // resource_dir is server-computed and read-only; the server ignores any value
      // the client sends (FS-11.R4, TS-03.R12). Carried through only to satisfy the type.
      resource_dir: initial?.resource_dir ?? "",
    });
  };

  const cwdWarning = warnings?.find((w) => w.code === "cwd_not_found");

  return (
    <form onSubmit={handleSubmit(submit)} className="config-form" data-slot="form">
      {isEdit && (
        <div className="form-field">
          <label>Project ID</label>
          <input value={initial.project} disabled className="form-input-disabled" />
        </div>
      )}
      <div className="form-field">
        <label>Title</label>
        <input {...register("title")} placeholder="e.g. My App" />
        {errors.title && <span className="form-error">{errors.title.message}</span>}
      </div>
      <div className="form-field">
        <label>Color</label>
        <ProjectColorPicker value={color} onChange={setColor} disabled={submitting} />
      </div>
      <div className="form-field">
        <label>Working directory (cwd)</label>
        <div className="field-with-action">
          <input {...register("cwd")} placeholder="~/Projects/my-app" />
          <BrowseDirectoryButton
            label="Browse for working directory"
            disabled={submitting}
            onPicked={(path) => {
              setBrowseError(null);
              setValue("cwd", path, { shouldValidate: true, shouldDirty: true });
            }}
            onError={setBrowseError}
          />
        </div>
        {errors.cwd && <span className="form-error">{errors.cwd.message}</span>}
        {cwdWarning && (
          <span className="form-warning">⚠ {cwdWarning.message} (save still succeeds)</span>
        )}
      </div>
      <div className="form-field">
        <label>Additional directories (add_dirs)</label>
        <ul className="string-list">
          {addDirs.map((dir, i) => (
            <li key={i}>
              <span>{dir}</span>
              <button
                type="button"
                aria-label={`Remove ${dir}`}
                onClick={() => setAddDirs((prev) => prev.filter((_, j) => j !== i))}
              >
                ✕
              </button>
            </li>
          ))}
        </ul>
        <div className="string-list-add">
          <input
            value={addDirsInput}
            onChange={(e) => setAddDirsInput(e.target.value)}
            placeholder="~/extra-dir"
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                const v = addDirsInput.trim();
                if (v) { setAddDirs((prev) => [...prev, v]); setAddDirsInput(""); }
              }
            }}
          />
          {/* Browsing only fills the pending entry; Add is still what puts it in
              the list, and only Update/Create saves the project (FS-04.R42). */}
          <BrowseDirectoryButton
            label="Browse for an additional directory"
            disabled={submitting}
            onPicked={(path) => {
              setBrowseError(null);
              setAddDirsInput(path);
            }}
            onError={setBrowseError}
          />
          <button
            type="button"
            onClick={() => {
              const v = addDirsInput.trim();
              if (v) { setAddDirs((prev) => [...prev, v]); setAddDirsInput(""); }
            }}
          >
            Add
          </button>
        </div>
      </div>
      <div className="form-field">
        <label>Context prompt</label>
        <textarea {...register("context_prompt")} rows={3} placeholder="Optional project context…" />
      </div>
      <div className="form-field">
        <label>Base branch</label>
        <input {...register("base_branch")} placeholder="Leave empty to use the default branch" />
        <span className="form-hint">
          New worktree projects forked from here branch off this branch. Empty detects the
          repository&apos;s default branch when the fork runs.
        </span>
      </div>
      <div className="form-field">
        <label>Setup command</label>
        <input {...register("setup_command")} placeholder="e.g. npm ci" />
        <span className="form-hint">
          AgentDeck runs this inside a new worktree checkout before the project is ready. It never
          runs at agent launch.
        </span>
      </div>
      {isEdit && initial?.resource_dir && (
        <div className="form-field">
          <label>Shared resources directory</label>
          <input
            value={initial.resource_dir}
            readOnly
            className="form-input-disabled"
            onFocus={(e) => e.currentTarget.select()}
          />
          <span className="form-hint">
            AgentDeck-owned, outside the repository, and shared by this project&apos;s agents.
            Kept even if you delete the project.
          </span>
        </div>
      )}
      {isEdit && (
        <p className="form-hint">Editing a project affects future launches only.</p>
      )}
      {browseError && <p className="form-error">{browseError}</p>}
      {error && <p className="form-error">{error}</p>}
      <div className="form-actions">
        <button type="button" onClick={onCancel} disabled={submitting}>
          Cancel
        </button>
        <button type="submit" disabled={submitting}>
          {submitting ? "Saving…" : isEdit ? "Update" : "Create"}
        </button>
      </div>
    </form>
  );
}
