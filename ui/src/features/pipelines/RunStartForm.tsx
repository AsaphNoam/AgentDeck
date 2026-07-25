import { useEffect, useMemo, useState } from "react";
import { useBackends, useConfig, useProjects } from "../../api/config";
import {
  sharedWorkspaceConflicts,
  usePipelineTemplates,
  useStartPipelineRun,
} from "../../api/pipelines";
import type {
  PipelineProposal,
  PipelineStartRequest,
  PipelineWorkspaceConflict,
} from "../../schemas/pipeline";

function requestID() {
  return typeof crypto !== "undefined" && "randomUUID" in crypto
    ? `ui_${crypto.randomUUID()}`
    : `ui_${Date.now()}_${Math.random().toString(16).slice(2)}`;
}

export function RunStartForm({
  proposal: proposalSeed,
  onStarted,
}: {
  proposal?: Extract<PipelineProposal, { kind: "start_run" }> | null;
  onStarted: (runID: string) => void;
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
  const [assignments, setAssignments] = useState<PipelineStartRequest["assignments"]>({});
  const [proposal, setProposal] = useState<typeof proposalSeed>();
  const [pendingRequest, setPendingRequest] = useState<PipelineStartRequest | null>(null);
  const [conflicts, setConflicts] = useState<PipelineWorkspaceConflict[]>([]);
  const [notice, setNotice] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

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
    setAssignments(structuredClone(payload.assignments));
    setProposal(proposalSeed);
    setPendingRequest(null);
    setConflicts([]);
    setNotice("Review the exact AgentDecker run proposal before confirming Start.");
    setError(null);
  }, [proposalSeed]);

  useEffect(() => {
    if (project || !config.data?.default_project) return;
    if (projects.data?.[config.data.default_project]) setProject(config.data.default_project);
  }, [config.data?.default_project, project, projects.data]);

  useEffect(() => {
    if (!template || proposal) return;
    setInputs((current) => Object.fromEntries(template.inputs.map((input) => [input.name, current[input.name] ?? ""])));
    setAssignments((current) => Object.fromEntries(template.stages.map((stage) => {
      const backendID = current[stage.id]?.backend || defaultBackend;
      const backend = backends.data?.backends[backendID];
      const model = current[stage.id]?.model || backend?.default_model || Object.keys(backend?.models ?? {})[0] || "";
      return [stage.id, { backend: backendID, model }];
    })));
  }, [backends.data, defaultBackend, proposal, template]);

  const edit = () => {
    setPendingRequest(null);
    setConflicts([]);
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
    assignments,
  });

  const submit = (acknowledge: boolean) => {
    const requestBody = pendingRequest ?? buildRequest();
    setPendingRequest(requestBody);
    setError(null);
    start.mutate(
      { requestBody, acknowledge },
      {
        onSuccess: (response) => {
          setConflicts([]);
          setNotice(response.replay ? "The original idempotent run was returned." : "Run started.");
          setProposal(undefined);
          onStarted(response.run.run.run_id);
        },
        onError: (reason) => {
          const shared = sharedWorkspaceConflicts(reason);
          setConflicts(shared);
          setError(shared.length === 0 ? (reason instanceof Error ? reason.message : String(reason)) : null);
        },
      },
    );
  };

  const requiredMissing = template?.inputs.some((input) => input.required && !inputs[input.name]?.trim()) ?? true;
  const assignmentsMissing = template?.stages.some((stage) => !assignments[stage.id]?.backend || !assignments[stage.id]?.model) ?? true;
  const cannotStart = !template || !project || !goal.trim() || requiredMissing || assignmentsMissing || start.isPending;

  return (
    <section className="pipeline-panel pipeline-run-start">
      <div className="pipeline-panel-header">
        <div>
          <p className="pipeline-eyebrow">Frozen run snapshot</p>
          <h2>Start run</h2>
        </div>
      </div>
      <div className="pipeline-form-grid">
        <label className="form-field"><span>Template</span><select value={templateID} onChange={(event) => { edit(); setTemplateID(event.target.value); }}>
          <option value="">Select a valid template</option>
          {(templates.data ?? []).filter((record) => record.valid).map((record) => <option key={record.id} value={record.id}>{record.template.title} ({record.id})</option>)}
        </select></label>
        <label className="form-field"><span>Run display name</span><input value={displayName} placeholder={template?.title || "Delivery run"} onChange={(event) => { edit(); setDisplayName(event.target.value); }} /></label>
        <label className="form-field"><span>Project</span><select value={project} onChange={(event) => { edit(); setProject(event.target.value); }}>
          <option value="">Select project</option>
          {Object.entries(projects.data ?? {}).map(([projectID, item]) => <option key={projectID} value={projectID}>{item.title} ({projectID})</option>)}
        </select></label>
      </div>
      <label className="form-field"><span>Run goal</span><textarea rows={3} value={goal} onChange={(event) => { edit(); setGoal(event.target.value); }} /></label>

      {template && template.inputs.length > 0 && <div className="pipeline-subsection">
        <h3>Named inputs</h3>
        <div className="pipeline-input-list">
          {template.inputs.map((input) => <label className="form-field" key={input.name}>
            <span>{input.name}{input.required ? " · required" : ""}</span>
            <small>{input.description}</small>
            <textarea rows={2} value={inputs[input.name] ?? ""} onChange={(event) => { edit(); setInputs((current) => ({ ...current, [input.name]: event.target.value })); }} />
          </label>)}
        </div>
      </div>}

      {template && <div className="pipeline-subsection">
        <h3>Stage runtimes</h3>
        <div className="pipeline-runtime-list">
          {template.stages.map((stage, index) => {
            const assignment = assignments[stage.id] ?? { backend: "", model: "" };
            const backend = backends.data?.backends[assignment.backend];
            return <div className="pipeline-runtime-row" key={stage.id}>
              <span className="pipeline-stage-number">{index + 1}</span>
              <div><strong>{stage.title}</strong><small>{stage.role}</small></div>
              <label className="form-field"><span>Backend</span><select value={assignment.backend} onChange={(event) => {
                edit();
                const backendID = event.target.value;
                const selected = backends.data?.backends[backendID];
                setAssignments((current) => ({ ...current, [stage.id]: { backend: backendID, model: selected?.default_model || Object.keys(selected?.models ?? {})[0] || "" } }));
              }}>
                <option value="">Select configured backend</option>
                {backendEntries.map(([backendID, item]) => <option key={backendID} value={backendID}>{item.name} ({backendID})</option>)}
              </select></label>
              <label className="form-field"><span>Model</span><select value={assignment.model} onChange={(event) => { edit(); setAssignments((current) => ({ ...current, [stage.id]: { ...assignment, model: event.target.value } })); }}>
                <option value="">Select configured model</option>
                {Object.entries(backend?.models ?? {}).map(([modelID, model]) => <option key={modelID} value={modelID}>{model.name} ({modelID})</option>)}
              </select></label>
            </div>;
          })}
        </div>
      </div>}

      {proposal && <pre className="pipeline-proposal-payload">{JSON.stringify(proposal.payload, null, 2)}</pre>}
      {conflicts.length > 0 && <div className="pipeline-warning">
        <strong>Shared project workspace</strong>
        <p>These active agents or runs use the same project directory. AgentDeck does not isolate their filesystem changes.</p>
        <ul>{conflicts.map((conflict) => <li key={`${conflict.kind}-${conflict.id}`}>{conflict.kind}: {conflict.name} <code>{conflict.id}</code></li>)}</ul>
        <button type="button" disabled={start.isPending} onClick={() => submit(true)}>Confirm shared workspace and start</button>
      </div>}
      {notice && <p className="form-info">{notice}</p>}
      {error && <p className="form-error">{error}</p>}
      <div className="form-actions">
        <button type="button" disabled={cannotStart} onClick={() => submit(false)}>
          {start.isPending ? "Starting…" : proposal ? "Confirm and start exact proposal" : "Start run"}
        </button>
      </div>
    </section>
  );
}
