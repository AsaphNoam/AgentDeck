import { useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { useRoles, useCreateRole, useUpdateRole, useDeleteRole } from "../../api/config";
import type { RoleResponse } from "../../schemas/role";
import { RoleForm } from "./RoleForm";

export function RolesEditor() {
  const { data: roles, isLoading } = useRoles();
  const createRole = useCreateRole();
  const updateRole = useUpdateRole();
  const deleteRole = useDeleteRole();

  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<RoleResponse | null>(null);
  const [formError, setFormError] = useState("");

  function openCreate() {
    setEditing(null);
    setFormError("");
    setOpen(true);
  }

  function openEdit(id: string, role: Omit<RoleResponse, "role">) {
    setEditing({ role: id, ...role });
    setFormError("");
    setOpen(true);
  }

  function handleSubmit(data: RoleResponse) {
    setFormError("");
    if (editing) {
      const { role, ...fields } = data;
      void role;
      updateRole.mutate(
        { id: editing.role, data: fields },
        {
          onSuccess: () => setOpen(false),
          onError: (e) => setFormError(String(e)),
        },
      );
    } else {
      createRole.mutate(data, {
        onSuccess: () => setOpen(false),
        onError: (e) => setFormError(String(e)),
      });
    }
  }

  function handleDelete(id: string) {
    if (!confirm(`Delete role "${id}"?`)) return;
    deleteRole.mutate(
      { id },
      {
        onError: (err) => {
          const e = err as { status?: number; body?: { agents?: string[] } };
          if (e?.status === 409) {
            const agents = e?.body?.agents ?? [];
            const msg =
              agents.length > 0
                ? `Role "${id}" is used by ${agents.length} running agent(s):\n${agents.join(", ")}\n\nDelete the role definition anyway? Running agents are unaffected.`
                : `Role "${id}" is in use. Delete the definition anyway?`;
            if (confirm(msg)) deleteRole.mutate({ id, force: true });
          }
        },
      },
    );
  }

  if (isLoading) return <p>Loading roles…</p>;

  const entries = Object.entries(roles ?? {});

  return (
    <div className="config-editor">
      <div className="config-editor-header">
        <h2>Roles</h2>
        <button onClick={openCreate}>New role</button>
      </div>

      {entries.length === 0 && (
        <p className="config-empty">No roles defined. Create one to get started.</p>
      )}
      <ul className="config-list">
        {entries.map(([id, role]) => (
          <li key={id} className="config-list-item">
            <div className="config-list-item-main">
              <strong>{role.title}</strong>
              <code className="config-slug">{id}</code>
              {role.skip_permissions != null && (
                <span className="config-badge">
                  {role.skip_permissions ? "always skip" : "always prompt"}
                </span>
              )}
              {role.system_prompt && (
                <p className="config-excerpt">{role.system_prompt.slice(0, 80)}</p>
              )}
            </div>
            <div className="config-list-item-actions">
              <button onClick={() => openEdit(id, role)}>Edit</button>
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
            <Dialog.Title>{editing ? "Edit role" : "New role"}</Dialog.Title>
            <RoleForm
              initial={editing ?? undefined}
              onSubmit={handleSubmit}
              onCancel={() => setOpen(false)}
              submitting={createRole.isPending || updateRole.isPending}
              error={formError}
            />
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>
    </div>
  );
}
