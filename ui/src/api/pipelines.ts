import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { z } from "zod";
import {
  pipelineRunDetailSchema,
  pipelineRunSummarySchema,
  pipelineStartResponseSchema,
  pipelineTemplateRecordSchema,
  type PipelineRunDetail,
  type PipelineStartRequest,
  type PipelineProposalCollections,
  type PipelineTemplate,
  pipelineListedProposalSchema,
  pipelineProposalCollectionsSchema,
} from "../schemas/pipeline";

export const PIPELINE_QUERY_KEYS = {
  templates: ["pipelines", "templates"] as const,
  runs: ["pipelines", "run-list"] as const,
  runDetails: ["pipelines", "run-detail"] as const,
  run: (id: string) => ["pipelines", "run-detail", id] as const,
  proposals: ["pipelines", "proposals"] as const,
};

interface PipelineErrorBody {
  error?: {
    code?: string;
    message?: string;
    details?: {
      pipeline_code?: string;
      diagnostics?: Array<{ field: string; code: string; message: string }>;
      code?: string;
      conflicts?: Array<{ kind: "agent" | "run"; id: string; name: string }>;
    };
  };
}

export class PipelineAPIError extends Error {
  status: number;
  body: PipelineErrorBody;

  constructor(status: number, body: PipelineErrorBody, fallback: string) {
    super(body.error?.message || fallback);
    this.status = status;
    this.body = body;
  }
}

async function request<S extends z.ZodTypeAny>(url: string, schema: S, init?: RequestInit): Promise<z.output<S>> {
  const response = await fetch(url, init);
  if (!response.ok) {
    const body = await response.json().catch(() => ({})) as PipelineErrorBody;
    throw new PipelineAPIError(response.status, body, `${response.status} ${response.statusText}`);
  }
  return schema.parse(await response.json()) as z.output<S>;
}

async function emptyRequest(url: string, init: RequestInit): Promise<void> {
  const response = await fetch(url, init);
  if (!response.ok) {
    const body = await response.json().catch(() => ({})) as PipelineErrorBody;
    throw new PipelineAPIError(response.status, body, `${response.status} ${response.statusText}`);
  }
}

export function listPipelineTemplates() {
  return request("/api/pipelines", z.array(pipelineTemplateRecordSchema));
}

export function validatePipelineTemplate(id: string, template: PipelineTemplate) {
  return request("/api/pipelines/validate", pipelineTemplateRecordSchema, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ id, template }),
  });
}

export function createPipelineTemplate(id: string, template: PipelineTemplate) {
  return request("/api/pipelines", pipelineTemplateRecordSchema, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ id, template }),
  });
}

export function updatePipelineTemplate(id: string, template: PipelineTemplate) {
  return request(`/api/pipelines/${encodeURIComponent(id)}`, pipelineTemplateRecordSchema, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(template),
  });
}

export function deletePipelineTemplate(id: string) {
  return emptyRequest(`/api/pipelines/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export async function listPipelineRuns(limit = 50, offset = 0) {
  const response = await fetch(`/api/pipeline-runs?limit=${limit}&offset=${offset}`);
  if (!response.ok) {
    const body = await response.json().catch(() => ({})) as PipelineErrorBody;
    throw new PipelineAPIError(response.status, body, `${response.status} ${response.statusText}`);
  }
  const runs = z.array(pipelineRunSummarySchema).parse(await response.json());
  const rawTotal = response.headers.get("X-Total-Count");
  const parsedTotal = rawTotal === null ? runs.length : Number(rawTotal);
  return { runs, total: Number.isSafeInteger(parsedTotal) && parsedTotal >= 0 ? parsedTotal : runs.length };
}

export function listPipelineProposals() {
  return request("/api/pipeline-proposals", pipelineProposalCollectionsSchema);
}

// The updated record is parsed with the same permissive listed shape the
// collections use: a decline of a record whose payload does not match its kind
// still succeeded, and rejecting its response would report a write that happened
// as a failure (FS-14.R51).
export function declinePipelineProposal(id: string) {
  return request(`/api/pipeline-proposals/${encodeURIComponent(id)}/decline`, pipelineListedProposalSchema, { method: "POST" });
}

export function deletePipelineProposal(id: string) {
  return emptyRequest(`/api/pipeline-proposals/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export function getPipelineRun(id: string) {
  return request(`/api/pipeline-runs/${encodeURIComponent(id)}`, pipelineRunDetailSchema);
}

export function startPipelineRun(requestBody: PipelineStartRequest, acknowledgeSharedWorkspace = false) {
  return request("/api/pipeline-runs", pipelineStartResponseSchema, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ ...requestBody, acknowledge_shared_workspace: acknowledgeSharedWorkspace }),
  });
}

export function continuePipelineRun(id: string, revision: number, input: string) {
  return request(`/api/pipeline-runs/${encodeURIComponent(id)}/continue`, pipelineRunDetailSchema, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ revision, input }),
  });
}

export function retryPipelineRun(id: string, revision: number) {
  return request(`/api/pipeline-runs/${encodeURIComponent(id)}/retry`, pipelineRunDetailSchema, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ revision }),
  });
}

export function stopPipelineRun(id: string, revision: number) {
  return request(`/api/pipeline-runs/${encodeURIComponent(id)}/stop`, pipelineRunDetailSchema, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ revision }),
  });
}

export function deletePipelineRun(id: string) {
  return emptyRequest(`/api/pipeline-runs/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export function usePipelineTemplates() {
  return useQuery({ queryKey: PIPELINE_QUERY_KEYS.templates, queryFn: listPipelineTemplates });
}

export function usePipelineRuns() {
  return useInfiniteQuery({
    queryKey: PIPELINE_QUERY_KEYS.runs,
    initialPageParam: 0,
    queryFn: ({ pageParam }) => listPipelineRuns(50, pageParam),
    getNextPageParam: (page, pages) => {
      const loaded = pages.reduce((total, item) => total + item.runs.length, 0);
      return loaded < page.total ? loaded : undefined;
    },
  });
}

export function usePipelineProposals() {
  return useQuery<PipelineProposalCollections>({ queryKey: PIPELINE_QUERY_KEYS.proposals, queryFn: listPipelineProposals });
}

// Reject and Delete both refetch the two collections rather than editing the
// cache: the server's durable claim decides what became of the offer, so a
// refused action refreshes to the real state instead of a guessed one
// (TS-09.R23/R32, FS-14.R57).
export function useDeclinePipelineProposal() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: declinePipelineProposal,
    onSettled: () => qc.invalidateQueries({ queryKey: PIPELINE_QUERY_KEYS.proposals }),
  });
}

export function useDeletePipelineProposal() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: deletePipelineProposal,
    onSettled: () => qc.invalidateQueries({ queryKey: PIPELINE_QUERY_KEYS.proposals }),
  });
}

export function usePipelineRun(id: string | null) {
  return useQuery({
    queryKey: PIPELINE_QUERY_KEYS.run(id ?? ""),
    queryFn: () => getPipelineRun(id!),
    enabled: Boolean(id),
  });
}

export function useSavePipelineTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, template, exists }: { id: string; template: PipelineTemplate; exists: boolean }) =>
      exists ? updatePipelineTemplate(id, template) : createPipelineTemplate(id, template),
    onSuccess: () => qc.invalidateQueries({ queryKey: PIPELINE_QUERY_KEYS.templates }),
  });
}

export function useDeletePipelineTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: deletePipelineTemplate,
    onSuccess: () => qc.invalidateQueries({ queryKey: PIPELINE_QUERY_KEYS.templates }),
  });
}

function updateRunCaches(qc: ReturnType<typeof useQueryClient>, detail: PipelineRunDetail) {
  qc.setQueryData(PIPELINE_QUERY_KEYS.run(detail.run.run_id), detail);
  void qc.invalidateQueries({ queryKey: PIPELINE_QUERY_KEYS.run(detail.run.run_id) });
  void qc.invalidateQueries({ queryKey: PIPELINE_QUERY_KEYS.runs });
}

export function useStartPipelineRun() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ requestBody, acknowledge }: { requestBody: PipelineStartRequest; acknowledge: boolean }) =>
      startPipelineRun(requestBody, acknowledge),
    onSuccess: (response) => updateRunCaches(qc, response.run),
  });
}

export function usePipelineControl(action: "continue" | "retry" | "stop") {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, revision, input = "" }: { id: string; revision: number; input?: string }) => {
      if (action === "continue") return continuePipelineRun(id, revision, input);
      if (action === "retry") return retryPipelineRun(id, revision);
      return stopPipelineRun(id, revision);
    },
    onSuccess: (detail) => updateRunCaches(qc, detail),
  });
}

export function useDeletePipelineRun() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: deletePipelineRun,
    onSuccess: () => qc.invalidateQueries({ queryKey: PIPELINE_QUERY_KEYS.runs }),
  });
}

export function sharedWorkspaceConflicts(error: unknown) {
  if (!(error instanceof PipelineAPIError)) return [];
  const details = error.body.error?.details;
  return details?.code === "shared_workspace_confirmation_required" ? details.conflicts ?? [] : [];
}

export function pipelineDiagnostics(error: unknown) {
  return error instanceof PipelineAPIError ? error.body.error?.details?.diagnostics ?? [] : [];
}
