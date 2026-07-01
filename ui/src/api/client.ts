import type { ArchiveResult, Capabilities, Layout, TrackedCommand, TrackedFile, TranscriptEvent } from "./types";

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

export function getTranscript(agentId: string) {
  return json<{ agent_id: string; events: TranscriptEvent[] }>(`/api/sessions/${agentId}/transcript`);
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

export function switchRuntime(agentId: string, body: { interface?: string; backend?: string; model?: string }) {
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

export function searchArchive(q: string, limit = 50, offset = 0, signal?: AbortSignal) {
  const params = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  if (q) params.set("q", q);
  return json<{ query: string; total: number; limit: number; offset: number; results: ArchiveResult[] }>(
    `/api/archive?${params}`,
    { signal },
  );
}

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
