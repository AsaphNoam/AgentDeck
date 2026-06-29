import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { NotificationsEditor } from "./NotificationsEditor";

let lastPut: unknown;

const configDoc = {
  version: 1,
  port: 4317,
  default_project: "my-app",
  default_role: "implementer",
  skip_permissions: false,
  onboarding_complete: true,
  notifications: {
    desktop_enabled: true,
    muted: { done: false, waiting_input: false, permission_required: false, budget_exceeded: false },
  },
  onboarding: { satisfied: true, steps: { backend: { done: true }, project: { done: true }, role: { done: true } } },
};

const server = setupServer(
  http.get("/api/config", () => HttpResponse.json(configDoc)),
  http.put("/api/config", async ({ request }) => {
    lastPut = await request.json();
    return HttpResponse.json({ ...configDoc, ...(lastPut as object) });
  }),
);

beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterEach(() => {
  cleanup();
  lastPut = undefined;
  server.resetHandlers();
});
afterAll(() => server.close());

function renderWithQuery(ui: React.ReactElement) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

describe("NotificationsEditor", () => {
  it("persists a per-type mute toggle", async () => {
    renderWithQuery(<NotificationsEditor />);
    const done = await screen.findByLabelText("Done");
    fireEvent.click(done);

    await waitFor(() =>
      expect(lastPut).toEqual({
        notifications: {
          desktop_enabled: true,
          muted: { done: true, waiting_input: false, permission_required: false, budget_exceeded: false },
        },
      }),
    );
  });
});
