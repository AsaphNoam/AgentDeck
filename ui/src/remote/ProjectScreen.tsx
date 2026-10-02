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
  const [busy, setBusy] = useState(false); const [error, setError] = useState("");
  const roleID = role || Object.keys(roles.data ?? {}).filter((id) => id !== "agentdecker")[0] || "";
  useEffect(() => {
    if (role || !roles.data) return;
    const available = Object.keys(roles.data).filter((id) => id !== "agentdecker");
    setRole(config.data?.default_role && available.includes(config.data.default_role) ? config.data.default_role : available[0] ?? "");
  }, [config.data?.default_role, role, roles.data]);
  const submit = async () => { setBusy(true); setError(""); try { const result = await launchAgent({ project, role: roleID, name: name.trim() || undefined, ...runtime }); navigate(`/agent/${encodeURIComponent(result.agent.agent_id)}`); } catch (err) { setError(errorText(err)); } finally { setBusy(false); } };
  return <form className="phone-card phone-form" aria-label="New agent" onSubmit={(event) => { event.preventDefault(); void submit(); }}><h2>New agent</h2><label className="phone-field">Role<select value={roleID} onChange={(event) => setRole(event.target.value)}>{Object.keys(roles.data ?? {}).filter((id) => id !== "agentdecker").map((id) => <option key={id} value={id}>{roles.data?.[id]?.title || id}</option>)}</select></label><label className="phone-field">Name <input value={name} onChange={(event) => setName(event.target.value)} /></label><RuntimeFields value={runtime} onChange={setRuntime} />{error && <p className="phone-error">{error}</p>}<div className="phone-actions"><button type="button" onClick={onClose}>Cancel</button><button className="phone-primary" type="submit" disabled={offline || busy || !roleID}>Create agent</button></div></form>;
}

function StartPipeline({ project, onClose }: { project: string; onClose: () => void }) {
  const offline = useConnection((state) => state.link !== "connected"); const templates = useQuery({ queryKey: ["templates"], queryFn: listPipelineTemplates });
  const valid = (templates.data ?? []).filter((template) => template.valid); const [templateID, setTemplateID] = useState(""); const template = valid.find((item) => item.id === templateID) ?? valid[0];
  const [name, setName] = useState(""); const [goal, setGoal] = useState(""); const [inputs, setInputs] = useState<Record<string, string>>({}); const [busy, setBusy] = useState(false); const [error, setError] = useState(""); const [conflicts, setConflicts] = useState<{ name: string }[]>([]);
  const submit = async (acknowledge = false) => { if (!template) return; setBusy(true); setError(""); try { const result = await startPipelineRun({ request_id: crypto.randomUUID(), template_id: template.id, display_name: name.trim() || template.template.title, project, goal, inputs, orchestrator: { backend: "", model: "", effort: "", fast: false }, dedicated_assignments: {} }, acknowledge); navigate(`/run/${encodeURIComponent(result.run.run.run_id)}`); } catch (err) { const shared = sharedWorkspaceConflicts(err); if (shared.length) setConflicts(shared); else setError([errorText(err), ...pipelineDiagnostics(err).map((item) => item.message)].join(": ")); } finally { setBusy(false); } };
  if (!template) return <p className="phone-empty">No valid pipeline templates. Create one on the Mac.</p>;
  return <form className="phone-card phone-form" aria-label="Start pipeline" onSubmit={(event) => { event.preventDefault(); void submit(); }}><h2>Start pipeline</h2><label className="phone-field">Template<select value={template.id} onChange={(event) => setTemplateID(event.target.value)}>{valid.map((item) => <option key={item.id} value={item.id}>{item.template.title}</option>)}</select></label><label className="phone-field">Display name <input value={name} placeholder={template.template.title} onChange={(event) => setName(event.target.value)} /></label><label className="phone-field">Run goal<textarea rows={3} value={goal} onChange={(event) => setGoal(event.target.value)} /></label>{template.template.inputs.map((input) => <label key={input.name} className="phone-field">{input.name}{input.required ? "" : " (optional)"}<input value={inputs[input.name] ?? ""} onChange={(event) => setInputs({ ...inputs, [input.name]: event.target.value })} /></label>)}{error && <p className="phone-error">{error}</p>}{conflicts.length > 0 && <div className="phone-card"><p>Other work is using this workspace: {conflicts.map((item) => item.name).join(", ")}.</p><button type="button" disabled={offline || busy} onClick={() => void submit(true)}>Start anyway</button></div>}<div className="phone-actions"><button type="button" onClick={onClose}>Cancel</button><button className="phone-primary" type="submit" disabled={offline || busy || !goal.trim()}>Start pipeline</button></div></form>;
}

export function ProjectScreen({ projectID }: { projectID: string }) {
  const agents = useConnection((state) => state.agents); const revision = useConnection((state) => state.revision); const [form, setForm] = useState<"agent" | "pipeline" | null>(null);
  // Project configuration follows the live revision so desktop archival reaches an open screen (FS-20.R39).
  const projects = useQuery({ queryKey: ["projects", revision], queryFn: () => phoneFetch<Record<string, { title: string; color: [number, number, number]; archived?: boolean }>>("/api/projects"), placeholderData: (previous) => previous });
  const home = useQuery({ queryKey: ["home", revision], queryFn: getHome, placeholderData: (previous) => previous });
  const project = useMemo(() => deriveDashboardProjects(projects.data, agents).find((item) => item.id === projectID), [agents, projectID, projects.data]);
  if (projects.data?.[projectID]?.archived) return <p className="phone-empty">Archived on the Mac. <button type="button" className="phone-link" onClick={() => navigate("/")}>Home</button></p>;
  if (!project) return <p className="phone-empty">{projects.data ? "This project is no longer on the Mac." : "Loading…"} <button type="button" className="phone-link" onClick={() => navigate("/")}>Home</button></p>;
  const runs = (home.data?.active_runs ?? []).filter((run) => run.project === projectID);
  return <div className="phone-project"><header className="phone-agent-header"><h1>{project.title}</h1><p className="phone-meta">{project.agents.length} agents · {project.stateSummary}</p></header><div className="phone-actions"><button type="button" className="phone-primary" onClick={() => setForm("agent")}>New agent</button><button type="button" onClick={() => setForm("pipeline")}>Start pipeline</button></div>{form === "agent" && <NewAgent project={projectID} onClose={() => setForm(null)} />}{form === "pipeline" && <StartPipeline project={projectID} onClose={() => setForm(null)} />}{runs.length > 0 && <section className="phone-section" aria-label="Active runs"><h2>Active runs</h2><ul className="phone-list">{runs.map((run) => <li key={run.id}><button className="phone-row" type="button" onClick={() => navigate(`/run/${encodeURIComponent(run.id)}`)}><span className="phone-row-title">{run.title}</span><span className="phone-row-reason">{run.reason}</span></button></li>)}</ul></section>}<section className="phone-section" aria-label="Agents"><h2>Agents</h2>{project.agents.length === 0 ? <p className="phone-empty">No agents yet.</p> : <ul className="phone-list">{project.agents.map((agent: AgentState) => <li key={agent.agent_id}><button type="button" className={`phone-row${agent.running ? "" : " phone-row-stopped"}`} onClick={() => navigate(`/agent/${encodeURIComponent(agent.agent_id)}`)}><span className="phone-row-title">{agent.name || `${agent.role}@${project.id}`}</span><span className="phone-row-reason">{agent.role} · {agent.backend} · {agent.model} · {agent.running ? agent.state.replace("_", " ") : "stopped"}</span><span className="phone-row-meta">{agent.detail}</span></button></li>)}</ul>}</section></div>;
}
