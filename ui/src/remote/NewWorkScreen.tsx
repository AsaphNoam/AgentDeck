import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { launchAgent, sendPrompt } from "../api/client";
import { listPipelineTemplates, pipelineDiagnostics, sharedWorkspaceConflicts, startPipelineRun } from "../api/pipelines";
import { phoneFetch } from "./api";
import { useConnection } from "./connection";
import { navigate } from "./router";

type Mode = "ask" | "task" | "pipeline";
const RESIDENT_ROLE = "agentdecker";
const errorText = (error: unknown) => (error instanceof Error ? error.message : String(error));

/** New work starts only against projects, roles, and templates already
 *  configured on the desktop, with the runtime the desktop would preselect —
 *  the phone omits runtime fields and the Mac resolves its defaults (FS-20.R15);
 *  for a pipeline run the tailnet filter fills every empty assignment. */
export function NewWorkScreen() {
  const offline = useConnection((state) => state.link !== "connected");
  const agents = useConnection((state) => state.agents);
  const projects = useQuery({
    queryKey: ["projects"],
    queryFn: () => phoneFetch<Record<string, { title: string; archived?: boolean }>>("/api/projects"),
  });
  const roles = useQuery({ queryKey: ["roles"], queryFn: () => phoneFetch<Record<string, { title: string }>>("/api/roles") });
  const templates = useQuery({ queryKey: ["templates"], queryFn: listPipelineTemplates });

  const active = useMemo(
    () => Object.entries(projects.data ?? {}).filter(([, p]) => !p.archived).sort(([a], [b]) => a.localeCompare(b)),
    [projects.data],
  );
  const [project, setProject] = useState("");
  const [mode, setMode] = useState<Mode>("ask");
  const [text, setText] = useState("");
  const [name, setName] = useState("");
  const [role, setRole] = useState("");
  const [templateId, setTemplateId] = useState("");
  const [inputs, setInputs] = useState<Record<string, string>>({});
  const [conflicts, setConflicts] = useState<{ name: string }[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const chosenProject = project || active[0]?.[0] || "";
  const roleIds = Object.keys(roles.data ?? {}).filter((id) => id !== RESIDENT_ROLE).sort();
  const chosenRole = role || roleIds[0] || "";
  const validTemplates = (templates.data ?? []).filter((t) => t.valid);
  const template = validTemplates.find((t) => t.id === templateId) ?? validTemplates[0];

  const submit = async (acknowledge = false) => {
    setBusy(true);
    setError(null);
    try {
      if (mode === "ask") {
        const resident = Object.values(agents).find(
          (a) => a.project === chosenProject && a.role === RESIDENT_ROLE && a.interface === "chat" && a.running && !a.archived,
        );
        const agentId = resident?.agent_id ?? (await launchAgent({ role: RESIDENT_ROLE, project: chosenProject })).agent.agent_id;
        await sendPrompt(agentId, text);
        navigate(`/agent/${encodeURIComponent(agentId)}`);
      } else if (mode === "task") {
        const created = await phoneFetch<{ task_id: string }>("/api/tasks", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ project: chosenProject, display_name: name.trim(), instruction: text, target_kind: "launch", role: chosenRole }),
        });
        navigate(`/task/${encodeURIComponent(created.task_id)}`);
      } else if (template) {
        const started = await startPipelineRun(
          {
            request_id: crypto.randomUUID(),
            template_id: template.id,
            display_name: name.trim() || template.template.title,
            project: chosenProject,
            goal: text,
            inputs,
            orchestrator: { backend: "", model: "", effort: "", fast: false },
            dedicated_assignments: {},
          },
          acknowledge,
        );
        navigate(`/run/${encodeURIComponent(started.run.run.run_id)}`);
      }
    } catch (err) {
      const shared = sharedWorkspaceConflicts(err);
      if (shared.length > 0) setConflicts(shared);
      else {
        const reasons = pipelineDiagnostics(err).map((diagnostic) => diagnostic.message);
        setError(reasons.length > 0 ? `${errorText(err)}: ${reasons.join("; ")}` : errorText(err));
      }
    } finally {
      setBusy(false);
    }
  };

  const missingInput = mode === "pipeline" && (template?.template.inputs ?? []).some((decl) => decl.required && !inputs[decl.name]?.trim());
  const ready =
    !!chosenProject &&
    !offline &&
    !busy &&
    (mode === "ask" ? !!text.trim() : mode === "task" ? !!text.trim() && !!name.trim() && !!chosenRole : !!template && !!text.trim() && !missingInput);

  if (projects.isLoading) return <p className="phone-empty">Loading…</p>;
  if (active.length === 0) return <p className="phone-empty">There are no active projects. Create one in AgentDeck on your Mac.</p>;

  return (
    <form
      className="phone-agent phone-form"
      onSubmit={(event) => {
        event.preventDefault();
        void submit();
      }}
    >
      <h1>New work</h1>
      <label className="phone-field">
        Project
        <select value={chosenProject} onChange={(event) => setProject(event.target.value)}>
          {active.map(([id, p]) => (
            <option key={id} value={id}>
              {p.title || id}
            </option>
          ))}
        </select>
      </label>
      <div className="phone-actions" role="radiogroup" aria-label="Kind of work">
        {(
          [
            ["ask", "Ask AgentDecker"],
            ["task", "New task"],
            ["pipeline", "Start pipeline"],
          ] as const
        ).map(([value, label]) => (
          <button
            key={value}
            type="button"
            role="radio"
            aria-checked={mode === value}
            className={mode === value ? "phone-primary" : undefined}
            onClick={() => {
              setMode(value);
              setConflicts([]);
              setError(null);
            }}
          >
            {label}
          </button>
        ))}
      </div>
      {mode === "task" && (
        <>
          <label className="phone-field">
            Task name
            <input maxLength={120} value={name} onChange={(event) => setName(event.target.value)} />
          </label>
          <label className="phone-field">
            Role
            <select value={chosenRole} onChange={(event) => setRole(event.target.value)}>
              {roleIds.map((id) => (
                <option key={id} value={id}>
                  {roles.data?.[id]?.title || id}
                </option>
              ))}
            </select>
          </label>
        </>
      )}
      {mode === "pipeline" &&
        (template ? (
          <>
            <label className="phone-field">
              Template
              <select value={template.id} onChange={(event) => setTemplateId(event.target.value)}>
                {validTemplates.map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.template.title}
                  </option>
                ))}
              </select>
            </label>
            <label className="phone-field">
              Run name
              <input maxLength={120} placeholder={template.template.title} value={name} onChange={(event) => setName(event.target.value)} />
            </label>
            {template.template.inputs.map((decl) => (
              <label key={decl.name} className="phone-field">
                {decl.name}
                {decl.required ? "" : " (optional)"}
                <input
                  aria-description={decl.description}
                  value={inputs[decl.name] ?? ""}
                  onChange={(event) => setInputs({ ...inputs, [decl.name]: event.target.value })}
                />
              </label>
            ))}
          </>
        ) : (
          <p className="phone-empty">No valid pipeline templates. Create one in AgentDeck on your Mac.</p>
        ))}
      <label className="phone-field">
        {mode === "pipeline" ? "Run goal" : "Instruction"}
        <textarea rows={4} value={text} onChange={(event) => setText(event.target.value)} />
      </label>
      {error && <p className="phone-error">{error}</p>}
      {conflicts.length > 0 && (
        <div className="phone-card">
          <p>Other work is already using this project's workspace: {conflicts.map((c) => c.name).join(", ")}.</p>
          <button type="button" disabled={offline || busy} onClick={() => void submit(true)}>
            Start anyway
          </button>
        </div>
      )}
      <button type="submit" className="phone-primary" disabled={!ready}>
        {mode === "ask" ? "Send to AgentDecker" : mode === "task" ? "Create task" : "Start run"}
      </button>
    </form>
  );
}
