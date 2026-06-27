import type { Layout, TranscriptEvent } from "./types";

async function json<T>(input: RequestInfo, init?: RequestInit): Promise<T> {
  const response = await fetch(input, init);
  if (!response.ok) throw new Error(`${response.status} ${response.statusText}`);
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
