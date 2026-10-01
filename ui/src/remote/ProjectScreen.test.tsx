import React from "react";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { ProjectScreen } from "./ProjectScreen";
import { useConnection } from "./connection";

const server = setupServer(
  http.get("/api/projects", () => HttpResponse.json({ app: { title: "App", color: [1, 2, 3] } })),
  http.get("/api/remote/home", () => HttpResponse.json({ needs_you: [], active_runs: [] })),
  http.get("/api/roles", () => HttpResponse.json({ implementer: { title: "Implementer" }, reviewer: { title: "Reviewer" } })),
  http.get("/api/config", () => HttpResponse.json({ default_role: "reviewer" })),
  http.get("/api/remote/runtime-options", () => HttpResponse.json({ backends: [{ id: "codex", name: "Codex", default: true, default_model: "preferred", models: [{ id: "fallback", name: "Fallback", efforts: [], fast: false }, { id: "preferred", name: "Preferred", efforts: ["high"], default_effort: "high", fast: true }] }] })),
);

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
beforeEach(() => useConnection.setState({ link: "connected", revision: 0, agents: {} }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

describe("ProjectScreen new agent", () => {
  it("uses the configured default role, an editable suggested name, and backend default model", async () => {
    render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><ProjectScreen projectID="app" /></QueryClientProvider>);
    fireEvent.click(await screen.findByRole("button", { name: "New agent" }));
    const role = await screen.findByRole("combobox", { name: "Role" });
    await waitFor(() => expect(role).toHaveValue("reviewer"));
    expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue("Reviewer");
    expect(screen.getByRole("combobox", { name: "Model" })).toHaveValue("preferred");
    fireEvent.change(screen.getByRole("textbox", { name: "Name" }), { target: { value: "Remote reviewer" } });
    expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue("Remote reviewer");
  });
});
