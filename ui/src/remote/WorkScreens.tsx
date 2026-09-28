import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  continuePipelineRun,
  getPipelineRun,
  repairPipelineCleanup,
  replacePipelineOrchestrator,
  retryPipelineRun,
  stopPipelineRun,
} from "../api/pipelines";
import type { Task, TaskArm } from "../schemas/task";
import { phoneFetch } from "./api";
import { useConnection } from "./connection";
import { navigate } from "./router";

const errorText = (error: unknown) => (error instanceof Error ? error.message : String(error));
const post = (url: string, body?: unknown) =>
  phoneFetch<unknown>(url, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body ?? {}) });

/** useAction runs one mutation at a time, reports the Mac's refusal verbatim
 *  (FS-20.R27), and refreshes the screen either way. */
function useAction(refresh: () => void) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const run = async (fn: () => Promise<unknown>, after?: () => void) => {
    setBusy(true);
    setError(null);
    try {
      await fn();
      after?.();
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
      refresh();
    }
  };
  return { busy, error, run };
}

function OpenConversation({ agentId }: { agentId?: string }) {
  if (!agentId) return null;
  return (
    <button type="button" className="phone-link" onClick={() => navigate(`/agent/${encodeURIComponent(agentId)}`)}>
      Open conversation
    </button>
  );
}

const armInput = (arm: TaskArm) => ({
  kind: arm.kind,
  source_kind: arm.source_kind,
  source_id: arm.source_id,
  satisfying_outcomes: arm.satisfying_outcomes,
  signal_name: arm.signal_name,
});

export function TaskScreen({ taskId }: { taskId: string }) {
  const offline = useConnection((state) => state.link !== "connected");
  const revision = useConnection((state) => state.revision);
  const client = useQueryClient();
  const task = useQuery({
    queryKey: ["task", taskId, revision],
    queryFn: () => phoneFetch<Task>(`/api/tasks/${encodeURIComponent(taskId)}`),
    placeholderData: (prev) => prev,
  });
  const { busy, error, run } = useAction(() => void client.invalidateQueries({ queryKey: ["task", taskId] }));
  const [outcome, setOutcome] = useState("success");
  const [summary, setSummary] = useState("");

  const t = task.data;
  if (!t) return <p className="phone-empty">{task.isError ? errorText(task.error) : "Loading…"}</p>;
  const id = encodeURIComponent(taskId);
  const open = t.state !== "finished";
  const disabled = offline || busy;

  return (
    <div className="phone-agent">
      <header className="phone-agent-header">
        <h1>{t.display_name}</h1>
        <p className="phone-meta">
          Task · {t.project} · {t.state.replace("_", " ")}
          {t.outcome ? ` · ${t.outcome}` : ""}
        </p>
      </header>
      {t.attention_reason && <p className="phone-card">{t.attention_reason}</p>}
      {t.outcome_summary && <p className="phone-quote">{t.outcome_summary}</p>}
      <OpenConversation agentId={t.assigned_agent_id} />
      {error && <p className="phone-error">{error}</p>}
      <div className="phone-actions">
        {t.retry_eligible && (
          <button type="button" className="phone-primary" disabled={disabled} onClick={() => void run(() => post(`/api/tasks/${id}/retry`))}>
            Retry
          </button>
        )}
        {t.state === "dependency_failed" && (
          <>
            <button type="button" disabled={disabled} onClick={() => void run(() => post(`/api/tasks/${id}/rearm`, { arms: t.arms.map(armInput) }))}>
              Re-arm with its prerequisites
            </button>
            <button type="button" disabled={disabled} onClick={() => void run(() => post(`/api/tasks/${id}/rearm`, { arms: [] }))}>
              Re-arm without prerequisites
            </button>
          </>
        )}
        {open && (
          <button type="button" className="phone-danger" disabled={disabled} onClick={() => void run(() => post(`/api/tasks/${id}/cancel`))}>
            Cancel task
          </button>
        )}
      </div>
      {open && (
        <form
          className="phone-card phone-form"
          aria-label="Record result"
          onSubmit={(event) => {
            event.preventDefault();
            void run(() => post(`/api/tasks/${id}/result`, { outcome, summary }), () => setSummary(""));
          }}
        >
          <p className="phone-card-kicker">Record result</p>
          <label className="phone-field">
            Outcome
            <select value={outcome} onChange={(event) => setOutcome(event.target.value)}>
              <option value="success">Success</option>
              <option value="failure">Failure</option>
              <option value="blocked">Blocked</option>
            </select>
          </label>
          <label className="phone-field">
            Summary
            <textarea rows={2} value={summary} onChange={(event) => setSummary(event.target.value)} />
          </label>
          <button type="submit" disabled={disabled || !summary.trim()}>
            Record result
          </button>
        </form>
      )}
    </div>
  );
}

export function RunScreen({ runId }: { runId: string }) {
  const offline = useConnection((state) => state.link !== "connected");
  const revision = useConnection((state) => state.revision);
  const client = useQueryClient();
  const detail = useQuery({ queryKey: ["run", runId, revision], queryFn: () => getPipelineRun(runId), placeholderData: (prev) => prev });
  const { busy, error, run } = useAction(() => void client.invalidateQueries({ queryKey: ["run"] }));
  const [input, setInput] = useState("");

  const d = detail.data;
  if (!d) return <p className="phone-empty">{detail.isError ? errorText(detail.error) : "Loading…"}</p>;
  const s = d.run;
  // Every control carries the revision it was decided against.
  const rev = s.revision;
  const disabled = offline || busy;
  const controls = d.controls;
  const stageIndex = d.template.stages.findIndex((stage) => stage.id === s.current_stage_id);
  const stage = stageIndex >= 0 ? d.template.stages[stageIndex] : undefined;

  return (
    <div className="phone-agent">
      <header className="phone-agent-header">
        <h1>{s.display_name}</h1>
        <p className="phone-meta">
          Pipeline · {s.project} · {s.state}
          {stage ? ` · Stage ${stageIndex + 1} of ${d.template.stages.length} · ${stage.title}` : ""}
        </p>
      </header>
      {s.attention_reason && <p className="phone-card">{s.attention_reason}</p>}
      {s.final_outcome && <p className="phone-meta">Outcome: {s.final_outcome}</p>}
      <OpenConversation agentId={s.current_agent_id} />
      {error && <p className="phone-error">{error}</p>}
      {controls.continue.eligible && (
        <form
          className="phone-card phone-form"
          aria-label="Continue"
          onSubmit={(event) => {
            event.preventDefault();
            void run(() => continuePipelineRun(runId, rev, input), () => setInput(""));
          }}
        >
          <label className="phone-field">
            Input for the stage (optional)
            <textarea rows={2} value={input} onChange={(event) => setInput(event.target.value)} />
          </label>
          <button type="submit" className="phone-primary" disabled={disabled}>
            Continue
          </button>
        </form>
      )}
      <div className="phone-actions">
        {controls.retry.eligible && (
          <button type="button" disabled={disabled} onClick={() => void run(() => retryPipelineRun(runId, rev))}>
            Retry stage
          </button>
        )}
        {controls.replace.eligible && d.orchestrator && (
          <button type="button" disabled={disabled} onClick={() => void run(() => replacePipelineOrchestrator(runId, rev, d.orchestrator!))}>
            Replace orchestrator
          </button>
        )}
        {controls.repair_cleanup.eligible && (
          <button type="button" disabled={disabled} onClick={() => void run(() => repairPipelineCleanup(runId, rev))}>
            Retry cleanup
          </button>
        )}
        {controls.stop.eligible && (
          <button type="button" className="phone-danger" disabled={disabled} onClick={() => void run(() => stopPipelineRun(runId, rev))}>
            Stop run
          </button>
        )}
      </div>
    </div>
  );
}
