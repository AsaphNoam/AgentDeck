import React from "react";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
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
  http.post("/api/sessions/a1/archive", () => HttpResponse.json({})),
  http.get("/api/remote/runtime-options", () => HttpResponse.json({ backends: [] })),
);

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
beforeEach(() => {
  permissionStatus = 200;
  promptStatus = 200;
  annotationStatus = 202;
  posts.length = 0;
  reads.length = 0;
  live = fullWindow;
  useConnection.setState({ link: "connected", agents: { a1: agent }, transcriptRev: {} });
  useAnnotationStore.setState({ bySource: {}, overallBySource: {}, editedAt: {}, collapsedBySource: {} });
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
    expect(reads[0]).toContain("limit=150");
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

  it("loads earlier windows on request and keeps them contiguous with the live one", async () => {
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
    fireEvent.click(screen.getByRole("button", { name: "Show earlier" }));
    expect(await screen.findByText("Older question")).toBeInTheDocument();
    expect(reads).toContain("?limit=150&before_seq=151");
    await waitFor(() => expect(reads).toContain("?limit=150&since_seq=150"));
    expect(screen.getByText("Newest question")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Show earlier" })).toBeNull();
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

  it("uses an inline archive confirmation that says restore is desktop-only", async () => {
    renderScreen();
    fireEvent.click(screen.getByRole("tab", { name: "Manage" }));
    fireEvent.click(await screen.findByRole("button", { name: "Archive" }));
    expect(screen.getByText("Archive this agent? Restore is available on the desktop.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Archive agent" })).toBeInTheDocument();
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

  // Reproduces the phone-chat-streamed-deltas investigation finding (HANDOFF review findings):
  // the server stores a streamed reply as one assistant_text delta per chunk, and the phone
  // renders each chunk as its own transcript row. The fix un-skips this test.
  it.skip("renders a streamed reply as one message, not one row per delta", async () => {
    const delta = (seq: number, text: string) => ({ agent_id: "a1", seq, type: "assistant_text", ts: "", data: { delta: text } });
    live = {
      agent_id: "a1",
      events: [{ agent_id: "a1", seq: 1, type: "user_text", ts: "", data: { text: "Status?" } }, delta(2, "All "), delta(3, "tests "), delta(4, "pass now.")],
      has_more: false,
    };
    renderScreen();
    const conversation = await screen.findByRole("list", { name: "Conversation" });
    await waitFor(() => expect(conversation).toHaveTextContent("All tests pass now."));
    expect(conversation.children).toHaveLength(2);
  });
});
