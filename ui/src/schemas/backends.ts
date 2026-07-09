import { z } from "zod";

export const modelSchema = z.object({
  name: z.string(),
  model: z.string().min(1, "model is required"),
  env: z.record(z.string()).optional(),
});

// backendTypeSchema is the single source of the backend type union; every other
// mapping (labels, per-type fields) derives from it.
export const backendTypeSchema = z.enum([
  "claude-acp",
  "codex-acp",
  "opencode-acp",
  "openhands-acp",
]);

export type BackendType = z.infer<typeof backendTypeSchema>;

export const backendSchema = z.object({
  name: z.string(),
  type: backendTypeSchema,
  default: z.boolean().optional(),
  default_model: z.string(),
  models: z.record(modelSchema),
  env: z.record(z.string()).optional(),
});

export const backendsConfigSchema = z.object({
  version: z.literal(2),
  backends: z.record(backendSchema),
});

export type BackendsConfig = z.infer<typeof backendsConfigSchema>;
export type Backend = z.infer<typeof backendSchema>;
export type Model = z.infer<typeof modelSchema>;

export const credResultSchema = z.object({
  status: z.enum(["ok", "failed", "skipped"]),
  detail: z.string().optional(),
});

export type CredResult = z.infer<typeof credResultSchema>;

export const backendsResponseSchema = backendsConfigSchema.extend({
  credentials: z.record(credResultSchema).optional(),
});

export type BackendsResponse = z.infer<typeof backendsResponseSchema>;
