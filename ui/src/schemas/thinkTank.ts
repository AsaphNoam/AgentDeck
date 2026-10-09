import { z } from "zod";

/** Think Tank wire shapes (TS-14 §3, version 1). Go's encoder defines them; the
 *  room fixture pins that (INV §11). Collections are always arrays. */

export const THINK_TANK_PHASES = ["setup", "openings", "discussion", "closing", "ended"] as const;

export const thinkTankMemberSchema = z.object({
  agent_id: z.string(),
  name: z.string(),
  project: z.string(),
  role: z.enum(["participant", "judge"]),
  order: z.number(),
  limit: z.number(),
  completed: z.number(),
  may_leave: z.boolean(),
  state: z.enum(["active", "departed", "exhausted"]),
  setup_state: z.enum(["ready", "pending", "failed", "launching", "abandoned"]),
  setup_error: z.string().optional().default(""),
  exists: z.boolean(),
  archived: z.boolean().optional().default(false),
  running: z.boolean(),
  agent_status: z.string().optional().default(""),
});

export const thinkTankAttemptSchema = z.object({
  attempt_id: z.string(),
  agent_id: z.string(),
  turn: z.enum(["opening", "discussion", "closing", "judge"]),
  state: z.string(),
  failure: z.string().optional().default(""),
  started_at: z.string(),
});

export const thinkTankSummarySchema = z.object({
  version: z.literal(1),
  room_id: z.string(),
  title: z.string().optional().default(""),
  goal: z.string(),
  origin_project: z.string(),
  /** A pipeline-owned room's run and stage (FS-21.R41). */
  pipeline: z.object({
    run_id: z.string(),
    stage_id: z.string(),
    task_id: z.string(),
    pinned: z.boolean().optional().default(false),
  }).optional(),
  phase: z.enum(THINK_TANK_PHASES),
  control: z.enum(["running", "pause_requested", "paused", "end_requested"]),
  hold: z.string().optional().default(""),
  end_reason: z.string().optional().default(""),
  judge_status: z.string().optional().default(""),
  participants: z.array(z.string()),
  revision: z.number(),
  created_at: z.string(),
  updated_at: z.string(),
  ended_at: z.string().optional(),
  active_agent_id: z.string().optional().default(""),
  /** Every running attempt; several only during independent openings, when
   *  active_agent_id/active are absent (TS-14.R27). */
  active_attempts: z.array(thinkTankAttemptSchema).optional().default([]),
  /** Card-level roster and allowances (TS-14.R27). */
  roster: z.array(z.object({
    agent_id: z.string(),
    name: z.string(),
    project: z.string(),
    role: z.enum(["participant", "judge"]),
    state: z.string(),
    limit: z.number(),
    completed: z.number(),
    remaining: z.number(),
    exists: z.boolean(),
  })).optional().default([]),
  total_remaining: z.number().optional().default(0),
  judge_enabled: z.boolean().optional().default(false),
  judge_name: z.string().optional().default(""),
});

export const thinkTankDetailSchema = thinkTankSummarySchema.extend({
  openings: z.boolean(),
  members: z.array(thinkTankMemberSchema),
  active: thinkTankAttemptSchema.optional(),
  next: z.object({
    agent_id: z.string(),
    turn: z.string(),
    waiting: z.string().optional().default(""),
  }).optional(),
  pending: z.array(z.object({
    input_id: z.string(),
    kind: z.enum(["user", "annotation"]),
    body: z.string(),
    context: z.unknown().optional(),
    created_at: z.string(),
  })),
  failed: z.array(thinkTankAttemptSchema),
  judge: z.object({
    enabled: z.boolean(),
    status: z.string().optional().default(""),
    agent_id: z.string().optional().default(""),
    error: z.string().optional().default(""),
    config: z.record(z.string(), z.unknown()).optional(),
  }),
  deletable: z.boolean(),
});

export const thinkTankEntrySchema = z.object({
  seq: z.number(),
  kind: z.enum(["opening", "reply", "departure", "closing", "user", "annotation", "synthesis", "missing_opening", "stage_context"]),
  agent_id: z.string().optional().default(""),
  agent_name: z.string().optional().default(""),
  project: z.string().optional().default(""),
  body: z.string(),
  attempt_id: z.string().optional().default(""),
  input_id: z.string().optional().default(""),
  context: z.unknown().optional(),
  undiscussed: z.boolean().optional().default(false),
  created_at: z.string(),
});

export const thinkTankActivitySchema = z.object({
  version: z.literal(1),
  room_id: z.string(),
  seq: z.number(),
  attempt_id: z.string(),
  agent_id: z.string(),
  agent_name: z.string(),
  project: z.string(),
  source_seq: z.number(),
  truncated: z.boolean().optional().default(false),
  /** Server-derived publication state stays correct when the entry window is clipped. */
  published: z.boolean().optional().default(false),
  event: z.object({
    type: z.string(),
    seq: z.number().optional(),
    ts: z.string().optional(),
    activity_id: z.string().optional().default(""),
    parent_activity_id: z.string().optional().default(""),
    data: z.unknown().optional(),
    truncated: z.boolean().optional(),
  }).optional(),
  created_at: z.string(),
});

export const thinkTankUpdateSchema = z.object({
  version: z.literal(1),
  room_id: z.string(),
  revision: z.number().optional().default(0),
  deleted: z.boolean().optional().default(false),
});

export const thinkTankSourceSchema = z.object({
  source_id: z.string(),
  agent_id: z.string(),
  agent_name: z.string(),
  project: z.string(),
  attempt_ids: z.array(z.string()).optional().default([]),
});

export type ThinkTankMember = z.output<typeof thinkTankMemberSchema>;
export type ThinkTankSummary = z.output<typeof thinkTankSummarySchema>;
export type ThinkTankDetail = z.output<typeof thinkTankDetailSchema>;
export type ThinkTankEntry = z.output<typeof thinkTankEntrySchema>;
export type ThinkTankActivity = z.output<typeof thinkTankActivitySchema>;
export type ThinkTankSource = z.output<typeof thinkTankSourceSchema>;
