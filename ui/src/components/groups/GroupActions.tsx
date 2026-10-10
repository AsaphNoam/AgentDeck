import { useRef, useState } from "react";
import { actOnGroup, type GroupActionResult } from "../../api/client";
import type { AgentState } from "../../api/types";
import { ConfirmDialog } from "../ui";

type Action = "stop" | "archive";
type Selection = { action: Action; group: string; agents: AgentState[] };

/** Mounted by the project, so results survive the last archived group card. */
export function useGroupActions({ project, projectTitle, disabled = false }: { project?: string; projectTitle?: string; disabled?: boolean }) {
  const [selection, setSelection] = useState<Selection | null>(null);
  const [pending, setPending] = useState(false);
  const inFlight = useRef(false);
  const [error, setError] = useState("");
  const [outcome, setOutcome] = useState<{ action: Action; data: GroupActionResult; names: Record<string, string> } | null>(null);
  const submit = async () => {
    if (!project || !selection || disabled || inFlight.current) return;
    inFlight.current = true;
    setPending(true);
    setError("");
    try {
      const data = await actOnGroup(project, selection.group, selection.action);
      setOutcome({ action: selection.action, data, names: Object.fromEntries(selection.agents.map((agent) => [agent.agent_id, agent.name || agent.agent_id])) });
      setSelection(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      inFlight.current = false;
      setPending(false);
    }
  };
  const label = selection?.action === "archive" ? "Archive group" : "Stop group";
  const failures = outcome?.data.results.filter((result) => !result.ok) ?? [];
  return {
    pending,
    open: (action: Action, group: string, agents: AgentState[]) => {
      if (disabled || inFlight.current || !project || group === "_ungrouped") return;
      setError("");
      setSelection({ action, group, agents });
    },
    feedback: <>
      {outcome && <section className="group-action-result" role="status" aria-label="Group action result">
        <p>{outcome.action === "archive" ? "Archive" : "Stop"} group “{outcome.data.group}” in {projectTitle || outcome.data.project}: {outcome.data.results.filter((result) => result.ok).length} succeeded{failures.length ? `, ${failures.length} failed` : "."}</p>
        {failures.length > 0 && <ul>{failures.map((result) => <li key={result.agent_id}>{outcome.names[result.agent_id] || result.agent_id}: {result.error?.message || "Action failed"}</li>)}</ul>}
        <button type="button" onClick={() => setOutcome(null)}>Dismiss</button>
      </section>}
      {selection && <ConfirmDialog open title={`${label} “${selection.group}”?`} confirmLabel={label} destructive pending={pending} confirmDisabled={disabled} onCancel={() => { if (!inFlight.current) setSelection(null); }} onConfirm={() => void submit()}>
        <p>Project: {projectTitle || project}. Group: {selection.group}.</p>
        <p>{selection.agents.length} members · {selection.agents.filter((agent) => agent.running).length} running.</p>
        <p>{selection.action === "archive" ? "Stop running members and archive all members. Conversations are retained; restore agents individually from Archive." : "Stop running members. The group and conversations remain available to resume."}</p>
        {error && <p className="form-error" role="alert">{error}</p>}
      </ConfirmDialog>}
    </>,
  };
}
