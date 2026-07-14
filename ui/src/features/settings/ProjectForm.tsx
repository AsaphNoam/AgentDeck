import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import type { ProjectResponse, FieldWarning } from "../../schemas/project";

const colorChannel = z.number().int().min(0).max(255);

const schema = z.object({
  // Project id is server-derived from the title (FS-04.R31); the create form no
  // longer collects it. Edit mode shows the existing id read-only.
  title: z.string().min(1, "title is required").max(120),
  colorR: colorChannel,
  colorG: colorChannel,
  colorB: colorChannel,
  cwd: z.string().min(1, "cwd is required"),
  context_prompt: z.string(),
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
  const [color, setColor] = useState<[number, number, number]>(
    initial?.color ?? [128, 128, 128],
  );
  const [addDirs, setAddDirs] = useState<string[]>(initial?.add_dirs ?? []);
  const [addDirsInput, setAddDirsInput] = useState("");
  const {
    register,
    handleSubmit,
    formState: { errors },
    setValue,
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      title: initial?.title ?? "",
      colorR: initial?.color[0] ?? 128,
      colorG: initial?.color[1] ?? 128,
      colorB: initial?.color[2] ?? 128,
      cwd: initial?.cwd ?? "",
      context_prompt: initial?.context_prompt ?? "",
    },
  });

  function updateColor(ch: 0 | 1 | 2, value: number) {
    const next = [...color] as [number, number, number];
    next[ch] = value;
    setColor(next);
    const keys = ["colorR", "colorG", "colorB"] as const;
    setValue(keys[ch], value);
  }

  const submit = (vals: FormValues) => {
    onSubmit({
      // Empty id on create tells the server to derive it from the title (R31).
      project: isEdit ? initial!.project : "",
      title: vals.title,
      color: [vals.colorR, vals.colorG, vals.colorB],
      cwd: vals.cwd,
      add_dirs: addDirs,
      context_prompt: vals.context_prompt,
    });
  };

  const cwdWarning = warnings?.find((w) => w.code === "cwd_not_found");

  return (
    <form onSubmit={handleSubmit(submit)} className="config-form">
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
        <label>Color (RGB)</label>
        <div className="color-picker">
          <div
            className="color-swatch"
            style={{ background: `rgb(${color[0]},${color[1]},${color[2]})` }}
          />
          {(["R", "G", "B"] as const).map((ch, i) => (
            <div key={ch} className="color-channel">
              <label>{ch}</label>
              <input
                type="number"
                min={0}
                max={255}
                value={color[i]}
                onChange={(e) => updateColor(i as 0 | 1 | 2, Number(e.target.value))}
              />
            </div>
          ))}
        </div>
        {(errors.colorR || errors.colorG || errors.colorB) && (
          <span className="form-error">Each channel must be 0–255</span>
        )}
      </div>
      <div className="form-field">
        <label>Working directory (cwd)</label>
        <input {...register("cwd")} placeholder="~/Projects/my-app" />
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
      {isEdit && (
        <p className="form-hint">Editing a project affects future launches only.</p>
      )}
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
