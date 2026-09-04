import { pipelineProposalSchema, type PipelineListedProposal, type PipelineProposal, type PipelineTemplateRecord } from "../../schemas/pipeline";

// One draft must never push the template library or the run ledger off the
// surface, so every summarized value is clipped here rather than at each call
// site (FS-14.R51).
export const proposalSummaryLimit = 120;

export function clipProposalValue(value: string, limit = proposalSummaryLimit) {
  const chars = [...value];
  if (chars.length <= limit) return value;
  return chars.slice(0, limit - 1).join("") + "…";
}

export interface ProposalSummaryField {
  label: string;
  value: string;
}

// asPipelineProposal is the one place a listed record becomes a typed proposal.
// A payload that does not match its kind cannot be summarized or rendered for
// exact approval, but the record still lists with its kind and proposal id
// rather than disappearing (FS-14.R51).
export function asPipelineProposal(entry: PipelineListedProposal): PipelineProposal | null {
  const parsed = pipelineProposalSchema.safeParse(entry);
  return parsed.success ? parsed.data : null;
}

export function proposalKindLabel(kind: string) {
  if (kind === "save_template") return "Save template";
  if (kind === "start_run") return "Start run";
  return kind;
}

// summarizeProposal states what the offer asks for. A start proposal's durable
// payload names its template by id alone, so the title is resolved from that
// template as it stands now: a rename follows, and a template deleted since the
// proposal was made says so instead of hiding the offer (FS-14.R51).
export function summarizeProposal(
  proposal: PipelineProposal,
  templates: PipelineTemplateRecord[] | undefined,
): ProposalSummaryField[] {
  if (proposal.kind === "save_template") {
    const stages = proposal.payload.template.stages.length;
    return [
      { label: "Template", value: clipProposalValue(proposal.payload.template.title || proposal.payload.id) },
      { label: "Stages", value: `${stages} stage${stages === 1 ? "" : "s"}` },
    ];
  }
  return [
    { label: "Template", value: resolveTemplateTitle(proposal.payload.template_id, templates) },
    { label: "Run", value: clipProposalValue(proposal.payload.display_name) },
    { label: "Goal", value: clipProposalValue(proposal.payload.goal) },
  ];
}

function resolveTemplateTitle(templateID: string, templates: PipelineTemplateRecord[] | undefined) {
  // Until the library has loaded, the id is the honest answer: claiming the
  // template is gone before knowing would be worse than naming it plainly.
  if (!templates) return clipProposalValue(templateID);
  const record = templates.find((entry) => entry.id === templateID);
  if (!record) return `${clipProposalValue(templateID)} (template is gone)`;
  return clipProposalValue(record.template.title || record.id);
}
