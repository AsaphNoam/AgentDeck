import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach, vi } from "vitest";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { ProjectStep } from "./ProjectStep";

const server = setupServer();

beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterEach(() => {
  cleanup();
  server.resetHandlers();
});
afterAll(() => server.close());

function renderWithQuery(ui: React.ReactElement) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: 0 }, mutations: { retry: 0 } } });
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

describe("ProjectStep", () => {
  // FS-04.R26 / INV §8: an over-long title is repairable, so the wizard must
  // show the rejected field and its reason instead of a bare "Error: HTTP 400".
  it("surfaces field-level validation details from a rejected project", async () => {
    server.use(
      http.post("/api/projects", () =>
        HttpResponse.json({
          error: "validation_failed",
          errors: [{ field: "title", code: "too_long", message: "title must be at most 120 characters" }],
        }, { status: 400 }),
      ),
    );
    const onDone = vi.fn();
    renderWithQuery(<ProjectStep onDone={onDone} />);

    fireEvent.change(screen.getByPlaceholderText("e.g. My App"), { target: { value: "T".repeat(200) } });
    fireEvent.change(screen.getByPlaceholderText("~/Projects/my-app"), { target: { value: "/tmp/app" } });
    fireEvent.click(screen.getByText("Create project"));

    expect(await screen.findByText(/title must be at most 120 characters/)).toBeInTheDocument();
    expect(screen.getByText(/title:/)).toBeInTheDocument();
    expect(screen.queryByText(/HTTP 400/)).toBeNull();
    expect(onDone).not.toHaveBeenCalled();
  });
});
