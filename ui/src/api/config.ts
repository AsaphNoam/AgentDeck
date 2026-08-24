import {
  useQuery,
  useMutation,
  useQueryClient,
  QueryClient,
} from "@tanstack/react-query";
import type { RoleResponse } from "../schemas/role";
import type { ProjectResponse } from "../schemas/project";
import type {
  BackendsResponse,
  BackendsConfig,
  BackendType,
  CreateBackendResponse,
} from "../schemas/backends";
import type { Config } from "../schemas/config";

// Query keys — used for cache invalidation.
export const QUERY_KEYS = {
  roles: ["roles"] as const,
  projects: ["projects"] as const,
  backends: ["backends"] as const,
  config: ["config"] as const,
};

// httpError builds the structured error the editors rely on: it preserves the
// status code and parsed body so callers can branch on 409 (in-use) and read
// `body.agents` to offer the ?force=true retry.
async function httpError(res: Response): Promise<Error> {
  const body = await res.json().catch(() => ({}));
  return Object.assign(new Error(`HTTP ${res.status}`), { status: res.status, body });
}

// configErrorMessage extracts a human-readable reason from the structured error
// httpError attaches, so the editors stop showing a bare "HTTP 400"/"HTTP 409"
// and instead surface the field-level validation message or conflict reason the
// server actually sent. Handles all three server envelopes: validation_failed
// ({errors:[{field,message}]}), in-use/duplicate ({message}), and the runtime
// APIError shape ({error:{message}}), falling back to the raw Error message.
export function configErrorMessage(err: unknown): string {
  const e = err as {
    message?: string;
    body?: {
      error?: string | { message?: string };
      message?: string;
      errors?: Array<{ field?: string; message?: string }>;
    };
  };
  const body = e?.body;
  if (body) {
    if (Array.isArray(body.errors) && body.errors.length > 0) {
      const parts = body.errors
        .map((fe) => (fe.field ? `${fe.field}: ${fe.message ?? ""}` : fe.message ?? ""))
        .filter(Boolean);
      if (parts.length > 0) return parts.join("; ");
    }
    if (typeof body.message === "string" && body.message) return body.message;
    if (body.error && typeof body.error === "object" && body.error.message) return body.error.message;
  }
  return e?.message ?? String(err);
}

async function json<T>(input: string, init?: RequestInit): Promise<T> {
  const res = await fetch(input, init);
  if (!res.ok) {
    throw await httpError(res);
  }
  return res.json() as Promise<T>;
}

// The backend catalog is whole-document editable, so its GET/POST/PUT responses
// retain the server's strong ETag beside the JSON payload. It is an HTTP
// validator, not a persisted catalog field.
export type CatalogETagged<T> = T & { catalogEtag?: string };

async function jsonWithCatalogETag<T>(input: string, init?: RequestInit): Promise<CatalogETagged<T>> {
  const res = await fetch(input, init);
  if (!res.ok) {
    throw await httpError(res);
  }
  return { ...(await res.json() as T), catalogEtag: res.headers.get("ETag") ?? undefined };
}

// ---- Roles ----

export function useRoles() {
  return useQuery({
    queryKey: QUERY_KEYS.roles,
    queryFn: () => json<Record<string, Omit<RoleResponse, "role">>>("/api/roles"),
  });
}

export function useCreateRole() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: RoleResponse) =>
      json<RoleResponse>("/api/roles", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(data),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: QUERY_KEYS.roles }),
  });
}

export function useUpdateRole() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: Omit<RoleResponse, "role"> }) =>
      json<RoleResponse>(`/api/roles/${id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(data),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: QUERY_KEYS.roles }),
  });
}

export function useDeleteRole() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, force }: { id: string; force?: boolean }) =>
      fetch(`/api/roles/${id}${force ? "?force=true" : ""}`, { method: "DELETE" }).then(async (r) => {
        if (!r.ok && r.status !== 204) throw await httpError(r);
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: QUERY_KEYS.roles }),
  });
}

// ---- Directory browsing ----

// pickDirectory opens the host's standard folder panel and resolves to the one
// absolute directory the person selected, or null when they dismissed it
// (TS-03.R26). Cancel is a bodyless 204, so this bypasses json() the same way
// useDeleteProject does. It is a plain function rather than a mutation because
// nothing is cached or invalidated: the answer only fills a form field.
export async function pickDirectory(): Promise<string | null> {
  const res = await fetch("/api/directory-picker", { method: "POST" });
  if (res.status === 204) return null;
  if (!res.ok) throw await httpError(res);
  const body = (await res.json()) as { path?: string };
  return body.path ?? null;
}

// ---- Projects ----

export function useProjects() {
  return useQuery({
    queryKey: QUERY_KEYS.projects,
    queryFn: () => json<Record<string, Omit<ProjectResponse, "project">>>("/api/projects"),
  });
}

export function useCreateProject() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: ProjectResponse) =>
      json<ProjectResponse>("/api/projects", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(data),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: QUERY_KEYS.projects }),
  });
}

export function useUpdateProject() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: Omit<ProjectResponse, "project"> }) =>
      json<ProjectResponse>(`/api/projects/${id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(data),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: QUERY_KEYS.projects }),
  });
}

export function useDeleteProject() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, force }: { id: string; force?: boolean }) =>
      fetch(`/api/projects/${id}${force ? "?force=true" : ""}`, { method: "DELETE" }).then(async (r) => {
        if (!r.ok && r.status !== 204) throw await httpError(r);
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: QUERY_KEYS.projects }),
  });
}

// ---- Backends ----

export function useBackends() {
  return useQuery({
    queryKey: QUERY_KEYS.backends,
    queryFn: () => jsonWithCatalogETag<BackendsConfig>("/api/backends"),
  });
}

export interface CreateBackendRequest {
  backend_id: string;
  name: string;
  type: BackendType;
  connect_native_configuration?: boolean;
}

// useCreateBackend adds exactly one backend through the item-scoped create
// (TS-03.R23). It deliberately does NOT invalidate the backends query: the
// Settings editor merges the created card into its draft, so an unrelated
// unsaved edit is never discarded by a background refetch (FS-04.R40).
export function useCreateBackend() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: CreateBackendRequest) =>
      jsonWithCatalogETag<CreateBackendResponse>("/api/backends", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(req),
      }),
    // A create-and-connect binds the source server-side, so the already-mounted
    // global config-source query (a sibling backend's panel) is now stale and
    // would render the new backend as unbound with a "Use my … configuration"
    // action (FS-08.A11, TS-03.R23, INV §1/§10). Invalidate that query — but not
    // the backends query, whose refetch must not re-seed the draft. The literal
    // key mirrors configSourcesKey() to avoid a config↔configSources import cycle.
    onSuccess: (res) => {
      if (res.connection?.status === "connected") {
        void qc.invalidateQueries({ queryKey: ["config-sources"] });
      }
    },
  });
}

export function usePutBackends() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ catalogEtag, ...data }: CatalogETagged<BackendsConfig>) =>
      jsonWithCatalogETag<BackendsResponse>("/api/backends", {
        method: "PUT",
        headers: { "Content-Type": "application/json", "If-Match": catalogEtag ?? "" },
        body: JSON.stringify(data),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: QUERY_KEYS.backends }),
  });
}

// ---- Config / onboarding ----

export function useConfig() {
  return useQuery({
    queryKey: QUERY_KEYS.config,
    queryFn: () => json<Config>("/api/config"),
    refetchInterval: 10_000, // poll for onboarding state changes
  });
}

export function usePutConfig() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: Partial<Pick<Config, "onboarding_complete" | "default_project" | "default_role" | "appearance_skin" | "notifications" | "task_concurrency">>) =>
      json<Config>("/api/config", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(data),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: QUERY_KEYS.config }),
  });
}

// ---- Launch (New Agent) ----

export interface LaunchParams {
  name?: string;
  role: string;
  project: string;
  backend?: string;
  model?: string;
  effort?: string;
  interface: "chat" | "terminal";
}

export function useLaunchAgent() {
  return useMutation({
    mutationFn: (params: LaunchParams) =>
      json<{ agent: { agent_id: string; name: string } }>("/api/sessions", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(params),
      }),
  });
}

// Shared QueryClient for the app (singleton).
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 5_000, retry: 1 },
  },
});
