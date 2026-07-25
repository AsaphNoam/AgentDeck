import { useState } from "react";
import { useSearchParams } from "react-router-dom";
import { PageHeader } from "../../components/ui";
import type { PipelineProposal } from "../../schemas/pipeline";
import { AgentDeckerBuilder } from "./AgentDeckerBuilder";
import { RunBrowser } from "./RunBrowser";
import { RunStartForm } from "./RunStartForm";
import { TemplateEditor, type TemplateEditorSeed } from "./TemplateEditor";

export function PipelinesPage() {
  const [search, setSearch] = useSearchParams();
  const [templateSeed, setTemplateSeed] = useState<TemplateEditorSeed | null>(null);
  const [runProposal, setRunProposal] = useState<Extract<PipelineProposal, { kind: "start_run" }> | null>(null);
  const selectedRun = search.get("run");

  const selectRun = (runID: string | null) => {
    if (runID) setSearch({ run: runID });
    else setSearch({});
  };

  return <div className="pipelines-page">
    <PageHeader
      eyebrow="Sequential orchestration"
      title="Pipelines"
      description="Define model-neutral stages, choose each runtime when a run starts, and keep every agent and transcript in AgentDeck."
    />
    <AgentDeckerBuilder
      onTemplateProposal={(proposal) => {
        setTemplateSeed({ id: proposal.payload.id, template: proposal.payload.template, proposal });
        document.querySelector(".pipeline-template-editor")?.scrollIntoView({ behavior: "smooth", block: "start" });
      }}
      onRunProposal={(proposal) => {
        setRunProposal(proposal);
        document.querySelector(".pipeline-run-start")?.scrollIntoView({ behavior: "smooth", block: "start" });
      }}
    />
    <div className="pipeline-authoring-grid">
      <TemplateEditor seed={templateSeed} />
      <RunStartForm proposal={runProposal} onStarted={selectRun} />
    </div>
    <RunBrowser selectedID={selectedRun} onSelect={selectRun} />
  </div>;
}
