import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach } from "vitest";
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
  server.resetHandlers();
  promptBodies = [];
  failFileSearch = false;
});
afterAll(() => server.close());

// type sets the textarea value and caret (jsdom moves the caret to the end on a
// value set, but we pass it explicitly so detectTrigger reads the real cursor).
function type(el: HTMLTextAreaElement, value: string) {
  fireEvent.change(el, { target: { value, selectionStart: value.length, selectionEnd: value.length } });
}

describe("Composer autocomplete", () => {
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
