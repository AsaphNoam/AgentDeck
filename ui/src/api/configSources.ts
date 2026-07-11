import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type {
  ConfigSourcesResponse,
  PreviewResponse,
  SourceMode,
  SourceOverrides,
  SourceProvider,
} from "../schemas/configSources";

// Query key factory — federation state is project-scoped (project layers depend
// on project.cwd), so the key carries the project id.
export const configSourcesKey = (projectId: string) => ["config-sources", projectId] as const;

async function httpError(res: Response): Promise<Error> {
  const body = await res.json().catch(() => ({}));
  return Object.assign(new Error(`HTTP ${res.status}`), { status: res.status, body });
}

async function json<T>(input: string, init?: RequestInit): Promise<T> {
  const res = await fetch(input, init);
  if (!res.ok) throw await httpError(res);
  return res.json() as Promise<T>;
}

export interface PreviewRequest {
  provider: SourceProvider;
  root: string; // "auto" | absolute path
  profile?: string;
  mode: SourceMode;
  claims?: string[];
  project: string;
}

export interface BindRequest {
  backendId: string;
  previewToken: string;
  overrides?: SourceOverrides;
}

// useConfigSources fetches discovery candidates + active bindings for a project.
// Disabled until a project id is known (federation is project-scoped).
export function useConfigSources(projectId: string | undefined) {
  return useQuery({
    queryKey: configSourcesKey(projectId ?? ""),
    queryFn: () => json<ConfigSourcesResponse>(`/api/config-sources?project=${encodeURIComponent(projectId!)}`),
    enabled: !!projectId,
  });
}

export function usePreviewConfigSource() {
  return useMutation({
    mutationFn: (req: PreviewRequest) =>
      json<PreviewResponse>("/api/config-sources/preview", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(req),
      }),
  });
}

export function useBindConfigSource(projectId: string | undefined) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ backendId, previewToken, overrides }: BindRequest) =>
      json(`/api/config-sources/${encodeURIComponent(backendId)}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ preview_token: previewToken, overrides: overrides ?? {} }),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: configSourcesKey(projectId ?? "") }),
  });
}

export function useRefreshConfigSource(projectId: string | undefined) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (backendId: string) =>
      json(`/api/config-sources/${encodeURIComponent(backendId)}/refresh?project=${encodeURIComponent(projectId ?? "")}`, {
        method: "POST",
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: configSourcesKey(projectId ?? "") }),
  });
}

export function useDeleteConfigSource(projectId: string | undefined) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ backendId, detach }: { backendId: string; detach?: boolean }) =>
      fetch(
        `/api/config-sources/${encodeURIComponent(backendId)}?detach=${detach ? "true" : "false"}&project=${encodeURIComponent(projectId ?? "")}`,
        { method: "DELETE" },
      ).then(async (r) => {
        if (!r.ok && r.status !== 204) throw await httpError(r);
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: configSourcesKey(projectId ?? "") }),
  });
}
