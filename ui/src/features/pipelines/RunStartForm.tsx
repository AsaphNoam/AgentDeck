import { AutoGrowTextarea } from "../../components/ui";
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
import { displayLabel, displayLabels } from "../../lib/labels";

type Assignment = PipelineStartRequest["orchestrator"];
type RoomAssignments = NonNullable<PipelineStartRequest["think_tank_assignments"]>;
type RuntimeSlot = { key: string; label: string; editLabel?: string; field: string; value: Assignment; set: (value: Assignment) => void };

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
    <label className="form-field"><span>Backend</span><select value={value.backend} onChange={(event) => { const backendID = event.target.value; const selected = backends?.[backendID]; const model = selected?.default_model || Object.keys(selected?.models ?? {})[0] || ""; onChange({ backend: backendID, model, effort: selected?.models[model]?.default_effort || "", fast: false }); }}><option value="">Select configured backend</option>{displayLabels(entries.map(([backendID, item]) => [backendID, item.name])).map(([backendID, name]) => <option key={backendID} value={backendID}>{name}</option>)}</select></label>
    <label className="form-field"><span>Model</span><select value={value.model} onChange={(event) => { const model = backend?.models[event.target.value]; onChange({ ...value, model: event.target.value, effort: model?.default_effort || "", fast: false }); }}><option value="">Select configured model</option>{displayLabels(Object.entries(backend?.models ?? {}).map(([modelID, model]) => [modelID, model.name])).map(([modelID, name]) => <option key={modelID} value={modelID}>{name}</option>)}</select></label>
    {(backend?.models[value.model]?.efforts ?? []).length > 0 && <label className="form-field"><span>Effort</span><select value={value.effort} onChange={(event) => onChange({ ...value, effort: event.target.value })}>{(backend?.models[value.model]?.efforts ?? []).map((effort) => <option key={effort} value={effort}>{effort}</option>)}</select></label>}
    {backend?.models[value.model]?.fast && <label className="form-field"><span>Speed</span><span><input type="checkbox" checked={value.fast} onChange={(event) => onChange({ ...value, fast: event.target.checked })} /> Fast mode</span></label>}
  </div>;
}

function runtimeSummary(value: PipelineRuntimeAssignment, backends: Record<string, Backend> | undefined) {
  const backend = backends?.[value.backend];
  const model = backend?.models[value.model];
  const backendLabel = backend ? displayLabel(Object.entries(backends ?? {}).map(([id, item]) => [id, item.name]), value.backend) : value.backend || "Choose a backend";
  const modelLabel = model ? displayLabel(Object.entries(backend.models).map(([id, item]) => [id, item.name]), value.model) : value.model || "choose a model";
  const fastLabel = model?.fast ? (value.fast ? "Fast mode on" : "Fast mode off") : "";
  return [backendLabel, modelLabel, value.effort, fastLabel].filter(Boolean).join(" · ");
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
  const [roomAssignments, setRoomAssignments] = useState<RoomAssignments>({});
  const [proposal, setProposal] = useState<typeof proposalSeed>();
  const [pendingRequest, setPendingRequest] = useState<PipelineStartRequest | null>(null);
  const [conflicts, setConflicts] = useState<PipelineWorkspaceConflict[]>([]);
  const [diagnostics, setDiagnostics] = useState<PipelineDiagnostic[]>([]);
  const [notice, setNotice] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [step, setStep] = useState(0);
  const [runtimeOpen, setRuntimeOpen] = useState(false);
  const [seededTemplate, setSeededTemplate] = useState<unknown>(null);
  const [runtimeSource, setRuntimeSource] = useState<"defaults" | "proposal" | "selected">("defaults");
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
    setRoomAssignments(structuredClone(payload.think_tank_assignments ?? {}));
    setProposal(proposalSeed);
    setRuntimeSource("proposal");
    setPendingRequest(null);
    setConflicts([]);
    setDiagnostics([]);
    setNotice("Review the exact Chucky run proposal before confirming Start.");
    setError(null);
    setStep(0);
  }, [proposalSeed]);

  useEffect(() => {
    const field = diagnostics[0]?.field;
    if (!field) return;
    setStep(0);
    if (field.startsWith("orchestrator") || field.startsWith("assignments.") || field.startsWith("dedicated_assignments.") || field.startsWith("think_tank_assignments.")) setRuntimeOpen(true);
  }, [diagnostics]);

  useEffect(() => {
    const field = diagnostics[0]?.field;
    if (!field || step !== 0) return;
    const owner = [...(formRef.current?.querySelectorAll<HTMLElement>("[data-field]") ?? [])]
      .find((item) => field === item.dataset.field || field.startsWith(`${item.dataset.field}.`));
    owner?.querySelector<HTMLElement>("input, select, textarea")?.focus();
  }, [diagnostics, runtimeOpen, step]);

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
    // Configured defaults fill only absent selections (FS-14.R80).
    const seed = (current?: Assignment): Assignment => {
      const backendID = current?.backend || defaultBackend;
      const backend = backends.data?.backends[backendID];
      const model = current?.model || backend?.default_model || Object.keys(backend?.models ?? {})[0] || "";
      return { backend: backendID, model, effort: current?.effort || backend?.models[model]?.default_effort || "", fast: current?.fast ?? false };
    };
    setOrchestrator((current) => seed(current));
    setDedicatedAssignments((current) => Object.fromEntries(template.stages.filter((stage) => stage.coordination === "dedicated").map((stage) => [stage.id, seed(current[stage.id])])));
    setRoomAssignments((current) => Object.fromEntries(template.stages.filter((stage) => stage.coordination === "think_tank" && stage.think_tank).map((stage) => [stage.id, {
      participants: Object.fromEntries(stage.think_tank!.participants.map((participant) => [participant.id, seed(current[stage.id]?.participants[participant.id])])),
      judge: seed(current[stage.id]?.judge),
    }])));
    if (backends.data) setSeededTemplate(template);
  }, [backends.data, defaultBackend, proposal, template]);

  const edit = () => {
    setPendingRequest(null);
    setConflicts([]);
    setDiagnostics([]);
    if (proposal) {
      setProposal(undefined);
      setRuntimeSource("selected");
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
    ...(Object.keys(roomAssignments).length > 0 ? { think_tank_assignments: roomAssignments } : {}),
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
          if (pipelineDiagnostics(reason).length > 0) setStep(0);
        },
      },
    );
  };

  const missingInputs = template?.inputs.filter((input) => input.required && !inputs[input.name]?.trim()) ?? [];
  const requiredMissing = template ? missingInputs.length > 0 : true;
  // Every runtime slot the run freezes: the standing owner when an ordinary
  // stage needs one, each dedicated coordinator, and each Think Tank
  // participant and judge (FS-14.R81, TS-08.R95).
  const blank: Assignment = { backend: "", model: "", effort: "", fast: false };
  const setRoomSlot = (stageID: string, participantID: string | null, value: Assignment) => setRoomAssignments((current) => {
    const room = current[stageID] ?? { participants: {}, judge: blank };
    return { ...current, [stageID]: participantID === null ? { ...room, judge: value } : { ...room, participants: { ...room.participants, [participantID]: value } } };
  });
  const slots: RuntimeSlot[] = template ? [
    ...(template.stages.some((stage) => stage.coordination !== "think_tank")
      ? [{ key: "orchestrator", label: `Standing owner · ${template.orchestrator_role}`, field: "orchestrator", value: orchestrator, set: setOrchestrator }]
      : []),
    ...template.stages.filter((stage) => stage.coordination === "dedicated").map((stage) => ({
      key: stage.id,
      label: `${stage.title} coordinator · ${stage.dedicated_role}`,
      editLabel: `${stage.title} · coordinator ${stage.dedicated_role}`,
      field: `dedicated_assignments.${stage.id}`,
      value: dedicatedAssignments[stage.id] ?? blank,
      set: (value: Assignment) => setDedicatedAssignments((current) => ({ ...current, [stage.id]: value })),
    })),
    ...template.stages.filter((stage) => stage.coordination === "think_tank" && stage.think_tank).flatMap((stage) => [
      ...stage.think_tank!.participants.map((participant) => ({
        key: `${stage.id}:${participant.id}`,
        label: `${stage.title} Think Tank · ${participant.id} (${participant.role})`,
        field: `think_tank_assignments.${stage.id}.participants.${participant.id}`,
        value: roomAssignments[stage.id]?.participants[participant.id] ?? blank,
        set: (value: Assignment) => setRoomSlot(stage.id, participant.id, value),
      })),
      {
        key: `${stage.id}:judge`,
        label: `${stage.title} Think Tank · judge (${stage.think_tank!.judge_role})`,
        field: `think_tank_assignments.${stage.id}.judge`,
        value: roomAssignments[stage.id]?.judge ?? blank,
        set: (value: Assignment) => setRoomSlot(stage.id, null, value),
      },
    ]),
  ] : [];
  const assignmentsMissing = !template || slots.some((slot) => !slot.value.backend || !slot.value.model);
  // FS-14.R80: defaults that cannot fill every assignment expose the controls
  // that must be set, instead of leaving the blocker behind a closed disclosure.
  useEffect(() => {
    if (template && seededTemplate === template && assignmentsMissing) setRuntimeOpen(true);
  }, [assignmentsMissing, seededTemplate, template]);
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
  const hasRoom = template?.stages.some((stage) => stage.coordination === "think_tank") ?? false;
  const blocker = setupBlocker ?? (assignmentsMissing
    ? `Customize runtimes for the standing owner and every dedicated coordinator${hasRoom ? ", Think Tank participant and judge" : ""}.`
    : null);

  const runtimeRows = slots;
  const runtimeSourceLabel = runtimeSource === "proposal"
    ? "Proposal selections"
    : runtimeSource === "defaults"
      ? "Configured defaults"
      : "Selected values";

  return (
    <section ref={formRef} className={stepMode ? "pipeline-run-start pipeline-run-start-dialog" : "pipeline-panel pipeline-run-start"} data-ui="pipeline-start-dialog">
      {!stepMode && <div className="pipeline-panel-header">
        <div>
          <p className="pipeline-eyebrow">Frozen run snapshot</p>
          <h2>Start run</h2>
        </div>
      </div>}
      {stepMode && <ol className="pipeline-start-steps" aria-label="Start run steps" data-slot="steps">
        {["Setup", "Review"].map((label, index) => <li key={label} className={step === index ? "pipeline-start-step-active" : step > index ? "pipeline-start-step-complete" : ""}><span>{index + 1}</span>{label}</li>)}
      </ol>}
      {(!stepMode || step === 0) && <div className="pipeline-start-pane" data-slot="content"><div className="pipeline-form-grid">
        <label className="form-field" data-field="template_id"><span>Template</span><select value={templateID} onChange={(event) => { edit(); setTemplateID(event.target.value); }}>
          <option value="">Select a valid template</option>
          {displayLabels((templates.data ?? []).filter((record) => record.valid).map((record) => [record.id, record.template.title])).map(([id, title]) => <option key={id} value={id}>{title}</option>)}
        </select></label>
        <label className="form-field" data-field="display_name"><span>Run display name</span><input value={displayName} placeholder={template?.title || "Delivery run"} onChange={(event) => { edit(); setDisplayName(event.target.value); }} /></label>
        <label className="form-field" data-field="project"><span>Project</span><select value={project} onChange={(event) => { edit(); setProject(event.target.value); }}>
          <option value="">Select project</option>
          {displayLabels(Object.entries(projects.data ?? {}).filter(([, item]) => !item.archived).map(([projectID, item]) => [projectID, item.title])).map(([projectID, title]) => <option key={projectID} value={projectID}>{title}</option>)}
        </select></label>
      </div>
      <label className="form-field" data-field="goal"><span>Run goal</span><AutoGrowTextarea rows={3} value={goal} onChange={(event) => { edit(); setGoal(event.target.value); }} /></label>

      {template && template.inputs.length > 0 && <div className="pipeline-subsection">
        <h3>Named inputs</h3>
        <div className="pipeline-input-list">
          {template.inputs.map((input) => <label className={input.required && !inputs[input.name]?.trim() ? "form-field pipeline-field-missing" : "form-field"} data-field={`inputs.${input.name}`} key={input.name}>
            <span>{input.name}{input.required ? " · required" : ""}</span>
            <small>{input.description}</small>
            <AutoGrowTextarea rows={2} value={inputs[input.name] ?? ""} onChange={(event) => { edit(); setInputs((current) => ({ ...current, [input.name]: event.target.value })); }} />
          </label>)}
        </div>
      </div>}
      {template && <section className="pipeline-runtime-summary" aria-label="Selected runtimes">
        <div className="pipeline-runtime-summary-heading"><h3>Selected runtimes</h3><span>{runtimeSourceLabel}</span></div>
        <ol>{runtimeRows.map((row) => <li key={row.key}><strong>{row.label}</strong><small>{runtimeSummary(row.value, backends.data?.backends)}</small></li>)}</ol>
      </section>}
      {template && <details className="pipeline-disclosure pipeline-runtime-customize" open={runtimeOpen} onToggle={(event) => setRuntimeOpen(event.currentTarget.open)}>
        <summary>Customize runtimes</summary>
        <div className="pipeline-disclosure-body"><div className="pipeline-runtime-list">
          {slots.map((slot, index) => <RuntimeAssignment key={slot.key} label={slot.editLabel ?? slot.label} field={slot.field} number={index + 1} value={slot.value} backends={backends.data?.backends} entries={backendEntries} onChange={(value) => { edit(); setRuntimeSource("selected"); slot.set(value); }} />)}
        </div></div>
      </details>}</div>}

      {template && (!stepMode || step === 1) && <div className={stepMode ? "pipeline-start-pane pipeline-start-review" : "pipeline-subsection"} data-slot="content">
        {stepMode && <>
        <div className="pipeline-review-hero"><p className="pipeline-eyebrow">Ready to launch</p><h3>{displayName || template?.title || "Untitled run"}</h3><p>{goal}</p></div>
        <dl className="pipeline-review-facts"><div><dt>Template</dt><dd>{template?.title || templateID}</dd></div><div><dt>Project</dt><dd>{projects.data?.[project]?.title || project}</dd></div><div><dt>Stages</dt><dd>{template?.stages.length ?? 0}</dd></div><div><dt>Named inputs</dt><dd>{Object.values(inputs).filter((value) => value.trim()).length}</dd></div></dl>
        <ol className="pipeline-review-runtimes">{runtimeRows.map((row, index) => <li key={row.key}><span>{index + 1}</span><div><strong>{row.label}</strong><small>{runtimeSummary(row.value, backends.data?.backends)}</small></div></li>)}</ol>
        </>}
      </div>}

      {proposal && (!stepMode || step === 1) && <pre className="pipeline-proposal-payload">{JSON.stringify(proposal.payload, null, 2)}</pre>}
      {conflicts.length > 0 && <div className="pipeline-warning">
        <strong>Shared project workspace</strong>
        <p>These active agents or runs use the same project directory. Chuck does not isolate their filesystem changes.</p>
        <ul>{conflicts.map((conflict) => <li key={`${conflict.kind}-${conflict.id}`}>{conflict.kind}: {conflict.name} <code>{conflict.id}</code></li>)}</ul>
        <button type="button" className="ad-button-primary" disabled={start.isPending} onClick={() => submit(true)}>Confirm shared workspace and start</button>
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
        {step === 0 && <button type="button" className="ad-button-primary" disabled={setupIncomplete || assignmentsMissing} onClick={() => setStep(1)}>Review</button>}
        {step === 1 && <button type="button" onClick={() => setStep(0)}>Back</button>}
        {step === 1 && <button type="button" className="ad-button-primary" disabled={cannotStart} onClick={() => submit(false)}>{start.isPending ? "Starting…" : proposal ? "Confirm and start exact proposal" : "Start run"}</button>}
      </div> : <div className="form-actions"><span className="pipeline-start-blocker">{blocker}</span><button type="button" className="ad-button-primary" disabled={cannotStart} onClick={() => submit(false)}>{start.isPending ? "Starting…" : proposal ? "Confirm and start exact proposal" : "Start run"}</button></div>}
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
