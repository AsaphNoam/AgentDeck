import React from "react";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { AgentScreen } from "./AgentScreen";
import { useConnection } from "./connection";
import type { AgentState } from "../api/types";

const agent: AgentState = {
  agent_id: "a1", name: "", role: "implementer", project: "my-app", backend: "claude", model: "m", fast: false,
  fast_available: false, steering_available: true, interface: "chat", created_at: "", running: true,
  state: "waiting_input", detail: "Permission: Bash", context_pct: 0, updated_at: 0, archived: false,
};

let permissionStatus = 200;
let promptStatus = 200;
const posts: string[] = [];
const events = [
  { agent_id: "a1", seq: 1, type: "user_text", ts: "", data: { text: "Clean the build" } },
  { agent_id: "a1", seq: 2, type: "assistant_text", ts: "", data: { text: "I will remove the dist folder." } },
  { agent_id: "a1", seq: 3, type: "permission_request", ts: "", data: { tool_call_id: "t1", name: "Bash", reason: "", args: { command: "rm -rf dist" } } },
];

const server = setupServer(
  http.get("/api/sessions/a1/transcript", () => HttpResponse.json({ agent_id: "a1", events })),
  http.get("/api/sessions/a1/prompt", () => HttpResponse.json({ error: { code: "not_found", message: "none" } }, { status: 404 })),
  http.post("/api/sessions/a1/permission", async ({ request }) => {
    posts.push(`permission ${JSON.stringify(await request.json())}`);
    if (permissionStatus === 409) return HttpResponse.json({ error: { code: "conflict", message: "permission already resolved for that tool_call_id" } }, { status: 409 });
    return HttpResponse.json({ resolved: true });
  }),
  http.post("/api/sessions/a1/prompt", async ({ request }) => {
    posts.push(`prompt ${JSON.stringify(await request.json())}`);
    if (promptStatus === 409) return HttpResponse.json({ error: { code: "conflict", message: "agent is not running" } }, { status: 409 });
    return HttpResponse.json({ accepted: true, delivery: "held" });
  }),
  http.post("/api/sessions/a1/steer", () => HttpResponse.json({ accepted: true, outcome: "steered" })),
  http.post("/api/sessions/a1/cancel", () => HttpResponse.json({})),
  http.post("/api/sessions/a1/stop", () => HttpResponse.json({})),
);

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
beforeEach(() => {
  permissionStatus = 200;
  promptStatus = 200;
  posts.length = 0;
  useConnection.setState({ link: "connected", agents: { a1: agent }, transcriptRev: {} });
});
afterEach(() => {
  cleanup();
  server.resetHandlers();
});
afterAll(() => server.close());

function renderScreen() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <AgentScreen agentId="a1" />
    </QueryClientProvider>,
  );
}

describe("AgentScreen", () => {
  it("shows a permission with its command and latest message and approves it", async () => {
    renderScreen();
    const card = await screen.findByRole("region", { name: "Permission request" });
    expect(card).toHaveTextContent("implementer@my-app needs permission");
    expect(card).toHaveTextContent("rm -rf dist");
    expect(card).toHaveTextContent("I will remove the dist folder.");
    fireEvent.click(screen.getByRole("button", { name: "Approve" }));
    await waitFor(() => expect(posts).toContain('permission {"tool_call_id":"t1","decision":"approve"}'));
  });

  it("tells the later answer what already happened", async () => {
    permissionStatus = 409;
    renderScreen();
    fireEvent.click(await screen.findByRole("button", { name: "Deny" }));
    expect(await screen.findByText("Already answered on the Mac or another phone.")).toBeInTheDocument();
  });

  it("holds a follow-up, offers Steer while busy, and keeps text when refused", async () => {
    useConnection.setState({ agents: { a1: { ...agent, state: "busy", detail: "" } } });
    renderScreen();
    const box = await screen.findByLabelText("Message");
    fireEvent.change(box, { target: { value: "also run tests" } });
    expect(screen.getByRole("button", { name: "Steer" })).toBeEnabled();
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(await screen.findByText(/Held — it will be sent/)).toBeInTheDocument();
    expect(box).toHaveValue("");

    promptStatus = 409;
    fireEvent.change(box, { target: { value: "keep me" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(await screen.findByText("agent is not running")).toBeInTheDocument();
    expect(box).toHaveValue("keep me");
    fireEvent.click(screen.getByRole("button", { name: "Cancel turn" }));
    fireEvent.click(screen.getByRole("button", { name: "Stop" }));
  });

  it("disables every action while the Mac is unreachable", async () => {
    renderScreen();
    await screen.findByRole("region", { name: "Permission request" });
    act(() => useConnection.setState({ link: "unreachable" }));
    for (const name of ["Approve", "Deny", "Stop"]) expect(screen.getByRole("button", { name })).toBeDisabled();
  });

  it("shows only status for a terminal agent", async () => {
    useConnection.setState({ agents: { a1: { ...agent, interface: "terminal", state: "busy" } } });
    renderScreen();
    expect(await screen.findByText(/terminal is on the Mac/)).toBeInTheDocument();
    expect(screen.queryByLabelText("Message")).toBeNull();
  });
});
