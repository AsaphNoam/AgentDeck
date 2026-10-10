import React from "react";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import type { AgentState } from "../../api/types";
import { useGroupActions } from "./GroupActions";

const server = setupServer();
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => { cleanup(); server.resetHandlers(); });
afterAll(() => server.close());
const agents = [{ agent_id: "a", name: "Builder", running: true }, { agent_id: "b", name: "Reviewer", running: false }] as AgentState[];
function Surface({ disabled = false }: { disabled?: boolean }) {
  const actions = useGroupActions({ project: "app", projectTitle: "My App", disabled });
  return <><button onClick={() => actions.open("archive", "work/review", agents)}>Archive</button>{actions.feedback}</>;
}

describe("Group actions", () => {
  it("confirms scope with Cancel focused and retains named partial results", async () => {
    const bodies: unknown[] = [];
    server.use(http.post("/api/projects/app/groups/archive", async ({ request }) => {
      bodies.push(await request.json());
      return HttpResponse.json({ project: "app", group: "work/review", results: [{ agent_id: "a", ok: true }, { agent_id: "b", ok: false, error: { code: "archive_failed", message: "Could not save" } }] });
    }));
    render(<Surface />);
    fireEvent.click(screen.getByText("Archive"));
    expect(screen.getByText("Project: My App. Group: work/review.")).toBeInTheDocument();
    expect(screen.getByText("2 members · 1 running.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Cancel" })).toHaveFocus();
    fireEvent.click(screen.getByRole("button", { name: "Archive group" }));
    await screen.findByText("Reviewer: Could not save");
    expect(bodies).toEqual([{ group: "work/review" }]);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByRole("status")).toHaveTextContent("1 succeeded, 1 failed");
  });

  it("keeps the confirmation on preflight failure and blocks offline actions", async () => {
    server.use(http.post("/api/projects/app/groups/archive", () => HttpResponse.json({ error: { code: "conflict", message: "Resume in progress" } }, { status: 409 })));
    const view = render(<Surface />);
    fireEvent.click(screen.getByText("Archive"));
    fireEvent.click(screen.getByRole("button", { name: "Archive group" }));
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("Resume in progress"));
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    view.rerender(<Surface disabled />);
    fireEvent.click(screen.getByText("Archive"));
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
