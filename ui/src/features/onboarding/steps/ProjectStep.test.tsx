import React from "react";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { ProjectStep } from "./ProjectStep";

const server = setupServer();

beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterEach(() => { cleanup(); server.resetHandlers(); });
afterAll(() => server.close());

describe("ProjectStep", () => {
  // FS-04.A19: onboarding uses the same six-preset picker and sends its RGB
  // triple through the unchanged project API.
  it("defaults to Slate and submits a selected preset", async () => {
    let created: Record<string, unknown> | null = null;
    server.use(http.post("/api/projects", async ({ request }) => {
      created = await request.json() as Record<string, unknown>;
      return HttpResponse.json({ project: "app", ...created }, { status: 201 });
    }));
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={queryClient}><ProjectStep onDone={() => {}} /></QueryClientProvider>);

    expect(screen.getAllByRole("button", { name: /project color$/ })).toHaveLength(6);
    expect(screen.getByRole("button", { name: "Slate project color" })).toHaveAttribute("aria-pressed", "true");
    fireEvent.change(screen.getByPlaceholderText("e.g. My App"), { target: { value: "App" } });
    fireEvent.change(screen.getByPlaceholderText("~/Projects/my-app"), { target: { value: "/tmp/app" } });
    fireEvent.click(screen.getByRole("button", { name: "Green project color" }));
    fireEvent.click(screen.getByRole("button", { name: "Create project" }));

    await waitFor(() => expect(created).toMatchObject({ color: [34, 197, 94] }));
  });
});
