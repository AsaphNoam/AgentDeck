import React from "react";
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { RemoteEditor } from "./RemoteEditor";
import type { RemoteStatus } from "../../api/remote";

let status: RemoteStatus;
const calls: string[] = [];

const server = setupServer(
  http.get("/api/remote", () => HttpResponse.json(status)),
  http.put("/api/remote", async ({ request }) => {
    const body = (await request.json()) as { enabled?: boolean; keep_awake?: boolean };
    calls.push(`put ${JSON.stringify(body)}`);
    if (body.enabled !== undefined) status = { ...status, state: body.enabled ? "starting" : "off" };
    if (body.keep_awake !== undefined) status = { ...status, keep_awake: body.keep_awake };
    return HttpResponse.json(status);
  }),
  http.post("/api/remote/pairings", () => {
    calls.push("pair");
    return HttpResponse.json({
      id: "p1",
      code: "ABCD2345",
      qr_url: "https://agentdeck.tail.ts.net/pair#ABCD2345",
      qr_svg: "<svg></svg>",
      expires_at: new Date(Date.now() + 5 * 60_000).toISOString(),
    });
  }),
  http.post("/api/remote/pairings/:id/:decision", ({ params }) => {
    calls.push(`${params.decision} ${params.id}`);
    return HttpResponse.json({});
  }),
  http.delete("/api/remote/devices/:id", ({ params }) => {
    calls.push(`revoke ${params.id}`);
    return new HttpResponse(null, { status: 204 });
  }),
);

const base: RemoteStatus = { state: "off", keep_awake: false, keep_awake_available: true, devices: [] };

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => {
  cleanup();
  calls.length = 0;
  server.resetHandlers();
});
afterAll(() => server.close());

function renderEditor() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <RemoteEditor />
    </QueryClientProvider>,
  );
}

describe("RemoteEditor", () => {
  it("states the certificate-log disclosure and turns remote control on", async () => {
    status = { ...base };
    renderEditor();
    expect(await screen.findByText(/public certificate-transparency logs/)).toBeInTheDocument();
    expect(screen.getByText("Off")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("checkbox", { name: "Remote control" }));
    await waitFor(() => expect(calls).toContain('put {"enabled":true}'));
    expect(await screen.findByText("Connecting…")).toBeInTheDocument();
  });

  it("shows sign-in and each repair", async () => {
    status = { ...base, state: "needs_login", auth_url: "https://login.tailscale.com/a/x" };
    renderEditor();
    expect(await screen.findByRole("link", { name: "Sign in to Tailscale" })).toHaveAttribute("href", "https://login.tailscale.com/a/x");
    cleanup();
    status = { ...base, state: "unavailable", reason: "magicdns_disabled" };
    renderEditor();
    expect(await screen.findByText(/Turn on MagicDNS/)).toBeInTheDocument();
    cleanup();
    status = { ...base, state: "unavailable", reason: "https_disabled" };
    renderEditor();
    expect(await screen.findByText(/Turn on HTTPS certificates/)).toBeInTheDocument();
  });

  it("pairs a phone with a code and answers the request", async () => {
    status = { ...base, state: "on", address: "https://agentdeck.tail.ts.net" };
    renderEditor();
    expect(await screen.findByText("https://agentdeck.tail.ts.net")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Pair a phone" }));
    expect(await screen.findByLabelText("Pairing code")).toHaveTextContent("ABCD2345");
    expect(screen.getByAltText("Pairing QR code")).toBeInTheDocument();
  });

  it("allows a pending phone and revokes a paired one", async () => {
    status = {
      ...base,
      state: "on",
      address: "https://agentdeck.tail.ts.net",
      pending_pairing: { id: "req1", name: "Pixel 9", requested_at: new Date().toISOString() },
      devices: [
        { id: "d1", name: "iPhone", paired_at: new Date().toISOString(), last_seen_at: new Date().toISOString(), notifications: "off" },
      ],
    };
    vi.spyOn(window, "confirm").mockReturnValue(true);
    renderEditor();
    expect(await screen.findByText("Allow this phone?")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Allow" }));
    await waitFor(() => expect(calls).toContain("allow req1"));
    fireEvent.click(screen.getByRole("button", { name: "Revoke" }));
    await waitFor(() => expect(calls).toContain("revoke d1"));
  });

  it("disables keep-awake off macOS and saves it on macOS", async () => {
    status = { ...base, keep_awake_available: false };
    renderEditor();
    expect(await screen.findByRole("checkbox", { name: /Keep this Mac awake/ })).toBeDisabled();
    cleanup();
    status = { ...base };
    renderEditor();
    fireEvent.click(await screen.findByRole("checkbox", { name: /Keep this Mac awake/ }));
    await waitFor(() => expect(calls).toContain('put {"keep_awake":true}'));
  });
});
