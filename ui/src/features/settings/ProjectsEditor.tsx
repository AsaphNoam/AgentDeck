import { useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  useProjects,
  useCreateProject,
  useUpdateProject,
  useDeleteProject,
  configErrorMessage,
} from "../../api/config";
import { QUERY_KEYS } from "../../api/config";
import { archiveProject, restoreProject } from "../../api/client";
import { useUiStore } from "../../store/uiStore";
import type { ProjectResponse, FieldWarning } from "../../schemas/project";
import { ConfirmDialog } from "../../components/ui";
import { ProjectForm } from "./ProjectForm";

type DeleteDialog = { id: string; resourceDir?: string; force?: boolean; agents?: string[]; error?: string };

export function ProjectsEditor() {
  const { data: projects, isLoading } = useProjects();
  const createProject = useCreateProject();
  const updateProject = useUpdateProject();
  const deleteProject = useDeleteProject();
  const pushError = useUiStore((state) => state.pushError);
  const queryClient = useQueryClient();
  const archive = useMutation({
    mutationFn: archiveProject,
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: QUERY_KEYS.projects }),
    onError: (err) => { const message = configErrorMessage(err); setArchiveError(message); pushError("Archive project failed", message); },
  });
  const restore = useMutation({
    mutationFn: restoreProject,
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: QUERY_KEYS.projects }),
    onError: (err) => pushError("Restore project failed", configErrorMessage(err)),
  });

  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<ProjectResponse | null>(null);
  const [formError, setFormError] = useState("");
  const [warnings, setWarnings] = useState<FieldWarning[]>([]);
  const [deleting, setDeleting] = useState<DeleteDialog | null>(null);
  const [archiveID, setArchiveID] = useState<string | null>(null);
  const [archiveError, setArchiveError] = useState("");

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
            setOpen(false);
          },
          onError: (e) => setFormError(configErrorMessage(e)),
        },
      );
    } else {
      createProject.mutate(data, {
        onSuccess: (resp) => {
          setWarnings(resp.warnings ?? []);
          setOpen(false);
        },
        onError: (e) => setFormError(String(e)),
      });
    }
  }

  function confirmDelete() {
    if (!deleting) return;
    deleteProject.mutate(
      { id: deleting.id, force: deleting.force },
      {
        onSuccess: () => setDeleting(null),
        onError: (err) => {
          const e = err as { status?: number; body?: { agents?: string[] } };
          if (e?.status === 409 && !deleting.force) {
            const agents = e?.body?.agents ?? [];
            setDeleting({ ...deleting, force: true, agents });
            return;
          }
          const message = configErrorMessage(err);
          setDeleting((current) => current ? { ...current, error: message } : current);
          pushError("Delete project failed", message);
        },
      },
    );
  }

  if (isLoading) return <p data-ui="config-editor" data-state="loading" data-variant="projects">Loading projects…</p>;

  const entries = Object.entries(projects ?? {});

  return (
    <div className="config-editor" data-ui="config-editor" data-state={formError ? "error" : entries.length === 0 ? "empty" : undefined} data-variant="projects">
      <div className="config-editor-header" data-slot="header">
        <h2>Projects</h2>
        <button onClick={openCreate}>New project</button>
      </div>

      {entries.length === 0 && (
        <p className="config-empty">No projects defined. Create one to get started.</p>
      )}
      <ul className="config-list" data-slot="list">
        {entries.map(([id, proj]) => (
          <li key={id} className="config-list-item" data-slot="item">
            <div className="config-list-item-main">
              <div
                className="project-color-swatch"
                style={{
                  background: proj.color
                    ? `rgb(${proj.color[0]},${proj.color[1]},${proj.color[2]})`
                    : "var(--ad-project-fallback)",
                }}
              />
              <strong>{proj.title}</strong>
              <code className="config-slug">{id}</code>
              <span>{proj.archived ? "Archived" : "Active"}</span>
              <span className="config-cwd">{proj.cwd}</span>
            </div>
            <div className="config-list-item-actions" data-slot="actions">
              <button onClick={() => openEdit(id, proj)}>Edit</button>
              {proj.archived ? (
                <button onClick={() => restore.mutate(id)} disabled={restore.isPending}>Restore</button>
              ) : (
                <button onClick={() => {
                  setArchiveError("");
                  setArchiveID(id);
                }} disabled={archive.isPending}>Archive</button>
              )}
              <button onClick={() => setDeleting({ id, resourceDir: proj.resource_dir })} className="btn-danger">
                Delete
              </button>
            </div>
          </li>
        ))}
      </ul>

      <Dialog.Root open={open} onOpenChange={setOpen}>
        <Dialog.Portal>
          <Dialog.Overlay className="dialog-overlay" data-ui="dialog" data-slot="overlay" />
          <Dialog.Content className="dialog-content" data-ui="dialog" data-slot="content" data-variant="default">
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
      {archiveID && projects?.[archiveID] && (
        <ConfirmDialog
          open
          title={`Archive project "${projects[archiveID].title}"?`}
          confirmLabel="Archive project"
          destructive
          pending={archive.isPending}
          onCancel={() => { setArchiveError(""); setArchiveID(null); }}
          onConfirm={() => archive.mutate(archiveID, { onSuccess: () => setArchiveID(null) })}
        >
          <p>Running agents will be stopped and every agent in this project will be archived.</p>
          {archiveError && <p className="form-error">{archiveError}</p>}
        </ConfirmDialog>
      )}
      {deleting && (
        <ConfirmDialog
          open
          title={deleting.force ? `Delete project "${deleting.id}" anyway?` : `Delete project "${deleting.id}"?`}
          confirmLabel={deleting.force ? "Delete anyway" : "Delete project"}
          destructive
          pending={deleteProject.isPending}
          onCancel={() => setDeleting(null)}
          onConfirm={confirmDelete}
        >
          {deleting.force ? (
            <>
              <p>Running agents keep their already-composed project configuration.</p>
              {deleting.agents && deleting.agents.length > 0 && <p>In use by: {deleting.agents.join(", ")}</p>}
            </>
          ) : (
            <>
              <p>This removes the project definition.</p>
              {deleting.resourceDir && <p>Its shared resources directory is kept: {deleting.resourceDir}</p>}
            </>
          )}
          {deleting.error && <p className="form-error">{deleting.error}</p>}
        </ConfirmDialog>
      )}
    </div>
  );
}
