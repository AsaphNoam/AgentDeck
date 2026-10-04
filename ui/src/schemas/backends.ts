import { z } from "zod";

export const modelSchema = z.object({
  name: z.string(),
  model: z.string().min(1, "model is required"),
  env: z.record(z.string()).optional(),
  efforts: z.array(z.string()).optional(),
  default_effort: z.string().optional(),
  fast: z.boolean().optional(),
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
  // Which Claude/Codex executable future processes use (FS-09.R75,
  // TS-03.R54). Omitted means Installed.
  provider_mode: z.enum(["installed", "bundled"]).optional(),
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

// Read-only adapter launch support (TS-03.R47). It is parsed separately from
// the editable catalog: a bad entry for one type/interface is dropped on its
// own, so it can neither fail the catalog nor erase another entry's support.
// An absent entry means "unknown" (offer retry), which differs from a parsed
// all-false value (known unsupported).
export const launchSupportSchema = z.object({
  available: z.boolean(),
  effort: z.boolean(),
  fast: z.boolean(),
});

export type LaunchSupport = z.infer<typeof launchSupportSchema>;
export type AgentInterface = "chat" | "terminal";
export type BackendSupport = Partial<Record<BackendType, Partial<Record<AgentInterface, LaunchSupport>>>>;

const AGENT_INTERFACES: AgentInterface[] = ["chat", "terminal"];

export function parseBackendSupport(raw: unknown): BackendSupport {
  const out: BackendSupport = {};
  if (!raw || typeof raw !== "object") return out;
  for (const type of backendTypeSchema.options) {
    const entry = (raw as Record<string, unknown>)[type];
    if (!entry || typeof entry !== "object") continue;
    for (const iface of AGENT_INTERFACES) {
      const parsed = launchSupportSchema.safeParse((entry as Record<string, unknown>)[iface]);
      if (parsed.success) (out[type] ??= {})[iface] = parsed.data;
    }
  }
  return out;
}

// launchSupportFor returns the parsed support for one backend type/interface,
// or undefined when that metadata is missing or unusable.
export function launchSupportFor(
  support: BackendSupport | undefined,
  type: BackendType | undefined,
  iface: AgentInterface,
): LaunchSupport | undefined {
  return type ? support?.[type]?.[iface] : undefined;
}

// Read-only next-start provider metadata per backend/model (TS-03.R52). Like
// backend_support it is parsed tolerantly: a malformed entry is dropped on its
// own and can never fail or erase the editable catalog.
export const providerRuntimeSchema = z.object({
  source: z.enum(["detected", "ambient", "backend", "model", "bundled"]),
  state: z.enum(["available", "missing", "not_executable", "invalid", "bundle_unavailable"]),
  path: z.string().optional(),
  version: z.string().optional(),
  checked_at: z.string().optional(),
});

export type ProviderRuntime = z.infer<typeof providerRuntimeSchema>;
export type ProviderRuntimes = Record<string, Record<string, ProviderRuntime>>;

export function parseProviderRuntimes(raw: unknown): ProviderRuntimes {
  const out: ProviderRuntimes = {};
  if (!raw || typeof raw !== "object") return out;
  for (const [backendId, models] of Object.entries(raw as Record<string, unknown>)) {
    if (!models || typeof models !== "object") continue;
    for (const [modelId, entry] of Object.entries(models as Record<string, unknown>)) {
      const parsed = providerRuntimeSchema.safeParse(entry);
      if (parsed.success) (out[backendId] ??= {})[modelId] = parsed.data;
    }
  }
  return out;
}

export type BackendsResponse = z.infer<typeof backendsResponseSchema> & {
  backend_support: BackendSupport;
  provider_runtimes: ProviderRuntimes;
};

// withBackendSupport replaces the wire read-only metadata (backend_support,
// provider_runtimes) with its tolerant parse, leaving the editable catalog
// exactly as the server sent it.
export function withBackendSupport<T extends object>(res: T): T & { backend_support: BackendSupport; provider_runtimes: ProviderRuntimes } {
  const wire = res as { backend_support?: unknown; provider_runtimes?: unknown };
  return {
    ...res,
    backend_support: parseBackendSupport(wire.backend_support),
    provider_runtimes: parseProviderRuntimes(wire.provider_runtimes),
  };
}

// Refresh provider result (TS-03.R53).
export const refreshProviderResponseSchema = z.object({
  runtime: providerRuntimeSchema,
  credentials: credResultSchema,
  catalog: z.object({
    status: z.enum(["added", "unchanged", "disabled", "unavailable"]),
    added_count: z.number(),
  }),
});

export type RefreshProviderResponse = z.infer<typeof refreshProviderResponseSchema>;

// editableBackendsConfig is the explicit PUT projection: only the editable
// catalog fields, never a spread of a response carrying read-only metadata.
export function editableBackendsConfig(doc: BackendsConfig): BackendsConfig {
  return { version: doc.version, backends: doc.backends };
}

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
