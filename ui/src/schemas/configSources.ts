import { z } from "zod";

// Mirrors the server configsource types (internal/configsource/types.go) and the
// federation API envelopes (internal/server/config_sources.go). Only display-safe
// fields cross the wire: no raw source contents and no secret values — env keys
// are names only.

export const sourceProviderSchema = z.enum(["claude-code", "codex"]);
export type SourceProvider = z.infer<typeof sourceProviderSchema>;

export const sourceModeSchema = z.enum(["linked", "mirrored"]);
export type SourceMode = z.infer<typeof sourceModeSchema>;

export const sourceOverridesSchema = z.object({
  model: z.string().nullable().optional(),
  effort: z.string().nullable().optional(),
});
export type SourceOverrides = z.infer<typeof sourceOverridesSchema>;

export const fieldProvenanceSchema = z.object({
  scope: z.string(),
  path: z.string().optional(),
  key: z.string(),
});

export const effectiveModelSchema = z.object({
  id: z.string(),
  name: z.string().optional(),
  source: z.string(),
});

export const assetSchema = z.object({
  kind: z.string(),
  name: z.string().optional(),
  path: z.string(),
  scope: z.string(),
  sha256: z.string().optional(),
  detachability: z.string(),
  status: z.string(),
});
export type Asset = z.infer<typeof assetSchema>;

export const environmentKeySchema = z.object({
  name: z.string(),
  scope: z.string(),
  configured: z.boolean(),
});

export const effectiveSchema = z.object({
  model: z.string().nullable().optional(),
  fallback_model: z.string().nullable().optional(),
  effort: z.string().nullable().optional(),
  verbosity: z.string().nullable().optional(),
  provider: z.string().nullable().optional(),
  models: z.array(effectiveModelSchema).nullable().default([]),
  assets: z.array(assetSchema).nullable().default([]),
  environment_keys: z.array(environmentKeySchema).nullable().default([]),
  mcp_servers: z.array(z.string()).nullable().default([]),
  provenance: z.record(fieldProvenanceSchema).default({}),
});
export type Effective = z.infer<typeof effectiveSchema>;

export const skippedPathSchema = z.object({ path: z.string(), reason: z.string() });
export const fingerprintSchema = z.object({ path: z.string(), sha256: z.string(), size: z.number() });

export const reportSchema = z.object({
  files_read: z.array(z.object({ path: z.string(), scope: z.string(), kind: z.string() })).nullable().default([]),
  skipped: z.array(skippedPathSchema).nullable().default([]),
  unknown_keys: z
    .array(z.object({ path: z.string(), key: z.string(), disposition: z.string() }))
    .nullable()
    .default([]),
  warnings: z.array(z.string()).nullable().default([]),
  fingerprints: z.array(fingerprintSchema).nullable().default([]),
  approved_roots: z.array(z.string()).nullable().default([]),
  source_digest: z.string().default(""),
});
export type Report = z.infer<typeof reportSchema>;

export const candidateSchema = z.object({
  provider: z.string(),
  root: z.string(),
  profile: z.string().optional(),
  found: z.boolean(),
  health: z.string(),
  warnings: z.array(z.string()).nullable().default([]),
});
export type Candidate = z.infer<typeof candidateSchema>;

export const bindingViewSchema = z.object({
  backend_id: z.string(),
  provider: z.string(),
  mode: z.string(),
  root: z.string(),
  profile: z.string().optional(),
  claims: z.array(z.string()).nullable().default([]),
  overrides: sourceOverridesSchema.optional(),
  approved_roots: z.array(z.string()).nullable().default([]),
  health: z.string().optional(),
  stale: z.boolean().default(false),
  generation: z.number().optional(),
});
export type BindingView = z.infer<typeof bindingViewSchema>;

export const configSourcesResponseSchema = z.object({
  bindings: z.array(bindingViewSchema).nullable().default([]),
  candidates: z.array(candidateSchema).nullable().default([]),
});
export type ConfigSourcesResponse = z.infer<typeof configSourcesResponseSchema>;

export const previewResponseSchema = z.object({
  preview_token: z.string(),
  expires_at: z.string(),
  effective: effectiveSchema,
  report: reportSchema,
});
export type PreviewResponse = z.infer<typeof previewResponseSchema>;

// The SSE config_source_update payload (internal/configsource/manager.go Update).
export const configSourceUpdateSchema = z.object({
  backend_id: z.string(),
  project_id: z.string(),
  generation: z.number(),
  health: z.string(),
  changed: z.array(z.string()).nullable().default([]),
  stale: z.boolean().default(false),
});
export type ConfigSourceUpdate = z.infer<typeof configSourceUpdateSchema>;
