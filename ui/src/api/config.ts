import {
  useQuery,
  useMutation,
  useQueryClient,
  QueryClient,
} from "@tanstack/react-query";
import type { RoleResponse } from "../schemas/role";
import type { ProjectResponse } from "../schemas/project";
import type { BackendsResponse, BackendsConfig } from "../schemas/backends";
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
    queryFn: () => json<BackendsConfig>("/api/backends"),
  });
}

export function usePutBackends() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: BackendsConfig) =>
      json<BackendsResponse>("/api/backends", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
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
    mutationFn: (data: Partial<Pick<Config, "onboarding_complete" | "default_project" | "default_role" | "notifications">>) =>
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
