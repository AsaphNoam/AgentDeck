import React from "react";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { AgentScreen } from "./AgentScreen";
import { useConnection } from "./connection";
import type { AgentState } from "../api/types";
import { useAnnotationStore } from "../store/annotationStore";

// jsdom has no layout for the diff library; a tap on its line number is what
// the shared DiffBlock listens for (FS-13.R17).
vi.mock("react-diff-viewer-continued", () => ({
  default: ({ onLineNumberClick }: { onLineNumberClick: (lineId: string) => void }) => (
    <button type="button" onClick={() => onLineNumberClick("L-2")}>Old line 2</button>
  ),
}));

const agent: AgentState = {
  agent_id: "a1", name: "", role: "implementer", project: "my-app", backend: "claude", model: "m", fast: false,
  fast_available: false, steering_available: true, interface: "chat", created_at: "", running: true,
  state: "waiting_input", detail: "Permission: Bash", context_pct: 0, updated_at: 0, archived: false,
};

let permissionStatus = 200;
let promptStatus = 200;
let annotationStatus = 202;
let resumeStatus = 200;
const posts: string[] = [];
const events = [
  { agent_id: "a1", seq: 1, type: "user_text", ts: "", data: { text: "Clean the build" } },
  { agent_id: "a1", seq: 2, type: "assistant_text", ts: "", data: { text: "I will remove the dist folder." } },
  { agent_id: "a1", seq: 3, type: "permission_request", ts: "", data: { tool_call_id: "t1", name: "Bash", reason: "", args: { command: "rm -rf dist" } } },
  { agent_id: "a1", seq: 4, type: "diff", ts: "", data: { path: "main.go", old_text: "first\nsecond\nthird", new_text: "replacement" } },
];
const fullWindow = { agent_id: "a1", events, has_more: false, pending_permission: events[2], latest_assistant: "I will remove the dist folder." };
let live: Record<string, unknown> = fullWindow;
let earlierPage: Record<string, unknown> = {};
const reads: string[] = [];

const server = setupServer(
  http.get("/api/sessions/a1/transcript", ({ request }) => {
    const url = new URL(request.url);
    reads.push(url.search);
    return HttpResponse.json(url.searchParams.has("before_seq") ? earlierPage : live);
  }),
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
  http.post("/api/sessions/a1/annotations", async ({ request }) => {
    posts.push(`annotations ${JSON.stringify(await request.json())}`);
    if (annotationStatus === 409) return HttpResponse.json({ error: { code: "conflict", message: "the recipient agent is not running" } }, { status: 409 });
    return HttpResponse.json({ accepted: true, seq: 5 }, { status: 202 });
  }),
  http.post("/api/sessions", async ({ request }) => {
    posts.push(`launch ${JSON.stringify(await request.json())}`);
    return HttpResponse.json({ agent: { agent_id: "a3", name: "fresh" } }, { status: 201 });
  }),
  http.post("/api/sessions/a1/steer", () => HttpResponse.json({ accepted: true, outcome: "steered" })),
  http.post("/api/sessions/a1/cancel", () => HttpResponse.json({})),
  http.post("/api/sessions/a1/stop", () => HttpResponse.json({})),
  http.post("/api/sessions/a1/resume", () => resumeStatus === 409
    ? HttpResponse.json({ error: { code: "conflict", message: "Resume refused for review" } }, { status: 409 })
    : HttpResponse.json({ resumed: true })),
  http.post("/api/sessions/a1/archive", () => HttpResponse.json({})),
  http.get("/api/sessions/a1/files", () => HttpResponse.json({ agent_id: "a1", files: [] })),
  http.get("/api/remote/runtime-options", () => HttpResponse.json({ backends: [] })),
);

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
beforeEach(() => {
  permissionStatus = 200;
  promptStatus = 200;
  annotationStatus = 202;
  resumeStatus = 200;
  posts.length = 0;
  reads.length = 0;
  live = fullWindow;
  useConnection.setState({ link: "connected", agents: { a1: agent }, transcriptRev: {} });
  useAnnotationStore.setState({ bySource: {}, overallBySource: {}, editedAt: {}, collapsedBySource: {} });
  Object.defineProperty(window, "scrollY", { value: 300, writable: true, configurable: true });
  window.scrollTo = vi.fn();
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
  // FS-20.A16: live busy state, connection loss and inactive agents.
  it("shows live work and removes it when busy ends or the connection is lost", async () => {
    renderScreen();
    expect(screen.queryByText("Working…")).not.toBeInTheDocument();
    act(() => useConnection.setState({ agents: { a1: { ...agent, state: "busy" } } }));
    expect(screen.getByText("Working…")).toBeInTheDocument();
    act(() => useConnection.setState({ link: "unreachable" }));
    expect(screen.queryByText("Working…")).not.toBeInTheDocument();
    act(() => useConnection.setState({ link: "connected" }));
    expect(screen.getByText("Working…")).toBeInTheDocument();
    for (const state of ["idle", "waiting_input", "error"] as const) {
      act(() => useConnection.setState({ agents: { a1: { ...agent, state } } }));
      expect(screen.queryByText("Working…")).not.toBeInTheDocument();
    }
    for (const stopped of [{ ...agent, state: "busy" as const, running: false }, { ...agent, state: "busy" as const, interface: "terminal" as const }]) {
      act(() => useConnection.setState({ agents: { a1: stopped } }));
      expect(screen.queryByText("Working…")).not.toBeInTheDocument();
    }
    await screen.findByText("This is a terminal agent. Its terminal is on the Mac; the phone shows its status only.");
  });

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
    // The phone draft caps near 40% of the viewport and scrolls inside (FS-02.R66, TS-08.R88).
    expect(box).toHaveStyle({ maxHeight: "40vh", overflowY: "auto" });
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
    await waitFor(() => expect(screen.getByRole("button", { name: "Stop" })).toBeEnabled());
    const stopOpener = screen.getByRole("button", { name: "Stop" });
    stopOpener.focus();
    fireEvent.click(stopOpener);
    const confirm = screen.getByRole("dialog", { name: "Stop this agent?" });
    act(() => useConnection.setState({ link: "unreachable" }));
    expect(within(confirm).getByRole("button", { name: "Stop" })).toBeDisabled();
    act(() => useConnection.setState({ link: "connected" }));
    fireEvent.click(within(confirm).getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Stop this agent?" })).not.toBeInTheDocument());
    expect(document.activeElement).toBe(stopOpener);
    fireEvent.click(stopOpener);
    fireEvent.click(within(screen.getByRole("dialog", { name: "Stop this agent?" })).getByRole("button", { name: "Stop" }));
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Stop this agent?" })).not.toBeInTheDocument());
  });

  it("disables every action while the Mac is unreachable", async () => {
    renderScreen();
    await screen.findByRole("region", { name: "Permission request" });
    act(() => useConnection.setState({ link: "unreachable" }));
    for (const name of ["Approve", "Deny", "Stop"]) expect(screen.getByRole("button", { name })).toBeDisabled();
  });

  it("shows a refused Resume on Files", async () => {
    resumeStatus = 409;
    useConnection.setState({ agents: { a1: { ...agent, running: false, state: "idle" } } });
    renderScreen();
    fireEvent.click(await screen.findByRole("tab", { name: "Files" }));
    fireEvent.click(screen.getByRole("button", { name: "Resume" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Resume refused for review");
  });

  it("shows a pending permission and latest reply that fall before the window", async () => {
    live = {
      agent_id: "a1",
      events: [
        { agent_id: "a1", seq: 900, type: "permission_resolved", ts: "", data: { tool_call_id: "t0", decision: "approve" } },
        { agent_id: "a1", seq: 901, type: "tool_call", ts: "", data: { tool_call_id: "c1", name: "Read" } },
      ],
      has_more: true,
      pending_permission: events[2],
      latest_assistant: "Reply from before the window",
    };
    renderScreen();
    const card = await screen.findByRole("region", { name: "Permission request" });
    expect(card).toHaveTextContent("rm -rf dist");
    expect(card).toHaveTextContent("Reply from before the window");
    expect(reads[0]).toContain("limit=750");
  });

  it("folds tool results into their collapsed tool line", async () => {
    live = {
      agent_id: "a1",
      events: [
        { agent_id: "a1", seq: 10, type: "tool_call", ts: "", data: { tool_call_id: "c1", name: "Edit", args: { path: "main.go" } } },
        { agent_id: "a1", seq: 11, type: "tool_result", ts: "", data: { tool_call_id: "c1", content: { diff: "raw-result-json" } } },
        { agent_id: "a1", seq: 12, type: "tool_call", ts: "", data: { tool_call_id: "c2", name: "Bash" } },
        { agent_id: "a1", seq: 13, type: "tool_result", ts: "", data: { tool_call_id: "c2", content: "second-result" } },
        { agent_id: "a1", seq: 14, type: "assistant_text", ts: "", data: { text: "Done." } },
      ],
      has_more: false,
      pending_permission: null,
      latest_assistant: "Done.",
    };
    renderScreen();
    const toggle = await screen.findByRole("button", { name: "Ran 2 tools" });
    expect(screen.getByRole("list", { name: "Conversation" })).not.toHaveTextContent("raw-result-json");
    expect(screen.queryByText(/second-result/)).toBeNull();
    fireEvent.click(toggle);
    expect(await screen.findByText(/raw-result-json/)).toBeInTheDocument();
    expect(screen.getByText(/second-result/)).toBeInTheDocument();
  });

  it("replays notices as compact rows in their original order (FS-03.A49)", async () => {
    live = {
      agent_id: "a1",
      events: [
        { agent_id: "a1", seq: 20, type: "assistant_text", ts: "", data: { text: "Before." } },
        { agent_id: "a1", seq: 21, type: "notice", ts: "", data: { severity: "warning", title: "Usage limit", description: "90% used." } },
        { agent_id: "a1", seq: 22, type: "notice", ts: "", data: { severity: "mystery", title: "Hook failed" } },
        { agent_id: "a1", seq: 23, type: "assistant_text", ts: "", data: { text: "After." } },
      ],
      has_more: false,
      pending_permission: null,
      latest_assistant: "After.",
    };
    renderScreen();
    const notes = await screen.findAllByRole("note");
    expect(notes.map((note) => note.textContent)).toEqual(["warningUsage limit90% used.", "infoHook failed"]);
    const conversation = screen.getByRole("list", { name: "Conversation" }).textContent ?? "";
    expect(conversation.indexOf("Before.")).toBeLessThan(conversation.indexOf("Usage limit"));
    expect(conversation.indexOf("Hook failed")).toBeLessThan(conversation.indexOf("After."));
  });

  it("shows a steer-backgrounded command as continuing in the background (FS-03.A48)", async () => {
    live = {
      agent_id: "a1",
      events: [
        { agent_id: "a1", seq: 30, type: "tool_call", ts: "", data: { tool_call_id: "tc_bg", name: "Bash", args: { command: "npm run build" } } },
        { agent_id: "a1", seq: 31, type: "background_task_state", ts: "", data: { task_id: "task_1", tool_call_id: "tc_bg", name: "npm run build", state: "running" } },
      ],
      has_more: false,
      pending_permission: null,
      latest_assistant: "",
    };
    renderScreen();
    fireEvent.click(await screen.findByRole("button", { name: "Ran 1 tool" }));
    expect(await screen.findByText("Continues in background")).toBeInTheDocument();
  });

  it("loads earlier windows on upward scroll and keeps the viewport anchored", async () => {
    live = {
      agent_id: "a1",
      events: [{ agent_id: "a1", seq: 151, type: "user_text", ts: "", data: { text: "Newest question" } }],
      has_more: true,
      pending_permission: null,
      latest_assistant: "",
    };
    earlierPage = {
      agent_id: "a1",
      events: [{ agent_id: "a1", seq: 150, type: "user_text", ts: "", data: { text: "Older question" } }],
      has_more: false,
      pending_permission: null,
      latest_assistant: "",
    };
    renderScreen();
    await screen.findByText("Newest question");
    expect(screen.queryByRole("region", { name: "Permission request" })).toBeNull();
    live = { ...live, has_more: false };
    window.scrollY = 100;
    fireEvent.scroll(window);
    expect(await screen.findByText("Older question")).toBeInTheDocument();
    expect(reads).toContain("?limit=750&before_seq=151");
    await waitFor(() => expect(reads).toContain("?limit=750&since_seq=151"));
    expect(window.scrollTo).toHaveBeenCalled();
    expect(screen.getByText("Newest question")).toBeInTheDocument();
  });

  it("keeps requested earlier history when one live event arrives during pagination", async () => {
    // MOBILE-HISTORY-01: the anchor refetch currently drops all older pages
    // when one new event pushes the original full tail past its window limit.
    const history = Array.from({ length: 1500 }, (_, index) => ({
      agent_id: "a1", seq: index + 1, type: "user_text", ts: "",
      data: { text: `History question ${index + 1}` },
    }));
    server.use(http.get("/api/sessions/a1/transcript", ({ request }) => {
      const url = new URL(request.url);
      reads.push(url.search);
      const before = Number(url.searchParams.get("before_seq"));
      const since = Number(url.searchParams.get("since_seq"));
      const eligible = history.filter((event) => (!before || event.seq < before) && event.seq > since);
      const response = { agent_id: "a1", events: eligible.slice(-750), has_more: eligible.length > 750, pending_permission: null, latest_assistant: "" };
      if (before) history.push({ agent_id: "a1", seq: 1501, type: "user_text", ts: "", data: { text: "New live question" } });
      return HttpResponse.json(response);
    }));
    renderScreen();
    await screen.findByText("History question 751");
    window.scrollY = 100;
    fireEvent.scroll(window);
    await screen.findByText("New live question");
    expect(screen.getByText("History question 1")).toBeInTheDocument();
  });

  it("loads three older windows on repeated upward scrolls without duplicate events", async () => {
    const history = Array.from({ length: 2251 }, (_, index) => ({
      agent_id: "a1", seq: index + 1, type: "user_text", ts: "", data: { text: `Paged question ${index + 1}` },
    }));
    server.use(http.get("/api/sessions/a1/transcript", ({ request }) => {
      const url = new URL(request.url);
      reads.push(url.search);
      const before = Number(url.searchParams.get("before_seq"));
      const since = Number(url.searchParams.get("since_seq"));
      const eligible = history.filter((event) => (!before || event.seq < before) && event.seq > since);
      return HttpResponse.json({ agent_id: "a1", events: eligible.slice(-750), has_more: eligible.length > 750, pending_permission: null, latest_assistant: "" });
    }));
    renderScreen();
    await screen.findByText("Paged question 1502");
    for (const expected of ["Paged question 752", "Paged question 2"]) {
      window.scrollY = 300;
      fireEvent.scroll(window);
      window.scrollY = 100;
      fireEvent.scroll(window);
      await screen.findByText(expected);
    }
    expect(screen.getAllByText("Paged question 1502")).toHaveLength(1);
    expect(reads.filter((read) => read.includes("before_seq"))).toHaveLength(2);
  });

  it("bounds retained older payload bytes while keeping the adjoining live edge", async () => {
    live = { agent_id: "a1", events: [{ agent_id: "a1", seq: 801, type: "user_text", ts: "", data: { text: "Live edge" } }], has_more: true, pending_permission: null, latest_assistant: "" };
    earlierPage = { agent_id: "a1", events: Array.from({ length: 800 }, (_, index) => ({
      agent_id: "a1", seq: index + 1, type: "user_text", ts: "", data: { text: `${index + 1}:${"x".repeat(5000)}` },
    })), has_more: false, pending_permission: null, latest_assistant: "" };
    renderScreen();
    await screen.findByText("Live edge");
    window.scrollY = 100;
    fireEvent.scroll(window);
    await screen.findByText(/^800:/);
    expect(screen.queryByText(/^1:/)).toBeNull();
    expect(screen.getByText("Live edge")).toBeInTheDocument();
  });

  it("recovers two successive live gaps without losing the loaded history", async () => {
    const history = Array.from({ length: 1500 }, (_, index) => ({ agent_id: "a1", seq: index + 1, type: "user_text", ts: "", data: { text: `Gap question ${index + 1}` } }));
    let failNextGapRead = false;
    server.use(http.get("/api/sessions/a1/transcript", ({ request }) => {
      const url = new URL(request.url);
      const before = Number(url.searchParams.get("before_seq"));
      const since = Number(url.searchParams.get("since_seq"));
      if (before && failNextGapRead) {
        failNextGapRead = false;
        return HttpResponse.json({ error: { code: "internal", message: "temporary history read failure" } }, { status: 500 });
      }
      const eligible = history.filter((event) => (!before || event.seq < before) && event.seq > since);
      return HttpResponse.json({ agent_id: "a1", events: eligible.slice(-750), has_more: eligible.length > 750, pending_permission: null, latest_assistant: "" });
    }));
    renderScreen();
    await screen.findByText("Gap question 751");
    window.scrollY = 100;
    fireEvent.scroll(window);
    await screen.findByText("Gap question 1");
    for (const end of [3100, 4700]) {
      for (let seq = history.length + 1; seq <= end; seq++) history.push({ agent_id: "a1", seq, type: "user_text", ts: "", data: { text: `Gap question ${seq}` } });
      if (end === 3100) failNextGapRead = true;
      act(() => useConnection.setState({ transcriptRev: { a1: end } }));
      if (end === 3100) {
        await screen.findByText("temporary history read failure");
        window.scrollY = 300;
        fireEvent.scroll(window);
        window.scrollY = 100;
        fireEvent.scroll(window);
      }
      await screen.findByText(`Gap question ${end}`, {}, { timeout: 5000 });
    }
    expect(screen.getAllByText("Gap question 4700")).toHaveLength(1);
  });

  it("shows only status for a terminal agent", async () => {
    useConnection.setState({ agents: { a1: { ...agent, interface: "terminal", state: "busy" } } });
    renderScreen();
    expect(await screen.findByText(/terminal is on the Mac/)).toBeInTheDocument();
    expect(screen.queryByLabelText("Message")).toBeNull();
    // Terminal agents are not an annotation surface (FS-13.R13, FS-20.R32).
    expect(screen.queryByText("Tap line numbers to select a range.")).toBeNull();
    expect(screen.queryByRole("button", { name: /Annotate lines/ })).toBeNull();
  });

  it("confirms archive in a sheet and keeps refusal visible there", async () => {
    server.use(http.post("/api/sessions/a1/archive", () => HttpResponse.json({ error: { code: "conflict", message: "Archiving is unavailable right now." } }, { status: 409 })));
    renderScreen();
    fireEvent.click(screen.getByRole("tab", { name: "Manage" }));
    fireEvent.click(await screen.findByRole("button", { name: "Archive" }));
    const confirm = screen.getByRole("dialog", { name: "Archive this agent?" });
    expect(within(confirm).getByText("Restore is available on the desktop.")).toBeInTheDocument();
    fireEvent.click(within(confirm).getByRole("button", { name: "Archive agent" }));
    expect(await within(confirm).findByRole("alert")).toHaveTextContent("Archiving is unavailable right now.");
  });

  // FS-20.R39 — archival from the desktop reaches an open phone screen.
  it("shows the archived state when the desktop archives the open agent", async () => {
    renderScreen();
    expect(await screen.findByText("Clean the build")).toBeInTheDocument();
    act(() => useConnection.setState({ agents: { a1: { ...agent, running: false, archived: true } } }));
    expect(await screen.findByText(/Archived on the Mac/)).toBeInTheDocument();
    expect(screen.queryByText("Clean the build")).toBeNull();
    expect(screen.queryByRole("tab", { name: "Manage" })).toBeNull();
    expect(screen.getByRole("button", { name: "Project" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Home" })).toBeInTheDocument();
  });

  // FS-20.R37 — Open diff shows exactly the requested diff, not another one.
  it("opens the selected file's diff from Files", async () => {
    const requested = { agent_id: "a1", seq: 40, type: "diff", ts: "", data: { path: "other.go", old_text: "a", new_text: "requested change" } };
    server.use(
      http.get("/api/sessions/a1/files", () => HttpResponse.json({ agent_id: "a1", files: [{ path: "other.go", edit_count: 1, last_ts: "2026-10-02T00:00:00Z", has_diff: true, diff_refs: [{ seq: 40, tool_call_id: "t9" }] }] })),
      http.get("/api/sessions/a1/transcript", ({ request }) => {
        const url = new URL(request.url);
        reads.push(url.search);
        return HttpResponse.json(url.searchParams.get("before_seq") === "41" ? { agent_id: "a1", events: [requested], has_more: true } : live);
      }),
    );
    renderScreen();
    fireEvent.click(screen.getByRole("tab", { name: "Files" }));
    fireEvent.click(await screen.findByRole("button", { name: "Open diff" }));
    const card = await screen.findByRole("region", { name: "Requested diff" });
    await waitFor(() => expect(card.textContent).toContain("other.go"));
    expect(card.textContent).not.toContain("main.go");
    expect(reads).toContain("?limit=1&before_seq=41");
    expect(screen.getByRole("tab", { name: "Chat" })).toHaveAttribute("aria-selected", "true");
  });

  // FS-20.R36 — a model choice resets effort to that model's default, and the
  // draft follows a runtime change made elsewhere.
  it("switches runtime with the selected model's own effort", async () => {
    const switches: unknown[] = [];
    server.use(
      http.get("/api/remote/runtime-options", () => HttpResponse.json({ backends: [{ id: "claude", name: "Claude", default: true, default_model: "m", models: [
        { id: "m", name: "M", efforts: ["max"], default_effort: "max", fast: false },
        { id: "n", name: "N", efforts: ["low", "high"], default_effort: "low", fast: false },
      ] }] })),
      http.post("/api/sessions/a1/switch-runtime", async ({ request }) => {
        switches.push(await request.json());
        return HttpResponse.json({ history_handoff: "native_resume" });
      }),
    );
    useConnection.setState({ agents: { a1: { ...agent, state: "idle", effort: "max" } } });
    renderScreen();
    fireEvent.click(screen.getByRole("tab", { name: "Manage" }));
    const model = await screen.findByRole("combobox", { name: "Model" });
    await waitFor(() => expect(model.querySelectorAll("option")).toHaveLength(2));
    fireEvent.change(model, { target: { value: "n" } });
    fireEvent.click(screen.getByRole("button", { name: "Switch runtime" }));
    await waitFor(() => expect(switches).toEqual([{ backend: "claude", model: "n", effort: "low" }]));

    act(() => useConnection.setState({ agents: { a1: { ...agent, state: "idle", model: "n", effort: "high" } } }));
    await waitFor(() => expect(screen.getByRole("combobox", { name: "Model" })).toHaveValue("n"));
    // The live Effort control offers the live model's vocabulary.
    const efforts = screen.getAllByRole("combobox", { name: "Effort" });
    expect(efforts[efforts.length - 1]).toHaveValue("high");
    expect([...efforts[efforts.length - 1].querySelectorAll("option")].map((o) => o.getAttribute("value"))).toEqual(["", "low", "high"]);
  });
});

// FS-20.A11/A12 — agent management, Commands, and Open file send the desktop's
// requests and show its refusals.
describe("AgentScreen management and views", () => {
  const idle: AgentState = { ...agent, state: "idle", detail: "", fast_available: true, effort: "high", clone: { available: true, reason: "" } };
  const options = { backends: [{ id: "claude", name: "Claude", default: true, default_model: "m", models: [{ id: "m", name: "M", efforts: ["low", "high"], default_effort: "high", fast: true }] }] };
  let bodies: string[] = [];
  let refuse = false;
  const record = (name: string, reply: unknown) => async ({ request }: { request: Request }) => {
    bodies.push(`${name} ${await request.text()}`);
    if (refuse) return HttpResponse.json({ error: { code: "conflict", message: `${name} refused by the Mac` } }, { status: 409 });
    return HttpResponse.json(reply);
  };

  beforeEach(() => {
    bodies = [];
    refuse = false;
    window.history.replaceState(null, "", "/agent/a1");
    useConnection.setState({ agents: { a1: idle } });
    server.use(
      http.get("/api/remote/runtime-options", () => HttpResponse.json(options)),
      http.post("/api/sessions/a1/rename", record("rename", {})),
      http.post("/api/sessions/a1/session-config", record("config", { fast: true })),
      http.post("/api/sessions/a1/clone", record("clone", { agent: { agent_id: "a7" }, history_handoff: "native_fork", forked_from_agent_id: "a1", forked_from_seq: 4 })),
    );
  });

  function openManage() {
    renderScreen();
    fireEvent.click(screen.getByRole("tab", { name: "Manage" }));
  }

  it("renames the agent", async () => {
    openManage();
    fireEvent.change(screen.getByRole("textbox", { name: "Name" }), { target: { value: "builder" } });
    fireEvent.click(screen.getByRole("button", { name: "Rename" }));
    await waitFor(() => expect(bodies).toEqual(['rename {"name":"builder"}']));
    await waitFor(() => expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue(""));
  });

  it("shows the Mac's reason and keeps the draft when a rename is refused", async () => {
    refuse = true;
    openManage();
    fireEvent.change(screen.getByRole("textbox", { name: "Name" }), { target: { value: "builder" } });
    fireEvent.click(screen.getByRole("button", { name: "Rename" }));
    expect(await screen.findByText("rename refused by the Mac")).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue("builder");
  });

  it("changes fast mode and effort on the running agent", async () => {
    openManage();
    fireEvent.click(screen.getByRole("checkbox", { name: "Fast mode" }));
    await waitFor(() => expect(bodies).toEqual(['config {"fast":true}']));
    const efforts = await screen.findAllByRole("combobox", { name: "Effort" });
    await waitFor(() => expect(efforts[efforts.length - 1]).toBeEnabled());
    fireEvent.change(efforts[efforts.length - 1], { target: { value: "low" } });
    await waitFor(() => expect(bodies).toEqual(['config {"fast":true}', 'config {"effort":"low"}']));
  });

  it("clones an idle agent and opens the clone", async () => {
    openManage();
    fireEvent.click(screen.getByRole("button", { name: "Clone" }));
    await waitFor(() => expect(window.location.pathname).toBe("/agent/a7"));
    expect(bodies).toEqual(["clone "]);
  });

  it("shows why Clone is unavailable while a turn is active", () => {
    useConnection.setState({ agents: { a1: { ...idle, state: "busy", clone: { available: false, reason: "Wait for the current turn to finish." } } } });
    openManage();
    expect(screen.getByRole("button", { name: "Clone" })).toBeDisabled();
    expect(screen.getByText("Wait for the current turn to finish.")).toBeInTheDocument();
  });

  // FS-20.A14: the phone agent views are Chat, Files and Manage; Commands is gone.
  it("offers Chat, Files and Manage without Commands", () => {
    renderScreen();
    expect(screen.getAllByRole("tab").map((tab) => tab.textContent)).toEqual(["Chat", "Files", "Manage"]);
  });

  it("opens a changed file's current text", async () => {
    const paths: string[] = [];
    server.use(
      http.get("/api/sessions/a1/files", () => HttpResponse.json({ agent_id: "a1", files: [{ path: "src/main.go", edit_count: 2, last_ts: "2026-10-02T00:00:00Z", has_diff: false, diff_refs: [] }] })),
      http.get("/api/sessions/a1/file", ({ request }) => {
        paths.push(new URL(request.url).searchParams.get("path") ?? "");
        return HttpResponse.json({ path: "src/main.go", content: "package main\n", truncated: false });
      }),
    );
    renderScreen();
    fireEvent.click(screen.getByRole("tab", { name: "Files" }));
    fireEvent.click(await screen.findByRole("button", { name: "Open file" }));
    const content = await screen.findByRole("region", { name: "File content" });
    await waitFor(() => expect(content).toHaveTextContent("package main"));
    expect(paths).toEqual(["src/main.go"]);
  });
});

// FS-20.R32 — the phone's diff-line annotate-and-assign form over the shared
// FS-13 tray and batch request.
describe("AgentScreen diff annotations", () => {
  const idle = { ...agent, state: "idle" as const, detail: "" };
  const draft = { seq: 4, path: "main.go", side: "old", start_line: 2, end_line: 2, excerpt: "second" };

  async function annotateLine() {
    renderScreen();
    expect(await screen.findByText("Tap line numbers to select a range.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Old line 2" }));
    fireEvent.click(screen.getByRole("button", { name: "Annotate lines 2–2" }));
    const form = await screen.findByRole("region", { name: "Annotate and assign" });
    expect(form).toHaveTextContent("main.go:2");
    expect(screen.getByRole("button", { name: "Send annotations" })).toBeDisabled();
    const instruction = screen.getByLabelText("Instruction");
    await waitFor(() => expect(instruction).toHaveFocus());
    fireEvent.change(instruction, { target: { value: "rename this" } });
    return instruction;
  }

  it("sends a selected range to this agent and clears the draft", async () => {
    useConnection.setState({ agents: { a1: idle } });
    await annotateLine();
    fireEvent.click(screen.getByRole("button", { name: "Send annotations" }));
    expect(await screen.findByRole("status")).toHaveTextContent("Annotations sent.");
    expect(posts).toContain(`annotations ${JSON.stringify({ annotations: [{ ...draft, instruction: "rename this" }], target: { kind: "self" } })}`);
    expect(useAnnotationStore.getState().bySource.a1).toBeUndefined();
    expect(screen.queryByRole("region", { name: "Annotate and assign" })).toBeNull();
  });

  it("keeps the draft and shows the Mac's reason when delivery fails", async () => {
    annotationStatus = 409;
    const other: AgentState = { ...idle, agent_id: "a2", name: "reviewer-1", role: "reviewer" };
    useConnection.setState({ agents: { a1: idle, a2: other } });
    const instruction = await annotateLine();
    fireEvent.click(screen.getByRole("radio", { name: "Another agent" }));
    fireEvent.change(screen.getByLabelText("Agent"), { target: { value: "a2" } });
    fireEvent.click(screen.getByRole("button", { name: "Send annotations" }));
    expect(await screen.findByText("the recipient agent is not running")).toBeInTheDocument();
    expect(posts).toContain(`annotations ${JSON.stringify({ annotations: [{ ...draft, instruction: "rename this" }], target: { kind: "agent", agent_id: "a2" } })}`);
    expect(instruction).toHaveValue("rename this");
    expect(useAnnotationStore.getState().bySource.a1).toEqual([{ ...draft, instruction: "rename this" }]);
  });

  it("launches a new task with the source's role and project, then assigns to it", async () => {
    useConnection.setState({ agents: { a1: idle } });
    await annotateLine();
    fireEvent.click(screen.getByRole("radio", { name: "New task" }));
    expect(screen.getByText(/Launches a new implementer agent in my-app/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Send annotations" }));
    expect(await screen.findByRole("status")).toHaveTextContent("Annotations sent.");
    expect(posts).toEqual([
      `launch ${JSON.stringify({ role: "implementer", project: "my-app" })}`,
      `annotations ${JSON.stringify({ annotations: [{ ...draft, instruction: "rename this" }], target: { kind: "agent", agent_id: "a3" } })}`,
    ]);
  });

  // The server stores a streamed reply as one assistant_text delta per chunk; the phone must
  // fold them into one message as the desktop does (FS-20.R13, TS-08.R73).
  it("renders a streamed reply as one message, not one row per delta", async () => {
    const delta = (seq: number, text: string) => ({ agent_id: "a1", seq, type: "assistant_text", ts: "", data: { delta: text } });
    live = {
      agent_id: "a1",
      events: [{ agent_id: "a1", seq: 1, type: "user_text", ts: "", data: { text: "Status?" } }, delta(2, "All "), delta(3, "tests "), delta(4, "pass now.")],
      has_more: false,
    };
    renderScreen();
    const conversation = await screen.findByRole("list", { name: "Conversation" });
    await waitFor(() => expect(conversation).toHaveTextContent("All tests pass now."));
    expect(within(conversation).getAllByRole("listitem")).toHaveLength(2);
  });
});
