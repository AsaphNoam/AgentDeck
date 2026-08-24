import { z } from "zod";

export const TASK_STATES = [
  "armed",
  "ready",
  "starting",
  "running",
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
  state: z.enum(TASK_STATES),
  outcome: z.string().optional().default(""),
  outcome_source: z.string().optional().default(""),
  outcome_summary: z.string().optional().default(""),
  attention_reason: z.string().optional().default(""),
  created_by_kind: z.string(),
  created_by_agent_id: z.string().optional().default(""),
  assigned_agent_id: z.string().optional().default(""),
  start_attempt_count: z.number().optional().default(0),
  revision: z.number(),
  created_at: z.string(),
  arms: z.array(taskArmSchema).nullable().optional().default([]),
  attachments: z.array(taskAttachmentSchema).nullable().optional().default([]),
});

export const taskListSchema = z.object({ tasks: z.array(taskSchema) });

export type Task = z.output<typeof taskSchema>;
export type TaskArm = z.output<typeof taskArmSchema>;
