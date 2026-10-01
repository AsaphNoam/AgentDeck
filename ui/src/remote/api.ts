// Phone-side calls to the tailnet-only routes (TS-13 §3). Shared session, task,
// and pipeline calls come from ui/src/api; the device cookie rides along
// automatically on this same origin.

export class PhoneAPIError extends Error {
  status: number;
  code: string;
  /** The typed refusal in `error.details.code`, when the Mac sends one. */
  detailCode: string;

  constructor(status: number, code: string, message: string, detailCode = "") {
    super(message);
    this.status = status;
    this.code = code;
    this.detailCode = detailCode;
  }
}

/** Thrown when the Mac could not be reached at all (FS-20.R23). */
export class MacUnreachableError extends Error {}

export async function phoneFetch<T>(url: string, init?: RequestInit): Promise<T> {
  let response: Response;
  try {
    response = await fetch(url, { credentials: "same-origin", ...init });
  } catch {
    throw new MacUnreachableError("The Mac is unreachable.");
  }
  if (!response.ok) {
    const body = (await response.json().catch(() => ({}))) as { error?: { code?: string; message?: string; details?: { code?: unknown } } };
    if (response.status >= 502 && !body.error) throw new MacUnreachableError("The Mac is unreachable.");
    const detailCode = typeof body.error?.details?.code === "string" ? body.error.details.code : "";
    throw new PhoneAPIError(response.status, body.error?.code || "error", body.error?.message || `${response.status} ${response.statusText}`, detailCode);
  }
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

const json = (method: string, body?: unknown): RequestInit => ({
  method,
  headers: { "Content-Type": "application/json" },
  body: body === undefined ? undefined : JSON.stringify(body),
});

export interface AttentionItem {
  kind: "agent" | "task" | "run";
  id: string;
  title: string;
  project: string;
  state: string;
  reason: string;
  agent_id?: string;
  stage_number?: number;
  stage_count?: number;
  outcome?: string;
  since: string;
}

export interface HomeLists {
  needs_you: AttentionItem[];
  active_runs: AttentionItem[];
}

/** The Mac's secret-free runtime catalog for Replace orchestrator (TS-13.R15). */
export interface RuntimeOptions {
  backends: {
    id: string;
    name: string;
    default?: boolean;
    models: { id: string; name: string; efforts: string[]; default_effort?: string; default_model?: boolean; fast: boolean }[];
  }[];
}

export const getRuntimeOptions = () => phoneFetch<RuntimeOptions>("/api/remote/runtime-options");
export const getHome = () => phoneFetch<HomeLists>("/api/remote/home");
export const checkPaired = () => phoneFetch<unknown>("/api/health");
export const claimPairing = (code: string, name: string) =>
  phoneFetch<{ pending_id: string }>("/api/remote/pair", json("POST", { code, name }));
export const waitPairing = (pendingID: string) =>
  phoneFetch<{ status: "waiting" | "allowed" }>(`/api/remote/pair/${encodeURIComponent(pendingID)}`);
export const unpairSelf = () => phoneFetch<void>("/api/remote/self", json("DELETE"));
