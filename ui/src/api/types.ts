export type AgentStatus = "busy" | "idle" | "waiting_input" | "done" | "error" | "unknown";

export interface AgentState {
  agent_id: string;
  name: string;
  role: string;
  project: string;
  backend: string;
  model: string;
  effort?: string;
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
	archived: boolean;
  removed?: boolean;
  hydrated?: boolean;
  pipeline?: {
    run_id: string;
    run_name: string;
    stage_id: string;
    attempt_id: string;
    attempt_no: number;
  };
}

export type NotificationType = "done" | "waiting_input" | "permission_required" | "budget_exceeded" | "pipeline_needs_attention" | "pipeline_completed";

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
  type: "state_update" | "new_message" | "notification" | "pipeline_update" | "ping";
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
  resolved?: PermissionResolution;
  data?: unknown;
  [key: string]: unknown;
}

export type PermissionResolution = "approve" | "deny" | "cancelled" | "timeout";

export interface AnnotationDraft {
  seq: number;
  excerpt: string;
  instruction: string;
  path?: string;
  side?: "old" | "new";
  start_line?: number;
  end_line?: number;
}

export interface AnnotationTarget {
  kind: "self" | "agent";
  agent_id?: string;
}

export interface AnnotationBatch {
  annotations: AnnotationDraft[];
  overall_instruction?: string;
  target: AnnotationTarget;
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
  effort?: string;
  interface: string;
  group?: string;
  created_at: string;
  updated_at: string;
  turn_count: number;
  files_touched: number;
  commands_run: number;
  active: boolean;
	archived: boolean;
  matched_in?: string[];
  snippet?: string;
}

export interface ArchiveProjectGroup {
  project: string;
  title: string;
  color: [number, number, number];
  project_status: "active" | "archived" | "missing";
  archived_agent_count: number;
  results: ArchiveResult[];
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
