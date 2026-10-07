import { useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { z } from "zod";
import {
  thinkTankActivitySchema,
  thinkTankDetailSchema,
  thinkTankEntrySchema,
  thinkTankSourceSchema,
  thinkTankSummarySchema,
  type ThinkTankActivity,
  type ThinkTankEntry,
} from "../schemas/thinkTank";
import { TaskAPIError } from "./tasks";

/** Think Tank REST (TS-14 §3). SSE `think_tank_update` / `think_tank_activity`
 *  only invalidate these queries; REST stays the committed history
 *  (TS-14.R16). */

export const THINK_TANK_KEYS = {
  all: ["think-tanks"] as const,
  list: (project: string) => ["think-tanks", "list", project] as const,
  room: (id: string) => ["think-tanks", "room", id] as const,
  entries: (id: string) => ["think-tanks", "room", id, "entries"] as const,
  activity: (id: string) => ["think-tanks", "room", id, "activity"] as const,
  files: (id: string) => ["think-tanks", "room", id, "files"] as const,
  commands: (id: string) => ["think-tanks", "room", id, "commands"] as const,
  results: (agentID: string) => ["think-tanks", "results", agentID] as const,
};

export const thinkTankResultSchema = z.object({
  result_id: z.number(),
  room_id: z.string(),
  room_title: z.string(),
  room_available: z.boolean(),
  entry_seq: z.number(),
  attempt_id: z.string(),
  body: z.string(),
  event_seq: z.number(),
  completed_at: z.string(),
});
export type ThinkTankResult = z.output<typeof thinkTankResultSchema>;

/** useThinkTankResults reads a judge's retained synthesis results for its
 *  ordinary chat and archive (FS-21.R50, TS-14.R26). Room updates refetch it. */
export function useThinkTankResults(agentID: string) {
  return useQuery({
    queryKey: THINK_TANK_KEYS.results(agentID),
    enabled: Boolean(agentID),
    queryFn: async ({ signal }) => {
      const page = z.object({ results: z.array(thinkTankResultSchema), complete: z.boolean(), next_after: z.number().optional() });
      const all: ThinkTankResult[] = [];
      let after = 0;
      for (;;) {
        const data = await request(`/api/sessions/${encodeURIComponent(agentID)}/think-tank-results?after=${after}`, page, { signal });
        all.push(...data.results);
        if (data.complete || data.next_after === undefined) return all;
        after = data.next_after;
      }
    },
  });
}

/** Browser-held room history is bounded; longer rooms show their newest part
 *  and say so (TS-14.R16, INV §16). */
export const THINK_TANK_ENTRY_WINDOW = 5000;
export const THINK_TANK_ACTIVITY_WINDOW = 5000;

export interface ThinkTankLaunch {
  role?: string;
  project: string;
  backend?: string;
  model?: string;
  effort?: string;
  fast?: boolean;
  name?: string;
}

export interface ThinkTankParticipantInput {
  agent_id?: string;
  new?: ThinkTankLaunch;
  limit: number;
  may_leave: boolean;
}

export interface CreateThinkTankInput {
  command_id: string;
  title: string;
  goal: string;
  origin_project: string;
  openings: boolean;
  judge?: ThinkTankLaunch;
  participants: ThinkTankParticipantInput[];
}

async function request<S extends z.ZodTypeAny>(url: string, schema: S, init?: RequestInit): Promise<z.output<S>> {
  const response = await fetch(url, init);
  if (!response.ok) {
    const body = await response.json().catch(() => ({})) as {
      error?: { code?: string; message?: string; details?: { code?: string } };
    };
    throw new TaskAPIError(
      response.status,
      body.error?.details?.code || body.error?.code || "error",
      body.error?.message || `${response.status} ${response.statusText}`,
    );
  }
  if (response.status === 204) return undefined as z.output<S>;
  return schema.parse(await response.json());
}

const post = (body: unknown): RequestInit => ({
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify(body),
});

const roomURL = (id: string, suffix = "") => `/api/think-tanks/${encodeURIComponent(id)}${suffix}`;

export function newCommandID(): string {
  return typeof crypto !== "undefined" && "randomUUID" in crypto
    ? crypto.randomUUID()
    : `cmd-${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

export function useThinkTanks(project?: string, enabled = true) {
  return useQuery({
    queryKey: THINK_TANK_KEYS.list(project ?? ""),
    enabled,
    queryFn: ({ signal }) => request(
      `/api/think-tanks${project ? `?project=${encodeURIComponent(project)}` : ""}`,
      z.object({ rooms: z.array(thinkTankSummarySchema) }),
      { signal },
    ),
    select: (data) => data.rooms,
  });
}

export function useThinkTank(id: string | undefined) {
  return useQuery({
    queryKey: THINK_TANK_KEYS.room(id ?? ""),
    enabled: Boolean(id),
    queryFn: ({ signal }) => request(roomURL(id ?? ""), thinkTankDetailSchema, { signal }),
  });
}

/** fetchAllEntries pages published entries in order until complete, keeping
 *  only the newest window. */
async function fetchAllEntries(id: string, signal: AbortSignal): Promise<{ entries: ThinkTankEntry[]; clipped: boolean }> {
  let after = 0;
  let entries: ThinkTankEntry[] = [];
  let clipped = false;
  for (;;) {
    const page = await request(roomURL(id, `/entries?after=${after}`),
      z.object({ entries: z.array(thinkTankEntrySchema), complete: z.boolean() }), { signal });
    entries = entries.concat(page.entries);
    if (entries.length > THINK_TANK_ENTRY_WINDOW) {
      entries = entries.slice(-THINK_TANK_ENTRY_WINDOW);
      clipped = true;
    }
    if (page.complete || page.entries.length === 0) return { entries, clipped };
    after = page.entries[page.entries.length - 1].seq;
  }
}

export function useThinkTankEntries(id: string | undefined) {
  return useQuery({
    queryKey: THINK_TANK_KEYS.entries(id ?? ""),
    enabled: Boolean(id),
    queryFn: ({ signal }) => fetchAllEntries(id ?? "", signal),
  });
}

type ActivityWindow = { activity: ThinkTankActivity[]; clipped: boolean };

/** A live activity event refills only the records after the cached tail;
 *  every other refetch (room update, reconnect, or a walk that never
 *  finished) walks the whole window, because publishing a blind opening
 *  reveals records older than that tail (TS-14.R16, INV §16). */
const activityTailRooms = new Set<string>();
const activityWalkRooms = new Set<string>();

export function noteThinkTankActivity(qc: QueryClient, id: string) {
  if (qc.getQueryData(THINK_TANK_KEYS.activity(id))) activityTailRooms.add(id);
  void qc.invalidateQueries({ queryKey: THINK_TANK_KEYS.activity(id) });
}

async function fetchActivity(qc: QueryClient, id: string, signal: AbortSignal): Promise<ActivityWindow> {
  const cached = qc.getQueryData<ActivityWindow>(THINK_TANK_KEYS.activity(id));
  if (activityTailRooms.delete(id) && cached && !activityWalkRooms.has(id)) {
    return fetchAllActivity(id, signal, cached);
  }
  activityWalkRooms.add(id);
  const result = await fetchAllActivity(id, signal);
  activityWalkRooms.delete(id);
  return result;
}

export async function fetchAllActivity(id: string, signal: AbortSignal, from?: ActivityWindow): Promise<ActivityWindow> {
  let out: ThinkTankActivity[] = from?.activity ?? [];
  let after = out.length ? out[out.length - 1].seq : 0;
  let clipped = from?.clipped ?? false;
  for (;;) {
    const page = await request(roomURL(id, `/activity?after=${after}`),
      z.object({ activity: z.array(thinkTankActivitySchema), complete: z.boolean() }), { signal });
    out = out.concat(page.activity);
    if (out.length > THINK_TANK_ACTIVITY_WINDOW) { out = out.slice(-THINK_TANK_ACTIVITY_WINDOW); clipped = true; }
    if (page.complete || page.activity.length === 0) return { activity: out, clipped };
    after = page.activity[page.activity.length - 1].seq;
  }
}

export function useThinkTankActivity(id: string | undefined) {
  const qc = useQueryClient();
  return useQuery({
    queryKey: THINK_TANK_KEYS.activity(id ?? ""),
    enabled: Boolean(id),
    queryFn: ({ signal }) => fetchActivity(qc, id ?? "", signal),
  });
}

export const thinkTankFileSchema = z.object({
  source_id: z.string(),
  agent_name: z.string(),
  project: z.string(),
  path: z.string(),
  last_seq: z.number(),
});

export const thinkTankCommandSchema = z.object({
  source_id: z.string(),
  agent_name: z.string(),
  project: z.string(),
  tool_call_id: z.string(),
  command: z.string(),
  status: z.string().optional().default(""),
  seq: z.number(),
});

export function useThinkTankFiles(id: string | undefined, enabled: boolean) {
  return useQuery({
    queryKey: THINK_TANK_KEYS.files(id ?? ""),
    enabled: Boolean(id) && enabled,
    queryFn: ({ signal }) => request(roomURL(id ?? "", "/files"),
      z.object({ sources: z.array(thinkTankSourceSchema), files: z.array(thinkTankFileSchema), clipped: z.boolean().optional().default(false) }), { signal }),
  });
}

export function useThinkTankCommands(id: string | undefined, enabled: boolean) {
  return useQuery({
    queryKey: THINK_TANK_KEYS.commands(id ?? ""),
    enabled: Boolean(id) && enabled,
    queryFn: ({ signal }) => request(roomURL(id ?? "", "/commands"),
      z.object({ sources: z.array(thinkTankSourceSchema), commands: z.array(thinkTankCommandSchema), clipped: z.boolean().optional().default(false) }), { signal }),
  });
}

export function sourceFileURL(roomID: string, sourceID: string, path: string): string {
  return roomURL(roomID, `/sources/${encodeURIComponent(sourceID)}/file?path=${encodeURIComponent(path)}`);
}

function useRoomMutation<Input, Output>(run: (input: Input) => Promise<Output>) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: run,
    onSettled: () => {
      client.invalidateQueries({ queryKey: THINK_TANK_KEYS.all });
    },
  });
}

export function useCreateThinkTank() {
  return useRoomMutation((input: CreateThinkTankInput) =>
    request("/api/think-tanks", thinkTankDetailSchema, post(input)));
}

export function useThinkTankControl(id: string) {
  return useRoomMutation((action: "pause" | "resume" | "end") =>
    request(roomURL(id, `/${action}`), thinkTankDetailSchema, post({})));
}

export function useThinkTankRetry(id: string) {
  return useRoomMutation((input: { target: "setup" | "turn" | "judge"; judge?: ThinkTankLaunch }) =>
    request(roomURL(id, "/retry"), thinkTankDetailSchema, post(input)));
}

export function useThinkTankTurnLimit(id: string) {
  return useRoomMutation((input: { agent_id: string; command_id: string; expected_limit: number; limit: number }) =>
    request(roomURL(id, `/participants/${encodeURIComponent(input.agent_id)}/turn-limit`), thinkTankDetailSchema,
      post({ command_id: input.command_id, expected_limit: input.expected_limit, limit: input.limit })));
}

export function useThinkTankMessage(id: string) {
  return useRoomMutation((input: { command_id: string; body: string }) =>
    request(roomURL(id, "/messages"), z.object({ input_id: z.string() }).passthrough(), post(input)));
}

export interface RoomAnnotationWire {
  anchor: "entry" | "activity" | "file";
  seq?: number;
  source_id?: string;
  path?: string;
  side?: "old" | "new";
  start_line?: number;
  end_line?: number;
  excerpt: string;
  instruction: string;
}

export interface RoomAnnotationInput {
  command_id: string;
  annotations: RoomAnnotationWire[];
  overall_instruction?: string;
  target: { kind: "room" } | { kind: "agent"; agent_id: string };
}

export function useThinkTankAnnotations(id: string) {
  return useRoomMutation((input: RoomAnnotationInput) =>
    request(roomURL(id, "/annotations"), z.object({ input_id: z.string() }).passthrough(), post(input)));
}

export function useDeleteThinkTank(id: string) {
  return useRoomMutation(() => request(roomURL(id), z.undefined(), { method: "DELETE" }));
}
