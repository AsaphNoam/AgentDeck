import React from "react";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { TaskConcurrencyEditor } from "./TaskConcurrencyEditor";

let lastPut: unknown;
const configDoc = {
  version: 1,
  port: 4317,
  default_project: "my-app",
  default_role: "implementer",
  skip_permissions: false,
  onboarding_complete: true,
  task_concurrency: 10,
  notifications: { desktop_enabled: true, muted: {} },
  onboarding: { satisfied: true, steps: { backend: { done: true }, project: { done: true }, role: { done: true } } },
};

const server = setupServer(
  http.get("/api/config", () => HttpResponse.json(configDoc)),
  http.put("/api/config", async ({ request }) => {
    lastPut = await request.json();
    return HttpResponse.json({ ...configDoc, ...(lastPut as object) });
  }),
);

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => {
  cleanup();
  lastPut = undefined;
  server.resetHandlers();
});
afterAll(() => server.close());

describe("TaskConcurrencyEditor", () => {
  it("round-trips a positive whole-number budget and refuses zero", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><TaskConcurrencyEditor /></QueryClientProvider>);

    const input = await screen.findByLabelText("Concurrent task runtimes");
    await waitFor(() => expect(input).toHaveValue(10));
    fireEvent.change(input, { target: { value: "0" } });
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    expect(screen.getByText("Enter a positive whole number.")).toBeInTheDocument();

    fireEvent.change(input, { target: { value: "4" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(lastPut).toEqual({ task_concurrency: 4 }));
  });
});
