import React from "react";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, renderHook, screen, waitFor, within } from "@testing-library/react";
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
  http.get("/api/think-tanks/tt_fixture/commands", () => HttpResponse.json({ sources: [], commands: [{ source_id: "src_1", agent_name: "Ari", project: "alpha", tool_call_id: "t1", command: "go test ./...", status: "completed", seq: 4 }] })),
  http.get("/api/sessions/:agentId/file-search", () => HttpResponse.json({ agent_id: "a_one", files: [] })),
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

  // FS-21.R39: a published contribution is not followed by its attempt's
  // tools and changes; only the canonical room entry is shown.
  it("shows no tools and changes between published contributions", async () => {
    detail = { ...room(), phase: "ended", active: undefined };
    const activity = [
      { type: "assistant_text", data: { delta: "Drafting a cache plan." } },
      { type: "tool_call", data: { tool_call_id: "c1", name: "Read" } },
      { type: "assistant_text", data: { delta: "Final attempt answer." } },
      { type: "turn_end", data: { stop_reason: "end_turn" } },
    ].map((event, i) => ({ version: 1, room_id: "tt_fixture", seq: i+1, attempt_id: "tta_1", agent_id: "a_one", agent_name: "Ari", project: "alpha", source_seq: i+1, created_at: "2026-10-06T09:00:00Z", event }));
    server.use(http.get("/api/think-tanks/tt_fixture/activity", () => HttpResponse.json({ activity, complete: true })));
    renderRoom();
    expect(await screen.findByText("LRU")).toBeTruthy();
    await waitFor(() => expect(document.querySelector('[data-slot="activity"]')).toBeNull());
    expect(screen.queryByText("Final attempt answer.")).toBeNull();
    expect(screen.queryByText(/tools and changes/)).toBeNull();
  });

  // FS-21.R39, TS-14.R15: the activity wire retains publication knowledge
  // when the corresponding contribution has fallen outside the entry window.
  it("does not revive clipped published activity as an unfinished turn", async () => {
    detail = { ...room(), phase: "ended", active: undefined };
    const clippedEntries = fixture.entries.filter((entry) => entry.attempt_id !== "tta_1");
    const activity = [{
      version: 1, room_id: "tt_fixture", seq: 1, attempt_id: "tta_1", agent_id: "a_one",
      agent_name: "Ari", project: "alpha", source_seq: 1, published: true,
      created_at: "2026-10-06T09:00:00Z",
      event: { type: "tool_call", data: { tool_call_id: "old", name: "Read" } },
    }];
    server.use(
      http.get("/api/think-tanks/tt_fixture/entries", () => HttpResponse.json({ entries: clippedEntries, complete: true })),
      http.get("/api/think-tanks/tt_fixture/activity", () => HttpResponse.json({ activity, complete: true })),
    );
    renderRoom();
    await screen.findByRole("heading", { name: "Cache choice" });
    await waitFor(() => expect(screen.queryByText("Ari's unfinished turn")).toBeNull());
    expect(document.querySelector('[data-slot="activity"]')).toBeNull();
  });

  // FS-03.A57, TS-08.R104: an unfinished attempt (no published entry) reuses
  // the quiet turn projection, so its tools stay inspectable.
  it.each(["discussion", "ended"])("keeps an unfinished turn's tools inspectable in %s while refusing stale approvals", async (phase) => {
    detail = { ...room(), phase, active: undefined };
    const activity = [
      { type: "tool_call", data: { tool_call_id: "c1", name: "Bash", args: { command: "inspect me" } } },
      { type: "tool_result", data: { tool_call_id: "c1", content: "x".repeat(650)+"tail", status: "completed" } },
      { type: "permission_request", data: { tool_call_id: "old", name: "Stale permission", reason: "Old turn" } },
      { type: "diff", data: { path: "notes.md", old_text: "old", new_text: "new" } },
    ].map((event, i) => ({ version: 1, room_id: "tt_fixture", seq: i+1, attempt_id: "tta_unfinished", agent_id: "a_one", agent_name: "Ari", project: "alpha", source_seq: i+1, created_at: "2026-10-06T09:00:00Z", event }));
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
  it("renders the title, goal disclosure, attributed discussion and participants", async () => {
    renderRoom();
    expect(await screen.findByRole("heading", { name: "Cache choice" })).toBeTruthy();
    // FS-21.R43: the full goal is subordinate to the title.
    expect(screen.getByText("Room goal", { selector: "summary > strong" })).toBeTruthy();
    expect(await screen.findByText("LRU")).toBeTruthy();
    expect(screen.getByText("Consider eviction")).toBeTruthy();
    const participantLinks = screen.getAllByRole("link", { name: "Ari" });
    expect(participantLinks.some((link) => link.getAttribute("href") === "/agent/a_one")).toBe(true);
    expect(screen.queryByRole("link", { name: "Gone" })).toBeNull();
    expect(screen.getByText("Agent deleted")).toBeTruthy();
    expect(screen.getByText("2 / 2 turns")).toBeTruthy();
  });

  // FS-21.A36, R49: a refused increase keeps the draft and its command id;
  // the saved increase sends the expected ceiling and acknowledges.
  it("raises a participant's turn limit and keeps the draft on refusal", async () => {
    const sent: { command_id: string; expected_limit: number; limit: number }[] = [];
    server.use(http.post("/api/think-tanks/tt_fixture/participants/a_one/turn-limit", async ({ request }) => {
      sent.push(await request.json() as (typeof sent)[number]);
      if (sent.length === 1) return HttpResponse.json({ error: { code: "conflict", message: "busy database" } }, { status: 409 });
      return HttpResponse.json(detail);
    }));
    renderRoom();
    const raise = await screen.findAllByRole("button", { name: "Raise turn limit" });
    fireEvent.click(raise[0]);
    const input = screen.getByLabelText("New turn limit for Ari") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "6" } });
    expect(screen.getByText("1 used · limit 3 → 6 · 5 left after saving")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByText("busy database")).toBeTruthy();
    expect((screen.getByLabelText("New turn limit for Ari") as HTMLInputElement).value).toBe("6");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(sent).toHaveLength(2));
    expect(sent[1]).toMatchObject({ expected_limit: 3, limit: 6, command_id: sent[0].command_id });
  });

  // FS-21.A35, R48: concurrent openings name every writer with no singular
  // speaker, and a failed opening retries by its own attempt.
  it("shows concurrent openings and retries a failed one by attempt", async () => {
    const attempt = (id: string, agent: string, state: string) =>
      ({ attempt_id: id, agent_id: agent, turn: "opening", state, failure: "", started_at: "2026-10-07T00:00:00Z" });
    const sent: unknown[] = [];
    server.use(http.post("/api/think-tanks/tt_fixture/retry", async ({ request }) => {
      sent.push(await request.json());
      return HttpResponse.json(detail);
    }));
    detail = { ...room(), phase: "openings", active: undefined, active_agent_id: "",
      active_attempts: [attempt("tta_1", "a_one", "running")], hold: "", failed: [] };
    const second = { ...detail, active_attempts: [attempt("tta_1", "a_one", "running"), attempt("tta_2", "a_gone", "running")] };
    detail = second;
    const { unmount } = renderRoom();
    expect(await screen.findByText("2 openings are being written: Ari, Gone.")).toBeTruthy();
    unmount();
    detail = { ...second, active_attempts: [attempt("tta_1", "a_one", "running")], hold: "1 opening(s) failed.",
      failed: [attempt("tta_2", "a_gone", "failed")] };
    renderRoom();
    fireEvent.click(await screen.findByRole("button", { name: "Retry Gone’s opening" }));
    await waitFor(() => expect(sent).toEqual([{ target: "turn", attempt_id: "tta_2" }]));
    detail = room();
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
    expect((screen.getByLabelText("Files and commands from") as HTMLSelectElement).value).toBe("a_one");
    fireEvent.change(box, { target: { value: "see @no", selectionStart: 7 } });
    const option = await screen.findByRole("option", { name: "notes.md" });
    fireEvent.mouseDown(option);
    await waitFor(() => expect(box.value).toBe("see @notes.md (Ari) "));
  });

  it("selects a participant with Enter before sending, and wires UTF-8 mention offsets", async () => {
    renderRoom();
    const box = await screen.findByLabelText("Message the room") as HTMLTextAreaElement;
    fireEvent.change(box, { target: { value: "é @Ar", selectionStart: 5 } });
    expect(await screen.findByRole("option", { name: /@Ari/ })).toBeTruthy();
    fireEvent.keyDown(box, { key: "Enter" });
    expect(box.value).toBe("é @Ari ");
    expect(document.querySelector<HTMLElement>(".composer-notice")).toHaveTextContent("Addressed to Ari");
    fireEvent.keyDown(box, { key: "Enter" });
    await waitFor(() => expect(posted).toHaveLength(1));
    expect(posted[0]).toMatchObject({ body: "é @Ari ", mentions: [{ agent_id: "a_one", start: 3, end: 7 }] });
  });

  it("drops an addressee when its selected label is edited, while plain @ text stays unresolved", async () => {
    renderRoom();
    const box = await screen.findByLabelText("Message the room") as HTMLTextAreaElement;
    fireEvent.change(box, { target: { value: "@Ar", selectionStart: 3 } });
    await screen.findByRole("option", { name: /@Ari/ });
    fireEvent.keyDown(box, { key: "Enter" });
    expect(box.value).toBe("@Ari ");
    fireEvent.change(box, { target: { value: "@Ar ", selectionStart: 4 } });
    expect(document.querySelector(".composer-notice")).toBeNull();
    fireEvent.keyDown(box, { key: "Enter" });
    await waitFor(() => expect(posted).toHaveLength(1));
    expect(posted[0]).toMatchObject({ body: "@Ar ", mentions: [] });

    fireEvent.change(box, { target: { value: "plain @Ari", selectionStart: 10 } });
    // Clicking Send submits the ordinary typed token without accepting its open suggestion.
    fireEvent.click(screen.getByRole("button", { name: "Send to room" }));
    await waitFor(() => expect(posted).toHaveLength(2));
    expect(posted[1]).toMatchObject({ body: "plain @Ari", mentions: [] });
  });

  it("keeps a refused addressed draft, body, mentions and command id for retry", async () => {
    let attempts = 0;
    server.use(http.post("/api/think-tanks/tt_fixture/messages", async ({ request }) => {
      const body = await request.json();
      posted.push(body);
      attempts++;
      if (attempts === 1) return HttpResponse.json({ error: { code: "conflict", message: "room is busy" } }, { status: 409 });
      return HttpResponse.json({ version: 1, input_id: "tti_retry", published_seq: 0 });
    }));
    renderRoom();
    const box = await screen.findByLabelText("Message the room") as HTMLTextAreaElement;
    fireEvent.change(box, { target: { value: "@Ar", selectionStart: 3 } });
    await screen.findByRole("option", { name: /@Ari/ });
    fireEvent.keyDown(box, { key: "Enter" });
    fireEvent.click(screen.getByRole("button", { name: "Send to room" }));
    await screen.findByText("room is busy");
    expect(box.value).toBe("@Ari ");
    expect(document.querySelector<HTMLElement>(".composer-notice")).toHaveTextContent("Addressed to Ari");
    const first = posted[0] as { command_id: string; body: string; mentions: unknown[] };
    expect(first.mentions).toEqual([{ agent_id: "a_one", start: 0, end: 4 }]);
    fireEvent.click(screen.getByRole("button", { name: "Send to room" }));
    await waitFor(() => expect(posted).toHaveLength(2));
    expect(posted[1]).toEqual(first);
  });

  it("uses Shift+Enter for a newline without sending", async () => {
    renderRoom();
    const box = await screen.findByLabelText("Message the room") as HTMLTextAreaElement;
    fireEvent.change(box, { target: { value: "line one", selectionStart: 8 } });
    fireEvent.keyDown(box, { key: "Enter", shiftKey: true });
    // fireEvent does not apply the browser's default textarea insertion.
    fireEvent.change(box, { target: { value: "line one\n", selectionStart: 9 } });
    expect(box.value).toBe("line one\n");
    expect(posted).toHaveLength(0);
    fireEvent.change(box, { target: { value: "@Ar", selectionStart: 3 } });
    await screen.findByRole("option", { name: /@Ari/ });
    // Even an open picker leaves Shift+Enter's default newline untouched.
    expect(fireEvent.keyDown(box, { key: "Enter", shiftKey: true })).toBe(true);
    expect(box.value).toBe("@Ar");
    expect(posted).toHaveLength(0);
  });

  it("wires two selected mentions with UTF-8 offsets after text before them is edited", async () => {
    renderRoom();
    const box = await screen.findByLabelText("Message the room") as HTMLTextAreaElement;
    fireEvent.change(box, { target: { value: "é @Ar", selectionStart: 5 } });
    await screen.findByRole("option", { name: /@Ari/ });
    fireEvent.keyDown(box, { key: "Enter" });
    fireEvent.change(box, { target: { value: "é @Ari and @Ar", selectionStart: 14 } });
    await screen.findByRole("option", { name: /@Ari/ });
    fireEvent.keyDown(box, { key: "Enter" });
    fireEvent.click(screen.getByRole("button", { name: "Send to room" }));
    await waitFor(() => expect(posted).toHaveLength(1));
    expect(posted[0]).toMatchObject({
      body: "é @Ari and @Ari ",
      mentions: [
        { agent_id: "a_one", start: 3, end: 7 },
        { agent_id: "a_one", start: 12, end: 16 },
      ],
    });
  });

  it("renders persisted addressee identity from an entry context snapshot", async () => {
    const addressed = {
      ...fixture.entries[1],
      context: { addressees: [{ agent_id: "a_one", name: "Ari", project: "alpha" }] },
    };
    server.use(http.get("/api/think-tanks/tt_fixture/entries", () => HttpResponse.json({ entries: [addressed], complete: true })));
    renderRoom();
    expect(await screen.findByText("Addressed to Ari (alpha) · shared with the room")).toBeTruthy();
  });

  it("disambiguates duplicate names by project and identity", async () => {
    const base = room();
    const duplicate = { ...base.members[0], agent_id: "a_two", project: "alpha", order: 2 };
    detail = { ...base, members: [...base.members, duplicate] };
    renderRoom();
    const box = await screen.findByLabelText("Message the room") as HTMLTextAreaElement;
    fireEvent.change(box, { target: { value: "@", selectionStart: 1 } });
    expect(await screen.findByRole("option", { name: /@Ari \(alpha · a_one\)/ })).toBeTruthy();
    expect(screen.getByRole("option", { name: /@Ari \(alpha · a_two\)/ })).toBeTruthy();
  });

  it("offers exhausted live targets with their limit and excludes departed or missing targets", async () => {
    const base = room();
    const exhausted = { ...base.members[0], agent_id: "a_tired", name: "Tired", state: "exhausted" as const, completed: 2, limit: 2 };
    const departed = { ...base.members[0], agent_id: "a_left", name: "Left", state: "departed" as const };
    detail = { ...base, members: [...base.members, exhausted, departed] };
    renderRoom();
    const box = await screen.findByLabelText("Message the room") as HTMLTextAreaElement;
    fireEvent.change(box, { target: { value: "@", selectionStart: 1 } });
    expect(await screen.findByRole("option", { name: /@Tired/ })).toBeTruthy();
    expect(screen.getByText("No turns left")).toBeTruthy();
    expect(screen.queryByRole("option", { name: /@Left/ })).toBeNull();
    expect(screen.queryByRole("option", { name: /@Gone/ })).toBeNull();
  });

  // FS-12.R64: Files/Commands carry counts and open a dialog of sourced rows;
  // a live member's card shows its runtime from the agent store.
  it("lists room commands in a dialog and shows a live member's runtime", async () => {
    useAgentStore.setState({ agents: { a_one: { agent_id: "a_one", model: "opus", effort: "high" } as never } });
    renderRoom();
    expect(await screen.findByText("opus · high")).toBeTruthy();
    const open = await screen.findByRole("button", { name: /Commands 1/ });
    fireEvent.click(open);
    const dialog = await screen.findByRole("dialog", { name: "Commands" });
    expect(within(dialog).getByText("go test ./...")).toBeTruthy();
    expect(within(dialog).getByText(/^Ari · .+ · completed$/)).toBeTruthy();
    fireEvent.click(within(dialog).getByRole("button", { name: "Close" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  // FS-21.R46, FS-12.R64: an ended room replaces the composer with a read-only note.
  it("makes room input read-only once the discussion has ended", async () => {
    detail = { ...room(), phase: "ended", end_reason: "operator", judge_status: "", judge: { enabled: false }, deletable: true };
    renderRoom();
    expect(await screen.findByText(/You ended the discussion/)).toBeTruthy();
    expect(screen.getByText("This room is read-only")).toBeTruthy();
    expect(screen.queryByLabelText("Message the room")).toBeNull();
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
