import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { getTranscript, launchAgent, sendPrompt } from "../../api/client";
import { useBackends, useConfig, useProjects, useRoles } from "../../api/config";
import type { TranscriptEvent } from "../../api/types";
import { pipelineProposalSchema, type PipelineProposal } from "../../schemas/pipeline";
import { useTranscriptStore } from "../../store/transcriptStore";
import { useAgentStore } from "../../store/agentStore";

const BUILDER_KEY = "agentdeck.pipeline-builder-agent";
const EMPTY_EVENTS: TranscriptEvent[] = [];

export function shouldDropBuilderSession(builderID: string | null, live: boolean, hydrated: boolean, hydrating: boolean, justLaunched: boolean) {
  return Boolean(builderID && hydrated && !hydrating && !live && !justLaunched);
}

export function extractPipelineProposals(events: TranscriptEvent[]): PipelineProposal[] {
  const toolNames = new Map<string, string>();
  const proposals = new Map<string, PipelineProposal>();
  for (const event of events) {
    const kind = String(event.kind ?? event.type ?? "");
    const callID = String(event.tool_call_id ?? "");
    if (kind === "tool_call" && callID) {
      const name = String(event.name ?? "");
      const title = String(event.title ?? "");
      toolNames.set(callID, [name, title].find((value) => ["propose_pipeline_template", "propose_pipeline_run"].includes(value)) ?? name);
    }
    if (kind !== "tool_result" || !callID || !["propose_pipeline_template", "propose_pipeline_run"].includes(toolNames.get(callID) ?? "")) continue;
    for (const candidate of jsonCandidates(event.content ?? event.result)) {
      try {
        const parsed = JSON.parse(candidate) as { ok?: boolean; proposal?: unknown };
        const result = pipelineProposalSchema.safeParse(parsed.proposal);
        if (parsed.ok === true && result.success) proposals.set(result.data.digest, result.data);
      } catch {
        // Tool renderers accept arbitrary content; unrelated text is not a proposal.
      }
    }
  }
  return [...proposals.values()];
}

function jsonCandidates(value: unknown): string[] {
  if (typeof value === "string") return [value];
  if (Array.isArray(value)) return value.flatMap(jsonCandidates);
  if (!value || typeof value !== "object") return [];
  const object = value as Record<string, unknown>;
  const direct = typeof object.text === "string" ? [object.text] : [];
  return [...direct, ...Object.values(object).flatMap(jsonCandidates)];
}

export function AgentDeckerBuilder({
  onTemplateProposal,
  onRunProposal,
}: {
  onTemplateProposal: (proposal: Extract<PipelineProposal, { kind: "save_template" }>) => void;
  onRunProposal: (proposal: Extract<PipelineProposal, { kind: "start_run" }>) => void;
}) {
  const navigate = useNavigate();
  const roles = useRoles();
  const backends = useBackends();
  const config = useConfig();
  const projects = useProjects();
  const [open, setOpen] = useState(false);
  const [project, setProject] = useState("");
  const [backendID, setBackendID] = useState("");
  const [modelID, setModelID] = useState("");
  const [description, setDescription] = useState("");
  const [builderID, setBuilderID] = useState<string | null>(() => localStorage.getItem(BUILDER_KEY));
  const justLaunchedBuilder = useRef<string | null>(null);
  const [launching, setLaunching] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const events = useTranscriptStore((state) => builderID ? state.byAgent[builderID] ?? EMPTY_EVENTS : EMPTY_EVENTS);
  const setTranscript = useTranscriptStore((state) => state.setTranscript);
  // A stopped builder keeps its identity row in the agent store, so presence is
  // not liveness: classifying by presence never expires the persisted id and
  // leaves a dead "Open AgentDecker chat" link behind (INV §1).
  const builderRunning = useAgentStore((state) => (builderID ? state.agents[builderID]?.running === true : false));
  const agentsHydrated = useAgentStore((state) => state.hydrated);
  const agentsHydrating = useAgentStore((state) => state.hydrating);
  const proposals = useMemo(() => extractPipelineProposals(events), [events]);

  const backendEntries = Object.entries(backends.data?.backends ?? {});
  const defaultBackend = backendEntries.find(([, backend]) => backend.default)?.[0] ?? backendEntries[0]?.[0] ?? "";

  // Seed the default project only when it still resolves to a configured project,
  // exactly as RunStartForm does (INV §2). The builder previously launched into
  // `default_project` unconditionally with no picker, so a seeded-but-absent default
  // (the shipped `my-app`, whose cwd is missing on a fresh box) could only be
  // discovered as a rejected launch, with nothing on this page able to change it.
  useEffect(() => {
    if (project || !config.data?.default_project) return;
    if (projects.data?.[config.data.default_project]) setProject(config.data.default_project);
  }, [config.data?.default_project, project, projects.data]);

  // A project removed from the catalog after selection (deleted in Settings or
  // another tab, then the query refetches) must not stay launchable: clear the
  // stale id so readiness and the seed effect re-evaluate against the current
  // catalog rather than posting an id the launch would reject (FS-14.R26).
  useEffect(() => {
    if (project && projects.data && !projects.data[project]) setProject("");
  }, [project, projects.data]);

  useEffect(() => {
    if (!backendID && defaultBackend) setBackendID(defaultBackend);
  }, [backendID, defaultBackend]);

  useEffect(() => {
    const backend = backends.data?.backends[backendID];
    setModelID(backend?.default_model ?? Object.keys(backend?.models ?? {})[0] ?? "");
  }, [backendID, backends.data]);

  useEffect(() => {
    if (!builderID || !agentsHydrated || agentsHydrating || !builderRunning) return;
    if (justLaunchedBuilder.current === builderID) justLaunchedBuilder.current = null;
    void getTranscript(builderID)
      .then((transcript) => setTranscript(transcript.agent_id, transcript.events))
      .catch(() => undefined);
  }, [agentsHydrated, agentsHydrating, builderID, builderRunning, setTranscript]);

  useEffect(() => {
    if (!shouldDropBuilderSession(builderID, builderRunning, agentsHydrated, agentsHydrating, justLaunchedBuilder.current === builderID)) return;
    localStorage.removeItem(BUILDER_KEY);
    setBuilderID(null);
  }, [agentsHydrated, agentsHydrating, builderID, builderRunning]);

  const launchBuilder = async () => {
    if (!description.trim() || !project) return;
    setLaunching(true);
    setError(null);
    try {
      const response = await launchAgent({
        role: "agentdecker",
        project,
        backend: backendID,
        model: modelID,
        interface: "chat",
        name: "Pipeline Builder",
      });
      const agentID = response.agent.agent_id;
      justLaunchedBuilder.current = agentID;
      localStorage.setItem(BUILDER_KEY, agentID);
      setBuilderID(agentID);
      await sendPrompt(agentID, [
        "Help me design this AgentDeck pipeline:",
        description.trim(),
        "Ask any clarifying questions in chat. When the design is ready, call propose_pipeline_template with the exact model-neutral draft. Do not save or start anything yourself.",
      ].join("\n\n"));
      navigate(`/agent/${agentID}`);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setLaunching(false);
    }
  };

  const selectedBackend = backends.data?.backends[backendID];
  const projectEntries = Object.entries(projects.data ?? {});
  // Readiness follows a project that is still a current catalog member, not merely
  // a non-empty selection: a selection that has since left the catalog (or a
  // default naming a project that no longer exists) must hold the launch closed
  // rather than enabling a button whose only outcome is a rejected launch.
  const builderReady = Boolean(roles.data?.agentdecker && project && projects.data?.[project] && backendID && modelID && description.trim());

  return <section className="pipeline-panel pipeline-builder">
    <div className="pipeline-panel-header">
      <div><p className="pipeline-eyebrow">Guided drafting</p><h2>Create with AgentDecker</h2></div>
      <button type="button" onClick={() => setOpen((value) => !value)}>{open ? "Close builder setup" : "Create with AgentDecker"}</button>
    </div>
    {open && <div className="pipeline-builder-form">
      <p>Choose the project and chat runtime for the ordinary AgentDecker session. This choice is not stored in the model-neutral template.</p>
      {!roles.data?.agentdecker && <p className="form-error">The configured <code>agentdecker</code> role is required.</p>}
      {projectEntries.length === 0 && <p className="form-error">Configure a project before launching the builder.</p>}
      <div className="pipeline-form-grid">
        <label className="form-field"><span>Project</span><select value={project} onChange={(event) => setProject(event.target.value)}>
          <option value="">Select project</option>
          {projectEntries.map(([projectID, item]) => <option key={projectID} value={projectID}>{item.title} ({projectID})</option>)}
        </select></label>
        <label className="form-field"><span>Configured backend</span><select value={backendID} onChange={(event) => setBackendID(event.target.value)}>
          <option value="">Select backend</option>
          {backendEntries.map(([id, backend]) => <option key={id} value={id}>{backend.name} ({id})</option>)}
        </select></label>
        <label className="form-field"><span>Configured model</span><select value={modelID} onChange={(event) => setModelID(event.target.value)}>
          <option value="">Select model</option>
          {Object.entries(selectedBackend?.models ?? {}).map(([id, model]) => <option key={id} value={id}>{model.name} ({id})</option>)}
        </select></label>
      </div>
      <label className="form-field"><span>Describe the pipeline</span><textarea rows={4} value={description} onChange={(event) => setDescription(event.target.value)} placeholder="Implement a change, review it, validate it, and loop through a fix when validation fails." /></label>
      {error && <p className="form-error">{error}</p>}
      <div className="form-actions"><button type="button" disabled={!builderReady || launching} onClick={() => void launchBuilder()}>{launching ? "Launching…" : "Launch AgentDecker builder"}</button></div>
    </div>}

    {builderID && <div className="pipeline-builder-session">
      <p>Builder session: <code>{builderID}</code></p>
      <Link to={`/agent/${builderID}`}>Open AgentDecker chat</Link>
    </div>}
    {proposals.length > 0 && <div className="pipeline-proposal-list">
      <h3>Pending exact proposals</h3>
      {proposals.map((proposal) => <article className="pipeline-proposal" key={proposal.digest}>
        <div><strong>{proposal.kind === "save_template" ? "Save template" : "Start run"}</strong><code>{proposal.proposal_id}</code></div>
        <pre className="pipeline-proposal-payload">{JSON.stringify(proposal.payload, null, 2)}</pre>
        {proposal.kind === "save_template"
          ? <button type="button" onClick={() => onTemplateProposal(proposal)}>Review exact Save proposal</button>
          : <button type="button" onClick={() => onRunProposal(proposal)}>Review exact Start proposal</button>}
      </article>)}
    </div>}
  </section>;
}
