import { AutoGrowTextarea, ConfirmDialog } from "../components/ui";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { continuePipelineRun, getPipelineRun, repairPipelineCleanup, replacePipelineOrchestrator, retryPipelineRun, stopPipelineRun } from "../api/pipelines";
import type { PipelineRuntimeAssignment as RuntimeAssignment } from "../schemas/pipeline";
import { getRuntimeOptions } from "./api";
import { useConnection } from "./connection";
import { navigate } from "./router";
import { PhoneIcon } from "./PhoneIcon";
import { PhoneSheet } from "./PhoneSheet";

const errorText = (error: unknown) => error instanceof Error ? error.message : String(error);

function ReplaceForm({ standing, reason, disabled, onSubmit }: { standing: RuntimeAssignment; reason: string; disabled: boolean; onSubmit: (runtime: RuntimeAssignment) => void }) {
  const options = useQuery({ queryKey: ["runtime-options"], queryFn: getRuntimeOptions });
  const [runtime, setRuntime] = useState<RuntimeAssignment>(standing);
  const backends = options.data?.backends ?? [];
  const backend = backends.find((item) => item.id === runtime.backend);
  const model = backend?.models.find((item) => item.id === runtime.model);
  const chooseModel = (backendID: string, modelID: string) => {
    const next = backends.find((item) => item.id === backendID)?.models.find((item) => item.id === modelID);
    setRuntime({ backend: backendID, model: modelID, effort: next?.default_effort ?? "", fast: false });
  };
  return <form className="phone-card phone-form" aria-label="Replace orchestrator" onSubmit={(event) => { event.preventDefault(); onSubmit(runtime); }}><p className="phone-card-kicker">Replace orchestrator</p><p className="phone-meta">{reason || "Cancels the unfinished assignment and retains stage work for the replacement."}</p>{options.isError && <p className="phone-error">{errorText(options.error)}</p>}<label className="phone-field">Backend<select value={runtime.backend} onChange={(event) => chooseModel(event.target.value, "")}><option value="">Choose…</option>{runtime.backend && !backend && <option value={runtime.backend}>{runtime.backend} (not configured)</option>}{backends.map((item) => <option key={item.id} value={item.id}>{item.name || item.id}</option>)}</select></label><label className="phone-field">Model<select value={runtime.model} onChange={(event) => chooseModel(runtime.backend, event.target.value)}><option value="">Choose…</option>{runtime.model && !model && <option value={runtime.model}>{runtime.model} (not configured)</option>}{(backend?.models ?? []).map((item) => <option key={item.id} value={item.id}>{item.name || item.id}</option>)}</select></label>{(model?.efforts.length || runtime.effort) ? <label className="phone-field">Effort<select value={runtime.effort} onChange={(event) => setRuntime({ ...runtime, effort: event.target.value })}><option value="">Model default</option>{runtime.effort && !model?.efforts.includes(runtime.effort) && <option value={runtime.effort}>{runtime.effort} (not available)</option>}{(model?.efforts ?? []).map((effort) => <option key={effort} value={effort}>{effort}</option>)}</select></label> : null}{(model?.fast || runtime.fast) && <label className="phone-check"><input type="checkbox" checked={runtime.fast} onChange={(event) => setRuntime({ ...runtime, fast: event.target.checked })} /> Fast mode</label>}<button type="submit" className="ad-button-primary" disabled={disabled || !runtime.backend || !runtime.model}>Replace orchestrator</button></form>;
}

export function RunScreen({ runId }: { runId: string }) {
  const offline = useConnection((state) => state.link !== "connected");
  const revision = useConnection((state) => state.revision);
  const client = useQueryClient();
  const detail = useQuery({ queryKey: ["run", runId, revision], queryFn: () => getPipelineRun(runId), placeholderData: (previous) => previous });
  const [input, setInput] = useState(""); const [busy, setBusy] = useState(false); const [error, setError] = useState("");
  const [stopOpen, setStopOpen] = useState(false);
  const [replaceOpen, setReplaceOpen] = useState(false);
  const run = async (operation: () => Promise<unknown>, after?: () => void) => { setBusy(true); setError(""); try { await operation(); after?.(); } catch (err) { setError(errorText(err)); } finally { setBusy(false); void client.invalidateQueries({ queryKey: ["run"] }); } };
  const data = detail.data;
  if (!data) return <p className="phone-empty">{detail.isError ? errorText(detail.error) : "Loading…"}</p>;
  const { run: pipeline, controls, template } = data; const rev = pipeline.revision; const disabled = offline || busy;
  const stageIndex = template.stages.findIndex((stage) => stage.id === pipeline.current_stage_id); const stage = stageIndex >= 0 ? template.stages[stageIndex] : undefined;
  const stageTasks = data.stage_tasks;
  const currentTask = stageTasks.find((task) => task.task_id === pipeline.current_task_id)
    ?? stageTasks.find((task) => task.stage_id === pipeline.current_stage_id && task.state !== "completed");
  const terminal = pipeline.state === "completed" || pipeline.state === "stopped";
  const headingStage = stage?.title ?? pipeline.current_stage_id;
  const stageTitles = new Map(template.stages.map((item) => [item.id, item.title]));
  const inputs = Object.entries(data.inputs);
  return <div className="phone-agent phone-run-page">
    <button type="button" className="phone-link phone-run-back ad-button-secondary" onClick={() => navigate(`/project/${encodeURIComponent(pipeline.project)}`)}><PhoneIcon name="back" size={16} />Project</button>
    <header className="phone-agent-header phone-page-heading">
      <p className="phone-eyebrow">PIPELINE RUN · {pipeline.project}</p>
      <h1>{pipeline.display_name || template.title}</h1>
      <p className="phone-meta">{pipeline.goal}</p>
      <span className="phone-status" data-tone={runTone(pipeline.state)}>{humanize(pipeline.final_outcome || pipeline.state)}</span>
    </header>

    <section className="phone-run-summary" aria-label="Current stage">
      <p className="phone-eyebrow">{terminal ? "FINAL POSITION" : "CURRENT STAGE"}</p>
      <strong>{headingStage || "No stage recorded"}</strong>
      {stage && <span>Stage {stageIndex + 1} of {template.stages.length}</span>}
    </section>

    {pipeline.attention_reason && <p className="phone-card phone-run-attention"><strong>Needs attention</strong><span>{humanize(pipeline.attention_reason)}</span></p>}
    {pipeline.current_agent_id && <button type="button" className="phone-link ad-button-secondary" onClick={() => navigate(`/agent/${encodeURIComponent(pipeline.current_agent_id)}`)}>Open conversation</button>}
    {error && <p className="phone-error" role="alert">{error}</p>}

    {controls.continue.eligible && <form className="phone-card phone-form" aria-label="Continue" onSubmit={(event) => { event.preventDefault(); void run(() => continuePipelineRun(runId, rev, input), () => setInput("")); }}><label className="phone-field">Input for the stage (optional)<AutoGrowTextarea rows={2} value={input} onChange={(event) => setInput(event.target.value)} /></label><button type="submit" className="ad-button-primary" disabled={disabled}>Continue</button></form>}
    <div className="phone-actions">{controls.retry.eligible && <button type="button" disabled={disabled} onClick={() => void run(() => retryPipelineRun(runId, rev))}>Retry stage</button>}{controls.repair_cleanup.eligible && <button type="button" disabled={disabled} onClick={() => void run(() => repairPipelineCleanup(runId, rev))}>Retry cleanup</button>}{controls.stop.eligible && <button type="button" className="ad-button-danger" disabled={disabled} onClick={() => setStopOpen(true)}>Stop run</button>}{controls.replace.eligible && <button type="button" disabled={disabled} onClick={() => setReplaceOpen(true)}>Replace orchestrator</button>}</div>
    {replaceOpen && <PhoneSheet title="Replace orchestrator" onClose={() => setReplaceOpen(false)}><ReplaceForm key={rev} standing={data.orchestrator ?? pipeline.orchestrator} reason={controls.replace.reason} disabled={disabled || !controls.replace.eligible} onSubmit={(runtime) => void run(() => replacePipelineOrchestrator(runId, rev, runtime), () => setReplaceOpen(false))} />{error && <p className="phone-error" role="alert">{error}</p>}</PhoneSheet>}
    <ConfirmDialog open={stopOpen} title="Stop this pipeline?" confirmLabel="Stop run" destructive pending={busy} confirmDisabled={offline || !controls.stop.eligible} onCancel={() => setStopOpen(false)} onConfirm={() => void run(() => stopPipelineRun(runId, rev), () => setStopOpen(false))}><p>Stop this work on your Mac? You can still view its recorded output.</p>{error && <p className="phone-error" role="alert">{error}</p>}</ConfirmDialog>

    <section className="phone-section phone-run-progress" aria-labelledby="phone-run-progress">
      <div className="phone-section-title"><div><p className="phone-eyebrow">STAGE HISTORY</p><h2 id="phone-run-progress">Progress</h2></div><span>{stageTasks.length} attempt{stageTasks.length === 1 ? "" : "s"}</span></div>
      {stageTasks.length ? <ol className="phone-timeline">{stageTasks.map((task) => {
        const outcome = task.result?.outcome || task.state;
        const title = stageTitles.get(task.stage_id) || task.stage_id;
        const current = task.task_id === currentTask?.task_id;
        return <li key={task.task_id} data-current={current || undefined} data-tone={runTone(outcome)}>
          <span className="phone-timeline-marker"><PhoneIcon name={outcome === "completed" || outcome === "success" ? "check" : "pipeline"} size={15} /></span>
          <div className="phone-timeline-content">
            <div className="phone-timeline-heading"><strong>{title}</strong><span className="phone-status" data-tone={runTone(outcome)}>{humanize(outcome)}</span></div>
            <p className="phone-meta">Stage {task.stage_index + 1} · Attempt {task.attempt_number}{task.room ? " · Think Tank" : task.standing_owner.name ? ` · ${task.standing_owner.name}` : ""}</p>
            {task.result?.summary ? <p>{task.result.summary}</p> : <p className="phone-meta">{task.room ? (task.attention_reason || "The room has not published a judge synthesis yet.") : current ? "The stage has not reported an outcome yet." : "No outcome was recorded."}</p>}
          </div>
        </li>;
      })}</ol> : <p className="phone-meta">No stage attempts have been recorded yet.</p>}
    </section>

    <details className="phone-details">
      <summary><PhoneIcon name="arrow" size={15} /><span>Goal &amp; required inputs</span></summary>
      <div className="phone-details-body"><section><h3>Goal</h3><p>{pipeline.goal}</p></section>
        {inputs.length > 0 && <section><h3>Run inputs</h3><dl>{inputs.map(([name, value]) => <div key={name}><dt>{name}</dt><dd>{value}</dd></div>)}</dl></section>}
        {data.values.length > 0 && <section><h3>Recorded values</h3><dl>{data.values.map((item) => <div key={item.name}><dt>{item.name}</dt><dd>{item.value}</dd></div>)}</dl></section>}
      </div>
    </details>
  </div>;
}

function humanize(value: string) {
  const label = value.replace(/[_-]+/g, " ");
  return label ? label[0].toUpperCase() + label.slice(1) : label;
}

function runTone(value: string) {
  if (["completed", "success", "connected"].includes(value)) return "green";
  if (["failed", "failure", "blocked", "stopped", "cleanup_failed"].includes(value)) return "coral";
  if (["paused", "waiting", "queued", "running"].includes(value)) return "amber";
  return "muted";
}
