import React from "react";
import fs from "node:fs";
import path from "node:path";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { FileViewer } from "./FileViewer";
import { fileLinkFromParams, writeFileLinkParams } from "../../lib/fileLinkParams";

const GO_SOURCE = "package state\n\nfunc Read() {}\n";

function fileBody(overrides: Record<string, unknown> = {}) {
  return {
    agent_id: "a_1",
    path: "internal/state/messages.go",
    size: GO_SOURCE.length,
    mod_time: "2026-09-10T10:00:00Z",
    line_count: 3,
    content: GO_SOURCE,
    truncated: false,
    language: "go",
    ...overrides,
  };
}

let reads: string[] = [];

const server = setupServer(
  http.get("/api/sessions/:id/file", ({ request }) => {
    const requested = new URL(request.url).searchParams.get("path") ?? "";
    reads.push(requested);
    if (requested.endsWith(".md")) {
      return HttpResponse.json(fileBody({ path: requested, content: "# Title\n\nSee [other](other.go).\n", language: "markdown", line_count: 3 }));
    }
    return HttpResponse.json(fileBody({ path: requested }));
  }),
);

beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterEach(() => { cleanup(); server.resetHandlers(); reads = []; });
afterAll(() => server.close());

function refuse(code: string, message: string, status = 422) {
  server.use(http.get("/api/sessions/:id/file", () =>
    HttpResponse.json({ error: { code, message, details: {} } }, { status })));
}

describe("FileViewer", () => {
  // FS-03.A35 (R52) — the viewer's content, its line anchor, replace-on-open,
  // Reload, and the Markdown toggle.
  it("renders the file's text with line numbers and names the file it read", async () => {
    render(<FileViewer agentId="a_1" link={{ path: "internal/state/messages.go" }} onClose={vi.fn()} />);

    expect(await screen.findByText(/package/)).toBeInTheDocument();
    expect(screen.getByText("internal/state/messages.go")).toBeInTheDocument();
    // Line numbering is what makes a cited line findable at all.
    expect(document.querySelector('[data-file-line="1"]')).not.toBeNull();
    expect(document.querySelector('[data-file-line="3"]')).not.toBeNull();
    // The panel states when it read, because it deliberately does not watch the file.
    expect(screen.getByText(/read/)).toBeInTheDocument();
  });

  it("scrolls to and marks a cited line", async () => {
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView = scrollIntoView;
    render(<FileViewer agentId="a_1" link={{ path: "internal/state/messages.go", line: 2 }} onClose={vi.fn()} />);

    await waitFor(() => expect(document.querySelector('[data-file-marked="true"]')).not.toBeNull());
    expect(document.querySelectorAll('[data-file-marked="true"]')).toHaveLength(1);
    expect(document.querySelector('[data-file-marked="true"]')?.getAttribute("data-file-line")).toBe("2");
    expect(scrollIntoView).toHaveBeenCalled();
  });

  it("replaces the open file when another is opened, holding one file and no tabs", async () => {
    const view = render(<FileViewer agentId="a_1" link={{ path: "internal/state/messages.go" }} onClose={vi.fn()} />);
    await screen.findByText("internal/state/messages.go");

    view.rerender(<FileViewer agentId="a_1" link={{ path: "ui/src/routes.tsx" }} onClose={vi.fn()} />);
    await screen.findByText("ui/src/routes.tsx");

    expect(screen.queryByText("internal/state/messages.go")).not.toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Close" })).toHaveLength(1);
    expect(reads).toEqual(["internal/state/messages.go", "ui/src/routes.tsx"]);
  });

  it("re-reads only on Reload, never by watching or polling", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      render(<FileViewer agentId="a_1" link={{ path: "internal/state/messages.go" }} onClose={vi.fn()} />);
      await waitFor(() => expect(reads).toHaveLength(1));

      await vi.advanceTimersByTimeAsync(30_000);
      expect(reads).toHaveLength(1);

      fireEvent.click(screen.getByRole("button", { name: "Reload" }));
      await waitFor(() => expect(reads).toHaveLength(2));
    } finally {
      vi.useRealTimers();
    }
  });

  it("offers Rendered/Source for Markdown only, with the rendered form going through the sanitized renderer", async () => {
    render(<FileViewer agentId="a_1" link={{ path: "docs/notes.md" }} onClose={vi.fn()} onOpenFile={vi.fn()} />);

    // Rendered is the default: the heading is a heading, not literal `# Title`.
    expect(await screen.findByRole("heading", { name: "Title" })).toBeInTheDocument();
    // The rendered form gains no capability beyond an assistant message: its own
    // file link is the same upgraded control, not a browser navigation.
    expect(screen.getByRole("button", { name: "other" })).toHaveAttribute("data-file-path", "other.go");

    fireEvent.click(screen.getByRole("button", { name: "Source" }));
    await waitFor(() => expect(screen.queryByRole("heading", { name: "Title" })).not.toBeInTheDocument());
    expect(document.querySelector('[data-file-line="1"]')).not.toBeNull();

    cleanup();
    render(<FileViewer agentId="a_1" link={{ path: "internal/state/messages.go" }} onClose={vi.fn()} />);
    await screen.findByText(/package/);
    expect(screen.queryByRole("button", { name: "Rendered" })).not.toBeInTheDocument();
  });

  it("closes without reading anything else", async () => {
    const onClose = vi.fn();
    render(<FileViewer agentId="a_1" link={{ path: "internal/state/messages.go" }} onClose={onClose} />);
    await screen.findByText(/package/);

    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalledOnce();
  });

  // FS-03.A37 (R55) — every refusal names its reason in the viewer's own surface,
  // leaving the transcript readable.
  it("states each containment refusal in its own surface", async () => {
    for (const [code, message] of [
      ["path_refused", "that path is outside this agent's working directory"],
      ["not_found", "that file no longer exists"],
      ["workspace_unavailable", "this agent's working directory is no longer available"],
      ["not_a_file", "that path names a directory, not a file"],
      ["not_text", "that file is not text"],
    ] as const) {
      refuse(code, message);
      render(<FileViewer agentId="a_1" link={{ path: "anything" }} onClose={vi.fn()} />);
      expect(await screen.findByRole("alert")).toHaveTextContent(message);
      cleanup();
      server.resetHandlers();
    }
  });

  it("labels a bounded partial read rather than implying the whole file", async () => {
    server.use(http.get("/api/sessions/:id/file", () =>
      HttpResponse.json(fileBody({ truncated: true, line_count: 2000, size: 9_000_000 }))));
    render(<FileViewer agentId="a_1" link={{ path: "build.log" }} onClose={vi.fn()} />);

    expect(await screen.findByText(/larger than the viewer reads/)).toBeInTheDocument();
  });

  it("ignores a slower earlier read landing after a newer one", async () => {
    // Opening another file while the first is still in flight must not be
    // overwritten when the first answer arrives (INV §1).
    let release: (() => void) | null = null;
    server.use(http.get("/api/sessions/:id/file", async ({ request }) => {
      const requested = new URL(request.url).searchParams.get("path") ?? "";
      if (requested === "slow.go") {
        await new Promise<void>((resolve) => { release = resolve; });
        return HttpResponse.json(fileBody({ path: "slow.go", content: "STALE\n" }));
      }
      return HttpResponse.json(fileBody({ path: requested, content: "FRESH\n" }));
    }));

    const view = render(<FileViewer agentId="a_1" link={{ path: "slow.go" }} onClose={vi.fn()} />);
    view.rerender(<FileViewer agentId="a_1" link={{ path: "fast.go" }} onClose={vi.fn()} />);
    await screen.findByText(/FRESH/);

    release?.();
    await new Promise((resolve) => setTimeout(resolve, 10));
    expect(screen.queryByText(/STALE/)).not.toBeInTheDocument();
    expect(screen.getByText(/FRESH/)).toBeInTheDocument();
  });
});

// FS-03.A36 (R53, R54) — the address is the open-file state, and the two layout
// forms are one component in two CSS states.
describe("the open file as part of the address", () => {
  it("round-trips ?file= and ?fileLine= and clears both on Close", () => {
    expect(fileLinkFromParams(new URLSearchParams("tab=files"))).toBeNull();
    expect(fileLinkFromParams(new URLSearchParams("file=main.go"))).toEqual({ path: "main.go" });
    expect(fileLinkFromParams(new URLSearchParams("file=main.go&fileLine=12"))).toEqual({ path: "main.go", line: 12 });
    // A parameter that cannot be resolved opens the viewer rather than failing
    // silently; a nonsense line just drops the anchor (FS-03.R54).
    expect(fileLinkFromParams(new URLSearchParams("file=main.go&fileLine=nope"))).toEqual({ path: "main.go" });

    const opened = writeFileLinkParams(new URLSearchParams("tab=transcript"), { path: "a/b.go", line: 4 });
    expect(opened.get("tab")).toBe("transcript");
    expect(opened.get("file")).toBe("a/b.go");
    expect(opened.get("fileLine")).toBe("4");

    const closed = writeFileLinkParams(opened, null);
    expect(closed.get("tab")).toBe("transcript");
    expect(closed.get("file")).toBeNull();
    expect(closed.get("fileLine")).toBeNull();

    // The two keys are always written together, so a stale line cannot outlive
    // the file that cited it.
    expect(writeFileLinkParams(opened, { path: "c.go" }).get("fileLine")).toBeNull();
  });

  // jsdom evaluates no CSS and knows nothing of container queries, so the
  // stylesheet is the only witness that the viewer docks in its own leading track
  // and that the width cap relaxes by state rather than measurement (INV §13).
  it("docks through the transcript region's own grid with no measured width", () => {
    const css = fs.readFileSync(path.resolve(__dirname, "../../styles/features/agent.css"), "utf8");

    expect(css).toMatch(/\.transcript-wrap \{[^}]*grid-template-columns: auto minmax\(0, 1fr\) auto/);
    expect(css).toMatch(/\.file-viewer \{[^}]*grid-column: 1/);
    expect(css).toMatch(/\.transcript-view \{[^}]*grid-column: 2/);
    // The tray keeps the trailing track, so a docked tray and an open file
    // coexist rather than compete (FS-03.R53).
    expect(css).toMatch(/\.annotation-tray \{[^}]*grid-column: 3/);
    // Narrow form: the column takes width from the transcript.
    expect(css).toMatch(/\.file-viewer \{[^}]*width: min\(46cqi, 520px\)/);
    expect(css).toMatch(/\.file-viewer-body \[data-file-marked="true"\]/);
    // Docked form: the relaxed content cap pays for it instead.
    const docked = css.match(/@container transcript \(min-width: 860px\) \{([\s\S]*?)\n\}/);
    expect(docked).not.toBeNull();
    expect(docked![1]).toMatch(/\.file-viewer \{[^}]*width: min\(38cqi, 640px\)/);
    expect(css).toMatch(/\.chat-panel\[data-file-open="true"\] \{[^}]*max-width/);
  });
});
