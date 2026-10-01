import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { continuePipelineRun, getPipelineRun, repairPipelineCleanup, replacePipelineOrchestrator, retryPipelineRun, stopPipelineRun } from "../api/pipelines";
import type { PipelineRuntimeAssignment as RuntimeAssignment } from "../schemas/pipeline";
import { getRuntimeOptions } from "./api";
import { useConnection } from "./connection";
import { navigate } from "./router";

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
  return <form className="phone-card phone-form" aria-label="Replace orchestrator" onSubmit={(event) => { event.preventDefault(); onSubmit(runtime); }}><p className="phone-card-kicker">Replace orchestrator</p><p className="phone-meta">{reason || "Cancels the unfinished assignment and retains stage work for the replacement."}</p>{options.isError && <p className="phone-error">{errorText(options.error)}</p>}<label className="phone-field">Backend<select value={runtime.backend} onChange={(event) => chooseModel(event.target.value, "")}><option value="">Choose…</option>{runtime.backend && !backend && <option value={runtime.backend}>{runtime.backend} (not configured)</option>}{backends.map((item) => <option key={item.id} value={item.id}>{item.name || item.id}</option>)}</select></label><label className="phone-field">Model<select value={runtime.model} onChange={(event) => chooseModel(runtime.backend, event.target.value)}><option value="">Choose…</option>{runtime.model && !model && <option value={runtime.model}>{runtime.model} (not configured)</option>}{(backend?.models ?? []).map((item) => <option key={item.id} value={item.id}>{item.name || item.id}</option>)}</select></label>{(model?.efforts.length || runtime.effort) ? <label className="phone-field">Effort<select value={runtime.effort} onChange={(event) => setRuntime({ ...runtime, effort: event.target.value })}><option value="">Model default</option>{runtime.effort && !model?.efforts.includes(runtime.effort) && <option value={runtime.effort}>{runtime.effort} (not available)</option>}{(model?.efforts ?? []).map((effort) => <option key={effort} value={effort}>{effort}</option>)}</select></label> : null}{(model?.fast || runtime.fast) && <label className="phone-check"><input type="checkbox" checked={runtime.fast} onChange={(event) => setRuntime({ ...runtime, fast: event.target.checked })} /> Fast mode</label>}<button type="submit" disabled={disabled || !runtime.backend || !runtime.model}>Replace orchestrator</button></form>;
}

export function RunScreen({ runId }: { runId: string }) {
  const offline = useConnection((state) => state.link !== "connected");
  const revision = useConnection((state) => state.revision);
  const client = useQueryClient();
  const detail = useQuery({ queryKey: ["run", runId, revision], queryFn: () => getPipelineRun(runId), placeholderData: (previous) => previous });
  const [input, setInput] = useState(""); const [busy, setBusy] = useState(false); const [error, setError] = useState("");
  const run = async (operation: () => Promise<unknown>, after?: () => void) => { setBusy(true); setError(""); try { await operation(); after?.(); } catch (err) { setError(errorText(err)); } finally { setBusy(false); void client.invalidateQueries({ queryKey: ["run"] }); } };
  const data = detail.data;
  if (!data) return <p className="phone-empty">{detail.isError ? errorText(detail.error) : "Loading…"}</p>;
  const { run: pipeline, controls, template } = data; const rev = pipeline.revision; const disabled = offline || busy;
  const stageIndex = template.stages.findIndex((stage) => stage.id === pipeline.current_stage_id); const stage = stageIndex >= 0 ? template.stages[stageIndex] : undefined;
  return <div className="phone-agent"><header className="phone-agent-header"><h1>{pipeline.display_name}</h1><p className="phone-meta">Pipeline · {pipeline.project} · {pipeline.state}{stage ? ` · Stage ${stageIndex + 1} of ${template.stages.length} · ${stage.title}` : ""}</p></header>{pipeline.attention_reason && <p className="phone-card">{pipeline.attention_reason}</p>}{pipeline.final_outcome && <p className="phone-meta">Outcome: {pipeline.final_outcome}</p>}{pipeline.current_agent_id && <button type="button" className="phone-link" onClick={() => navigate(`/agent/${encodeURIComponent(pipeline.current_agent_id)}`)}>Open conversation</button>}{error && <p className="phone-error">{error}</p>}{controls.continue.eligible && <form className="phone-card phone-form" aria-label="Continue" onSubmit={(event) => { event.preventDefault(); void run(() => continuePipelineRun(runId, rev, input), () => setInput("")); }}><label className="phone-field">Input for the stage (optional)<textarea rows={2} value={input} onChange={(event) => setInput(event.target.value)} /></label><button type="submit" className="phone-primary" disabled={disabled}>Continue</button></form>}<div className="phone-actions">{controls.retry.eligible && <button type="button" disabled={disabled} onClick={() => void run(() => retryPipelineRun(runId, rev))}>Retry stage</button>}{controls.repair_cleanup.eligible && <button type="button" disabled={disabled} onClick={() => void run(() => repairPipelineCleanup(runId, rev))}>Retry cleanup</button>}{controls.stop.eligible && <button type="button" className="phone-danger" disabled={disabled} onClick={() => void run(() => stopPipelineRun(runId, rev))}>Stop run</button>}</div>{controls.replace.eligible && <ReplaceForm key={rev} standing={data.orchestrator ?? pipeline.orchestrator} reason={controls.replace.reason} disabled={disabled} onSubmit={(runtime) => void run(() => replacePipelineOrchestrator(runId, rev, runtime))} />}</div>;
}
