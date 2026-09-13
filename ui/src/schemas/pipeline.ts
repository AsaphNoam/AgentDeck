import { z } from "zod";

export const pipelineDiagnosticSchema = z.object({
  field: z.string(),
  code: z.string(),
  message: z.string(),
});

export const pipelineValueDeclSchema = z.object({
  name: z.string(),
  description: z.string(),
  required: z.boolean(),
});

export const pipelineStageInputSchema = z.object({
  name: z.string(),
  value: z.string(),
  required: z.boolean(),
});

export const pipelineStageOutputSchema = z.object({
  name: z.string(),
  value: z.string(),
  description: z.string(),
});

export const pipelineStageSchema = z.object({
  id: z.string(),
  title: z.string(),
  objective: z.string(),
  inputs: z.array(pipelineStageInputSchema),
  outputs: z.array(pipelineStageOutputSchema),
  coordination: z.enum(["standing", "dedicated"]).optional().default("standing"),
  dedicated_role: z.string().optional().default(""),
  approval_after_success: z.boolean().optional().default(false),
});

export const pipelineTemplateSchema = z.object({
  version: z.literal(2),
  title: z.string(),
  orchestrator_role: z.string(),
  inputs: z.array(pipelineValueDeclSchema),
  stages: z.array(pipelineStageSchema),
});

export const pipelineTemplateRecordSchema = z.object({
  id: z.string(),
  template: pipelineTemplateSchema,
  valid: z.boolean(),
  diagnostics: z.array(pipelineDiagnosticSchema),
});

export const pipelineRuntimeAssignmentSchema = z.object({
  backend: z.string(),
  model: z.string(),
  effort: z.string().optional().default(""),
  fast: z.boolean().optional().default(false),
});

export const pipelineStartRequestSchema = z.object({
  request_id: z.string(),
  template_id: z.string(),
  display_name: z.string(),
  project: z.string(),
  goal: z.string(),
  inputs: z.record(z.string()),
  orchestrator: pipelineRuntimeAssignmentSchema,
  dedicated_assignments: z.record(pipelineRuntimeAssignmentSchema),
});

export const pipelineRunSchema = z.object({
  run_id: z.string(),
  template_id: z.string(),
  template_snapshot: pipelineTemplateSchema,
  display_name: z.string(),
  project: z.string(),
  goal: z.string(),
  inputs: z.record(z.string()),
  orchestrator: pipelineRuntimeAssignmentSchema.optional().default({ backend: "", model: "", effort: "", fast: false }),
  dedicated_assignments: z.record(pipelineRuntimeAssignmentSchema).optional().default({}),
  state: z.string(),
  revision: z.number().int(),
  pending_action: z.string(),
  current_stage_id: z.string(),
  current_task_id: z.string().optional().default(""),
  orchestrator_agent_id: z.string().optional().default(""),
  current_attempt_id: z.string(),
  current_agent_id: z.string(),
  attention_reason: z.string(),
  final_outcome: z.string(),
  created_at: z.string(),
  updated_at: z.string(),
});

export const pipelineTaskWorkSchema: z.ZodType<{
  task_id: string; display_name: string; state: string; outcome?: string; summary?: string; agent_id?: string; agent_name?: string; route?: "live" | "archive" | "unavailable"; children?: unknown[];
}> = z.object({
  task_id: z.string(),
  display_name: z.string(),
  state: z.string(),
  outcome: z.string().optional().default(""),
  summary: z.string().optional().default(""),
  agent_id: z.string().optional().default(""),
  agent_name: z.string().optional().default(""),
  route: z.enum(["live", "archive", "unavailable"]).optional().default("unavailable"),
  children: z.array(z.lazy(() => pipelineTaskWorkSchema)).optional().default([]),
});

export const pipelineStageTaskSchema = z.object({
  task_id: z.string(),
  run_id: z.string(),
  stage_id: z.string(),
  stage_index: z.number().int(),
  attempt_number: z.number().int(),
  state: z.string(),
  assignment_text: z.string(),
  standing_owner: z.object({
    agent_id: z.string().optional().default(""),
    name: z.string().optional().default(""),
    state: z.string().optional().default(""),
    route: z.enum(["live", "archive", "unavailable"]).optional().default("unavailable"),
    runtime: pipelineRuntimeAssignmentSchema.optional(),
  }),
  coordinator: z.object({
    task_id: z.string().optional().default(""),
    agent_id: z.string().optional().default(""),
    name: z.string().optional().default(""),
    state: z.string().optional().default(""),
    route: z.enum(["live", "archive", "unavailable"]).optional().default("unavailable"),
    report_summary: z.string().optional().default(""),
    runtime: pipelineRuntimeAssignmentSchema.optional(),
  }).optional(),
  result: z.object({
    outcome: z.string().optional().default(""),
    summary: z.string().optional().default(""),
    details: z.string().optional().default(""),
    checks: z.string().optional().default(""),
    outputs: z.record(z.string()).optional().default({}),
  }).optional(),
  work: z.array(pipelineTaskWorkSchema).optional().default([]),
  cleanup: z.object({ state: z.string().optional().default(""), reason: z.string().optional().default("") }).optional(),
  created_at: z.string(),
  updated_at: z.string(),
});

export const pipelineValueSchema = z.object({
  run_id: z.string(),
  name: z.string(),
  value: z.string(),
  source_kind: z.string(),
  source_attempt_id: z.string().optional().default(""),
  updated_at: z.string(),
});

export const pipelineRunDetailSchema = z.object({
  run: pipelineRunSchema,
  template: pipelineTemplateSchema,
  inputs: z.record(z.string()),
  orchestrator: pipelineRuntimeAssignmentSchema.optional(),
  dedicated_assignments: z.record(pipelineRuntimeAssignmentSchema).optional().default({}),
  stage_tasks: z.array(pipelineStageTaskSchema),
  assignments: z.record(pipelineRuntimeAssignmentSchema).optional().default({}),
  values: z.array(pipelineValueSchema),
  diagnostics: z.array(pipelineDiagnosticSchema),
  controls: z.object({
    continue: z.object({ eligible: z.boolean(), reason: z.string().optional().default("") }),
    retry: z.object({ eligible: z.boolean(), reason: z.string().optional().default("") }),
    replace: z.object({ eligible: z.boolean(), reason: z.string().optional().default("") }),
    stop: z.object({ eligible: z.boolean(), reason: z.string().optional().default("") }),
    repair_cleanup: z.object({ eligible: z.boolean(), reason: z.string().optional().default("") }),
  }),
});

export const pipelineRunSummarySchema = z.object({
  run_id: z.string(),
  template_id: z.string(),
  display_name: z.string(),
  project: z.string(),
  state: z.string(),
  revision: z.number().int(),
  pending_action: z.string(),
  current_stage_id: z.string(),
  current_stage_title: z.string().optional().default(""),
  current_agent_id: z.string(),
  attention_reason: z.string(),
  final_outcome: z.string(),
  updated_at: z.string(),
  diagnostics: z.array(pipelineDiagnosticSchema),
});

export const pipelineWorkspaceConflictSchema = z.object({
  kind: z.enum(["agent", "run"]),
  id: z.string(),
  name: z.string(),
});

export const pipelineStartResponseSchema = z.object({
  run: pipelineRunDetailSchema,
  replay: z.boolean(),
  workspace_conflicts: z.array(pipelineWorkspaceConflictSchema),
});

export const pipelineUpdateSchema = z.object({
  run_id: z.string(),
  display_name: z.string(),
  revision: z.number().int(),
  state: z.string(),
  current_stage_id: z.string(),
  current_agent_id: z.string(),
  attention_reason: z.string(),
  final_outcome: z.string(),
});

// created_at dates a pending offer; declined_at is present only on a record a
// person rejected. A payload that does not match its kind's shape still has to
// list with its kind and proposal id, so both payloads stay permissive here and
// the surface summarizes what it can (FS-14.R51).
const proposalRecordFields = {
  proposal_id: z.string(),
  digest: z.string(),
  created_at: z.string(),
  declined_at: z.string().optional(),
};

export const pipelineTemplateProposalSchema = z.object({
  ...proposalRecordFields,
  kind: z.literal("save_template"),
  payload: z.object({ id: z.string(), template: pipelineTemplateSchema }),
});

export const pipelineRunProposalSchema = z.object({
  ...proposalRecordFields,
  kind: z.literal("start_run"),
  payload: pipelineStartRequestSchema,
});

export const pipelineProposalSchema = z.discriminatedUnion("kind", [
  pipelineTemplateProposalSchema,
  pipelineRunProposalSchema,
]);

// The list keeps its payload opaque so one record whose payload does not match
// its kind cannot fail the whole surface; the summary narrows each entry with
// pipelineProposalSchema and falls back to the kind and proposal id it always
// has (FS-14.R51). Both collections are always present and never null
// (INV §11, TS-03.R36).
export const pipelineListedProposalSchema = z.object({
  ...proposalRecordFields,
  kind: z.string(),
  payload: z.unknown(),
});

export const pipelineProposalCollectionsSchema = z.object({
  pending: z.array(pipelineListedProposalSchema),
  declined: z.array(pipelineListedProposalSchema),
});

export type PipelineDiagnostic = z.infer<typeof pipelineDiagnosticSchema>;
export type PipelineTemplate = z.infer<typeof pipelineTemplateSchema>;
export type PipelineTemplateRecord = z.infer<typeof pipelineTemplateRecordSchema>;
export type PipelineRuntimeAssignment = z.infer<typeof pipelineRuntimeAssignmentSchema>;
export type PipelineStartRequest = z.infer<typeof pipelineStartRequestSchema>;
export type PipelineRun = z.infer<typeof pipelineRunSchema>;
export type PipelineValue = z.infer<typeof pipelineValueSchema>;
export type PipelineRunDetail = z.infer<typeof pipelineRunDetailSchema>;
export type PipelineRunSummary = z.infer<typeof pipelineRunSummarySchema>;
export type PipelineWorkspaceConflict = z.infer<typeof pipelineWorkspaceConflictSchema>;
export type PipelineStartResponse = z.infer<typeof pipelineStartResponseSchema>;
export type PipelineUpdate = z.infer<typeof pipelineUpdateSchema>;
export type PipelineProposal = z.infer<typeof pipelineProposalSchema>;
export type PipelineListedProposal = z.infer<typeof pipelineListedProposalSchema>;
export type PipelineProposalCollections = z.infer<typeof pipelineProposalCollectionsSchema>;
