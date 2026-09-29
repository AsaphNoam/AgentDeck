import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { TaskArmInput } from "../api/tasks";
import {
  continuePipelineRun,
  getPipelineRun,
  listPipelineRuns,
  repairPipelineCleanup,
  replacePipelineOrchestrator,
  retryPipelineRun,
  stopPipelineRun,
} from "../api/pipelines";
import { WORK_RESULT_OUTCOMES, taskActions, type Task, type TaskArm } from "../schemas/task";
import type { PipelineRuntimeAssignment as RuntimeAssignment } from "../schemas/pipeline";
import { getRuntimeOptions, phoneFetch, PhoneAPIError } from "./api";
import { useConnection } from "./connection";
import { navigate } from "./router";

const errorText = (error: unknown) => (error instanceof Error ? error.message : String(error));
const post = (url: string, body?: unknown) =>
  phoneFetch<unknown>(url, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body ?? {}) });

/** useAction runs one mutation at a time, reports the Mac's refusal verbatim
 *  (FS-20.R27), and refreshes the screen either way. `after` may return a
 *  confirmation, which the screen keeps showing after the form remounts. */
function useAction(refresh: () => void) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const run = async (fn: () => Promise<unknown>, after?: () => string | void) => {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await fn();
      setNotice(after?.() || null);
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
      refresh();
    }
  };
  return { busy, error, notice, run };
}

function OpenConversation({ agentId }: { agentId?: string }) {
  if (!agentId) return null;
  return (
    <button type="button" className="phone-link" onClick={() => navigate(`/agent/${encodeURIComponent(agentId)}`)}>
      Open conversation
    </button>
  );
}

type SourceKind = "task" | "pipeline_run";
type ArmDraft = { key: number; kind: TaskArm["kind"]; source_kind: SourceKind; source_id: string; satisfying_outcomes: string[]; signal_name: string };

let nextArmKey = 0;
const armDraft = (arm?: TaskArm): ArmDraft => ({
  key: nextArmKey++,
  kind: arm?.kind ?? "work_result",
  source_kind: arm?.source_kind === "pipeline_run" ? "pipeline_run" : "task",
  source_id: arm?.source_id ?? "",
  satisfying_outcomes: arm?.satisfying_outcomes ?? ["success"],
  signal_name: arm?.signal_name ?? "",
});
const armInput = (arm: ArmDraft): TaskArmInput =>
  arm.kind === "signal"
    ? { kind: "signal", signal_name: arm.signal_name.trim() }
    : { kind: "work_result", source_kind: arm.source_kind, source_id: arm.source_id, satisfying_outcomes: arm.satisfying_outcomes };

/** The first incomplete prerequisite, named in the editor's own words, so a
 *  shape refusal never reaches the person as a server field name. */
function armProblem(arms: ArmDraft[]): string | null {
  for (const [index, arm] of arms.entries()) {
    const n = index + 1;
    if (arm.kind === "signal" && !arm.signal_name.trim()) return `Name the signal for prerequisite ${n}.`;
    if (arm.kind === "work_result" && !arm.source_id) return `Choose the ${arm.source_kind === "task" ? "task" : "pipeline run"} for prerequisite ${n}.`;
    if (arm.kind === "work_result" && arm.satisfying_outcomes.length === 0) return `Choose at least one outcome for prerequisite ${n}.`;
  }
  return null;
}

/** Plain language for the Mac's typed Re-arm refusals (FS-20.R30, INV §8). */
function rearmRefusal(err: unknown): string {
  if (err instanceof PhoneAPIError) {
    if (err.detailCode === "dependency_cycle") return "That would make these tasks wait on each other in a loop. Choose a different prerequisite.";
    if (err.detailCode === "unusable_source") return "A prerequisite names a task or run that can't be waited on here. Choose a different one.";
    if (err.detailCode === "invalid_state") return "This task can no longer be re-armed. It has moved on since you opened it.";
    if (err.code === "conflict") return "The task changed on the Mac. Check it and try again.";
  }
  return errorText(err);
}

/** RearmEditor starts from the task's current prerequisites and submits the
 *  complete replacement set; the Mac is the one graph/state validator, and a
 *  refusal keeps the draft (FS-20.R30, FS-16.R23). */
function RearmEditor({ task, disabled, run }: { task: Task; disabled: boolean; run: ReturnType<typeof useAction>["run"] }) {
  const [arms, setArms] = useState(() => (task.arms ?? []).map(armDraft));
  const tasks = useQuery({
    queryKey: ["tasks", task.project],
    queryFn: () => phoneFetch<{ tasks: Task[] }>(`/api/tasks?project=${encodeURIComponent(task.project)}`),
  });
  const runs = useQuery({ queryKey: ["runs"], queryFn: () => listPipelineRuns() });
  const sources: Record<SourceKind, { id: string; label: string }[]> = {
    task: (tasks.data?.tasks ?? []).filter((item) => item.task_id !== task.task_id).map((item) => ({ id: item.task_id, label: `${item.display_name} · ${item.state.replace("_", " ")}` })),
    pipeline_run: (runs.data?.runs ?? []).filter((item) => item.project === task.project).map((item) => ({ id: item.run_id, label: `${item.display_name || item.template_id} · ${item.state}` })),
  };
  const update = (key: number, change: Partial<ArmDraft>) => setArms((current) => current.map((arm) => (arm.key === key ? { ...arm, ...change } : arm)));
  const [problem, setProblem] = useState<string | null>(null);
  // The confirmation names what now gates the task, in the editor's labels.
  const applied = () => {
    if (arms.length === 0) return "Re-armed with no prerequisites.";
    const names = arms.map((arm) =>
      arm.kind === "signal"
        ? `signal “${arm.signal_name.trim()}”`
        : `${sources[arm.source_kind].find((option) => option.id === arm.source_id)?.label.split(" · ")[0] ?? arm.source_id} (${arm.satisfying_outcomes.join(" or ")})`,
    );
    return `Re-armed. It now waits for ${names.join(", ")}.`;
  };

  return (
    <form
      className="phone-card phone-form"
      aria-label="Re-arm"
      onSubmit={(event) => {
        event.preventDefault();
        const incomplete = armProblem(arms);
        setProblem(incomplete);
        if (incomplete) return;
        void run(async () => {
          try {
            await post(`/api/tasks/${encodeURIComponent(task.task_id)}/rearm`, { arms: arms.map(armInput) });
          } catch (err) {
            throw new Error(rearmRefusal(err));
          }
        }, applied);
      }}
    >
      <p className="phone-card-kicker">Re-arm</p>
      <p className="phone-meta">Re-arm replaces every prerequisite with the list below. An empty list removes them all.</p>
      {arms.map((arm, index) => {
        const label = `Prerequisite ${index + 1}`;
        const options = sources[arm.source_kind];
        return (
          <fieldset key={arm.key} className="phone-arm" aria-label={label}>
            <legend>{label}</legend>
            <label className="phone-field">
              Wait for
              <select
                value={arm.kind === "signal" ? "signal" : arm.source_kind}
                onChange={(event) =>
                  event.target.value === "signal"
                    ? update(arm.key, { kind: "signal" })
                    : update(arm.key, { kind: "work_result", source_kind: event.target.value as SourceKind, source_id: "", satisfying_outcomes: ["success"] })
                }
              >
                <option value="task">A task's result</option>
                <option value="pipeline_run">A pipeline run's result</option>
                <option value="signal">A named signal</option>
              </select>
            </label>
            {arm.kind === "signal" ? (
              <label className="phone-field">
                Signal name
                <input value={arm.signal_name} onChange={(event) => update(arm.key, { signal_name: event.target.value })} />
              </label>
            ) : (
              <>
                <label className="phone-field">
                  {arm.source_kind === "task" ? "Task" : "Pipeline run"}
                  <select value={arm.source_id} onChange={(event) => update(arm.key, { source_id: event.target.value })}>
                    <option value="">Choose…</option>
                    {arm.source_id && !options.some((option) => option.id === arm.source_id) && <option value={arm.source_id}>{arm.source_id}</option>}
                    {options.map((option) => (
                      <option key={option.id} value={option.id}>
                        {option.label}
                      </option>
                    ))}
                  </select>
                </label>
                <div className="phone-arm-outcomes" role="group" aria-label={`${label} outcomes`}>
                  {WORK_RESULT_OUTCOMES[arm.source_kind].map((outcome) => (
                    <label key={outcome.value}>
                      <input
                        type="checkbox"
                        checked={arm.satisfying_outcomes.includes(outcome.value)}
                        onChange={(event) =>
                          update(arm.key, {
                            satisfying_outcomes: event.target.checked
                              ? [...arm.satisfying_outcomes, outcome.value]
                              : arm.satisfying_outcomes.filter((value) => value !== outcome.value),
                          })
                        }
                      />
                      {outcome.label}
                    </label>
                  ))}
                </div>
              </>
            )}
            <button type="button" className="phone-link" onClick={() => setArms((current) => current.filter((item) => item.key !== arm.key))}>
              Remove {label.toLowerCase()}
            </button>
          </fieldset>
        );
      })}
      <button type="button" onClick={() => setArms((current) => [...current, armDraft()])}>
        Add prerequisite
      </button>
      {problem && <p className="phone-error">{problem}</p>}
      <button type="submit" className="phone-primary" disabled={disabled}>
        Re-arm
      </button>
    </form>
  );
}

export function TaskScreen({ taskId }: { taskId: string }) {
  const offline = useConnection((state) => state.link !== "connected");
  const revision = useConnection((state) => state.revision);
  const client = useQueryClient();
  const task = useQuery({
    queryKey: ["task", taskId, revision],
    queryFn: () => phoneFetch<Task>(`/api/tasks/${encodeURIComponent(taskId)}`),
    placeholderData: (prev) => prev,
  });
  const { busy, error, notice, run } = useAction(() => void client.invalidateQueries({ queryKey: ["task", taskId] }));
  const [outcome, setOutcome] = useState("success");
  const [summary, setSummary] = useState("");

  const t = task.data;
  if (!t) return <p className="phone-empty">{task.isError ? errorText(task.error) : "Loading…"}</p>;
  const id = encodeURIComponent(taskId);
  const actions = taskActions(t);
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
      {notice && (
        <p className="phone-card" role="status">
          {notice}
        </p>
      )}
      <div className="phone-actions">
        {actions.retry && (
          <button type="button" className="phone-primary" disabled={disabled} onClick={() => void run(() => post(`/api/tasks/${id}/retry`))}>
            Retry
          </button>
        )}
        {actions.cancel && (
          <button type="button" className="phone-danger" disabled={disabled} onClick={() => void run(() => post(`/api/tasks/${id}/cancel`))}>
            Cancel task
          </button>
        )}
      </div>
      {/* Keyed by revision: an accepted Re-arm resets the draft to the new
          arms, while a refusal leaves the revision and the draft untouched. */}
      {actions.rearm && <RearmEditor key={t.revision} task={t} disabled={disabled} run={run} />}
      {actions.recordResult && (
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

/** ReplaceForm offers the Mac's configured runtime choices, preselected to the
 *  run's standing assignment; the Mac revalidates the submitted assignment
 *  against its current configuration before launching it (FS-20.R31). */
function ReplaceForm({ standing, reason, disabled, onSubmit }: {
  standing: RuntimeAssignment;
  reason: string;
  disabled: boolean;
  onSubmit: (runtime: RuntimeAssignment) => void;
}) {
  const options = useQuery({ queryKey: ["runtime-options"], queryFn: getRuntimeOptions });
  const [runtime, setRuntime] = useState<RuntimeAssignment>(standing);
  const backends = options.data?.backends ?? [];
  const backend = backends.find((item) => item.id === runtime.backend);
  const model = backend?.models.find((item) => item.id === runtime.model);
  const chooseModel = (backendID: string, modelID: string) => {
    const next = backends.find((item) => item.id === backendID)?.models.find((item) => item.id === modelID);
    setRuntime({ backend: backendID, model: modelID, effort: next?.default_effort ?? "", fast: false });
  };

  return (
    <form
      className="phone-card phone-form"
      aria-label="Replace orchestrator"
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit(runtime);
      }}
    >
      <p className="phone-card-kicker">Replace orchestrator</p>
      <p className="phone-meta">{reason || "Cancels the unfinished assignment and retains stage work for the replacement."}</p>
      {options.isError && <p className="phone-error">{errorText(options.error)}</p>}
      <label className="phone-field">
        Backend
        <select value={runtime.backend} onChange={(event) => chooseModel(event.target.value, "")}>
          <option value="">Choose…</option>
          {runtime.backend && !backend && <option value={runtime.backend}>{runtime.backend} (not configured)</option>}
          {backends.map((item) => (
            <option key={item.id} value={item.id}>
              {item.name || item.id}
            </option>
          ))}
        </select>
      </label>
      <label className="phone-field">
        Model
        <select value={runtime.model} onChange={(event) => chooseModel(runtime.backend, event.target.value)}>
          <option value="">Choose…</option>
          {runtime.model && !model && <option value={runtime.model}>{runtime.model} (not configured)</option>}
          {(backend?.models ?? []).map((item) => (
            <option key={item.id} value={item.id}>
              {item.name || item.id}
            </option>
          ))}
        </select>
      </label>
      {(model?.efforts.length || runtime.effort) ? (
        <label className="phone-field">
          Effort
          <select value={runtime.effort} onChange={(event) => setRuntime({ ...runtime, effort: event.target.value })}>
            <option value="">Model default</option>
            {runtime.effort && !model?.efforts.includes(runtime.effort) && <option value={runtime.effort}>{runtime.effort} (not available)</option>}
            {(model?.efforts ?? []).map((effort) => (
              <option key={effort} value={effort}>
                {effort}
              </option>
            ))}
          </select>
        </label>
      ) : null}
      {(model?.fast || runtime.fast) && (
        <label className="phone-check">
          <input type="checkbox" checked={runtime.fast} onChange={(event) => setRuntime({ ...runtime, fast: event.target.checked })} />
          Fast mode
        </label>
      )}
      <button type="submit" disabled={disabled || !runtime.backend || !runtime.model}>
        Replace orchestrator
      </button>
    </form>
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
      {controls.replace.eligible && (
        <ReplaceForm
          key={rev}
          standing={d.orchestrator ?? s.orchestrator}
          reason={controls.replace.reason}
          disabled={disabled}
          onSubmit={(runtime) => void run(() => replacePipelineOrchestrator(runId, rev, runtime))}
        />
      )}
    </div>
  );
}
