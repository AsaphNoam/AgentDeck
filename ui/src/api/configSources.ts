import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { QUERY_KEYS } from "./config";
import type {
  BindResponse,
  ConfigSourcesResponse,
  PreviewResponse,
  SourceMode,
  SourceOverrides,
  SourceProvider,
} from "../schemas/configSources";

// Query key factory. A binding is backend-global (TS-03.R22): the normal surface
// omits the project entirely and uses the "" key. An explicit project is still
// accepted for the compatible project-scoped API form.
export const configSourcesKey = (projectId?: string) => ["config-sources", projectId ?? ""] as const;

// projectQuery renders the optional project parameter, omitting it entirely when
// no project is selected so the server performs a user-level resolution rather
// than rejecting the request.
function projectQuery(projectId: string | undefined, separator: "?" | "&" = "?"): string {
  return projectId ? `${separator}project=${encodeURIComponent(projectId)}` : "";
}

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
  // Omitted by the normal connection: it previews the provider's user-level
  // source, and the launched agent's actual project resolves later.
  project?: string;
}

export interface BindRequest {
  backendId: string;
  previewToken: string;
  overrides?: SourceOverrides;
  // enableModelSync turns on the target backend's continuing model sync and runs
  // its immediate add-only import (FS-09.R47). The normal connection sends it.
  enableModelSync?: boolean;
}

// useConfigSources fetches discovery candidates + active bindings. With no
// project it returns the global surface, which is what the normal panel uses.
export function useConfigSources(projectId?: string) {
  return useQuery({
    queryKey: configSourcesKey(projectId),
    queryFn: () => json<ConfigSourcesResponse>(`/api/config-sources${projectQuery(projectId)}`),
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

export function useBindConfigSource(projectId?: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ backendId, previewToken, overrides, enableModelSync }: BindRequest) =>
      json<BindResponse>(`/api/config-sources/${encodeURIComponent(backendId)}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          preview_token: previewToken,
          overrides: overrides ?? {},
          enable_model_sync: enableModelSync ?? false,
        }),
      }),
    // An enabled bind changes the durable backend catalog as well as the
    // binding, so both authorities are invalidated (FS-08.A11).
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: configSourcesKey(projectId) });
      void qc.invalidateQueries({ queryKey: QUERY_KEYS.backends });
    },
  });
}

export function useRefreshConfigSource(projectId?: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (backendId: string) =>
      json(`/api/config-sources/${encodeURIComponent(backendId)}/refresh${projectQuery(projectId)}`, {
        method: "POST",
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: configSourcesKey(projectId) }),
  });
}

export function useDeleteConfigSource(projectId?: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ backendId, detach }: { backendId: string; detach?: boolean }) =>
      fetch(
        `/api/config-sources/${encodeURIComponent(backendId)}?detach=${detach ? "true" : "false"}${projectQuery(projectId, "&")}`,
        { method: "DELETE" },
      ).then(async (r) => {
        if (!r.ok && r.status !== 204) throw await httpError(r);
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: configSourcesKey(projectId) }),
  });
}
