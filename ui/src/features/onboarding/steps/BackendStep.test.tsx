import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach, vi } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { delay } from "msw";
import { BackendStep } from "./BackendStep";

// Mirrors the real seed (internal/config/seed.go): a "claude" backend with two
// pre-configured models and a real default_model — never the literal "default".
const seededBackendsDoc = {
  version: 2,
  backends: {
    claude: {
      name: "Claude",
      type: "claude-acp",
      default: true,
      default_model: "sonnet-4-6",
      models: {
        "sonnet-4-6": { name: "Sonnet 4.6", model: "claude-sonnet-4-6" },
        "opus-4-7": { name: "Opus 4.7", model: "claude-opus-4-7" },
      },
    },
  },
};

let lastPutBody: unknown = null;

const server = setupServer(
  http.get("/api/backends", () => HttpResponse.json(seededBackendsDoc)),
  http.put("/api/backends", async ({ request }) => {
    lastPutBody = await request.json();
    return HttpResponse.json({
      ...seededBackendsDoc,
      credentials: { claude: { status: "ok", detail: "" } },
    });
  }),
);

beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterEach(() => {
  cleanup();
  server.resetHandlers();
  lastPutBody = null;
});
afterAll(() => server.close());

function renderWithQuery(ui: React.ReactElement) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

describe("BackendStep", () => {
  it("keeps validation disabled until the backends query has loaded", async () => {
    server.use(
      http.get("/api/backends", async () => {
        await delay(200);
        return HttpResponse.json(seededBackendsDoc);
      }),
    );

    renderWithQuery(<BackendStep onDone={vi.fn()} />);

    const button = screen.getByText("Validate & Continue");
    expect(button).toBeDisabled();
    fireEvent.click(button);

    await waitFor(() => {
      expect(screen.getByDisplayValue("claude-sonnet-4-6")).toBeInTheDocument();
    });
    expect(button).not.toBeDisabled();
    expect(lastPutBody).toBeNull();
  });

  it("submitting without touching model inputs preserves the seeded models map (no 'default' placeholder)", async () => {
    const onDone = vi.fn();
    renderWithQuery(<BackendStep onDone={onDone} />);

    // Wait for the pre-fill effect to populate the model fields from the seeded backend.
    await waitFor(() => {
      expect(screen.getByDisplayValue("claude-sonnet-4-6")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Validate & Continue"));

    await waitFor(() => expect(lastPutBody).not.toBeNull());

    const body = lastPutBody as { backends: Record<string, { default_model: string; models: Record<string, { model: string }> }> };
    const claude = body.backends.claude;

    // The seeded models must survive untouched.
    expect(claude.models["sonnet-4-6"]).toMatchObject({ name: "Sonnet 4.6", model: "claude-sonnet-4-6" });
    expect(claude.models["opus-4-7"]).toEqual({ name: "Opus 4.7", model: "claude-opus-4-7" });

    // default_model must point at a real seeded model, never the literal placeholder "default".
    expect(claude.default_model).not.toBe("default");
    expect(claude.models[claude.default_model]).toBeDefined();

    // No model in the payload should ever be the literal string "default".
    for (const model of Object.values(claude.models)) {
      expect(model.model).not.toBe("default");
    }

    await waitFor(() => expect(onDone).toHaveBeenCalled());
  });
});
