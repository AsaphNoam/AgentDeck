import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach, vi } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { Composer } from "./Composer";
import { getChatDraft } from "./drafts";
import { useHeldStore } from "../../store/heldStore";
import { useTranscriptStore } from "../../store/transcriptStore";

// Files returned by the mock file-search, narrowed by the q param so the test can
// assert query filtering (FS-03.R30/A15).
const filesFor = (q: string) => {
  const all = ["src/app.ts", "src/util.ts", "README.md"];
  return q ? all.filter((f) => f.toLowerCase().includes(q.toLowerCase())) : all;
};

let promptBodies: string[] = [];
let failFileSearch = false;
let resolvePrompt: ((response: Response) => void) | null = null;

const server = setupServer(
  http.get("/api/sessions/:id/file-search", ({ request }) => {
    if (failFileSearch) return new HttpResponse(null, { status: 409 });
    const q = new URL(request.url).searchParams.get("q") ?? "";
    return HttpResponse.json({ agent_id: "a_1", files: filesFor(q) });
  }),
  http.get("/api/sessions/:id/available-commands", () =>
    HttpResponse.json({
      agent_id: "a_1",
      commands: [
        { name: "review", description: "Review code", input_hint: "branch" },
        { name: "$plan", description: "Plan work" },
      ],
    }),
  ),
  http.post("/api/sessions/:id/prompt", async ({ request }) => {
    promptBodies.push(await request.text());
    return HttpResponse.json({}, { status: 202 });
  }),
);

beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterEach(() => {
  cleanup();
  localStorage.clear();
  useHeldStore.setState({ byAgent: {}, afterSeqByAgent: {} });
  useTranscriptStore.setState({ byAgent: {}, rawByAgent: {}, pending: {} });
  server.resetHandlers();
  promptBodies = [];
  failFileSearch = false;
  resolvePrompt = null;
  vi.restoreAllMocks();
});
afterAll(() => server.close());

// type sets the textarea value and caret (jsdom moves the caret to the end on a
// value set, but we pass it explicitly so detectTrigger reads the real cursor).
function type(el: HTMLTextAreaElement, value: string) {
  fireEvent.change(el, { target: { value, selectionStart: value.length, selectionEnd: value.length } });
}

describe("Composer autocomplete", () => {
  // FS-03.A19: browser-local drafts follow the selected chat through navigation
  // and remounts, but never cross to another agent.
  it("restores distinct drafts for their matching chat after navigation and remount", async () => {
    const view = render(<Composer agentId="a_1" busy={false} />);
    const ta = screen.getByRole("textbox") as HTMLTextAreaElement;
    type(ta, "first draft");

    view.rerender(<Composer agentId="a_2" busy={false} />);
    await waitFor(() => expect(ta.value).toBe(""));
    type(ta, "second draft");

    view.rerender(<Composer agentId="a_1" busy={false} />);
    await waitFor(() => expect(ta.value).toBe("first draft"));
    view.unmount();

    render(<Composer agentId="a_2" busy={false} />);
    expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("second draft");
  });

  it("clears the stored draft only after an accepted prompt or manual emptying", async () => {
    const view = render(<Composer agentId="a_1" busy={false} />);
    const ta = screen.getByRole("textbox") as HTMLTextAreaElement;
    type(ta, "send this");
    fireEvent.keyDown(ta, { key: "Enter" });
    await waitFor(() => expect(promptBodies).toHaveLength(1));
    view.unmount();

    render(<Composer agentId="a_1" busy={false} />);
    expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("");
    type(screen.getByRole("textbox") as HTMLTextAreaElement, "remove this");
    type(screen.getByRole("textbox") as HTMLTextAreaElement, "");
    cleanup();

    render(<Composer agentId="a_1" busy={false} />);
    expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("");
  });

  it("keeps the live composer usable when browser storage is unavailable", async () => {
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("storage unavailable"); });
    render(<Composer agentId="a_1" busy={false} />);
    const ta = screen.getByRole("textbox") as HTMLTextAreaElement;
    type(ta, "send without storage");
    fireEvent.keyDown(ta, { key: "Enter" });

    await waitFor(() => expect(promptBodies).toHaveLength(1));
    expect(JSON.parse(promptBodies[0]).text).toBe("send without storage");
  });

  it("opens the file picker on `@` at a word boundary but not inside a word", async () => {
    render(<Composer agentId="a_1" busy={false} />);
    const ta = screen.getByRole("textbox") as HTMLTextAreaElement;

    type(ta, "look at @src");
    expect(await screen.findByText("src/app.ts")).toBeInTheDocument();
    expect(screen.getByText("src/util.ts")).toBeInTheDocument();

    // `@` inside a word (email) must not open the picker.
    type(ta, "mail user@example.com");
    await waitFor(() => expect(screen.queryByRole("listbox")).not.toBeInTheDocument());
  });

  it("filters, navigates with arrows, and accepts a file inserting `@<path> `", async () => {
    render(<Composer agentId="a_1" busy={false} />);
    const ta = screen.getByRole("textbox") as HTMLTextAreaElement;

    type(ta, "@app");
    expect(await screen.findByText("src/app.ts")).toBeInTheDocument();
    expect(screen.queryByText("src/util.ts")).not.toBeInTheDocument();

    type(ta, "@src");
    await screen.findByText("src/util.ts");
    // Highlight starts at 0 (app.ts); ArrowDown moves to util.ts, Enter accepts it.
    fireEvent.keyDown(ta, { key: "ArrowDown" });
    fireEvent.keyDown(ta, { key: "Enter" });

    await waitFor(() => expect(ta.value).toBe("@src/util.ts "));
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });

  it("dismisses the picker on Escape", async () => {
    render(<Composer agentId="a_1" busy={false} />);
    const ta = screen.getByRole("textbox") as HTMLTextAreaElement;
    type(ta, "@src");
    await screen.findByText("src/app.ts");
    fireEvent.keyDown(ta, { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("listbox")).not.toBeInTheDocument());
  });

  it("submits the unchanged prompt when the picker is closed", async () => {
    render(<Composer agentId="a_1" busy={false} />);
    const ta = screen.getByRole("textbox") as HTMLTextAreaElement;
    type(ta, "hello there");
    fireEvent.keyDown(ta, { key: "Enter" });
    await waitFor(() => expect(promptBodies.length).toBe(1));
    expect(JSON.parse(promptBodies[0]).text).toBe("hello there");
    expect(ta.value).toBe("");
  });

  it("opens the command picker on `#` and inserts `/<name> ` including `/$skill`", async () => {
    render(<Composer agentId="a_1" busy={false} />);
    const ta = screen.getByRole("textbox") as HTMLTextAreaElement;

    type(ta, "#");
    expect(await screen.findByText("/review")).toBeInTheDocument();
    expect(screen.getByText("/$plan")).toBeInTheDocument();

    // Filter to the codex-style skill and accept it.
    type(ta, "#plan");
    await screen.findByText("/$plan");
    fireEvent.keyDown(ta, { key: "Enter" });
    await waitFor(() => expect(ta.value).toBe("/$plan "));
  });

  it("submits selected file and command text exactly as displayed, keeping the trailing space", async () => {
    render(<Composer agentId="a_1" busy={false} />);
    const ta = screen.getByRole("textbox") as HTMLTextAreaElement;

    // Accept a file: inserts `@src/app.ts ` (with trailing space).
    type(ta, "@app");
    await screen.findByText("src/app.ts");
    fireEvent.keyDown(ta, { key: "Enter" });
    await waitFor(() => expect(ta.value).toBe("@src/app.ts "));

    // Continue and accept a command: inserts `/$plan ` (with trailing space).
    type(ta, "@src/app.ts #plan");
    await screen.findByText("/$plan");
    fireEvent.keyDown(ta, { key: "Enter" });
    await waitFor(() => expect(ta.value).toBe("@src/app.ts /$plan "));

    // The picker is closed after accepting, so Enter submits. The request body must
    // retain the inserted trailing space (FS-03.R31/R33, INV §1).
    fireEvent.keyDown(ta, { key: "Enter" });
    await waitFor(() => expect(promptBodies.length).toBe(1));
    expect(JSON.parse(promptBodies[0]).text).toBe("@src/app.ts /$plan ");
  });

  // FS-03.R35/A18 — the composer of a stopped chat agent is the wake surface: it
  // submits the ordinary prompt request, and a rejected wake shows the server's
  // own error while keeping the draft.
  it("submits the ordinary prompt request for a stopped agent", async () => {
    render(<Composer agentId="a_1" busy={false} />);
    const ta = screen.getByRole("textbox") as HTMLTextAreaElement;
    type(ta, "wake up");
    fireEvent.keyDown(ta, { key: "Enter" });
    await waitFor(() => expect(promptBodies.length).toBe(1));
    expect(JSON.parse(promptBodies[0]).text).toBe("wake up");
    expect(ta.value).toBe("");
  });

  it("shows the server error and restores the draft when the wake is rejected", async () => {
    server.use(
      http.post("/api/sessions/:id/prompt", () =>
        HttpResponse.json(
          { error: { code: "agent_archived", message: "agent is archived; restore it before resuming" } },
          { status: 409 },
        ),
      ),
    );
    render(<Composer agentId="a_1" busy={false} />);
    const ta = screen.getByRole("textbox") as HTMLTextAreaElement;
    type(ta, "wake up");
    fireEvent.keyDown(ta, { key: "Enter" });

    expect(await screen.findByText(/agent is archived; restore it before resuming/)).toBeInTheDocument();
    await waitFor(() => expect(ta.value).toBe("wake up"));
    cleanup();

    render(<Composer agentId="a_1" busy={false} />);
    expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("wake up");
  });

  it("does not restore a rejected send into a different chat opened while it was pending", async () => {
    server.use(
      http.post("/api/sessions/:id/prompt", () => new Promise<Response>((resolve) => { resolvePrompt = resolve; })),
    );
    const view = render(<Composer agentId="a_1" busy={false} />);
    const ta = screen.getByRole("textbox") as HTMLTextAreaElement;
    type(ta, "first draft");
    fireEvent.keyDown(ta, { key: "Enter" });

    view.rerender(<Composer agentId="a_2" busy={false} />);
    await waitFor(() => expect(ta.value).toBe(""));
    type(ta, "second draft");
    resolvePrompt?.(HttpResponse.json({ error: { code: "agent_archived", message: "agent is archived" } }, { status: 409 }));

    await waitFor(() => expect(ta.value).toBe("second draft"));
    view.unmount();
    render(<Composer agentId="a_1" busy={false} />);
    expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("first draft");
  });

  // INV §1/§5 — a delayed send's completion is scoped to the draft generation it
  // submitted, so a newer same-agent draft typed while the request is in flight
  // is never discarded on success nor overwritten on failure.
  it("keeps a newer same-agent draft typed while a successful send is in flight", async () => {
    server.use(
      http.post("/api/sessions/:id/prompt", () => new Promise<Response>((resolve) => { resolvePrompt = resolve; })),
    );
    const view = render(<Composer agentId="a_1" busy={false} />);
    const ta = screen.getByRole("textbox") as HTMLTextAreaElement;
    type(ta, "message A");
    fireEvent.keyDown(ta, { key: "Enter" });
    await waitFor(() => expect(ta.value).toBe(""));

    // The person starts a new draft for the same chat before A's send resolves.
    type(ta, "message B");
    resolvePrompt?.(HttpResponse.json({}, { status: 202 }));
    await new Promise((resolve) => setTimeout(resolve, 0));

    // A's accepted send must not discard the newer draft B.
    expect(ta.value).toBe("message B");
    view.unmount();
    render(<Composer agentId="a_1" busy={false} />);
    expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("message B");
  });

  it("keeps a newer same-agent draft typed while a rejected send is in flight", async () => {
    server.use(
      http.post("/api/sessions/:id/prompt", () => new Promise<Response>((resolve) => { resolvePrompt = resolve; })),
    );
    const view = render(<Composer agentId="a_1" busy={false} />);
    const ta = screen.getByRole("textbox") as HTMLTextAreaElement;
    type(ta, "message A");
    fireEvent.keyDown(ta, { key: "Enter" });
    await waitFor(() => expect(ta.value).toBe(""));

    type(ta, "message B");
    resolvePrompt?.(HttpResponse.json({ error: { code: "agent_archived", message: "agent is archived" } }, { status: 409 }));
    await new Promise((resolve) => setTimeout(resolve, 0));

    // A's rejection must not overwrite the newer draft B with the sent text.
    expect(ta.value).toBe("message B");
    view.unmount();
    render(<Composer agentId="a_1" busy={false} />);
    expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("message B");
  });

  it("stays usable when the file source is unavailable", async () => {
    failFileSearch = true;
    render(<Composer agentId="a_1" busy={false} />);
    const ta = screen.getByRole("textbox") as HTMLTextAreaElement;

    type(ta, "@src");
    // Empty/unavailable state renders and never blocks submission.
    expect(await screen.findByText("No matching files")).toBeInTheDocument();

    type(ta, "send this");
    fireEvent.keyDown(ta, { key: "Enter" });
    await waitFor(() => expect(promptBodies.length).toBe(1));
    expect(JSON.parse(promptBodies[0]).text).toBe("send this");
  });
});

// FS-03.A31/A32/A33 — the composer half of the two deliberate actions on a busy
// agent: Send queues and is withdrawable, Steer delivers into the running turn
// and only where the live session advertises it, and a lost agent returns the
// held text without ever overwriting text typed since.
describe("Composer queued follow-up and steering", () => {
  it("holds a message sent to a busy agent instead of echoing it into the transcript", async () => {
    server.use(http.post("/api/sessions/:id/prompt", async ({ request }) => {
      promptBodies.push(await request.text());
      return HttpResponse.json({ accepted: true, agent_id: "a_1", delivery: "held" }, { status: 202 });
    }));
    render(<Composer agentId="a_1" busy />);
    const ta = screen.getByRole("textbox") as HTMLTextAreaElement;

    type(ta, "also check the tests");
    fireEvent.keyDown(ta, { key: "Enter" });

    await waitFor(() => expect(useHeldStore.getState().byAgent.a_1).toBe("also check the tests"));
    expect(ta.value).toBe("");
    // Send stays present on a busy agent; Cancel is still offered beside it.
    expect(screen.getByRole("button", { name: "Send" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeInTheDocument();
  });

  it("does not echo a message when an idle-looking submit is held by the server", async () => {
    server.use(http.post("/api/sessions/:id/prompt", () =>
      HttpResponse.json({ accepted: true, agent_id: "a_1", delivery: "held", after_seq: 8 }, { status: 202 })));
    render(<Composer agentId="a_1" busy={false} />);
    const ta = screen.getByRole("textbox") as HTMLTextAreaElement;

    type(ta, "raced with another turn");
    fireEvent.keyDown(ta, { key: "Enter" });

    await waitFor(() => expect(useHeldStore.getState().byAgent.a_1).toBe("raced with another turn"));
    expect(useTranscriptStore.getState().byAgent.a_1 ?? []).toHaveLength(0);
  });

  it("withdraws the held message through the prompt resource", async () => {
    let withdrawn = 0;
    server.use(
      http.post("/api/sessions/:id/prompt", () =>
        HttpResponse.json({ accepted: true, agent_id: "a_1", delivery: "held" }, { status: 202 })),
      http.delete("/api/sessions/:id/prompt", () => {
        withdrawn++;
        return HttpResponse.json({ accepted: true, agent_id: "a_1", delivery: "withdrawn" });
      }),
    );
    render(<Composer agentId="a_1" busy />);
    const ta = screen.getByRole("textbox") as HTMLTextAreaElement;
    type(ta, "never mind");
    fireEvent.keyDown(ta, { key: "Enter" });
    await waitFor(() => expect(useHeldStore.getState().byAgent.a_1).toBe("never mind"));

    fireEvent.click(screen.getByRole("button", { name: "Withdraw queued" }));
    await waitFor(() => expect(useHeldStore.getState().byAgent.a_1).toBeUndefined());
    expect(withdrawn).toBe(1);
  });

  it("offers Steer only where the live session advertises it", () => {
    const view = render(<Composer agentId="a_1" busy steerable={false} />);
    expect(screen.queryByRole("button", { name: "Steer" })).not.toBeInTheDocument();
    view.unmount();

    render(<Composer agentId="a_1" busy steerable />);
    expect(screen.getByRole("button", { name: "Steer" })).toBeInTheDocument();
  });

  it("reports which of the two things the adapter did with a steered message", async () => {
    let outcome = "steered";
    server.use(http.post("/api/sessions/:id/steer", () =>
      HttpResponse.json({ accepted: true, agent_id: "a_1", outcome }, { status: 202 })));

    const view = render(<Composer agentId="a_1" busy steerable />);
    const ta = screen.getByRole("textbox") as HTMLTextAreaElement;
    type(ta, "use the other file");
    fireEvent.click(screen.getByRole("button", { name: "Steer" }));
    expect(await screen.findByText("Delivered into the running turn.")).toBeInTheDocument();
    expect(ta.value).toBe("");
    view.unmount();

    outcome = "new_turn";
    render(<Composer agentId="a_1" busy steerable />);
    type(screen.getByRole("textbox") as HTMLTextAreaElement, "too late");
    fireEvent.click(screen.getByRole("button", { name: "Steer" }));
    expect(await screen.findByText("That turn had already ended — sent as a new turn.")).toBeInTheDocument();
  });

  it("steers the held message when the composer is empty, and keeps typed text on a refusal", async () => {
    let steerBodies: string[] = [];
    server.use(
      http.post("/api/sessions/:id/prompt", () =>
        HttpResponse.json({ accepted: true, agent_id: "a_1", delivery: "held" }, { status: 202 })),
      http.post("/api/sessions/:id/steer", async ({ request }) => {
        const body = await request.text();
        steerBodies.push(body);
        if (JSON.parse(body).text !== "") {
          return HttpResponse.json({ error: { code: "internal", message: "the model cannot accept that" } }, { status: 500 });
        }
        return HttpResponse.json({ accepted: true, agent_id: "a_1", outcome: "steered" }, { status: 202 });
      }),
    );
    render(<Composer agentId="a_1" busy steerable />);
    const ta = screen.getByRole("textbox") as HTMLTextAreaElement;

    type(ta, "queued first");
    fireEvent.keyDown(ta, { key: "Enter" });
    await waitFor(() => expect(useHeldStore.getState().byAgent.a_1).toBe("queued first"));

    // Empty composer + a held message: Steer delivers the held one.
    fireEvent.click(screen.getByRole("button", { name: "Steer" }));
    await waitFor(() => expect(useHeldStore.getState().byAgent.a_1).toBeUndefined());
    expect(JSON.parse(steerBodies[0]).text).toBe("");

    // A refused steer leaves the composer's text exactly where it was.
    type(ta, "not acceptable");
    fireEvent.click(screen.getByRole("button", { name: "Steer" }));
    await waitFor(() => expect(screen.getByText(/Could not steer/)).toBeInTheDocument());
    expect(ta.value).toBe("not acceptable");
  });

  it("returns a held message to an empty composer when the agent stops, and discards it over typed text", async () => {
    server.use(http.post("/api/sessions/:id/prompt", () =>
      HttpResponse.json({ accepted: true, agent_id: "a_1", delivery: "held" }, { status: 202 })));

    const view = render(<Composer agentId="a_1" busy running />);
    const ta = screen.getByRole("textbox") as HTMLTextAreaElement;
    type(ta, "held text");
    fireEvent.keyDown(ta, { key: "Enter" });
    await waitFor(() => expect(useHeldStore.getState().byAgent.a_1).toBe("held text"));

    view.rerender(<Composer agentId="a_1" busy={false} running={false} />);
    await waitFor(() => expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("held text"));
    expect(useHeldStore.getState().byAgent.a_1).toBeUndefined();
    expect(getChatDraft("a_1")).toBe("held text");
    view.unmount();

    // Same stop, but the person has since typed: their text wins and the held
    // message is discarded rather than overwriting it.
    const second = render(<Composer agentId="a_2" busy running />);
    const ta2 = screen.getByRole("textbox") as HTMLTextAreaElement;
    type(ta2, "queue me");
    fireEvent.keyDown(ta2, { key: "Enter" });
    await waitFor(() => expect(useHeldStore.getState().byAgent.a_2).toBe("queue me"));
    type(ta2, "typed since");

    second.rerender(<Composer agentId="a_2" busy={false} running={false} />);
    await waitFor(() => expect(useHeldStore.getState().byAgent.a_2).toBeUndefined());
    expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("typed since");
    expect(getChatDraft("a_2")).toBe("typed since");
  });
});
