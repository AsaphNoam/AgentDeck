import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { CommandsTab } from "./CommandsTab";
import type { TrackedCommand } from "../../api/types";

const mockCommands: TrackedCommand[] = [
  {
    command: "npm test -- --watch=false",
    seq: 5,
    ts: "2026-06-28T10:00:05Z",
    tool_call_id: "tc_5",
    exit_status: "completed",
    exit_error: "",
  },
  {
    command: "git status",
    seq: 3,
    ts: "2026-06-28T10:00:03Z",
    tool_call_id: "tc_3",
    exit_status: "completed",
    exit_error: "",
  },
];

const server = setupServer(
  http.get("/api/sessions/:id/commands", () =>
    HttpResponse.json({ agent_id: "a_1", commands: mockCommands }),
  ),
);

beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterEach(() => { cleanup(); server.resetHandlers(); });
afterAll(() => server.close());

describe("CommandsTab", () => {
  it("renders command rows with command text and status", async () => {
    render(<CommandsTab agentId="a_1" />);
    expect(await screen.findByText("npm test -- --watch=false")).toBeInTheDocument();
    expect(screen.getByText("git status")).toBeInTheDocument();
    const completedTags = screen.getAllByText("completed");
    expect(completedTags.length).toBe(2);
  });

  it("filter narrows the list", async () => {
    render(<CommandsTab agentId="a_1" />);
    await screen.findByText("npm test -- --watch=false");
    const input = screen.getByRole("searchbox");
    fireEvent.change(input, { target: { value: "git" } });
    await waitFor(() => expect(screen.queryByText("npm test -- --watch=false")).not.toBeInTheDocument());
    expect(screen.getByText("git status")).toBeInTheDocument();
  });

  it("copy button is present for each command", async () => {
    render(<CommandsTab agentId="a_1" />);
    await screen.findByText("npm test -- --watch=false");
    const copyBtns = screen.getAllByRole("button", { name: /Copy/i });
    expect(copyBtns.length).toBe(2);
  });

  it("shows count label", async () => {
    render(<CommandsTab agentId="a_1" />);
    await screen.findByText("2 commands");
  });

  it("shows empty state when no commands", async () => {
    server.use(
      http.get("/api/sessions/:id/commands", () =>
        HttpResponse.json({ agent_id: "a_1", commands: [] }),
      ),
    );
    render(<CommandsTab agentId="a_1" />);
    await screen.findByText(/No commands tracked yet/);
  });
});
