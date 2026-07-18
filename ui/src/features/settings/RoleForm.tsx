import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import type { RoleResponse } from "../../schemas/role";

const schema = z.object({
  role: z
    .string()
    .regex(/^[a-z0-9][a-z0-9-]{0,62}$/, "must match ^[a-z0-9][a-z0-9-]{0,62}$")
    .optional(),
  title: z.string().min(1, "title is required").max(120),
  system_prompt: z.string(),
  skip_permissions: z.enum(["null", "true", "false"]),
});

type FormValues = z.infer<typeof schema>;

function skipPermissionsToValue(v: boolean | null | undefined): "null" | "true" | "false" {
  if (v === true) return "true";
  if (v === false) return "false";
  return "null";
}

function skipPermissionsFromValue(v: "null" | "true" | "false"): boolean | null {
  if (v === "true") return true;
  if (v === "false") return false;
  return null;
}

interface RoleFormProps {
  /** Initial values for edit mode. When present, the role id field is hidden. */
  initial?: RoleResponse;
  onSubmit: (data: RoleResponse) => void;
  onCancel: () => void;
  submitting?: boolean;
  error?: string;
}

export function RoleForm({ initial, onSubmit, onCancel, submitting, error }: RoleFormProps) {
  const isEdit = !!initial;
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      role: initial?.role,
      title: initial?.title ?? "",
      system_prompt: initial?.system_prompt ?? "",
      skip_permissions: skipPermissionsToValue(initial?.skip_permissions),
    },
  });

  const submit = (vals: FormValues) => {
    onSubmit({
      role: isEdit ? initial!.role : vals.role!,
      title: vals.title,
      system_prompt: vals.system_prompt,
      skip_permissions: skipPermissionsFromValue(vals.skip_permissions),
    });
  };

  return (
    <form onSubmit={handleSubmit(submit)} className="config-form" data-slot="form">
      {!isEdit && (
        <div className="form-field">
          <label>Role ID (slug)</label>
          <input {...register("role")} placeholder="e.g. security-reviewer" />
          {errors.role && <span className="form-error">{errors.role.message}</span>}
        </div>
      )}
      {isEdit && (
        <div className="form-field">
          <label>Role ID</label>
          <input value={initial.role} disabled className="form-input-disabled" />
          <span className="form-hint">Role ID cannot be changed after creation</span>
        </div>
      )}
      <div className="form-field">
        <label>Title</label>
        <input {...register("title")} placeholder="e.g. Security Reviewer" />
        {errors.title && <span className="form-error">{errors.title.message}</span>}
      </div>
      <div className="form-field">
        <label>System prompt</label>
        <textarea {...register("system_prompt")} rows={5} placeholder="Optional system prompt..." />
      </div>
      <div className="form-field">
        <label>Skip permissions</label>
        <select {...register("skip_permissions")}>
          <option value="null">Inherit global</option>
          <option value="true">Always skip</option>
          <option value="false">Always prompt</option>
        </select>
      </div>
      {isEdit && (
        <p className="form-hint">Editing a role affects future launches only.</p>
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
