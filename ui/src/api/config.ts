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

async function json<T>(input: string, init?: RequestInit): Promise<T> {
  const res = await fetch(input, init);
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw Object.assign(new Error(`HTTP ${res.status}`), { status: res.status, body });
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
      fetch(`/api/roles/${id}${force ? "?force=true" : ""}`, { method: "DELETE" }).then((r) => {
        if (!r.ok && r.status !== 204) throw new Error(`HTTP ${r.status}`);
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
      fetch(`/api/projects/${id}${force ? "?force=true" : ""}`, { method: "DELETE" }).then((r) => {
        if (!r.ok && r.status !== 204) throw new Error(`HTTP ${r.status}`);
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
    mutationFn: (data: Partial<Pick<Config, "onboarding_complete" | "default_project" | "default_role">>) =>
      json<Config>("/api/config", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(data),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: QUERY_KEYS.config }),
  });
}

// Shared QueryClient for the app (singleton).
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 5_000, retry: 1 },
  },
});
