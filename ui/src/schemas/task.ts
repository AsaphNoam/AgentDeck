import { z } from "zod";

export const TASK_STATES = [
  "armed",
  "ready",
  "starting",
  "running",
  "waiting",
  "interrupted",
  "finished",
  "dependency_failed",
] as const;

export const TASK_OUTCOMES = ["success", "failure", "blocked", "cancelled"] as const;

/** States that need a person: parked work and work whose agent went away (FS-02.R44). */
export const TASK_ATTENTION_STATES = ["dependency_failed", "interrupted"] as const;

export const taskArmSchema = z.object({
  arm_id: z.string(),
  task_id: z.string(),
  kind: z.enum(["work_result", "signal"]),
  source_kind: z.string().optional().default(""),
  source_id: z.string().optional().default(""),
  signal_name: z.string().optional().default(""),
  satisfying_outcomes: z.array(z.string()).nullable().optional().default([]),
  state: z.enum(["unsatisfied", "satisfied", "unsatisfiable"]),
  satisfied_at: z.string().optional(),
});

export const taskAttachmentSchema = z.object({
  task_id: z.string(),
  context_ref_id: z.string(),
  label: z.string().optional().default(""),
  description: z.string().optional().default(""),
  created_at: z.string(),
});

export const taskLineageSchema = z.object({
  parent_task_id: z.string().optional().default(""),
  pipeline_run_id: z.string().optional().default(""),
  pipeline_stage_id: z.string().optional().default(""),
  creation_attempt_id: z.string().optional().default(""),
});

const EMPTY_LINEAGE = { parent_task_id: "", pipeline_run_id: "", pipeline_stage_id: "", creation_attempt_id: "" };

export const taskSchema = z.object({
  task_id: z.string(),
  project: z.string(),
  display_name: z.string(),
  instruction: z.string(),
  target_kind: z.enum(["agent", "launch"]),
  target_agent_id: z.string().optional().default(""),
  role: z.string().optional().default(""),
  backend: z.string().optional().default(""),
  model: z.string().optional().default(""),
  effort: z.string().optional().default(""),
  fast: z.boolean().optional().default(false),
  state: z.enum(TASK_STATES),
  outcome: z.string().optional().default(""),
  outcome_source: z.string().optional().default(""),
  outcome_summary: z.string().optional().default(""),
  attention_reason: z.string().optional().default(""),
  created_by_kind: z.string(),
  created_by_agent_id: z.string().optional().default(""),
  assigned_agent_id: z.string().optional().default(""),
  start_attempt_count: z.number().optional().default(0),
  retry_eligible: z.boolean().optional().default(false),
  revision: z.number(),
  created_at: z.string(),
  updated_at: z.string().optional().default(""),
  started_at: z.string().optional(),
  finished_at: z.string().optional(),
  outcome_details: z.string().optional().default(""),
  // Runtime continuation and cleanup flags (FS-16.R33–R38) the view needs to
  // tell durable waiting and finishing apart from an unexpected interruption.
  pending_release: z.boolean().optional().default(false),
  continuation_pending: z.boolean().optional().default(false),
  pending_yield: z.boolean().optional().default(false),
  resume_needed: z.boolean().optional().default(false),
  cleanup_phase: z.string().optional().default(""),
  cleanup_failure_count: z.number().optional().default(0),
  cleanup_next_retry_at: z.string().optional(),
  cleanup_last_error: z.string().optional().default(""),
  cleanup_unsafe: z.boolean().optional().default(false),
  // Lineage is provenance (TS-10.R25); the server always emits the object, with
  // empty fields when a task has no recorded parent or pipeline run.
  lineage: taskLineageSchema.nullable().optional().transform((value) => value ?? EMPTY_LINEAGE),
  outputs: z.record(z.string(), z.string()).nullable().optional().transform((value) => value ?? {}),
  arms: z.array(taskArmSchema).nullable().optional().default([]),
  attachments: z.array(taskAttachmentSchema).nullable().optional().default([]),
});

export const taskListSchema = z.object({ tasks: z.array(taskSchema) });

export type Task = z.output<typeof taskSchema>;
export type TaskArm = z.output<typeof taskArmSchema>;

/** The outcomes a work-result prerequisite may wait for, by source kind (FS-16.R5). */
export const WORK_RESULT_OUTCOMES: Record<"task" | "pipeline_run", readonly { value: string; label: string }[]> = {
  task: [
    { value: "success", label: "Success" },
    { value: "failure", label: "Failure" },
    { value: "blocked", label: "Blocked" },
    { value: "cancelled", label: "Cancelled" },
  ],
  pipeline_run: [
    { value: "success", label: "Success" },
    { value: "failure", label: "Failure" },
    { value: "cancelled", label: "Cancelled" },
  ],
};

/** taskActions is the one FS-16.R22/R23 eligibility matrix the desktop Tasks
 *  view and the phone render controls from (INV §2). Retry eligibility is the
 *  server's own projection. */
export function taskActions(task: Pick<Task, "state" | "retry_eligible">) {
  return {
    cancel: task.state !== "finished",
    retry: task.retry_eligible,
    recordResult: task.state === "running" || task.state === "interrupted",
    rearm: task.state === "armed" || task.state === "ready" || task.state === "dependency_failed",
  };
}
