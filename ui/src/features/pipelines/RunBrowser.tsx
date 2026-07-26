import { Link } from "react-router-dom";
import {
  useDeletePipelineRun,
  usePipelineControl,
  usePipelineRun,
  usePipelineRuns,
} from "../../api/pipelines";
import { useState } from "react";
import { useAgentStore } from "../../store/agentStore";

export function attemptTranscriptPath(agentID: string, live: boolean) {
  return `/${live ? "agent" : "archive"}/${agentID}`;
}

export function RunBrowser({ selectedID, onSelect }: { selectedID: string | null; onSelect: (id: string | null) => void }) {
  const runs = usePipelineRuns();

  return (
    <section className="pipeline-panel pipeline-runs">
      <div className="pipeline-panel-header">
        <div>
          <p className="pipeline-eyebrow">Durable history</p>
          <h2>Runs</h2>
        </div>
        <button type="button" onClick={() => void runs.refetch()}>Refresh</button>
      </div>
      {runs.isLoading && <p className="pipeline-empty">Loading runs…</p>}
      {runs.error && <p className="form-error">{runs.error.message}</p>}
      {!runs.isLoading && (runs.data?.length ?? 0) === 0 && <p className="pipeline-empty">No pipeline runs yet.</p>}
      {(runs.data?.length ?? 0) > 0 && <div className="pipeline-run-layout">
        <ul className="pipeline-run-list">
          {(runs.data ?? []).map((run) => <li key={run.run_id}>
            <button type="button" disabled={run.diagnostics.length > 0} className={selectedID === run.run_id ? "pipeline-run-button pipeline-run-button-selected" : "pipeline-run-button"} onClick={() => onSelect(run.run_id)}>
              <span><strong>{run.display_name || run.template_id}</strong><small>{run.project} · {run.current_stage_id || "finished"}</small></span>
              <span className={`pipeline-state pipeline-state-${run.state}`}>{run.state}</span>
            </button>
            {run.diagnostics.map((diagnostic) => <small className="form-error" key={diagnostic.code}>{diagnostic.message}</small>)}
          </li>)}
        </ul>
        <RunDetail selectedID={selectedID} onDeleted={() => onSelect(null)} />
      </div>}
    </section>
  );
}

function RunDetail({ selectedID, onDeleted }: { selectedID: string | null; onDeleted: () => void }) {
  const detail = usePipelineRun(selectedID);
  const continueRun = usePipelineControl("continue");
  const retryRun = usePipelineControl("retry");
  const stopRun = usePipelineControl("stop");
  const deleteRun = useDeletePipelineRun();
  const liveAgents = useAgentStore((state) => state.agents);
  const [continuation, setContinuation] = useState("");
  const [error, setError] = useState<string | null>(null);

  if (!selectedID) return <div className="pipeline-run-detail pipeline-empty">Select a run to inspect its frozen setup and attempt history.</div>;
  if (detail.isLoading) return <div className="pipeline-run-detail pipeline-empty">Loading run…</div>;
  if (!detail.data) return <div className="pipeline-run-detail"><p className="form-error">{detail.error?.message ?? "Run unavailable."}</p></div>;

  const data = detail.data;
  const run = data.run;
  const stage = data.template.stages.find((item) => item.id === run.current_stage_id);
  const attempt = data.attempts.find((item) => item.attempt_id === run.current_attempt_id);
  const paused = run.state === "paused";
  const blocked = paused && run.attention_reason === "blocked" && attempt?.report_outcome === "blocked";
  const approval = paused && run.pending_action === "await_approval";
  const canContinue = blocked || approval;
  const canRetry = paused && !approval && run.attention_reason !== "loop_limit_reached";
  const terminal = run.state === "completed" || run.state === "stopped";
  const busy = continueRun.isPending || retryRun.isPending || stopRun.isPending || deleteRun.isPending;

  const control = (action: "continue" | "retry" | "stop") => {
    setError(null);
    const mutation = action === "continue" ? continueRun : action === "retry" ? retryRun : stopRun;
    mutation.mutate(
      { id: run.run_id, revision: run.revision, input: action === "continue" ? continuation : "" },
      {
        onSuccess: () => setContinuation(""),
        onError: (reason) => setError(reason instanceof Error ? reason.message : String(reason)),
      },
    );
  };

  return <article className="pipeline-run-detail">
    <div className="pipeline-run-title">
      <div><p className="pipeline-eyebrow"><code>{run.run_id}</code></p><h3>{run.display_name || data.template.title}</h3></div>
      <span className={`pipeline-state pipeline-state-${run.state}`}>{run.state}{run.final_outcome ? ` · ${run.final_outcome}` : ""}</span>
    </div>
    <p className="pipeline-goal">{run.goal}</p>
    <dl className="pipeline-facts">
      <div><dt>Project</dt><dd>{run.project}</dd></div>
      <div><dt>Current stage</dt><dd>{stage?.title ?? (run.current_stage_id || "—")}</dd></div>
      <div><dt>Agent</dt><dd>{run.current_agent_id || "—"}</dd></div>
      <div><dt>Runtime</dt><dd>{attempt ? `${attempt.backend} · ${attempt.model}` : "—"}</dd></div>
      <div><dt>Attempt / visit</dt><dd>{attempt ? `${attempt.attempt_no} / ${attempt.visit_no}` : "—"}</dd></div>
      <div><dt>Revision</dt><dd>{run.revision}</dd></div>
    </dl>
    {run.attention_reason && <div className="pipeline-warning"><strong>Needs attention</strong><p>{run.attention_reason.replace(/_/g, " ")}</p></div>}

    <div className="pipeline-run-actions">
      {run.current_agent_id && <Link className="pipeline-link-button" to={`/agent/${run.current_agent_id}`}>Open agent</Link>}
      {canContinue && <button type="button" disabled={busy || (blocked && !continuation.trim())} onClick={() => control("continue")}>{approval ? "Approve and continue" : "Continue"}</button>}
      {canRetry && <button type="button" disabled={busy} onClick={() => control("retry")}>Retry stage</button>}
      {!terminal && <button type="button" className="btn-danger" disabled={busy} onClick={() => control("stop")}>Stop run</button>}
      {terminal && <button type="button" className="btn-danger" disabled={busy} onClick={() => deleteRun.mutate(run.run_id, { onSuccess: onDeleted, onError: (reason) => setError(reason instanceof Error ? reason.message : String(reason)) })}>Delete run record</button>}
    </div>
    {blocked && <label className="form-field"><span>New input for the blocked stage</span><textarea rows={3} value={continuation} onChange={(event) => setContinuation(event.target.value)} /></label>}
    {error && <p className="form-error">{error}</p>}

    <div className="pipeline-subsection">
      <h4>Named values</h4>
      {data.values.length === 0 ? <p className="pipeline-empty">No values recorded.</p> : <dl className="pipeline-value-list">
        {data.values.map((value) => <div key={value.name}><dt>{value.name}<small>{value.source_kind}{value.source_attempt_id ? ` · ${value.source_attempt_id}` : ""}</small></dt><dd>{value.value}</dd></div>)}
      </dl>}
    </div>

    <div className="pipeline-subsection">
      <h4>Attempt history</h4>
      <ol className="pipeline-attempt-list">
        {data.attempts.map((item) => <li key={item.attempt_id}>
          <div className="pipeline-attempt-heading">
            <span className="pipeline-stage-number">{item.attempt_no}</span>
            <div><strong>{data.template.stages.find((candidate) => candidate.id === item.stage_id)?.title ?? item.stage_id}</strong><small>visit {item.visit_no} · {item.backend} · {item.model}</small></div>
            <span className="pipeline-state">{item.report_outcome || item.state}</span>
          </div>
          {item.report_summary && <p>{item.report_summary}</p>}
          {item.report_details && <details><summary>Details</summary><pre>{item.report_details}</pre></details>}
          {item.report_checks && <details><summary>Checks</summary><pre>{item.report_checks}</pre></details>}
          {item.agent_id && <Link to={attemptTranscriptPath(item.agent_id, Boolean(liveAgents[item.agent_id]))}>Open transcript</Link>}
        </li>)}
      </ol>
    </div>
  </article>;
}
