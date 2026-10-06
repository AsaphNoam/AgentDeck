import React from "react";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, renderHook, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { ThinkTankPage } from "./ThinkTankPage";
import { roomStatus } from "./roomText";
import { thinkTankDetailSchema } from "../../schemas/thinkTank";
import { useAgentStore } from "../../store/agentStore";
import { useAnnotationStore } from "../../store/annotationStore";
import fixture from "./fixtures/room.json";
import { THINK_TANK_KEYS, fetchAllActivity, noteThinkTankActivity, useThinkTankActivity } from "../../api/thinkTanks";

// The room comes from the Go-encoded fixture (internal/server/
// think_tank_handlers_test.go), so the page is tested against the real wire
// shape, including empty arrays and a deleted participant (INV §11/§17).
const room = () => structuredClone(fixture.room);
let detail: Record<string, unknown> = room();
let posted: unknown[] = [];

const server = setupServer(
  http.get("/api/projects", () => HttpResponse.json({ alpha: { title: "Alpha", cwd: "/tmp" } })),
  // The judge-settings form is mounted closed and reads the launch catalog.
  http.get("/api/roles", () => HttpResponse.json({ impl: { title: "Impl" } })),
  http.get("/api/config", () => HttpResponse.json({ default_role: "impl" })),
  http.get("/api/backends", () => HttpResponse.json({ version: 2, backends: {} })),
  http.get("/api/config-sources", () => HttpResponse.json([])),
  http.get("/api/think-tanks/tt_fixture", () => HttpResponse.json(detail)),
  http.get("/api/think-tanks/tt_fixture/entries", () => HttpResponse.json({ version: 1, entries: fixture.entries, complete: true })),
  http.get("/api/think-tanks/tt_fixture/activity", () => HttpResponse.json({ version: 1, activity: [], complete: true })),
  http.get("/api/think-tanks/tt_fixture/files", () => HttpResponse.json({ version: 1, sources: [], files: [] })),
  http.post("/api/think-tanks/tt_fixture/messages", async ({ request }) => {
    posted.push(await request.json());
    return HttpResponse.json({ version: 1, input_id: "tti_2", published_seq: 0 });
  }),
  http.post("/api/think-tanks/tt_fixture/pause", () => HttpResponse.json({ ...detail, control: "paused" })),
);

function renderRoom() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={["/think-tank/tt_fixture"]}>
        <Routes><Route path="/think-tank/:id" element={<ThinkTankPage />} /></Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
beforeEach(() => {
  detail = room();
  posted = [];
  useAgentStore.setState({ agents: {}, order: [], hydrated: true, hydrating: false });
  useAnnotationStore.setState({ bySource: {}, overallBySource: {}, editedAt: {}, collapsedBySource: {} });
});
afterEach(() => {
  cleanup();
  server.resetHandlers();
});
afterAll(() => server.close());

describe("ThinkTankPage", () => {
  it("opens a deleted participant's retained source directly and annotates its file", async () => {
    const entry = { ...fixture.entries[0], agent_id: "a_gone", body: "[notes](notes.md)", attempt_id: "tta_deleted" };
    server.use(
      http.get("/api/think-tanks/tt_fixture/entries", () => HttpResponse.json({ entries: [entry], complete: true })),
      http.get("/api/think-tanks/tt_fixture/files", () => HttpResponse.json({ sources: [{ source_id: "src_23", agent_id: "a_gone", agent_name: "Gone", project: "alpha", attempt_ids: ["tta_deleted"] }], files: [] })),
      http.get("/api/think-tanks/tt_fixture/sources/src_23/file", () => HttpResponse.json({ agent_id: "a_gone", path: "notes.md", content: "retained text", language: "text", size: 13, line_count: 1, truncated: false, mod_time: "2026-10-06T09:00:00Z" })),
    );
    const view = renderRoom();
    fireEvent.click(await screen.findByRole("button", { name: "notes" }));
    await screen.findByText("retained text");
    const body = view.container.querySelector(".file-viewer-body")!;
    const range = document.createRange(); range.selectNodeContents(body.querySelector('[data-file-line="1"]')!.lastElementChild!);
    window.getSelection()?.removeAllRanges(); window.getSelection()?.addRange(range);
    fireEvent.contextMenu(body);
    fireEvent.click(await screen.findByText("Annotate selection"));
    expect(Object.values(useAnnotationStore.getState().bySource).flat()).toEqual(expect.arrayContaining([expect.objectContaining({ room_anchor: "file", source_id: "src_23", path: "notes.md" })]));
    window.getSelection()?.removeAllRanges();
  });

  it.each(["discussion", "ended"])("keeps settled tools inspectable in %s while refusing stale approvals", async (phase) => {
    detail = { ...room(), phase, active: undefined };
    const activity = [
      { type: "tool_call", data: { tool_call_id: "c1", name: "Bash", args: { command: "inspect me" } } },
      { type: "tool_result", data: { tool_call_id: "c1", content: "x".repeat(650)+"tail", status: "completed" } },
      { type: "permission_request", data: { tool_call_id: "old", name: "Stale permission", reason: "Old turn" } },
      { type: "diff", data: { path: "notes.md", old_text: "old", new_text: "new" } },
    ].map((event, i) => ({ version: 1, room_id: "tt_fixture", seq: i+1, attempt_id: "tta_1", agent_id: "a_one", agent_name: "Ari", project: "alpha", source_seq: i+1, created_at: "2026-10-06T09:00:00Z", event }));
    server.use(http.get("/api/think-tanks/tt_fixture/activity", () => HttpResponse.json({ activity, complete: true })));
    renderRoom();
    fireEvent.click(await screen.findByRole("button", { name: "Ran 1 tool" }));
    fireEvent.click(screen.getByRole("button", { name: /Tool call: Bash/ }));
    expect(screen.getByText(/inspect me/)).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Show more" }));
    expect(screen.getByText(/tail$/)).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Approve" })).toBeNull();
    expect(screen.getByText("Cancelled")).toBeTruthy();
    const diff = document.querySelector('[data-variant="diff"]')!;
    fireEvent.contextMenu(diff);
    fireEvent.click(await screen.findByText("Annotate whole event"));
    expect(Object.values(useAnnotationStore.getState().bySource).flat()).toEqual(expect.arrayContaining([expect.objectContaining({ room_anchor: "activity", seq: 4 })]));
  });

  // TT2-05, TS-14.R16: a live activity event refills after the cached tail;
  // a room update still walks the whole window.
  it("refills live activity from the cached tail", async () => {
    let total = 2;
    const afters: number[] = [];
    const record = (seq: number) => ({ version: 1, room_id: "tt_fixture", seq, attempt_id: "tta_1", agent_id: "a_one", agent_name: "Ari", project: "alpha", source_seq: seq, created_at: "2026-10-06T09:00:00Z" });
    server.use(http.get("/api/think-tanks/tt_fixture/activity", ({ request }) => {
      const after = Number(new URL(request.url).searchParams.get("after"));
      afters.push(after);
      return HttpResponse.json({ complete: true, activity: Array.from({ length: total - after }, (_, i) => record(after + i + 1)) });
    }));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { result } = renderHook(() => useThinkTankActivity("tt_fixture"), {
      wrapper: ({ children }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>,
    });
    await waitFor(() => expect(result.current.data?.activity).toHaveLength(2));
    total = 3;
    noteThinkTankActivity(client, "tt_fixture");
    await waitFor(() => expect(result.current.data?.activity.map((a) => a.seq)).toEqual([1, 2, 3]));
    expect(afters).toEqual([0, 2]);
    await client.invalidateQueries({ queryKey: THINK_TANK_KEYS.room("tt_fixture") });
    await waitFor(() => expect(afters).toEqual([0, 2, 0]));
  });

  it("exposes the bounded activity window and retains its newest rows", async () => {
    server.use(http.get("/api/think-tanks/tt_fixture/activity", ({ request }) => {
      const after = Number(new URL(request.url).searchParams.get("after"));
      const count = Math.min(500, 5002-after);
      return HttpResponse.json({ complete: after+count===5002, activity: Array.from({ length: count }, (_,i) => ({ version: 1, room_id: "tt_fixture", seq: after+i+1, attempt_id: "tta_1", agent_id: "a_one", agent_name: "Ari", project: "alpha", source_seq: after+i+1, created_at: "2026-10-06T09:00:00Z" })) });
    }));
    const result = await fetchAllActivity("tt_fixture", new AbortController().signal);
    expect(result.clipped).toBe(true);
    expect(result.activity).toHaveLength(5000);
    expect(result.activity[0].seq).toBe(3);
    expect(result.activity.at(-1)?.seq).toBe(5002);
    renderRoom();
    expect(await screen.findByText(/Showing the newest 5,000 activity records/)).toBeTruthy();
  });
  // FS-21.A1, A16, A17, TS-08.R87: goal and phase lead, contributions are
  // attributed, live participants link to their own conversation and a
  // deleted one keeps attribution without a link.
  it("renders the goal, attributed discussion and participants", async () => {
    renderRoom();
    expect(await screen.findByRole("heading", { name: "Pick a cache" })).toBeTruthy();
    expect(await screen.findByText("LRU")).toBeTruthy();
    expect(screen.getByText("Consider eviction")).toBeTruthy();
    const participantLinks = screen.getAllByRole("link", { name: "Ari" });
    expect(participantLinks.some((link) => link.getAttribute("href") === "/agent/a_one")).toBe(true);
    expect(screen.queryByRole("link", { name: "Gone" })).toBeNull();
    expect(screen.getByText("Agent deleted")).toBeTruthy();
    expect(screen.getByText("2 of 2 turns")).toBeTruthy();
  });

  // FS-21.A9, R35: room messages go to the room with a stable command id.
  it("sends a room message and keeps controls available", async () => {
    renderRoom();
    const box = await screen.findByLabelText("Message the room");
    fireEvent.change(box, { target: { value: "Consider write-back" } });
    fireEvent.click(screen.getByRole("button", { name: "Send to room" }));
    await waitFor(() => expect(posted).toHaveLength(1));
    expect(posted[0]).toMatchObject({ body: "Consider write-back" });
    expect((posted[0] as { command_id: string }).command_id).toBeTruthy();
    expect(screen.getByRole("button", { name: "Pause" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "End discussion" })).toBeTruthy();
  });

  // FS-21.R35, A25: suggestions come from the chosen participant and the
  // inserted path names that participant's workspace.
  it("qualifies @ file suggestions with the chosen participant", async () => {
    server.use(http.get("/api/sessions/a_one/file-search", () => HttpResponse.json({ agent_id: "a_one", files: ["notes.md"] })));
    renderRoom();
    const box = await screen.findByLabelText("Message the room") as HTMLTextAreaElement;
    expect((screen.getByLabelText("Suggestions from") as HTMLSelectElement).value).toBe("a_one");
    fireEvent.change(box, { target: { value: "see @no", selectionStart: 7 } });
    const option = await screen.findByRole("option", { name: "notes.md" });
    fireEvent.mouseDown(option);
    await waitFor(() => expect(box.value).toBe("see @notes.md (Ari) "));
  });

  it("disables room input once the discussion has ended", async () => {
    detail = { ...room(), phase: "ended", end_reason: "operator", judge_status: "", judge: { enabled: false }, deletable: true };
    renderRoom();
    expect(await screen.findByText(/You ended the discussion/)).toBeTruthy();
    expect((screen.getByLabelText("Message the room") as HTMLTextAreaElement).disabled).toBe(true);
    expect(screen.getByRole("button", { name: "Delete" })).toBeTruthy();
  });
});

describe("roomStatus", () => {
  const base = thinkTankDetailSchema.parse(fixture.room);
  it.each(["launching", "abandoned"])("accepts the durable setup state %s", (setup_state) => {
    const wire = room();
    wire.members[0].setup_state = setup_state;
    expect(thinkTankDetailSchema.parse(wire).members[0].setup_state).toBe(setup_state);
  });
  // FS-21.R18, R28, R19: the current-action line names its reason.
  it("names pending pause, private-work waits and holds", () => {
    expect(roomStatus({ ...base, control: "pause_requested", active: { attempt_id: "a", agent_id: "a_one", turn: "discussion", state: "running", failure: "", started_at: "" } }).text)
      .toBe("Pausing after Ari's turn finishes.");
    expect(roomStatus({ ...base, next: { agent_id: "a_one", turn: "discussion", waiting: "Waiting for Ari to finish private work" } }).tone).toBe("waiting");
    expect(roomStatus({ ...base, hold: "Ari's turn failed: The provider reported an error." })).toEqual({ tone: "attention", text: "Ari's turn failed: The provider reported an error." });
  });
});
