import { z } from "zod";

export const modelSchema = z.object({
  name: z.string(),
  model: z.string().min(1, "model is required"),
  env: z.record(z.string()).optional(),
  efforts: z.array(z.string()).optional(),
  default_effort: z.string().optional(),
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
  // Opt-in startup model import (FS-09.R28/R45): codex-acp syncs the Codex CLI
  // model cache; claude-acp imports configured user-level Claude settings.
  autosync_models: z.boolean().optional(),
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

// Item-scoped create (TS-03.R23). `connection` is present only when the request
// asked to connect native configuration; a failure reports "unbound" beside the
// backend that was still created, never an overall error.
export const backendConnectionSchema = z.object({
  status: z.enum(["connected", "unbound"]),
  model_sync_enabled: z.boolean().optional(),
  models_added: z.number().optional(),
  error: z.object({ code: z.string(), message: z.string() }).optional(),
});

export const createBackendResponseSchema = z.object({
  backend_id: z.string(),
  backend: backendSchema,
  connection: backendConnectionSchema.optional(),
});

export type BackendConnection = z.infer<typeof backendConnectionSchema>;
export type CreateBackendResponse = z.infer<typeof createBackendResponseSchema>;
