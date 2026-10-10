import { AutoGrowTextarea } from "../components/ui";
import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { launchAgent } from "../api/client";
import { listPipelineTemplates, pipelineDiagnostics, sharedWorkspaceConflicts, startPipelineRun } from "../api/pipelines";
import type { AgentState } from "../api/types";
import { deriveDashboardProjects } from "../features/dashboard/projectDashboardData";
import { getHome, getRuntimeOptions, phoneFetch } from "./api";
import { useConnection } from "./connection";
import { navigate } from "./router";
import { useSuggestedName } from "../features/launch/useSuggestedName";
import { PhoneIcon } from "./PhoneIcon";
import { PhoneSheet } from "./PhoneSheet";
import { GroupPicker } from "../components/ui/GroupPicker";
import { useGroupActions } from "../components/groups/GroupActions";
import { groupAgents } from "../lib/groups";

const errorText = (error: unknown) => error instanceof Error ? error.message : String(error);

function RuntimeFields({ value, onChange }: { value: { backend: string; model: string; effort: string; fast: boolean }; onChange: (value: { backend: string; model: string; effort: string; fast: boolean }) => void }) {
  const options = useQuery({ queryKey: ["runtime-options"], queryFn: getRuntimeOptions });
  const backends = options.data?.backends ?? [];
  const backend = backends.find((item) => item.id === value.backend) ?? backends.find((item) => item.default) ?? backends[0];
  const model = backend?.models.find((item) => item.id === value.model) ?? backend?.models.find((item) => item.id === backend.default_model) ?? backend?.models[0];
  const pickBackend = (backendID: string) => {
    const next = backends.find((item) => item.id === backendID);
    const nextModel = next?.models.find((item) => item.id === next.default_model) ?? next?.models[0];
    onChange({ backend: backendID, model: nextModel?.id ?? "", effort: nextModel?.default_effort ?? "", fast: false });
  };
  const pickModel = (modelID: string) => {
    const next = backend?.models.find((item) => item.id === modelID);
    onChange({ ...value, backend: backend?.id ?? value.backend, model: modelID, effort: next?.default_effort ?? "", fast: false });
  };
  return <>
    <label className="phone-field">Backend<select value={backend?.id ?? value.backend} onChange={(event) => pickBackend(event.target.value)}><option value="">Choose…</option>{backends.map((item) => <option key={item.id} value={item.id}>{item.name || item.id}</option>)}</select></label>
    <label className="phone-field">Model<select value={model?.id ?? value.model} onChange={(event) => pickModel(event.target.value)}><option value="">Choose…</option>{(backend?.models ?? []).map((item) => <option key={item.id} value={item.id}>{item.name || item.id}</option>)}</select></label>
    {(model?.efforts.length ?? 0) > 0 && <label className="phone-field">Effort<select value={value.effort} onChange={(event) => onChange({ ...value, effort: event.target.value })}><option value="">Model default</option>{model!.efforts.map((effort) => <option key={effort} value={effort}>{effort}</option>)}</select></label>}
    {model?.fast && <label className="phone-check"><input type="checkbox" checked={value.fast} onChange={(event) => onChange({ ...value, fast: event.target.checked })} /> Fast mode</label>}
  </>;
}

function NewAgent({ project, onClose }: { project: string; onClose: () => void }) {
  const offline = useConnection((state) => state.link !== "connected");
  const roles = useQuery({ queryKey: ["roles"], queryFn: () => phoneFetch<Record<string, { title: string }>>("/api/roles") });
  const config = useQuery({ queryKey: ["config"], queryFn: () => phoneFetch<{ default_role?: string }>("/api/config") });
  const [role, setRole] = useState("");
  const [name, setName] = useSuggestedName(role);
  const [runtime, setRuntime] = useState({ backend: "", model: "", effort: "", fast: false });
  const allAgents = useConnection((state) => state.agents);
  const groups = useMemo(() => Object.values(allAgents).filter((agent) => !agent.archived).map((agent) => agent.group?.trim()).filter((label): label is string => !!label), [allAgents]);
  const [group, setGroup] = useState("");
  const [busy, setBusy] = useState(false); const [error, setError] = useState("");
  const roleID = role || Object.keys(roles.data ?? {}).filter((id) => id !== "chucky")[0] || "";
  useEffect(() => {
    if (role || !roles.data) return;
    const available = Object.keys(roles.data).filter((id) => id !== "chucky");
    setRole(config.data?.default_role && available.includes(config.data.default_role) ? config.data.default_role : available[0] ?? "");
  }, [config.data?.default_role, role, roles.data]);
  const submit = async () => { setBusy(true); setError(""); try { const result = await launchAgent({ project, role: roleID, name: name.trim() || undefined, group: group.trim() || undefined, ...runtime }); navigate(`/agent/${encodeURIComponent(result.agent.agent_id)}`); } catch (err) { setError(errorText(err)); } finally { setBusy(false); } };
  return <form className="phone-card phone-form" aria-label="New agent" onSubmit={(event) => { event.preventDefault(); void submit(); }}><h2>New agent</h2><label className="phone-field">Role<select value={roleID} onChange={(event) => setRole(event.target.value)}>{Object.keys(roles.data ?? {}).filter((id) => id !== "chucky").map((id) => <option key={id} value={id}>{roles.data?.[id]?.title || id}</option>)}</select></label><label className="phone-field">Name <input value={name} onChange={(event) => setName(event.target.value)} /></label><label className="phone-field">Group<GroupPicker id="phone-new-agent-group" value={group} groups={groups} onChange={setGroup} /></label><RuntimeFields value={runtime} onChange={setRuntime} />{error && <p className="phone-error">{error}</p>}<div className="phone-actions"><button type="button" onClick={onClose}>Cancel</button><button className="phone-primary" type="submit" disabled={offline || busy || !roleID}>Create agent</button></div></form>;
}

function StartPipeline({ project, onClose }: { project: string; onClose: () => void }) {
  const offline = useConnection((state) => state.link !== "connected"); const templates = useQuery({ queryKey: ["templates"], queryFn: listPipelineTemplates });
  const valid = (templates.data ?? []).filter((template) => template.valid); const [templateID, setTemplateID] = useState(""); const template = valid.find((item) => item.id === templateID) ?? valid[0];
  const [name, setName] = useState(""); const [goal, setGoal] = useState(""); const [inputs, setInputs] = useState<Record<string, string>>({}); const [busy, setBusy] = useState(false); const [error, setError] = useState(""); const [conflicts, setConflicts] = useState<{ name: string }[]>([]);
  const submit = async (acknowledge = false) => { if (!template) return; setBusy(true); setError(""); try { const result = await startPipelineRun({ request_id: crypto.randomUUID(), template_id: template.id, display_name: name.trim() || template.template.title, project, goal, inputs, orchestrator: { backend: "", model: "", effort: "", fast: false }, dedicated_assignments: {} }, acknowledge); navigate(`/run/${encodeURIComponent(result.run.run.run_id)}`); } catch (err) { const shared = sharedWorkspaceConflicts(err); if (shared.length) setConflicts(shared); else setError([errorText(err), ...pipelineDiagnostics(err).map((item) => item.message)].join(": ")); } finally { setBusy(false); } };
  if (!template) return <p className="phone-empty">No valid pipeline templates. Create one on the Mac.</p>;
  return <form className="phone-card phone-form" aria-label="Start pipeline" onSubmit={(event) => { event.preventDefault(); void submit(); }}><h2>Start pipeline</h2><label className="phone-field">Template<select value={template.id} onChange={(event) => setTemplateID(event.target.value)}>{valid.map((item) => <option key={item.id} value={item.id}>{item.template.title}</option>)}</select></label><label className="phone-field">Display name <input value={name} placeholder={template.template.title} onChange={(event) => setName(event.target.value)} /></label><label className="phone-field">Run goal<AutoGrowTextarea rows={3} value={goal} onChange={(event) => setGoal(event.target.value)} /></label>{template.template.inputs.map((input) => <label key={input.name} className="phone-field">{input.name}{input.required ? "" : " (optional)"}<input value={inputs[input.name] ?? ""} onChange={(event) => setInputs({ ...inputs, [input.name]: event.target.value })} /></label>)}{error && <p className="phone-error">{error}</p>}{conflicts.length > 0 && <div className="phone-card"><p>Other work is using this workspace: {conflicts.map((item) => item.name).join(", ")}.</p><button type="button" disabled={offline || busy} onClick={() => void submit(true)}>Start anyway</button></div>}<div className="phone-actions"><button type="button" onClick={onClose}>Cancel</button><button className="phone-primary" type="submit" disabled={offline || busy || !goal.trim()}>Start pipeline</button></div></form>;
}

export function ProjectScreen({ projectID }: { projectID: string }) {
  const agents = useConnection((state) => state.agents); const revision = useConnection((state) => state.revision); const [form, setForm] = useState<"agent" | "pipeline" | null>(null);
  const offline = useConnection((state) => state.link !== "connected");
  // Project configuration follows the live revision so desktop archival reaches an open screen (FS-20.R39).
  const projects = useQuery({ queryKey: ["projects", revision], queryFn: () => phoneFetch<Record<string, { title: string; color: [number, number, number]; archived?: boolean }>>("/api/projects"), placeholderData: (previous) => previous });
  const home = useQuery({ queryKey: ["home", revision], queryFn: getHome, placeholderData: (previous) => previous });
  const project = useMemo(() => deriveDashboardProjects(projects.data, agents).find((item) => item.id === projectID), [agents, projectID, projects.data]);
  const groupActions = useGroupActions({ project: projectID, projectTitle: project?.title, disabled: offline });
  const groups = useMemo(() => groupAgents(project?.agents ?? []), [project?.agents]);
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
  if (projects.data?.[projectID]?.archived) return <p className="phone-empty">Archived on the Mac. <button type="button" className="phone-link" onClick={() => navigate("/")}>Home</button></p>;
  if (!project) return <p className="phone-empty">{projects.data ? "This project is no longer on the Mac." : "Loading…"} <button type="button" className="phone-link" onClick={() => navigate("/")}>Home</button></p>;
  const runs = (home.data?.active_runs ?? []).filter((run) => run.project === projectID);
  return <div className="phone-project">
    <header className="phone-page-heading"><span className="phone-eyebrow">Project</span><h1>{project.title}</h1><p>{project.agents.length} agents · {project.stateSummary}</p></header>
    <div className="phone-actions"><button type="button" className="phone-primary" onClick={() => setForm("agent")}><PhoneIcon name="plus" size={16} />New agent</button><button type="button" onClick={() => setForm("pipeline")}><PhoneIcon name="pipeline" size={16} />Start pipeline</button></div>
    {form && <PhoneSheet title={form === "agent" ? "New agent" : "Start pipeline"} onClose={() => setForm(null)}>{form === "agent" ? <NewAgent project={projectID} onClose={() => setForm(null)} /> : <StartPipeline project={projectID} onClose={() => setForm(null)} />}</PhoneSheet>}
    {groupActions.feedback}
    {runs.length > 0 && <section className="phone-section" aria-label="Active runs"><div className="phone-section-title"><h2>Active pipelines</h2><span className="phone-count">{runs.length} runs</span></div><ul className="phone-list">{runs.map((run) => <li key={run.id}><button className="phone-row phone-run-card" type="button" onClick={() => navigate(`/run/${encodeURIComponent(run.id)}`)}><span className="phone-attention-meta"><span><PhoneIcon name="pipeline" size={16} />Pipeline</span><PhoneIcon name="arrow" size={17} /></span><span className="phone-row-title">{run.title}</span><span className="phone-row-reason">{run.reason}</span>{run.stage_number && run.stage_count ? <span className="phone-row-meta">Stage {run.stage_number} of {run.stage_count}</span> : null}</button></li>)}</ul></section>}
    <section className="phone-section" aria-label="Agents"><div className="phone-section-title"><h2>Agents</h2><span className="phone-count">{project.agents.length} agents</span></div>{project.agents.length === 0 ? <p className="phone-empty">No agents yet.</p> : <div className="phone-agent-groups">{groups.map((group) => { const isCollapsed = collapsed[group.key] ?? false; const running = group.agents.filter((agent) => agent.running).length; return <section className="phone-agent-group" key={group.key} aria-label={`${group.label} group`}><header className="phone-agent-group-header"><button type="button" aria-expanded={!isCollapsed} onClick={() => setCollapsed((value) => ({ ...value, [group.key]: !isCollapsed }))}><strong>{group.label}</strong><span>{group.agents.length} agents · {running} running</span></button>{group.key !== "_ungrouped" && <span className="phone-agent-group-actions"><button type="button" disabled={offline || groupActions.pending || running === 0} onClick={() => groupActions.open("stop", group.key, group.agents)}>Stop group</button><button type="button" disabled={offline || groupActions.pending} onClick={() => groupActions.open("archive", group.key, group.agents)}>Archive group</button></span>}</header>{!isCollapsed && <ul className="phone-list phone-agent-list">{group.agents.map((agent: AgentState) => <li key={agent.agent_id}><button type="button" data-tone={agent.running ? agent.state : "stopped"} className={`phone-row phone-agent-row${agent.running ? "" : " phone-row-stopped"}`} onClick={() => navigate(`/agent/${encodeURIComponent(agent.agent_id)}`)}><span className="phone-avatar"><PhoneIcon name="agent" /></span><span className="phone-row-text"><span className="phone-row-title">{agent.name || `${agent.role}@${project.id}`}</span><span className="phone-row-reason">{agent.detail || agent.role}</span><span className="phone-row-meta">{agent.role} · {agent.backend} · {agent.model}</span></span><span className="phone-agent-status"><span className="phone-status" data-tone={agent.running ? agent.state : "stopped"}>{agent.running ? agent.state.replace("_", " ") : "stopped"}</span><PhoneIcon name="arrow" size={16} /></span></button></li>)}</ul>}</section>; })}</div>}</section>
  </div>;
}
