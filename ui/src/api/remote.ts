import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

// Loopback-only remote-control management (TS-13 §3). The phone app never uses
// these routes; the tailnet listener refuses them.

export const REMOTE_QUERY_KEY = ["remote"] as const;

export type RemoteState = "off" | "needs_login" | "starting" | "on" | "unavailable";

export interface RemoteDevice {
  id: string;
  name: string;
  paired_at: string;
  last_seen_at: string;
  notifications: "on" | "off" | "expired";
}

export interface RemotePendingPairing {
  id: string;
  name: string;
  login?: string;
  requested_at: string;
}

export interface RemoteStatus {
  state: RemoteState;
  reason?: string;
  auth_url?: string;
  address?: string;
  keep_awake: boolean;
  keep_awake_available: boolean;
  pending_pairing?: RemotePendingPairing;
  devices: RemoteDevice[];
}

export interface RemotePairingCode {
  id: string;
  code: string;
  qr_url: string;
  qr_svg: string;
  expires_at: string;
}

export class RemoteAPIError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, init);
  if (!response.ok) {
    const body = (await response.json().catch(() => ({}))) as { error?: { code?: string; message?: string } };
    throw new RemoteAPIError(
      response.status,
      body.error?.code || "error",
      body.error?.message || `${response.status} ${response.statusText}`,
    );
  }
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

const jsonInit = (method: string, body?: unknown): RequestInit => ({
  method,
  headers: { "Content-Type": "application/json" },
  body: body === undefined ? undefined : JSON.stringify(body),
});

export function useRemote() {
  return useQuery({ queryKey: REMOTE_QUERY_KEY, queryFn: () => request<RemoteStatus>("/api/remote") });
}

function useRemoteMutation<V, R>(fn: (vars: V) => Promise<R>) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSettled: () => client.invalidateQueries({ queryKey: REMOTE_QUERY_KEY }),
  });
}

export function usePutRemote() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (body: { enabled?: boolean; keep_awake?: boolean }) =>
      request<RemoteStatus>("/api/remote", jsonInit("PUT", body)),
    onSuccess: (status) => client.setQueryData(REMOTE_QUERY_KEY, status),
    onSettled: () => client.invalidateQueries({ queryKey: REMOTE_QUERY_KEY }),
  });
}

export function useCreatePairing() {
  return useRemoteMutation(() => request<RemotePairingCode>("/api/remote/pairings", jsonInit("POST")));
}

export function usePairingDecision() {
  return useRemoteMutation(({ id, allow }: { id: string; allow: boolean }) =>
    request<unknown>(`/api/remote/pairings/${encodeURIComponent(id)}/${allow ? "allow" : "decline"}`, jsonInit("POST")),
  );
}

export function useRenameDevice() {
  return useRemoteMutation(({ id, name }: { id: string; name: string }) =>
    request<void>(`/api/remote/devices/${encodeURIComponent(id)}`, jsonInit("PATCH", { name })),
  );
}

export function useRevokeDevice() {
  return useRemoteMutation((id: string) =>
    request<void>(`/api/remote/devices/${encodeURIComponent(id)}`, jsonInit("DELETE")),
  );
}
