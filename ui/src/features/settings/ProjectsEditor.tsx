import { useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import {
  useProjects,
  useCreateProject,
  useUpdateProject,
  useDeleteProject,
} from "../../api/config";
import type { ProjectResponse, FieldWarning } from "../../schemas/project";
import { ProjectForm } from "./ProjectForm";

export function ProjectsEditor() {
  const { data: projects, isLoading } = useProjects();
  const createProject = useCreateProject();
  const updateProject = useUpdateProject();
  const deleteProject = useDeleteProject();

  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<ProjectResponse | null>(null);
  const [formError, setFormError] = useState("");
  const [warnings, setWarnings] = useState<FieldWarning[]>([]);

  function openCreate() {
    setEditing(null);
    setFormError("");
    setWarnings([]);
    setOpen(true);
  }

  function openEdit(id: string, proj: Omit<ProjectResponse, "project">) {
    setEditing({ project: id, ...proj });
    setFormError("");
    setWarnings([]);
    setOpen(true);
  }

  function handleSubmit(data: ProjectResponse) {
    setFormError("");
    setWarnings([]);
    if (editing) {
      const { project, ...fields } = data;
      void project;
      updateProject.mutate(
        { id: editing.project, data: fields },
        {
          onSuccess: (resp) => {
            setWarnings(resp.warnings ?? []);
            if ((resp.warnings ?? []).length === 0) setOpen(false);
          },
          onError: (e) => setFormError(String(e)),
        },
      );
    } else {
      createProject.mutate(data, {
        onSuccess: (resp) => {
          setWarnings(resp.warnings ?? []);
          if ((resp.warnings ?? []).length === 0) setOpen(false);
        },
        onError: (e) => setFormError(String(e)),
      });
    }
  }

  function handleDelete(id: string) {
    if (!confirm(`Delete project "${id}"?`)) return;
    deleteProject.mutate({ id });
  }

  if (isLoading) return <p>Loading projects…</p>;

  const entries = Object.entries(projects ?? {});

  return (
    <div className="config-editor">
      <div className="config-editor-header">
        <h2>Projects</h2>
        <button onClick={openCreate}>New project</button>
      </div>

      {entries.length === 0 && (
        <p className="config-empty">No projects defined. Create one to get started.</p>
      )}
      <ul className="config-list">
        {entries.map(([id, proj]) => (
          <li key={id} className="config-list-item">
            <div className="config-list-item-main">
              <div
                className="project-color-swatch"
                style={{
                  background: proj.color
                    ? `rgb(${proj.color[0]},${proj.color[1]},${proj.color[2]})`
                    : "#888",
                }}
              />
              <strong>{proj.title}</strong>
              <code className="config-slug">{id}</code>
              <span className="config-cwd">{proj.cwd}</span>
            </div>
            <div className="config-list-item-actions">
              <button onClick={() => openEdit(id, proj)}>Edit</button>
              <button onClick={() => handleDelete(id)} className="btn-danger">
                Delete
              </button>
            </div>
          </li>
        ))}
      </ul>

      <Dialog.Root open={open} onOpenChange={setOpen}>
        <Dialog.Portal>
          <Dialog.Overlay className="dialog-overlay" />
          <Dialog.Content className="dialog-content">
            <Dialog.Title>{editing ? "Edit project" : "New project"}</Dialog.Title>
            <ProjectForm
              initial={editing ?? undefined}
              onSubmit={handleSubmit}
              onCancel={() => setOpen(false)}
              submitting={createProject.isPending || updateProject.isPending}
              error={formError}
              warnings={warnings}
            />
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>
    </div>
  );
}
