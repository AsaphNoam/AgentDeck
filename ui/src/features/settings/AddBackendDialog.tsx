import { useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { useCreateBackend, configErrorMessage } from "../../api/config";
import type { Backend, BackendType, CreateBackendResponse } from "../../schemas/backends";
import { BACKEND_TYPE_LABELS, BACKEND_TYPE_OPTIONS } from "../../lib/backendTypes";

// Add backend is a dialog rather than an inline card insert (FS-04.R40): the
// person picks a provider type first, and the server builds that type's
// canonical starter backend and model. Nothing about the surrounding Settings
// draft is submitted, saved, or discarded.

// FEDERATED types are the only ones with a native configuration to connect.
const FEDERATED: BackendType[] = ["claude-acp", "codex-acp"];

// suggestedName is the editable display-name suggestion for a type, so choosing
// Codex never begins with a Claude identity.
function suggestedName(type: BackendType): string {
  return BACKEND_TYPE_LABELS[type];
}

// backendIdFor derives a valid, unused id from the display name. Ids are stable
// keys the source binding and every agent record reference, so a collision takes
// the next free suffix rather than reusing another backend's identity.
export function backendIdFor(name: string, type: BackendType, taken: Iterable<string>): string {
  const base =
    name
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-+|-+$/g, "")
      .slice(0, 40) || type.replace(/-acp$/, "");
  const used = new Set(taken);
  if (!used.has(base)) return base;
  for (let n = 2; ; n++) {
    const candidate = `${base}-${n}`;
    if (!used.has(candidate)) return candidate;
  }
}

export function AddBackendDialog({
  open,
  existingIds,
  onCancel,
  onCreated,
}: {
  open: boolean;
  existingIds: string[];
  onCancel: () => void;
  onCreated: (id: string, backend: Backend, connection: CreateBackendResponse["connection"]) => void;
}) {
  const create = useCreateBackend();
  const [type, setType] = useState<BackendType>("claude-acp");
  const [name, setName] = useState(suggestedName("claude-acp"));
  const [nameEdited, setNameEdited] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const federated = FEDERATED.includes(type);
  const providerLabel = BACKEND_TYPE_LABELS[type];

  const reset = () => {
    setType("claude-acp");
    setName(suggestedName("claude-acp"));
    setNameEdited(false);
    setError(null);
    create.reset();
  };

  const changeType = (next: BackendType) => {
    setType(next);
    // The suggestion follows the chosen provider until the person edits it.
    if (!nameEdited) setName(suggestedName(next));
  };

  const submit = (connect: boolean) => {
    setError(null);
    const trimmed = name.trim();
    if (!trimmed) {
      setError("Enter a name for this backend.");
      return;
    }
    create.mutate(
      {
        backend_id: backendIdFor(trimmed, type, existingIds),
        name: trimmed,
        type,
        connect_native_configuration: connect,
      },
      {
        onSuccess: (res) => {
          onCreated(res.backend_id, res.backend, res.connection);
          reset();
        },
        onError: (e) => setError(configErrorMessage(e)),
      },
    );
  };

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(next) => {
        if (next) return;
        // Cancel before submission changes nothing; a created backend is already
        // durable and closing never deletes it.
        reset();
        onCancel();
      }}
    >
      <Dialog.Portal>
        <Dialog.Overlay className="dialog-overlay" data-ui="dialog" data-slot="overlay" />
        <Dialog.Content className="dialog-content" data-ui="dialog" data-slot="content" data-variant="default">
          <Dialog.Title data-slot="title">Add backend</Dialog.Title>
          <div data-slot="body">
            <div className="form-field">
              <label htmlFor="add-backend-type">Provider</label>
              <select
                id="add-backend-type"
                value={type}
                onChange={(e) => changeType(e.target.value as BackendType)}
              >
                {BACKEND_TYPE_OPTIONS.map((t) => (
                  <option key={t} value={t}>
                    {BACKEND_TYPE_LABELS[t]} ({t})
                  </option>
                ))}
              </select>
            </div>
            <div className="form-field">
              <label htmlFor="add-backend-name">Name</label>
              <input
                id="add-backend-name"
                value={name}
                onChange={(e) => {
                  setNameEdited(true);
                  setName(e.target.value);
                }}
              />
            </div>
            <p className="form-hint">
              AgentDeck creates this backend with {providerLabel}'s standard starter model. You can
              edit its models afterwards.
            </p>
            {federated && (
              <p className="form-hint">
                Using your configuration reads {providerLabel}'s existing setup — models, instructions
                and tooling — without copying or modifying it.
              </p>
            )}
            {error && <p className="form-error">{error}</p>}
          </div>
          <div className="form-actions" data-slot="actions">
            <button type="button" onClick={onCancel} disabled={create.isPending}>
              Cancel
            </button>
            <button type="button" onClick={() => submit(false)} disabled={create.isPending}>
              {create.isPending ? "Working…" : "Create backend"}
            </button>
            {federated && (
              <button type="button" onClick={() => submit(true)} disabled={create.isPending}>
                Create and use my configuration
              </button>
            )}
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
