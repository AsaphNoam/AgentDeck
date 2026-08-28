import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { useRoles } from "../../api/config";
import {
  pipelineDiagnostics,
  useDeletePipelineTemplate,
  usePipelineTemplates,
  useSavePipelineTemplate,
  validatePipelineTemplate,
} from "../../api/pipelines";
import type {
  PipelineDiagnostic,
  PipelineProposal,
  PipelineTemplate,
} from "../../schemas/pipeline";

export interface TemplateEditorSeed {
  id: string;
  template: PipelineTemplate;
  proposal?: Extract<PipelineProposal, { kind: "save_template" }>;
}

const blankTransition = () => ({ stage: "", final: "success", approval: "automatic" as const });

const blankTemplate = (): PipelineTemplate => ({
  version: 1,
  title: "",
  inputs: [],
  stages: [{
    id: "work",
    title: "Work",
    role: "implementer",
    instruction: "",
    inputs: [],
    outputs: [],
    max_visits: 1,
    transitions: { success: blankTransition(), failure: { ...blankTransition(), final: "failure" } },
  }],
});

function copyTemplate(template: PipelineTemplate): PipelineTemplate {
  return structuredClone(template);
}

export function TemplateEditor({
  seed,
  createNew: createNewRoute = false,
  onSaved,
  onDeleted,
}: {
  seed?: TemplateEditorSeed | null;
  createNew?: boolean;
  onSaved?: (id: string) => void;
  onDeleted?: () => void;
}) {
  const templates = usePipelineTemplates();
  const roles = useRoles();
  const save = useSavePipelineTemplate();
  const remove = useDeletePipelineTemplate();
  const [id, setID] = useState("");
  const [draft, setDraft] = useState<PipelineTemplate>(blankTemplate);
  const [savedID, setSavedID] = useState<string | null>(null);
  const [proposal, setProposal] = useState<TemplateEditorSeed["proposal"]>();
  const [diagnostics, setDiagnostics] = useState<PipelineDiagnostic[]>([]);
  const [notice, setNotice] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [selectedStage, setSelectedStage] = useState(0);

  const applyDiagnostics = (items: PipelineDiagnostic[]) => {
    setDiagnostics(items);
    const field = items[0]?.field ?? "";
    const match = /stages(?:\.|\[)(\d+)/.exec(field);
    if (match) setSelectedStage(Math.min(Number(match[1]), draft.stages.length - 1));
  };

  useEffect(() => {
    if (!seed) return;
    setID(seed.id);
    setDraft(copyTemplate(seed.template));
    setSavedID(templates.data?.some((entry) => entry.id === seed.id) ? seed.id : null);
    setProposal(seed.proposal);
    setDiagnostics([]);
    setNotice(seed.proposal ? "Review the exact AgentDecker proposal below before confirming Save." : null);
    setError(null);
    setSelectedStage(0);
  }, [seed]);

  useEffect(() => {
    if (!createNewRoute || seed) return;
    setID("");
    setDraft(blankTemplate());
    setSavedID(null);
    setProposal(undefined);
    setDiagnostics([]);
    setNotice(null);
    setError(null);
    setSelectedStage(0);
  }, [createNewRoute, seed]);

  const mutate = (fn: (next: PipelineTemplate) => void) => {
    setDraft((current) => {
      const next = copyTemplate(current);
      fn(next);
      return next;
    });
    if (proposal) {
      setProposal(undefined);
      setNotice("The proposal changed. Its confirmation was invalidated; this is now a manual draft.");
    }
    setDiagnostics([]);
  };

  const validate = async () => {
    setError(null);
    try {
      const record = await validatePipelineTemplate(id, draft);
      applyDiagnostics(record.diagnostics);
      setNotice(record.valid ? "Template is valid." : null);
    } catch (reason) {
      applyDiagnostics(pipelineDiagnostics(reason));
      setError(reason instanceof Error ? reason.message : String(reason));
    }
  };

  const saveDraft = () => {
    setError(null);
    save.mutate(
      { id, template: draft, exists: templates.data?.some((entry) => entry.id === id) ?? false },
      {
        onSuccess: (record) => {
          setSavedID(record.id);
          setDraft(copyTemplate(record.template));
          applyDiagnostics(record.diagnostics);
          setNotice(proposal ? "Exact AgentDecker proposal saved." : "Template saved.");
          setProposal(undefined);
          onSaved?.(record.id);
        },
        onError: (reason) => {
          applyDiagnostics(pipelineDiagnostics(reason));
          setError(reason instanceof Error ? reason.message : String(reason));
        },
      },
    );
  };

  const deleteDraft = () => {
    if (!savedID) return;
    remove.mutate(savedID, {
      onSuccess: onDeleted,
      onError: (reason) => setError(reason instanceof Error ? reason.message : String(reason)),
    });
  };

  return (
    <section className="pipeline-template-editor" data-ui="pipeline-template-editor">
      <Link className="pipeline-back-link" to="/pipelines/templates">← All templates</Link>
      <div className="pipeline-editor-heading">
        <div>
          <p className="pipeline-eyebrow">Reusable definition · version 1</p>
          <h2>{draft.title || (savedID ? id : "New template")}</h2>
          <p>Shape one stage at a time. Runtime choices remain outside the reusable definition.</p>
        </div>
      </div>

      <div className="pipeline-editor-basics">
        <label className="form-field">
          <span>Template id</span>
          <input
            value={id}
            disabled={Boolean(savedID)}
            placeholder="delivery-check"
            onChange={(event) => { setID(event.target.value); setProposal(undefined); }}
          />
        </label>
        <label className="form-field">
          <span>Title</span>
          <input value={draft.title} onChange={(event) => mutate((next) => { next.title = event.target.value; })} />
        </label>
      </div>

      <details className="pipeline-disclosure pipeline-editor-inputs" data-slot="inputs">
        <summary>Run inputs <span>{draft.inputs.length}</span></summary>
        <div className="pipeline-disclosure-body">
        <div className="pipeline-subsection-header">
          <p>Named values collected before a run can start.</p>
          <button type="button" onClick={() => mutate((next) => next.inputs.push({ name: "", description: "", required: true }))}>Add input</button>
        </div>
        {draft.inputs.length === 0 && <p className="pipeline-empty">No run inputs.</p>}
        {draft.inputs.map((input, index) => (
          <div className="pipeline-inline-row" key={`input-${index}`}>
            <input aria-label={`Input ${index + 1} name`} placeholder="spec" value={input.name} onChange={(event) => mutate((next) => { next.inputs[index].name = event.target.value; })} />
            <input aria-label={`Input ${index + 1} description`} placeholder="Description" value={input.description} onChange={(event) => mutate((next) => { next.inputs[index].description = event.target.value; })} />
            <label className="pipeline-check"><input type="checkbox" checked={input.required} onChange={(event) => mutate((next) => { next.inputs[index].required = event.target.checked; })} /> Required</label>
            <button type="button" onClick={() => mutate((next) => { next.inputs.splice(index, 1); })}>Remove</button>
          </div>
        ))}
        </div>
      </details>

      <div className="pipeline-editor-workspace">
        <aside className="pipeline-stage-navigator" data-slot="navigator">
          <div className="pipeline-stage-nav-heading"><div><p className="pipeline-eyebrow">Flow order</p><h3>Stages</h3></div>
          <button type="button" onClick={() => mutate((next) => next.stages.push({
            id: `stage-${next.stages.length + 1}`,
            title: "",
            role: roles.data?.implementer ? "implementer" : Object.keys(roles.data ?? {})[0] ?? "implementer",
            instruction: "",
            inputs: [],
            outputs: [],
            max_visits: 1,
            transitions: { success: blankTransition(), failure: { ...blankTransition(), final: "failure" } },
          }))}>+</button></div>
          <ol>{draft.stages.map((stage, stageIndex) => <li key={`nav-${stageIndex}`}><button type="button" className={selectedStage === stageIndex ? "pipeline-stage-nav-item pipeline-stage-nav-item-active" : "pipeline-stage-nav-item"} onClick={() => setSelectedStage(stageIndex)}><span>{String(stageIndex + 1).padStart(2, "0")}</span><span><strong>{stage.title || stage.id || "Untitled stage"}</strong><small>{stage.role || "No role"}</small></span>{diagnostics.some((item) => item.field.includes(`stages.${stageIndex}`) || item.field.includes(`stages[${stageIndex}]`)) && <em>!</em>}</button></li>)}</ol>
        </aside>
        <div className="pipeline-stage-focus" data-slot="stage">
          {draft.stages.map((stage, stageIndex) => stageIndex === selectedStage && (
            <article className="pipeline-stage-card" key={`stage-${stageIndex}`}>
              <div className="pipeline-stage-heading">
                <span className="pipeline-stage-number">{stageIndex + 1}</span>
                <strong>{stage.title || stage.id || "Untitled stage"}</strong>
                <button type="button" disabled={draft.stages.length === 1} onClick={() => { mutate((next) => { next.stages.splice(stageIndex, 1); }); setSelectedStage(Math.max(0, stageIndex - 1)); }}>Remove stage</button>
              </div>
              <div className="pipeline-form-grid pipeline-form-grid-three">
                <label className="form-field"><span>Stage id</span><input value={stage.id} onChange={(event) => mutate((next) => { next.stages[stageIndex].id = event.target.value; })} /></label>
                <label className="form-field"><span>Title</span><input value={stage.title} onChange={(event) => mutate((next) => { next.stages[stageIndex].title = event.target.value; })} /></label>
                <label className="form-field"><span>Role</span><select value={stage.role} onChange={(event) => mutate((next) => { next.stages[stageIndex].role = event.target.value; })}>
                  {Object.entries(roles.data ?? {}).map(([roleID, role]) => <option key={roleID} value={roleID}>{role.title} ({roleID})</option>)}
                  {!roles.data?.[stage.role] && <option value={stage.role}>{stage.role || "Select role"}</option>}
                </select></label>
              </div>
              <label className="form-field"><span>Instruction</span><textarea rows={4} value={stage.instruction} onChange={(event) => mutate((next) => { next.stages[stageIndex].instruction = event.target.value; })} /></label>
              <label className="form-field pipeline-small-field"><span>Maximum visits</span><input type="number" min={1} value={stage.max_visits} onChange={(event) => mutate((next) => { next.stages[stageIndex].max_visits = Number(event.target.value); })} /></label>

              <StageBindings
                kind="inputs"
                stageIndex={stageIndex}
                items={stage.inputs}
                mutate={mutate}
              />
              <StageBindings
                kind="outputs"
                stageIndex={stageIndex}
                items={stage.outputs}
                mutate={mutate}
              />
              <div className="pipeline-transitions">
                {(["success", "failure"] as const).map((outcome) => {
                  const transition = stage.transitions[outcome];
                  const destinationType = transition.stage ? "stage" : "final";
                  return (
                    <fieldset key={outcome}>
                      <legend>{outcome} route</legend>
                      <select aria-label={`${stage.id} ${outcome} destination type`} value={destinationType} onChange={(event) => mutate((next) => {
                        next.stages[stageIndex].transitions[outcome] = event.target.value === "stage"
                          ? { stage: next.stages[stageIndex].id, final: "", approval: transition.approval }
                          : { stage: "", final: outcome, approval: transition.approval };
                      })}>
                        <option value="stage">Stage</option>
                        <option value="final">Final outcome</option>
                      </select>
                      {destinationType === "stage" ? (
                        <select aria-label={`${stage.id} ${outcome} stage`} value={transition.stage} onChange={(event) => mutate((next) => { next.stages[stageIndex].transitions[outcome].stage = event.target.value; })}>
                          <option value="">Select stage</option>
                          {draft.stages.map((candidate) => <option key={candidate.id} value={candidate.id}>{candidate.title || candidate.id}</option>)}
                        </select>
                      ) : (
                        <select aria-label={`${stage.id} ${outcome} final`} value={transition.final} onChange={(event) => mutate((next) => { next.stages[stageIndex].transitions[outcome].final = event.target.value; })}>
                          <option value="success">Success</option>
                          <option value="failure">Failure</option>
                        </select>
                      )}
                      <select aria-label={`${stage.id} ${outcome} approval`} value={transition.approval} onChange={(event) => mutate((next) => { next.stages[stageIndex].transitions[outcome].approval = event.target.value as "automatic" | "required"; })}>
                        <option value="automatic">Automatic</option>
                        <option value="required">Approval required</option>
                      </select>
                    </fieldset>
                  );
                })}
              </div>
            </article>
          ))}
        </div>
      </div>

      {diagnostics.length > 0 && (
        <ul className="pipeline-diagnostics">
          {diagnostics.map((diagnostic, index) => <li key={`${diagnostic.field}-${diagnostic.code}-${index}`}><code>{diagnostic.field}</code> — {diagnostic.message}</li>)}
        </ul>
      )}
      {notice && <p className="form-info">{notice}</p>}
      {error && <p className="form-error">{error}</p>}
      {proposal && <pre className="pipeline-proposal-payload">{JSON.stringify(proposal.payload, null, 2)}</pre>}
      <div className="form-actions pipeline-editor-actions" data-slot="actions">
        {savedID && <button type="button" className="btn-danger" disabled={remove.isPending} onClick={deleteDraft}>Delete</button>}
        <button type="button" disabled={!id || save.isPending} onClick={() => void validate()}>Validate</button>
        <button type="button" disabled={!id || save.isPending} onClick={saveDraft}>
          {save.isPending ? "Saving…" : proposal ? "Confirm and save exact proposal" : "Save template"}
        </button>
      </div>
    </section>
  );
}

type StageInput = PipelineTemplate["stages"][number]["inputs"][number];
type StageOutput = PipelineTemplate["stages"][number]["outputs"][number];

function StageBindings({
  kind,
  stageIndex,
  items,
  mutate,
}: {
  kind: "inputs" | "outputs";
  stageIndex: number;
  items: StageInput[] | StageOutput[];
  mutate: (fn: (next: PipelineTemplate) => void) => void;
}) {
  return (
    <div className="pipeline-bindings">
      <div className="pipeline-subsection-header">
        <h4>Stage {kind}</h4>
        <button type="button" onClick={() => mutate((next) => {
          if (kind === "inputs") next.stages[stageIndex].inputs.push({ name: "", value: "", required: true });
          else next.stages[stageIndex].outputs.push({ name: "", value: "", description: "" });
        })}>Add {kind === "inputs" ? "input" : "output"}</button>
      </div>
      {items.map((item, index) => (
        <div className="pipeline-inline-row" key={`${kind}-${index}`}>
          <input aria-label={`${kind} ${index + 1} local name`} placeholder="Local name" value={item.name} onChange={(event) => mutate((next) => { next.stages[stageIndex][kind][index].name = event.target.value; })} />
          <input aria-label={`${kind} ${index + 1} value`} placeholder="Run value" value={item.value} onChange={(event) => mutate((next) => { next.stages[stageIndex][kind][index].value = event.target.value; })} />
          {kind === "inputs" ? (
            <label className="pipeline-check"><input type="checkbox" checked={(item as StageInput).required} onChange={(event) => mutate((next) => { next.stages[stageIndex].inputs[index].required = event.target.checked; })} /> Required</label>
          ) : (
            <input aria-label={`${kind} ${index + 1} description`} placeholder="Description" value={(item as StageOutput).description} onChange={(event) => mutate((next) => { next.stages[stageIndex].outputs[index].description = event.target.value; })} />
          )}
          <button type="button" onClick={() => mutate((next) => { next.stages[stageIndex][kind].splice(index, 1); })}>Remove</button>
        </div>
      ))}
    </div>
  );
}
