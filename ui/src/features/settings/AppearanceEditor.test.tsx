import React from "react";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { delay, http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { AppearanceRoot } from "../appearance/AppearanceRoot";
import { useUiStore } from "../../store/uiStore";
import { AppearanceEditor } from "./AppearanceEditor";

const configDoc = {
  version: 1,
  port: 4317,
  default_project: "my-app",
  default_role: "implementer",
  skip_permissions: false,
  onboarding_complete: true,
  notifications: { desktop_enabled: true, muted: {} },
  onboarding: { satisfied: true, steps: { backend: { done: true }, project: { done: true }, role: { done: true } } },
};

let currentConfig: Record<string, unknown> = { ...configDoc };
let lastPut: unknown;

const server = setupServer(
  http.get("/api/config", () => HttpResponse.json(currentConfig)),
  http.put("/api/config", async ({ request }) => {
    lastPut = await request.json();
    currentConfig = { ...currentConfig, ...(lastPut as object) };
    return HttpResponse.json(currentConfig);
  }),
);

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => {
  cleanup();
  server.resetHandlers();
  currentConfig = { ...configDoc };
  lastPut = undefined;
  document.documentElement.removeAttribute("data-skin");
  useUiStore.setState({ toasts: [] });
});
afterAll(() => server.close());

function renderAppearance() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <AppearanceRoot />
      <AppearanceEditor />
    </QueryClientProvider>,
  );
}

describe("AppearanceEditor", () => {
  it("applies Sky & Grove immediately and persists it through the existing config route", async () => {
    server.use(
      http.put("/api/config", async ({ request }) => {
        lastPut = await request.json();
        await delay(80);
        currentConfig = { ...currentConfig, ...(lastPut as object) };
        return HttpResponse.json(currentConfig);
      }),
    );
    renderAppearance();

    fireEvent.click(await screen.findByLabelText(/Sky & Grove/));

    await waitFor(() => expect(document.documentElement.dataset.skin).toBe("sky-grove"));
    expect(lastPut).toEqual({ appearance_skin: "sky-grove" });
  });

  it("rolls an immediate Core selection back when persistence fails", async () => {
    currentConfig = { ...configDoc, appearance_skin: "sky-grove" };
    server.use(
      http.put("/api/config", async () => {
        await delay(40);
        return HttpResponse.json({ error: { code: "internal", message: "write failed" } }, { status: 500 });
      }),
    );
    renderAppearance();
    await waitFor(() => expect(document.documentElement.dataset.skin).toBe("sky-grove"));

    fireEvent.click(screen.getByLabelText(/AgentDeck Core/));
    await waitFor(() => expect(document.documentElement).not.toHaveAttribute("data-skin"));

    await waitFor(() => expect(document.documentElement.dataset.skin).toBe("sky-grove"));
    expect(useUiStore.getState().toasts.at(-1)?.title).toBe("Saving appearance failed");
  });

  it("explains an unsupported hand-edited appearance while using Core", async () => {
    currentConfig = {
      ...configDoc,
      appearance_skin: "forest-from-the-future",
      appearance_skin_warning: "unsupported",
    };
    renderAppearance();

    expect(await screen.findByText(/forest-from-the-future/)).toBeInTheDocument();
    // With an active warning, no radio is pre-checked so that clicking Core is
    // not inert — otherwise the unsupported value could never be repaired to Core.
    expect(screen.getByLabelText(/AgentDeck Core/)).not.toBeChecked();
    expect(document.documentElement).not.toHaveAttribute("data-skin");
  });

  it("repairs an unsupported stored appearance straight to Core", async () => {
    currentConfig = {
      ...configDoc,
      appearance_skin: "forest-from-the-future",
      appearance_skin_warning: "unsupported",
    };
    renderAppearance();
    // Wait for the warning (GET resolved → fieldset enabled) before clicking.
    await screen.findByText(/forest-from-the-future/);

    fireEvent.click(screen.getByLabelText(/AgentDeck Core/));

    await waitFor(() => expect(lastPut).toEqual({ appearance_skin: "" }));
    await waitFor(() => expect(screen.queryByText(/forest-from-the-future/)).toBeNull());
  });
});
