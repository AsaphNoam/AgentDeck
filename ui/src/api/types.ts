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
  tty?: string;
  driver?: string;
  state: AgentStatus;
  detail: string;
  last_trace?: string;
  busy_since?: string;
  context_pct: number;
  unread_messages?: number;
  last_sent_at?: string;
  updated_at: number;
  removed?: boolean;
  hydrated?: boolean;
}

export type NotificationType = "done" | "waiting_input" | "permission_required" | "budget_exceeded";

export interface NotificationPayload {
  type: "notification";
  notification_type: NotificationType;
  agent_id: string;
  agent_name?: string;
  address?: string;
  title: string;
  body?: string;
  detail?: Record<string, unknown>;
  ts: string;
}

export interface BusEvent<T = unknown> {
  type: "state_update" | "new_message" | "notification" | "ping";
  seq: number;
  ts: number;
  agent_id: string | null;
  data: T;
}

// RuntimeEvent is the raw wire shape emitted by the Go runtime (event.go) and
// delivered both over SSE `new_message` and by GET /api/sessions/{id}/transcript.
// The type-specific payload lives nested under `data` — it is NOT flattened on
// the wire. The UI must normalize this into a flat TranscriptEvent before render.
export interface RuntimeEvent {
  agent_id: string;
  seq: number;
  type: string;
  ts: string;
  data: Record<string, unknown>;
}

// TranscriptEvent is the flat, render-ready shape the store and renderers consume:
// `kind` plus the payload fields spread to the top level. normalizeEvent() maps a
// RuntimeEvent into this; locally-created events (e.g. the optimistic user message)
// are authored directly in this shape.
export interface TranscriptEvent {
  kind?: string;
  type?: string;
  seq?: number;
  ts?: string;
  message_id?: string;
  text?: string;
  delta?: string;
  resolved?: "approve" | "deny";
  data?: unknown;
  [key: string]: unknown;
}

export interface Layout {
  order: string[];
  density: { perRow: number; gap: number };
  groups?: Record<string, { collapsed: boolean }>;
}

export interface Capabilities {
  terminal: {
    available: boolean;
    default_driver: string;
    drivers: Record<string, boolean | { available: boolean; reason?: string }>;
  };
}

export interface ArchiveResult {
  agent_id: string;
  name: string;
  role: string;
  project: string;
  backend: string;
  model: string;
  interface: string;
  group?: string;
  created_at: string;
  updated_at: string;
  turn_count: number;
  files_touched: number;
  commands_run: number;
  active: boolean;
  matched_in?: string[];
  snippet?: string;
}

export interface DiffRef {
  seq: number;
  tool_call_id: string;
}

export interface TrackedFile {
  path: string;
  edit_count: number;
  first_seq: number;
  last_seq: number;
  first_ts: string;
  last_ts: string;
  has_diff: boolean;
  diff_refs: DiffRef[];
}

export interface TrackedCommand {
  command: string;
  seq: number;
  ts: string;
  tool_call_id: string;
  exit_status: string;
  exit_error: string;
}
