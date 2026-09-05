import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { launchAgent, sendPrompt } from "../../api/client";
import { useBackends, useConfig, useProjects, useRoles } from "../../api/config";
import {
  useDeclinePipelineProposal,
  useDeletePipelineProposal,
  usePipelineProposals,
  usePipelineTemplates,
} from "../../api/pipelines";
import type { PipelineListedProposal, PipelineProposal, PipelineTemplateRecord } from "../../schemas/pipeline";
import { useAgentStore } from "../../store/agentStore";
import { formatRelative } from "./RunBrowser";
import { asPipelineProposal, proposalKindLabel, summarizeProposal } from "./proposalSummary";

const BUILDER_KEY = "agentdeck.pipeline-builder-agent";

export function shouldDropBuilderSession(builderID: string | null, live: boolean, hydrated: boolean, hydrating: boolean, justLaunched: boolean) {
  return Boolean(builderID && hydrated && !hydrating && !live && !justLaunched);
}

export function AgentDeckerBuilder({
  onTemplateProposal,
  onRunProposal,
  proposalKind,
  showLauncher = true,
}: {
  onTemplateProposal?: (proposal: Extract<PipelineProposal, { kind: "save_template" }>) => void;
  onRunProposal?: (proposal: Extract<PipelineProposal, { kind: "start_run" }>) => void;
  proposalKind?: PipelineProposal["kind"];
  showLauncher?: boolean;
}) {
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
  // Reject and Delete refusals live on this panel rather than inside the card
  // that raised them. Their hooks await the durable refetch before the mutation
  // settles, and a real refusal (consumed, already declined, gone) moves or
  // removes that card, so a message the card owned would be discarded exactly
  // when it is the only explanation the person gets (INV §8, FS-14.R49/R57).
  const [proposalFailure, setProposalFailure] = useState<string | null>(null);
  const proposals = usePipelineProposals();
  // A start proposal names its template by id alone, so the library is what
  // resolves that id to a title as it stands now (FS-14.R51).
  const templates = usePipelineTemplates();
  // A stopped builder keeps its identity row in the agent store, so presence is
  // not liveness: classifying by presence never expires the persisted id and
  // leaves a dead "Open AgentDecker chat" link behind (INV §1).
  const builderRunning = useAgentStore((state) => (builderID ? state.agents[builderID]?.running === true : false));
  const agentsHydrated = useAgentStore((state) => state.hydrated);
  const agentsHydrating = useAgentStore((state) => state.hydrating);

  const backendEntries = Object.entries(backends.data?.backends ?? {});
  const defaultBackend = backendEntries.find(([, backend]) => backend.default)?.[0] ?? backendEntries[0]?.[0] ?? "";

  // Seed the default project only when it still resolves to a configured project,
  // exactly as RunStartForm does (INV §2). The builder previously launched into
  // `default_project` unconditionally with no picker, so a seeded-but-absent default
  // (the shipped `my-app`, whose cwd is missing on a fresh box) could only be
  // discovered as a rejected launch, with nothing on this page able to change it.
  useEffect(() => {
    if (project || !config.data?.default_project) return;
    if (projects.data?.[config.data.default_project] && !projects.data[config.data.default_project].archived) setProject(config.data.default_project);
  }, [config.data?.default_project, project, projects.data]);

  // A project removed from the catalog after selection (deleted in Settings or
  // another tab, then the query refetches) must not stay launchable: clear the
  // stale id so readiness and the seed effect re-evaluate against the current
  // catalog rather than posting an id the launch would reject (FS-14.R26).
  useEffect(() => {
    if (project && projects.data && (!projects.data[project] || projects.data[project].archived)) setProject("");
  }, [project, projects.data]);

  useEffect(() => {
    if (!backendID && defaultBackend) setBackendID(defaultBackend);
  }, [backendID, defaultBackend]);

  useEffect(() => {
    const backend = backends.data?.backends[backendID];
    setModelID(backend?.default_model ?? Object.keys(backend?.models ?? {})[0] ?? "");
  }, [backendID, backends.data]);

  useEffect(() => {
    if (builderRunning && justLaunchedBuilder.current === builderID) justLaunchedBuilder.current = null;
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
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setLaunching(false);
    }
  };

  const selectedBackend = backends.data?.backends[backendID];
  const projectEntries = Object.entries(projects.data ?? {}).filter(([, item]) => !item.archived);
  // Readiness follows a project that is still a current catalog member, not merely
  // a non-empty selection: a selection that has since left the catalog (or a
  // default naming a project that no longer exists) must hold the launch closed
  // rather than enabling a button whose only outcome is a rejected launch.
  const builderReady = Boolean(roles.data?.agentdecker && project && projects.data?.[project] && !projects.data[project].archived && backendID && modelID && description.trim());
  const ofThisKind = (proposal: PipelineListedProposal) => !proposalKind || proposal.kind === proposalKind;
  const pendingProposals = (proposals.data?.pending ?? []).filter(ofThisKind);
  const declinedProposals = (proposals.data?.declined ?? []).filter(ofThisKind);

  // A refusal that emptied both collections still has something to say, so the
  // panel stays mounted while its message is unread.
  if (!showLauncher && pendingProposals.length === 0 && declinedProposals.length === 0 && !proposalFailure) return null;

  return <section className="pipeline-panel pipeline-builder">
    {showLauncher && <div className="pipeline-panel-header">
      <div><p className="pipeline-eyebrow">Guided drafting</p><h2>Create with AgentDecker</h2></div>
      <button type="button" onClick={() => setOpen((value) => !value)}>{open ? "Close builder setup" : "Create with AgentDecker"}</button>
    </div>}
    {showLauncher && open && <div className="pipeline-builder-form">
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

    {showLauncher && builderID && <div className="pipeline-builder-session">
      <p>{!agentsHydrated ? "Loading builder session…" : builderRunning ? <>Builder session: <code>{builderID}</code></> : "The builder session has stopped. Its pending proposals remain available below."}</p>
      {builderRunning && <Link to={`/agent/${builderID}`}>Open AgentDecker chat</Link>}
    </div>}
    {proposalFailure && <p className="form-error">{proposalFailure}</p>}
    {pendingProposals.length > 0 && <div className="pipeline-proposal-list">
      <h3>Pending exact proposals</h3>
      {pendingProposals.map((entry) => <ProposalCard
        key={entry.proposal_id}
        entry={entry}
        templates={templates.data}
        onFailure={setProposalFailure}
        onTemplateProposal={onTemplateProposal}
        onRunProposal={onRunProposal}
      />)}
    </div>}
    {declinedProposals.length > 0 && <div className="pipeline-proposal-list pipeline-proposal-declined-list">
      <h3>Declined</h3>
      {declinedProposals.map((entry) => <ProposalCard key={entry.proposal_id} entry={entry} templates={templates.data} onFailure={setProposalFailure} />)}
    </div>}
  </section>;
}

// ProposalCard collapses one offer to a summary a person can scan and expands to
// the exact canonical payload an approval acts on. Expansion is per proposal and
// browser-local: it is component state rather than a stored preference, so a
// reload returns every proposal to collapsed (FS-14.R51).
function ProposalCard({
  entry,
  templates,
  onFailure,
  onTemplateProposal,
  onRunProposal,
}: {
  entry: PipelineListedProposal;
  templates: PipelineTemplateRecord[] | undefined;
  onFailure: (message: string | null) => void;
  onTemplateProposal?: (proposal: Extract<PipelineProposal, { kind: "save_template" }>) => void;
  onRunProposal?: (proposal: Extract<PipelineProposal, { kind: "start_run" }>) => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const decline = useDeclinePipelineProposal();
  const remove = useDeletePipelineProposal();
  const proposal = asPipelineProposal(entry);
  const summary = proposal ? summarizeProposal(proposal, templates) : [];
  const declined = Boolean(entry.declined_at);
  const busy = decline.isPending || remove.isPending;

  // Every mutation surfaces its failure through the panel above, which outlives
  // this card: the refetch that settles the mutation shows the durable state, and
  // when that state moves or removes the entry the explanation still stands (INV §8).
  const act = async (run: () => Promise<unknown>) => {
    onFailure(null);
    try {
      await run();
    } catch (reason) {
      onFailure(reason instanceof Error ? reason.message : String(reason));
    }
  };

  return <article className="pipeline-proposal">
    <div className="pipeline-proposal-summary">
      <strong>{proposalKindLabel(entry.kind)}</strong>
      <code>{entry.proposal_id}</code>
      {summary.map((field) => <span key={field.label} className="pipeline-proposal-field">
        <small>{field.label}</small>{field.value}
      </span>)}
      {!proposal && <span className="pipeline-proposal-field">This proposal cannot be summarized.</span>}
      <span className="pipeline-proposal-age">{declined
        ? `Declined ${formatRelative(entry.declined_at!)}`
        : `Pending ${formatRelative(entry.created_at)}`}</span>
    </div>
    <button type="button" className="pipeline-proposal-toggle" aria-expanded={expanded} onClick={() => setExpanded((value) => !value)}>
      {expanded ? "Hide exact payload" : "Show exact payload"}
    </button>
    {expanded && <pre className="pipeline-proposal-payload">{JSON.stringify(entry.payload, null, 2)}</pre>}
    <div className="pipeline-proposal-actions">
      {!declined && proposal?.kind === "save_template" && <button type="button" onClick={() => onTemplateProposal?.(proposal)}>Review exact Save proposal</button>}
      {!declined && proposal?.kind === "start_run" && <button type="button" onClick={() => onRunProposal?.(proposal)}>Review exact Start proposal</button>}
      {!declined && !proposal && <button type="button" disabled title="This proposal's payload cannot be read, so there is nothing exact to approve.">Review exact proposal</button>}
      {!declined && <button type="button" disabled={busy} onClick={() => void act(() => decline.mutateAsync(entry.proposal_id))}>Reject</button>}
      {declined && <button type="button" disabled={busy} onClick={() => void act(() => remove.mutateAsync(entry.proposal_id))}>Delete</button>}
    </div>
  </article>;
}
