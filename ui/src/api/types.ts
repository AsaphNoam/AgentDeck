export type AgentStatus = "busy" | "idle" | "waiting_input" | "done" | "error" | "unknown";

export interface AgentState {
  agent_id: string;
  name: string;
  role: string;
  project: string;
  backend: string;
  model: string;
  interface: string;
  group?: string;
  created_at: string;
  running: boolean;
  pid?: number;
  session_id?: string;
  started_at?: string;
  state: AgentStatus;
  detail: string;
  last_trace?: string;
  busy_since?: string;
  context_pct: number;
  updated_at: number;
  removed?: boolean;
  hydrated?: boolean;
}

export interface BusEvent<T = unknown> {
  type: "state_update" | "new_message" | "notification" | "ping";
  seq: number;
  ts: number;
  agent_id: string | null;
  data: T;
}

export interface TranscriptEvent {
  kind?: string;
  type?: string;
  message_id?: string;
  text?: string;
  delta?: string;
  data?: unknown;
  [key: string]: unknown;
}

export interface Layout {
  order: string[];
  density: { perRow: number; gap: number };
}
