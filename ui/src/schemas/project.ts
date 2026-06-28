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
  warnings: z.array(fieldWarningSchema).optional(),
});

export type ProjectResponse = z.infer<typeof projectResponseSchema>;
export type FieldWarning = z.infer<typeof fieldWarningSchema>;
