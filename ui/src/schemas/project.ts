import { z } from "zod";
import { slugSchema } from "./role";

const colorChannel = z.number().int().min(0).max(255);

export const projectSchema = z.object({
  project: slugSchema,
  title: z.string().min(1, "title is required").max(120),
  color: z.tuple([colorChannel, colorChannel, colorChannel]).default([128, 128, 128]),
  cwd: z.string().min(1, "cwd is required"),
  add_dirs: z.array(z.string()).default([]),
  context_prompt: z.string().default(""),
  archived: z.boolean().optional(),
  // Optional worktree settings (FS-04.R45). Absent on a legacy project file and
  // read as empty; base_branch empty means "auto-detect the default branch".
  base_branch: z.string().default(""),
  setup_command: z.string().default(""),
});

export type ProjectInput = z.infer<typeof projectSchema>;

export const fieldWarningSchema = z.object({
  field: z.string(),
  code: z.string(),
  message: z.string(),
});

export const projectResponseSchema = z.object({
  project: z.string(),
  title: z.string(),
  color: z.tuple([z.number(), z.number(), z.number()]),
  cwd: z.string(),
  add_dirs: z.array(z.string()),
  context_prompt: z.string(),
  // Server-computed, read-only absolute path to the project's AgentDeck-owned
  // shared-resources directory (TS-03.R12). Never sent back on create/update.
  resource_dir: z.string().default(""),
  warnings: z.array(fieldWarningSchema).optional(),
  archived: z.boolean().optional(),
  base_branch: z.string().default(""),
  setup_command: z.string().default(""),
});

export type ProjectResponse = z.infer<typeof projectResponseSchema>;
export type FieldWarning = z.infer<typeof fieldWarningSchema>;
