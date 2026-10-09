import React from "react";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { RoomList } from "./RoomList";
import fixture from "./fixtures/room.json";

// The summary comes from the Go-encoded room fixture (TS-14.R27).
const summary = () => structuredClone(fixture.room);
let rooms: unknown[] = [];
const server = setupServer(
  http.get("/api/projects", () => HttpResponse.json({ alpha: { title: "Alpha", cwd: "/tmp" } })),
  http.get("/api/think-tanks", () => HttpResponse.json({ version: 1, rooms, clipped: false })),
);
beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterEach(() => cleanup());
afterAll(() => server.close());

function renderList(project: string | null = "alpha") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter><RoomList project={project ?? undefined} /></MemoryRouter>
    </QueryClientProvider>,
  );
}

// FS-02.A53, FS-21.R44: a titled card with the whole roster, each member's
// remaining ceiling, separate room and participant links, the current
// speaker and attention; a deleted source keeps its name without a link.
describe("RoomList cards", () => {
  it("shows title, roster allowances, speaker and attention", async () => {
    rooms = [{ ...summary(), active_attempts: [{ attempt_id: "tta_1", agent_id: "a_one", turn: "discussion", state: "running", started_at: "2026-10-07T00:00:00Z" }] }];
    renderList();
    const card = (await screen.findByRole("link", { name: "Cache choice" })).closest("li")!;
    expect(screen.getByRole("link", { name: "Cache choice" }).getAttribute("href")).toBe("/think-tank/tt_fixture");
    const roster = within(card as HTMLElement).getByRole("list", { name: "Participants" });
    expect(within(roster).getByRole("link", { name: "Ari" }).getAttribute("href")).toBe("/agent/a_one");
    expect(await within(roster).findByText(/Alpha · 2 turns left/)).toBeTruthy();
    expect(within(roster).getByText("Speaking")).toBeTruthy();
    expect(within(card as HTMLElement).getByText("Ari is speaking.")).toBeTruthy();
    expect(within(card as HTMLElement).getByText("Active")).toBeTruthy();
    const budget = card.querySelector(".room-card-budget") as HTMLElement;
    expect(budget.textContent).toContain("2turns remaining");

    cleanup();
    rooms = [{ ...summary(), hold: "Ari's turn failed.", roster: summary().roster.map((m) => ({ ...m, exists: m.agent_id === "a_one" })) }];
    renderList();
    expect(await screen.findByText("Ari's turn failed.")).toBeTruthy();
    expect(screen.getByText("Needs attention")).toBeTruthy();
    expect(screen.getByText("Gone")).toBeTruthy();
    expect(screen.getByText(/agent deleted/)).toBeTruthy();
    expect(screen.queryByRole("link", { name: "Gone" })).toBeNull();
  });

  it("keeps the unused collective allowance on an ended card", async () => {
    rooms = [{ ...summary(), phase: "ended" }];
    renderList();
    const card = (await screen.findByRole("link", { name: "Cache choice" })).closest("li")!;
    const budget = card.querySelector(".room-card-budget") as HTMLElement;
    expect(budget.textContent).toContain("2unused turns");
    expect(within(card as HTMLElement).getByText("Ended")).toBeTruthy();
    expect(within(card as HTMLElement).getByText(/2 turns left/)).toBeTruthy();
    expect(within(card as HTMLElement).queryByText("turns remaining")).toBeNull();
  });

  // FS-21.R39: Archive discovery names the origin, including a removed project.
  it("names a removed origin project outside the project page", async () => {
    rooms = [{ ...summary(), origin_project: "ghost" }];
    renderList(null);
    const card = (await screen.findByRole("link", { name: "Cache choice" })).closest("li")!;
    expect((await within(card as HTMLElement).findByText(/project removed/)).textContent).toBe("Origin: ghost · project removed");
  });
});
