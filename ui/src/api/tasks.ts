import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { z } from "zod";
import { taskListSchema, taskSchema, type Task } from "../schemas/task";

export const TASK_QUERY_KEYS = {
  all: ["tasks"] as const,
  project: (project: string) => ["tasks", project] as const,
};

export interface TaskArmInput {
  kind: "work_result" | "signal";
  source_kind?: string;
  source_id?: string;
  satisfying_outcomes?: string[];
  signal_name?: string;
}

export interface CreateTaskInput {
  project: string;
  display_name: string;
  instruction: string;
  target_kind: "agent" | "launch";
  target_agent_id?: string;
  role?: string;
  backend?: string;
  model?: string;
  effort?: string;
  arms?: TaskArmInput[];
	attachments?: { context_ref_id: string; label?: string; description?: string }[];
}

/** TaskAPIError carries the shared envelope's stable code so a caller can act on
 *  the refusal rather than on its wording (TS-03.R3, FS-16.R20). */
export class TaskAPIError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
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

const json = (body: unknown): RequestInit => ({
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify(body),
});

export function useTasks(project: string | undefined) {
  return useQuery({
    queryKey: TASK_QUERY_KEYS.project(project ?? ""),
    enabled: Boolean(project),
    queryFn: () => request(`/api/tasks?project=${encodeURIComponent(project!)}`, taskListSchema),
    select: (data: { tasks: Task[] }) => data.tasks,
  });
}

/** useTaskAction is one hook for every per-task control, because they differ
 *  only in route and body and all invalidate the same list. */
function useTaskAction<Input>(
  project: string | undefined,
  run: (input: Input) => Promise<unknown>,
) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: run,
    onSuccess: () => {
      client.invalidateQueries({ queryKey: TASK_QUERY_KEYS.project(project ?? "") });
    },
  });
}

export function useCreateTask(project: string | undefined) {
  return useTaskAction(project, (input: CreateTaskInput) =>
    request("/api/tasks", taskSchema, json(input)));
}

export function useCancelTask(project: string | undefined) {
  return useTaskAction(project, (taskID: string) =>
    request(`/api/tasks/${encodeURIComponent(taskID)}/cancel`, taskSchema, json({})));
}

export function useRetryTask(project: string | undefined) {
  return useTaskAction(project, (taskID: string) =>
    request(`/api/tasks/${encodeURIComponent(taskID)}/retry`, taskSchema, json({})));
}

export function useRecordTaskResult(project: string | undefined) {
  return useTaskAction(project, (input: { taskID: string; outcome: string; summary: string; details?: string }) =>
    request(`/api/tasks/${encodeURIComponent(input.taskID)}/result`, taskSchema,
      json({ outcome: input.outcome, summary: input.summary, details: input.details })));
}

export function useRearmTask(project: string | undefined) {
  return useTaskAction(project, (input: { taskID: string; arms: TaskArmInput[] }) =>
    request(`/api/tasks/${encodeURIComponent(input.taskID)}/rearm`, taskSchema, json({ arms: input.arms })));
}

export function useDeleteTask(project: string | undefined) {
  return useTaskAction(project, (taskID: string) =>
    request(`/api/tasks/${encodeURIComponent(taskID)}`, z.undefined(), { method: "DELETE" }));
}

export function useFireSignal(project: string | undefined) {
  return useTaskAction(project, (name: string) =>
    request("/api/signals", z.object({ released: z.number() }), json({ project, name })));
}
