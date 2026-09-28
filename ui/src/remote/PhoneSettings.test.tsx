import React from "react";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { PhoneSettings } from "./PhoneSettings";
import { useConnection } from "./connection";

let notifications = "off";
const calls: string[] = [];
const server = setupServer(
  http.get("/api/remote/self", () => HttpResponse.json({ id: "d1", name: "Pixel", notifications, vapid_public_key: "BAAA" })),
  http.put("/api/remote/self/push", async ({ request }) => {
    calls.push(`put ${await request.text()}`);
    notifications = "on";
    return new HttpResponse(null, { status: 204 });
  }),
  http.delete("/api/remote/self", () => {
    calls.push("unpair");
    return new HttpResponse(null, { status: 204 });
  }),
);

const subscription = { toJSON: () => ({ endpoint: "https://fcm.googleapis.com/fcm/send/x", keys: { p256dh: "k", auth: "a" } }), unsubscribe: vi.fn() };

beforeAll(() => {
  server.listen({ onUnhandledRequest: "error" });
  Object.assign(window, { PushManager: class {} });
  Object.assign(window, { Notification: { permission: "default", requestPermission: vi.fn(async () => "granted") } });
  Object.defineProperty(navigator, "serviceWorker", {
    configurable: true,
    value: {
      ready: Promise.resolve({ pushManager: { getSubscription: async () => null, subscribe: vi.fn(async () => subscription) } }),
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
    },
  });
});
beforeEach(() => {
  notifications = "off";
  calls.length = 0;
  useConnection.setState({ link: "connected" });
});
afterEach(() => {
  cleanup();
  server.resetHandlers();
});
afterAll(() => server.close());

function renderSettings() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <PhoneSettings />
    </QueryClientProvider>,
  );
}

describe("PhoneSettings", () => {
  it("subscribes this phone to attention notifications", async () => {
    renderSettings();
    fireEvent.click(await screen.findByRole("button", { name: "Turn notifications on" }));
    await waitFor(() => expect(calls).toContain('put {"endpoint":"https://fcm.googleapis.com/fcm/send/x","keys":{"p256dh":"k","auth":"a"}}'));
    expect(await screen.findByRole("button", { name: "Turn notifications off" })).toBeInTheDocument();
  });

  it("offers to turn lapsed notifications back on", async () => {
    notifications = "expired";
    renderSettings();
    expect(await screen.findByRole("button", { name: "Turn notifications back on" })).toBeInTheDocument();
  });

  it("unpairs itself", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    renderSettings();
    fireEvent.click(await screen.findByRole("button", { name: "Unpair this phone" }));
    await waitFor(() => expect(useConnection.getState().link).toBe("unpaired"));
    expect(calls).toContain("unpair");
  });
});
