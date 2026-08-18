import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach, vi } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { Composer } from "./Composer";

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
