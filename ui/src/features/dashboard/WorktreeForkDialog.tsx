import { useEffect, useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { forkWorktreeProject, getWorktreeStatus } from "../../api/client";
import { configErrorMessage, QUERY_KEYS } from "../../api/config";

// FS-19.R1's creation form. It asks only for what varies between the source
// project and the fork: a title, the branch to create, and the base to branch
// from. Everything else — colour, context prompt, additional directories, base
// branch, setup command — is copied by the server.

// branchSlug turns a title into the branch suffix. It is deliberately narrow:
// lowercase alphanumerics and dashes only, which is a strict subset of what Git
// accepts, so a pre-filled branch never fails validation on the way out.
export function branchSlug(title: string): string {
  const slug = title
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
  return slug || "worktree";
}

interface WorktreeForkDialogProps {
  sourceID: string;
  sourceTitle: string;
  onClose: () => void;
}

export function WorktreeForkDialog({ sourceID, sourceTitle, onClose }: WorktreeForkDialogProps) {
  const queryClient = useQueryClient();
  const status = useQuery({
    queryKey: ["worktree", sourceID],
    queryFn: () => getWorktreeStatus(sourceID),
    staleTime: 0,
  });

  const [title, setTitle] = useState(sourceTitle);
  const [branch, setBranch] = useState(`agentdeck/${branchSlug(sourceTitle)}`);
  // The branch tracks the title until the person edits it themselves; after that
  // their value is theirs and retyping the title must not overwrite it.
  const [branchEdited, setBranchEdited] = useState(false);
  const [base, setBase] = useState("");
  const [baseEdited, setBaseEdited] = useState(false);
  const [error, setError] = useState("");
  const [warning, setWarning] = useState("");

  // The effective base is resolved by the server at use time, so it arrives with
  // the status query rather than being guessed here (FS-19.R2, TS-12.R9).
  useEffect(() => {
    if (!baseEdited && status.data?.base) setBase(status.data.base);
  }, [status.data?.base, baseEdited]);

  const fork = useMutation({
    mutationFn: () => forkWorktreeProject(sourceID, { title: title.trim(), branch: branch.trim(), base: base.trim() }),
    onSuccess: (result) => {
      // Invalidating the projects query is what puts the new card in the live
      // grid without a manual refresh, exactly like an ordinary create
      // (FS-02.R60, R42).
      void queryClient.invalidateQueries({ queryKey: QUERY_KEYS.projects });
      // A setup failure does not undo the fork, so the dialog stays open to
      // report it rather than closing over a warning nobody would see
      // (FS-19.R3).
      if (result.warning) setWarning(result.warning);
      else onClose();
    },
    onError: (err) => setError(configErrorMessage(err)),
  });

  const submit = (event: React.FormEvent) => {
    event.preventDefault();
    if (!title.trim()) { setError("Title is required."); return; }
    if (!branch.trim()) { setError("Branch is required."); return; }
    setError("");
    fork.mutate();
  };

  return (
    <Dialog.Root open onOpenChange={(open) => { if (!open) onClose(); }}>
      <Dialog.Portal>
        <Dialog.Overlay className="dialog-overlay" data-ui="dialog" data-slot="overlay" />
        <Dialog.Content className="dialog-content" data-ui="dialog" data-slot="content" data-variant="default">
          <Dialog.Title data-slot="title">New worktree project</Dialog.Title>
          {warning ? (
            <div className="config-form" data-slot="body">
              <p>
                <strong>{title.trim()}</strong> was created on branch <code>{branch.trim()}</code>.
              </p>
              <p className="form-warning">⚠ {warning}</p>
              <p className="form-hint">
                The project is ready to launch; run the setup yourself in its checkout if you need it.
              </p>
              <div className="form-actions" data-slot="actions">
                <button type="button" onClick={onClose}>Close</button>
              </div>
            </div>
          ) : (
            <form className="config-form" data-slot="body" onSubmit={submit}>
              <p className="form-hint">
                Creates a branch off <strong>{base || "the default branch"}</strong>, checks it out into a
                fresh AgentDeck-owned worktree, and copies {sourceTitle}&apos;s settings into a new project.
              </p>
              <div className="form-field">
                <label htmlFor="worktree-title">Title</label>
                <input
                  id="worktree-title"
                  value={title}
                  onChange={(event) => {
                    setTitle(event.target.value);
                    if (!branchEdited) setBranch(`agentdeck/${branchSlug(event.target.value)}`);
                  }}
                />
              </div>
              <div className="form-field">
                <label htmlFor="worktree-branch">Branch</label>
                <input
                  id="worktree-branch"
                  value={branch}
                  onChange={(event) => { setBranchEdited(true); setBranch(event.target.value); }}
                />
              </div>
              <div className="form-field">
                <label htmlFor="worktree-base">Base</label>
                <input
                  id="worktree-base"
                  value={base}
                  placeholder={status.isLoading ? "Detecting the default branch…" : "main"}
                  onChange={(event) => { setBaseEdited(true); setBase(event.target.value); }}
                />
                <span className="form-hint">
                  Change this to stack on someone else&apos;s feature branch. Nothing stacks unless you
                  say so here.
                </span>
              </div>
              {error && <p className="form-error">{error}</p>}
              <div className="form-actions" data-slot="actions">
                <button type="button" onClick={onClose} disabled={fork.isPending}>Cancel</button>
                <button type="submit" disabled={fork.isPending}>
                  {fork.isPending ? "Creating…" : "Create worktree project"}
                </button>
              </div>
            </form>
          )}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
