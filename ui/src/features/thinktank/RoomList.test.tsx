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

function renderList() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter><RoomList project="alpha" /></MemoryRouter>
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
    expect(within(roster).getByText("2 of 3 left")).toBeTruthy();
    expect(within(card as HTMLElement).getByText("Ari is speaking.")).toBeTruthy();
    expect(within(card as HTMLElement).getByText(/2 turns left in total/)).toBeTruthy();

    cleanup();
    rooms = [{ ...summary(), hold: "Ari's turn failed.", roster: summary().roster.map((m) => ({ ...m, exists: m.agent_id === "a_one" })) }];
    renderList();
    expect(await screen.findByText("Needs attention: Ari's turn failed.")).toBeTruthy();
    expect(screen.getByText("Gone (deleted)")).toBeTruthy();
    expect(screen.queryByRole("link", { name: "Gone" })).toBeNull();
  });

  it("keeps the unused collective allowance on an ended card", async () => {
    rooms = [{ ...summary(), phase: "ended" }];
    renderList();
    const card = (await screen.findByRole("link", { name: "Cache choice" })).closest("li")!;
    expect(within(card as HTMLElement).getByText("2 turns went unused")).toBeTruthy();
    expect(within(card as HTMLElement).getByText("2 of 3 left")).toBeTruthy();
    expect(within(card as HTMLElement).queryByText(/left in total/)).toBeNull();
  });
});
