import type { AnnotationBatch, ArchiveProjectGroup, ArchiveResult, AvailableCommand, Capabilities, Layout, TrackedCommand, TrackedFile, TranscriptEvent } from "./types";
import type { ProjectResponse, WorktreeStatus } from "../schemas/project";

async function json<T>(input: RequestInfo, init?: RequestInit): Promise<T> {
  const response = await fetch(input, init);
  if (!response.ok) {
    // Session routes return the §7.7 nested envelope { error: { code, message } };
    // surface that message so callers can show a meaningful toast instead of a
    // bare status line. Fall back to the status text if the body isn't JSON.
    let message = `${response.status} ${response.statusText}`;
    try {
      const body = (await response.json()) as { error?: { message?: string } };
      if (body?.error?.message) message = body.error.message;
    } catch {
      /* non-JSON body — keep the status line */
    }
    throw new Error(message);
  }
  return (await response.json()) as T;
}

export function getLayout() {
  return json<Layout>("/api/layout");
}

export function putLayout(layout: Layout) {
  return json<Layout>("/api/layout", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(layout),
  });
}

export function getTranscript(agentId: string, includeMeta = false) {
  const suffix = includeMeta ? "?include_meta=true" : "";
  return json<{ agent_id: string; events: TranscriptEvent[] }>(`/api/sessions/${agentId}/transcript${suffix}`);
}

// launchAgent POSTs a new session (techspec §7.1). Used by Clone to spin up a new
// agent from an existing one's config; the server auto-suggests a name when omitted.
export function launchAgent(body: {
  role: string;
  project: string;
  backend?: string;
  model?: string;
  effort?: string;
  interface?: string;
  name?: string;
  group?: string;
}) {
  return json<{ agent: { agent_id: string; name: string } }>("/api/sessions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function renameAgent(agentId: string, name: string) {
  return json<unknown>(`/api/sessions/${agentId}/rename`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  });
}

export function updateAgentIdentity(agentId: string, body: { name?: string; group?: string }) {
  return json<unknown>(`/api/sessions/${agentId}/identity`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function releaseGroup(group: string) {
  return json<{ group: string; stopped: Array<{ agent_id: string; ok: boolean; error?: string }> }>(
    `/api/groups/${encodeURIComponent(group)}/release`,
    { method: "POST" },
  );
}

export function switchRuntime(agentId: string, body: { interface?: string; backend?: string; model?: string; effort?: string }) {
  return json<{ history_handoff: "native_resume" | "primer" }>(`/api/sessions/${agentId}/switch-runtime`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function getCapabilities() {
  return json<Capabilities>("/api/capabilities");
}

export function stopAgent(agentId: string) {
  return json<unknown>(`/api/sessions/${agentId}/stop`, { method: "POST" });
}

export function sendPrompt(agentId: string, text: string) {
  return json<unknown>(`/api/sessions/${agentId}/prompt`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ text }),
  });
}

export function cancelTurn(agentId: string) {
  return json<unknown>(`/api/sessions/${agentId}/cancel`, { method: "POST" });
}

export function decidePermission(agentId: string, toolCallId: string, decision: "approve" | "deny") {
  return json<unknown>(`/api/sessions/${agentId}/permission`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ tool_call_id: toolCallId, decision }),
  });
}

export function sendAnnotations(agentId: string, batch: AnnotationBatch) {
  return json<{ accepted: true; seq: number }>(`/api/sessions/${agentId}/annotations`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(batch),
  });
}

export function searchArchive(q: string, limit = 50, offset = 0, signal?: AbortSignal) {
  const params = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  if (q) params.set("q", q);
  return json<{ query: string; search_mode: "full_text" | "metadata"; total: number; limit: number; offset: number; results: ArchiveProjectGroup[] }>(
    `/api/archive?${params}`,
    { signal },
  );
}

export function searchArchiveProject(project: string, q: string, limit = 50, offset = 0, signal?: AbortSignal) {
  const params = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  if (q) params.set("q", q);
  return json<{ search_mode: "full_text" | "metadata"; total: number; limit: number; offset: number; results: ArchiveResult[] }>(`/api/archive/projects/${encodeURIComponent(project)}?${params}`, { signal });
}

export function archiveAgent(agentId: string) { return json<unknown>(`/api/sessions/${agentId}/archive`, { method: "POST" }); }
export function restoreAgent(agentId: string) { return json<unknown>(`/api/sessions/${agentId}/restore`, { method: "POST" }); }
// deleteCheckout is sent only when the person consented in the archive dialog;
// its absence never deletes an AgentDeck-owned checkout (FS-19.R8).
export type CheckoutConsent = {
  deleteCheckout: boolean;
  dirtyKnown?: boolean;
  dirty?: boolean;
};

export function archiveProject(project: string, consent: CheckoutConsent = { deleteCheckout: false }) {
  const body: Record<string, boolean> = { delete_checkout: consent.deleteCheckout };
  if (consent.deleteCheckout) {
    body.dirty_known = consent.dirtyKnown ?? false;
    body.dirty = consent.dirty ?? false;
  }
  return json<{
    project: ProjectResponse;
    stopped_agent_ids: string[];
    archived_agent_ids: string[];
    checkout_deleted?: boolean;
    checkout_warning?: string;
  }>(`/api/projects/${encodeURIComponent(project)}/archive`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

// ---- Worktree projects (FS-19 / TS-12 §3) ----

export function getWorktreeStatus(project: string) {
  return json<WorktreeStatus>(`/api/projects/${encodeURIComponent(project)}/worktree`);
}

export function forkWorktreeProject(project: string, body: { title: string; branch: string; base: string }) {
  return json<{ project: ProjectResponse; branch: string; base: string; warning?: string }>(
    `/api/projects/${encodeURIComponent(project)}/worktree-fork`,
    { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) },
  );
}
export function restoreProject(project: string) { return json<ProjectResponse>(`/api/projects/${encodeURIComponent(project)}/restore`, { method: "POST" }); }

export function resumeAgent(agentId: string) {
  return json<{ agent: unknown; running: unknown; status: unknown; resumed: boolean }>(
    `/api/sessions/${agentId}/resume`,
    { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" },
  );
}

export function getTrackedFiles(agentId: string) {
  return json<{ agent_id: string; files: TrackedFile[] }>(`/api/sessions/${agentId}/files`);
}

export function getTrackedCommands(agentId: string) {
  return json<{ agent_id: string; commands: TrackedCommand[] }>(`/api/sessions/${agentId}/commands`);
}

// searchSessionFiles backs the composer `@` picker: bounded, ranked relative paths
// from the chat session's working directory (TS-03.R24). The query is sent as text.
export function searchSessionFiles(agentId: string, query: string) {
  return json<{ agent_id: string; files: string[] }>(
    `/api/sessions/${agentId}/file-search?q=${encodeURIComponent(query)}`,
  );
}

// getAvailableCommands backs the composer `#` picker: the running chat agent's
// latest ACP command/skill snapshot (TS-03.R24). A stopped/non-chat agent errors,
// which the composer treats as "no commands".
export function getAvailableCommands(agentId: string) {
  return json<{ agent_id: string; commands: AvailableCommand[] }>(
    `/api/sessions/${agentId}/available-commands`,
  );
}
