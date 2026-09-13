import * as Dialog from "@radix-ui/react-dialog";
import { useEffect, useMemo, useRef, useState } from "react";
import { useBackends, useConfig, useProjects } from "../../api/config";
import {
  pipelineDiagnostics,
  sharedWorkspaceConflicts,
  usePipelineTemplates,
  useStartPipelineRun,
} from "../../api/pipelines";
import type {
  PipelineDiagnostic,
  PipelineProposal,
  PipelineRuntimeAssignment,
  PipelineStartRequest,
  PipelineWorkspaceConflict,
} from "../../schemas/pipeline";
import type { Backend } from "../../schemas/backends";

function requestID() {
  return typeof crypto !== "undefined" && "randomUUID" in crypto
    ? `ui_${crypto.randomUUID()}`
    : `ui_${Date.now()}_${Math.random().toString(16).slice(2)}`;
}

function RuntimeAssignment({ label, field, number = 1, value, backends, entries, onChange }: {
  label: string;
  field: string;
  number?: number;
  value: PipelineRuntimeAssignment;
  backends: Record<string, Backend> | undefined;
  entries: [string, Backend][];
  onChange: (value: PipelineRuntimeAssignment) => void;
}) {
  const backend = backends?.[value.backend];
  return <div className="pipeline-runtime-row" data-field={field}>
    <span className="pipeline-stage-number">{number}</span>
    <div><strong>{label}</strong><small>Frozen for this run</small></div>
    <label className="form-field"><span>Backend</span><select value={value.backend} onChange={(event) => { const backendID = event.target.value; const selected = backends?.[backendID]; const model = selected?.default_model || Object.keys(selected?.models ?? {})[0] || ""; onChange({ backend: backendID, model, effort: selected?.models[model]?.default_effort || "", fast: false }); }}><option value="">Select configured backend</option>{entries.map(([backendID, item]) => <option key={backendID} value={backendID}>{item.name} ({backendID})</option>)}</select></label>
    <label className="form-field"><span>Model</span><select value={value.model} onChange={(event) => { const model = backend?.models[event.target.value]; onChange({ ...value, model: event.target.value, effort: model?.default_effort || "", fast: false }); }}><option value="">Select configured model</option>{Object.entries(backend?.models ?? {}).map(([modelID, model]) => <option key={modelID} value={modelID}>{model.name} ({modelID})</option>)}</select></label>
    {(backend?.models[value.model]?.efforts ?? []).length > 0 && <label className="form-field"><span>Effort</span><select value={value.effort} onChange={(event) => onChange({ ...value, effort: event.target.value })}>{(backend?.models[value.model]?.efforts ?? []).map((effort) => <option key={effort} value={effort}>{effort}</option>)}</select></label>}
    {backend?.models[value.model]?.fast && <label className="form-field"><span>Speed</span><span><input type="checkbox" checked={value.fast} onChange={(event) => onChange({ ...value, fast: event.target.checked })} /> Fast mode</span></label>}
  </div>;
}

export function RunStartForm({
  proposal: proposalSeed,
  onStarted,
  stepMode = false,
  onCancel,
}: {
  proposal?: Extract<PipelineProposal, { kind: "start_run" }> | null;
  onStarted: (runID: string) => void;
  stepMode?: boolean;
  onCancel?: () => void;
}) {
  const templates = usePipelineTemplates();
  const projects = useProjects();
  const backends = useBackends();
  const config = useConfig();
  const start = useStartPipelineRun();
  const [templateID, setTemplateID] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [project, setProject] = useState("");
  const [goal, setGoal] = useState("");
  const [inputs, setInputs] = useState<Record<string, string>>({});
  const [orchestrator, setOrchestrator] = useState<PipelineStartRequest["orchestrator"]>({ backend: "", model: "", effort: "", fast: false });
  const [dedicatedAssignments, setDedicatedAssignments] = useState<PipelineStartRequest["dedicated_assignments"]>({});
  const [proposal, setProposal] = useState<typeof proposalSeed>();
  const [pendingRequest, setPendingRequest] = useState<PipelineStartRequest | null>(null);
  const [conflicts, setConflicts] = useState<PipelineWorkspaceConflict[]>([]);
  const [diagnostics, setDiagnostics] = useState<PipelineDiagnostic[]>([]);
  const [notice, setNotice] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [step, setStep] = useState(0);
  const formRef = useRef<HTMLElement>(null);

  const templateRecord = useMemo(
    () => templates.data?.find((entry) => entry.id === templateID),
    [templateID, templates.data],
  );
  const template = templateRecord?.template;
  const backendEntries = Object.entries(backends.data?.backends ?? {});
  const defaultBackend = backendEntries.find(([, backend]) => backend.default)?.[0] ?? backendEntries[0]?.[0] ?? "";

  useEffect(() => {
    if (!proposalSeed) return;
    const payload = proposalSeed.payload;
    setTemplateID(payload.template_id);
    setDisplayName(payload.display_name);
    setProject(payload.project);
    setGoal(payload.goal);
    setInputs({ ...payload.inputs });
    setOrchestrator(structuredClone(payload.orchestrator));
    setDedicatedAssignments(structuredClone(payload.dedicated_assignments));
    setProposal(proposalSeed);
    setPendingRequest(null);
    setConflicts([]);
    setDiagnostics([]);
    setNotice("Review the exact AgentDecker run proposal before confirming Start.");
    setError(null);
    setStep(0);
  }, [proposalSeed]);

  useEffect(() => {
    const field = diagnostics[0]?.field;
    if (!field) return;
    const owner = [...(formRef.current?.querySelectorAll<HTMLElement>("[data-field]") ?? [])]
      .find((item) => field === item.dataset.field || field.startsWith(`${item.dataset.field}.`));
    owner?.querySelector<HTMLElement>("input, select, textarea")?.focus();
  }, [diagnostics, step]);

  useEffect(() => {
    if (project || !config.data?.default_project) return;
    if (projects.data?.[config.data.default_project] && !projects.data[config.data.default_project].archived) setProject(config.data.default_project);
  }, [config.data?.default_project, project, projects.data]);

  useEffect(() => {
    if (project && projects.data && (!projects.data[project] || projects.data[project].archived)) setProject("");
  }, [project, projects.data]);

  useEffect(() => {
    if (!template || proposal) return;
    setInputs((current) => Object.fromEntries(template.inputs.map((input) => [input.name, current[input.name] ?? ""])));
    setOrchestrator((current) => {
      const backendID = current.backend || defaultBackend;
      const backend = backends.data?.backends[backendID];
      const model = current.model || backend?.default_model || Object.keys(backend?.models ?? {})[0] || "";
      return { backend: backendID, model, effort: current.effort || backend?.models[model]?.default_effort || "", fast: current.fast ?? false };
    });
    setDedicatedAssignments((current) => Object.fromEntries(template.stages.filter((stage) => stage.coordination === "dedicated").map((stage) => {
      const backendID = current[stage.id]?.backend || defaultBackend;
      const backend = backends.data?.backends[backendID];
      const model = current[stage.id]?.model || backend?.default_model || Object.keys(backend?.models ?? {})[0] || "";
      return [stage.id, { backend: backendID, model, effort: current[stage.id]?.effort || backend?.models[model]?.default_effort || "", fast: current[stage.id]?.fast ?? false }];
    })));
  }, [backends.data, defaultBackend, proposal, template]);

  const edit = () => {
    setPendingRequest(null);
    setConflicts([]);
    setDiagnostics([]);
    if (proposal) {
      setProposal(undefined);
      setNotice("The proposed run changed. Its confirmation was invalidated; this is now a manual run setup.");
    }
  };

  const buildRequest = (): PipelineStartRequest => ({
    request_id: proposal?.payload.request_id || pendingRequest?.request_id || requestID(),
    template_id: templateID,
    display_name: displayName,
    project,
    goal,
    inputs,
    orchestrator,
    dedicated_assignments: dedicatedAssignments,
  });

  const submit = (acknowledge: boolean) => {
    const requestBody = pendingRequest ?? buildRequest();
    setPendingRequest(requestBody);
    setError(null);
    setDiagnostics([]);
    start.mutate(
      { requestBody, acknowledge },
      {
        onSuccess: (response) => {
          setConflicts([]);
          setDiagnostics([]);
          setNotice(response.replay ? "The original idempotent run was returned." : "Run started.");
          setProposal(undefined);
          onStarted(response.run.run.run_id);
        },
        onError: (reason) => {
          const shared = sharedWorkspaceConflicts(reason);
          setConflicts(shared);
          setDiagnostics(pipelineDiagnostics(reason));
          setError(shared.length === 0 ? (reason instanceof Error ? reason.message : String(reason)) : null);
          const fields = pipelineDiagnostics(reason).map((item) => item.field);
          if (fields.some((field) => field.startsWith("orchestrator") || field.startsWith("dedicated_assignments."))) setStep(1);
          else if (fields.length > 0) setStep(0);
        },
      },
    );
  };

  const missingInputs = template?.inputs.filter((input) => input.required && !inputs[input.name]?.trim()) ?? [];
  const requiredMissing = template ? missingInputs.length > 0 : true;
  const assignmentsMissing = !orchestrator.backend || !orchestrator.model || (template?.stages.some((stage) => stage.coordination === "dedicated" && (!dedicatedAssignments[stage.id]?.backend || !dedicatedAssignments[stage.id]?.model)) ?? true);
  const projectAvailable = Boolean(project && projects.data?.[project] && !projects.data[project].archived);
  const cannotStart = !template || !projectAvailable || !goal.trim() || requiredMissing || assignmentsMissing || start.isPending;
  const setupIncomplete = !template || !projectAvailable || !goal.trim() || requiredMissing;
  // FS-14.A15: a disabled step control names the value it is still waiting for
  // instead of leaving the person to hunt for it.
  const setupBlocker = !template
    ? "Select a template to continue."
    : !projectAvailable
      ? "Select an active project to continue."
      : !goal.trim()
        ? "Enter the run goal to continue."
        : missingInputs.length > 0
          ? `Fill the required named input${missingInputs.length === 1 ? "" : "s"}: ${missingInputs.map((input) => input.name).join(", ")}`
          : null;
  const blocker = setupBlocker ?? (assignmentsMissing && (!stepMode || step > 0) ? "Assign a runtime to the standing owner and every dedicated coordinator." : null);

  return (
    <section ref={formRef} className={stepMode ? "pipeline-run-start pipeline-run-start-dialog" : "pipeline-panel pipeline-run-start"} data-ui="pipeline-start-dialog">
      {!stepMode && <div className="pipeline-panel-header">
        <div>
          <p className="pipeline-eyebrow">Frozen run snapshot</p>
          <h2>Start run</h2>
        </div>
      </div>}
      {stepMode && <ol className="pipeline-start-steps" aria-label="Start run steps" data-slot="steps">
        {["Setup", "Runtimes", "Review"].map((label, index) => <li key={label} className={step === index ? "pipeline-start-step-active" : step > index ? "pipeline-start-step-complete" : ""}><span>{index + 1}</span>{label}</li>)}
      </ol>}
      {(!stepMode || step === 0) && <div className="pipeline-start-pane" data-slot="content"><div className="pipeline-form-grid">
        <label className="form-field" data-field="template_id"><span>Template</span><select value={templateID} onChange={(event) => { edit(); setTemplateID(event.target.value); }}>
          <option value="">Select a valid template</option>
          {(templates.data ?? []).filter((record) => record.valid).map((record) => <option key={record.id} value={record.id}>{record.template.title} ({record.id})</option>)}
        </select></label>
        <label className="form-field" data-field="display_name"><span>Run display name</span><input value={displayName} placeholder={template?.title || "Delivery run"} onChange={(event) => { edit(); setDisplayName(event.target.value); }} /></label>
        <label className="form-field" data-field="project"><span>Project</span><select value={project} onChange={(event) => { edit(); setProject(event.target.value); }}>
          <option value="">Select project</option>
          {Object.entries(projects.data ?? {}).filter(([, item]) => !item.archived).map(([projectID, item]) => <option key={projectID} value={projectID}>{item.title} ({projectID})</option>)}
        </select></label>
      </div>
      <label className="form-field" data-field="goal"><span>Run goal</span><textarea rows={3} value={goal} onChange={(event) => { edit(); setGoal(event.target.value); }} /></label>

      {template && template.inputs.length > 0 && <div className="pipeline-subsection">
        <h3>Named inputs</h3>
        <div className="pipeline-input-list">
          {template.inputs.map((input) => <label className={input.required && !inputs[input.name]?.trim() ? "form-field pipeline-field-missing" : "form-field"} data-field={`inputs.${input.name}`} key={input.name}>
            <span>{input.name}{input.required ? " · required" : ""}</span>
            <small>{input.description}</small>
            <textarea rows={2} value={inputs[input.name] ?? ""} onChange={(event) => { edit(); setInputs((current) => ({ ...current, [input.name]: event.target.value })); }} />
          </label>)}
        </div>
      </div>}</div>}

      {template && (!stepMode || step === 1) && <div className={stepMode ? "pipeline-start-pane" : "pipeline-subsection"} data-slot="content">
        <h3>Runtime assignments</h3>
        <div className="pipeline-runtime-list">
          <RuntimeAssignment label={`Standing owner · ${template.orchestrator_role}`} field="orchestrator" value={orchestrator} backends={backends.data?.backends} entries={backendEntries} onChange={(value) => { edit(); setOrchestrator(value); }} />
          {template.stages.filter((stage) => stage.coordination === "dedicated").map((stage, index) => <RuntimeAssignment key={stage.id} label={`${stage.title} · coordinator ${stage.dedicated_role}`} field={`dedicated_assignments.${stage.id}`} number={index + 2} value={dedicatedAssignments[stage.id] ?? { backend: "", model: "", effort: "", fast: false }} backends={backends.data?.backends} entries={backendEntries} onChange={(value) => { edit(); setDedicatedAssignments((current) => ({ ...current, [stage.id]: value })); }} />)}
        </div>
      </div>}

      {stepMode && step === 2 && <div className="pipeline-start-pane pipeline-start-review" data-slot="content">
        <div className="pipeline-review-hero"><p className="pipeline-eyebrow">Ready to launch</p><h3>{displayName || template?.title || "Untitled run"}</h3><p>{goal}</p></div>
        <dl className="pipeline-review-facts"><div><dt>Template</dt><dd>{template?.title || templateID}</dd></div><div><dt>Project</dt><dd>{projects.data?.[project]?.title || project}</dd></div><div><dt>Stages</dt><dd>{template?.stages.length ?? 0}</dd></div><div><dt>Named inputs</dt><dd>{Object.values(inputs).filter((value) => value.trim()).length}</dd></div></dl>
        <ol className="pipeline-review-runtimes"><li><span>1</span><div><strong>Standing owner · {template?.orchestrator_role}</strong><small>{[orchestrator.backend, orchestrator.model, orchestrator.effort].filter(Boolean).join(" · ")}</small></div></li>{template?.stages.filter((stage) => stage.coordination === "dedicated").map((stage, index) => { const runtime = dedicatedAssignments[stage.id]; return <li key={stage.id}><span>{index + 2}</span><div><strong>{stage.title} coordinator · {stage.dedicated_role}</strong><small>{[runtime?.backend, runtime?.model, runtime?.effort].filter(Boolean).join(" · ")}</small></div></li>; })}</ol>
      </div>}

      {proposal && (!stepMode || step === 2) && <pre className="pipeline-proposal-payload">{JSON.stringify(proposal.payload, null, 2)}</pre>}
      {conflicts.length > 0 && <div className="pipeline-warning">
        <strong>Shared project workspace</strong>
        <p>These active agents or runs use the same project directory. AgentDeck does not isolate their filesystem changes.</p>
        <ul>{conflicts.map((conflict) => <li key={`${conflict.kind}-${conflict.id}`}>{conflict.kind}: {conflict.name} <code>{conflict.id}</code></li>)}</ul>
        <button type="button" disabled={start.isPending} onClick={() => submit(true)}>Confirm shared workspace and start</button>
      </div>}
      {notice && <p className="form-info">{notice}</p>}
      {diagnostics.length > 0 && (
        <ul className="pipeline-diagnostics">
          {diagnostics.map((diagnostic, index) => <li key={`${diagnostic.field}-${diagnostic.code}-${index}`}><code>{diagnostic.field}</code> — {diagnostic.message}</li>)}
        </ul>
      )}
      {error && <p className="form-error">{error}</p>}
      {stepMode ? <div className="pipeline-start-actions" data-slot="actions">
        <button type="button" onClick={onCancel}>Cancel</button>
        <span className="pipeline-start-blocker">{blocker}</span>
        {step > 0 && <button type="button" onClick={() => setStep((value) => value - 1)}>Back</button>}
        {step < 2 && <button type="button" disabled={step === 0 ? setupIncomplete : assignmentsMissing} onClick={() => setStep((value) => value + 1)}>Next</button>}
        {step === 2 && <button type="button" disabled={cannotStart} onClick={() => submit(false)}>{start.isPending ? "Starting…" : proposal ? "Confirm and start exact proposal" : "Start run"}</button>}
      </div> : <div className="form-actions"><span className="pipeline-start-blocker">{blocker}</span><button type="button" disabled={cannotStart} onClick={() => submit(false)}>{start.isPending ? "Starting…" : proposal ? "Confirm and start exact proposal" : "Start run"}</button></div>}
    </section>
  );
}

export function RunStartDialog({
  open,
  onOpenChange,
  proposal,
  onStarted,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  proposal?: Extract<PipelineProposal, { kind: "start_run" }> | null;
  onStarted: (runID: string) => void;
}) {
  return <Dialog.Root open={open} onOpenChange={onOpenChange}>
    <Dialog.Portal>
      <Dialog.Overlay className="dialog-overlay" data-ui="dialog" data-slot="overlay" />
      <Dialog.Content className="dialog-content pipeline-start-modal" data-ui="dialog" data-slot="content" data-variant="default">
        <Dialog.Title data-slot="title">Start pipeline run</Dialog.Title>
        <Dialog.Description className="pipeline-start-description">Configure one frozen run snapshot. Nothing launches until the final review.</Dialog.Description>
        <RunStartForm stepMode proposal={proposal} onCancel={() => onOpenChange(false)} onStarted={(runID) => { onOpenChange(false); onStarted(runID); }} />
      </Dialog.Content>
    </Dialog.Portal>
  </Dialog.Root>;
}
