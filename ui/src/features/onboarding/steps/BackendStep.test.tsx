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
      default_model: "sonnet",
      models: {
        sonnet: { name: "Claude Sonnet", model: "sonnet" },
        opus: { name: "Claude Opus", model: "opus" },
      },
    },
    codex: {
      name: "Codex",
      type: "codex-acp",
      default: false,
      default_model: "gpt-5.6-sol",
      models: { "gpt-5.6-sol": { name: "GPT-5.6-Sol", model: "gpt-5.6-sol" } },
    },
  },
};

let lastPutBody: unknown = null;
let putCount = 0;

const server = setupServer(
  http.get("/api/backends", () => HttpResponse.json(seededBackendsDoc)),
  http.put("/api/backends", async ({ request }) => {
    lastPutBody = await request.json();
    putCount += 1;
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
  putCount = 0;
});
afterAll(() => server.close());

function renderWithQuery(ui: React.ReactElement) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

// The step is ready once the seeded catalog has loaded, which it signals by
// naming the default model it will use.
function waitForLoaded() {
  return screen.findByText(/Using this backend's default model/i);
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

    await waitForLoaded();
    expect(button).not.toBeDisabled();
    expect(lastPutBody).toBeNull();
  });

  // FS-04.R33: onboarding no longer asks for model identifiers at all. The step
  // must present neither an AgentDeck model id nor a provider model string.
  it("asks for no model id or provider model string", async () => {
    renderWithQuery(<BackendStep onDone={vi.fn()} />);
    await waitForLoaded();

    expect(screen.queryByText(/Default model ID/i)).toBeNull();
    expect(screen.queryByText(/Provider model string/i)).toBeNull();
    expect(screen.queryByDisplayValue("sonnet")).toBeNull();

    // Only the backend type selector and no free-text model entry.
    const textboxes = screen.queryAllByRole("textbox");
    expect(textboxes).toHaveLength(0);
  });

  // INV §3: the step submits the seeded catalog unchanged and only marks the
  // chosen backend default. It must never write back a partial on-screen view.
  it("submits the seeded catalog unchanged and uses its existing default model", async () => {
    const onDone = vi.fn();
    renderWithQuery(<BackendStep onDone={onDone} />);
    await waitForLoaded();

    fireEvent.click(screen.getByText("Validate & Continue"));
    await waitFor(() => expect(lastPutBody).not.toBeNull());

    const body = lastPutBody as {
      backends: Record<
        string,
        { default: boolean; default_model: string; models: Record<string, { name: string; model: string }> }
      >;
    };
    const claude = body.backends.claude;

    expect(claude.default_model).toBe("sonnet");
    expect(claude.models).toEqual(seededBackendsDoc.backends.claude.models);
    expect(claude.default).toBe(true);
    // The other seeded backend survives, demoted but otherwise intact.
    expect(body.backends.codex.models).toEqual(seededBackendsDoc.backends.codex.models);
    expect(body.backends.codex.default).toBe(false);

    await waitFor(() => expect(onDone).toHaveBeenCalled());
  });

  it("offers all four backend types and shows LLM fields for OpenHands", async () => {
    renderWithQuery(<BackendStep onDone={vi.fn()} />);
    await waitForLoaded();

    const typeSelect = screen.getByRole("combobox") as HTMLSelectElement;
    const values = Array.from(typeSelect.options).map((o) => o.value);
    expect(values).toEqual(["claude-acp", "codex-acp", "opencode-acp", "openhands-acp"]);

    // No LLM key field until OpenHands is selected.
    expect(screen.queryByText("LLM API key")).toBeNull();
    fireEvent.change(typeSelect, { target: { value: "openhands-acp" } });
    expect(screen.getByText("LLM API key")).toBeInTheDocument();
    expect(screen.getByText(/LLM base URL/i)).toBeInTheDocument();
  });

  it("explains a missing adapter in human terms", async () => {
    server.use(
      http.put("/api/backends", () =>
        HttpResponse.json({
          ...seededBackendsDoc,
          credentials: { claude: { status: "skipped", detail: "cli_not_installed" } },
        }),
      ),
    );
    renderWithQuery(<BackendStep onDone={vi.fn()} />);
    await waitForLoaded();
    fireEvent.click(screen.getByText("Validate & Continue"));
    expect(await screen.findByText(/adapter is not installed/i)).toBeInTheDocument();
    expect(screen.queryByText(/cli_not_installed/)).toBeNull();
  });

  // FS-04.R34: Claude and Codex sign in with their own tooling, and an unready
  // result must leave a retryable Check again rather than a dead end.
  it("names the provider sign-in command and offers Check again when unready", async () => {
    server.use(
      http.put("/api/backends", async ({ request }) => {
        lastPutBody = await request.json();
        putCount += 1;
        return HttpResponse.json({
          ...seededBackendsDoc,
          credentials: { claude: { status: "failed", detail: "not_logged_in" } },
        });
      }),
    );
    const onDone = vi.fn();
    renderWithQuery(<BackendStep onDone={onDone} />);
    await waitForLoaded();

    expect(screen.getByText(/agentdeck auth claude/)).toBeInTheDocument();

    fireEvent.click(screen.getByText("Validate & Continue"));
    expect(await screen.findByText(/is not signed in/i)).toBeInTheDocument();
    expect(onDone).not.toHaveBeenCalled();

    // The retry is the same backend save, not a new endpoint (TS-03.R15).
    const again = screen.getByText("Check again");
    fireEvent.click(again);
    await waitFor(() => expect(putCount).toBe(2));
  });

  // A signed-in Codex has no API key; the field must be optional, and the copy
  // must not imply a key is required (FS-09.R34).
  it("treats the Codex API key as an alternative to native sign-in", async () => {
    renderWithQuery(<BackendStep onDone={vi.fn()} />);
    await waitForLoaded();

    fireEvent.change(screen.getByRole("combobox"), { target: { value: "codex-acp" } });

    expect(screen.getByText(/agentdeck auth codex/)).toBeInTheDocument();
    expect(screen.getByText(/OpenAI API key \(optional\)/i)).toBeInTheDocument();
    expect(screen.getByText(/only an alternative/i)).toBeInTheDocument();
  });

  // INV §11/§8: a hand-edited catalog with no usable default must be explained,
  // not silently saved with an empty default_model.
  it("refuses to submit a backend whose catalog has no usable default model", async () => {
    server.use(
      http.get("/api/backends", () =>
        HttpResponse.json({
          version: 2,
          backends: { claude: { name: "Claude", type: "claude-acp", default: true, default_model: "", models: {} } },
        }),
      ),
    );
    renderWithQuery(<BackendStep onDone={vi.fn()} />);

    expect(await screen.findByText(/no usable default model/i)).toBeInTheDocument();
    expect(screen.getByText("Validate & Continue")).toBeDisabled();
    expect(lastPutBody).toBeNull();
  });

  // FS-04.R26 / INV §8: a repairable 400 must name its field and reason. The
  // shared config client preserves the validation envelope; the step once
  // stringified the raw Error and rendered only "Error: HTTP 400".
  it("surfaces field-level validation details from a rejected save", async () => {
    server.use(
      http.put("/api/backends", () =>
        HttpResponse.json({
          error: "validation_failed",
          errors: [{ field: "backends.claude.name", code: "too_long", message: "name must be at most 120 characters" }],
        }, { status: 400 }),
      ),
    );
    renderWithQuery(<BackendStep onDone={vi.fn()} />);
    await waitForLoaded();

    fireEvent.click(screen.getByText("Validate & Continue"));

    expect(await screen.findByText(/name must be at most 120 characters/)).toBeInTheDocument();
    expect(screen.getByText(/backends\.claude\.name/)).toBeInTheDocument();
    expect(screen.queryByText(/HTTP 400/)).toBeNull();
  });
});
