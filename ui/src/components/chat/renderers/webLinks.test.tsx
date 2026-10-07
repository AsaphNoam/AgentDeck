import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useAnnotationStore } from "../../../store/annotationStore";
import { useUiStore } from "../../../store/uiStore";
import { TranscriptView } from "../TranscriptView";
import { SanitizedMarkdown } from "./SanitizedMarkdown";

// FS-03.R78/A59, TS-08.R107: web links open in a new tab and offer Open in new
// tab / Copy link on right-click, composed with annotation where it applies.

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  window.getSelection()?.removeAllRanges();
  useAnnotationStore.setState({ bySource: {}, overallBySource: {}, editedAt: {}, collapsedBySource: {} });
  useUiStore.setState({ toasts: [] });
});

const LINKS = [
  "[docs](https://example.com/docs)",
  "[cdn](//cdn.example.com/x)",
  "[mail](mailto:a@example.com)",
  "[jump](#section)",
  "[bad](javascript:alert(1))",
  "[file](src/main.go)",
].join(" ");

function stubClipboard(writeText = vi.fn().mockResolvedValue(undefined)) {
  Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
  return writeText;
}

function renderTranscript(text: string, annotationsEnabled = true) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
  return render(
    <QueryClientProvider client={client}>
      <TranscriptView agentId="a1" events={[{ kind: "assistant_text", seq: 7, text }]} annotationsEnabled={annotationsEnabled} />
    </QueryClientProvider>,
  );
}

function menuLabels() {
  return screen.getAllByRole("menu").map((menu) => [...menu.querySelectorAll("button")].map((button) => button.textContent));
}

describe("web link rendering", () => {
  it("opens only web links in a new tab without opener access", () => {
    const { container } = render(<SanitizedMarkdown text={LINKS} onOpenFile={() => {}} />);

    for (const name of ["docs", "cdn"]) {
      const link = screen.getByRole("link", { name });
      expect(link).toHaveAttribute("target", "_blank");
      expect(link).toHaveAttribute("rel", "noopener noreferrer");
    }
    expect(screen.getByRole("link", { name: "cdn" })).toHaveAttribute("href", "//cdn.example.com/x");
    for (const name of ["mail", "jump"]) expect(screen.getByRole("link", { name })).not.toHaveAttribute("target");
    // Rejected URLs stay inert and local paths stay viewer controls.
    const bad = [...container.querySelectorAll("a")].find((a) => a.textContent === "bad");
    expect(bad).not.toHaveAttribute("target");
    expect(bad?.getAttribute("href") ?? "").not.toContain("javascript");
    expect(screen.getByRole("button", { name: "file" })).toHaveAttribute("data-file-path", "src/main.go");
  });

  it("offers link actions on a surface without annotation", async () => {
    const writeText = stubClipboard();
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    render(<SanitizedMarkdown text={LINKS} />);

    const menuOpened = !fireEvent.contextMenu(screen.getByRole("link", { name: "docs" }), { clientX: 4, clientY: 5 });
    expect(menuOpened).toBe(true);
    await screen.findByRole("menu");
    expect(menuLabels()).toEqual([["Open in new tab", "Copy link"]]);

    fireEvent.click(screen.getByRole("button", { name: "Copy link" }));
    expect(writeText).toHaveBeenCalledWith("https://example.com/docs");

    fireEvent.contextMenu(screen.getByRole("link", { name: "docs" }));
    fireEvent.click(await screen.findByRole("button", { name: "Open in new tab" }));
    expect(open).toHaveBeenCalledWith("https://example.com/docs", "_blank", "noopener,noreferrer");
  });

  it("reports a refused clipboard with the existing Copy failed feedback", async () => {
    stubClipboard(vi.fn().mockRejectedValue(new Error("denied")));
    render(<SanitizedMarkdown text={LINKS} />);

    fireEvent.contextMenu(screen.getByRole("link", { name: "docs" }));
    fireEvent.click(await screen.findByRole("button", { name: "Copy link" }));

    await waitFor(() => expect(useUiStore.getState().toasts.map((toast) => toast.title)).toContain("Copy failed"));
  });

  it("gives non-web links no link menu", async () => {
    render(<SanitizedMarkdown text={LINKS} />);

    expect(fireEvent.contextMenu(screen.getByRole("link", { name: "mail" }))).toBe(true);
    await Promise.resolve();
    expect(screen.queryByRole("menu")).toBeNull();
  });
});

describe("web links in an annotated transcript", () => {
  it("composes link actions with annotation in one menu, including inside a table", async () => {
    stubClipboard();
    renderTranscript("| Ref |\n| --- |\n| [docs](https://example.com/docs) |");

    fireEvent.contextMenu(screen.getByRole("link", { name: "docs" }), { clientX: 4, clientY: 5 });
    await Promise.resolve();

    expect(menuLabels()).toEqual([["Open in new tab", "Copy link", "Annotate whole event"]]);
    fireEvent.click(screen.getByRole("button", { name: "Annotate whole event" }));
    expect(useAnnotationStore.getState().bySource.a1).toHaveLength(1);
  });

  it("keeps selection copy beside the link actions", async () => {
    const writeText = stubClipboard();
    renderTranscript("See [docs](https://example.com/docs) now");
    const link = screen.getByRole("link", { name: "docs" });
    const range = document.createRange();
    range.selectNodeContents(link);
    window.getSelection()?.addRange(range);

    fireEvent.contextMenu(link);
    await Promise.resolve();

    expect(menuLabels()).toEqual([["Open in new tab", "Copy link", "Copy selection", "Annotate selection"]]);
    fireEvent.click(screen.getByRole("button", { name: "Copy selection" }));
    expect(writeText).toHaveBeenCalledWith("docs");
  });

  it("still offers link actions where annotation is disabled", async () => {
    renderTranscript("See [docs](https://example.com/docs)", false);

    fireEvent.contextMenu(screen.getByRole("link", { name: "docs" }));

    await screen.findByRole("menu");
    expect(menuLabels()).toEqual([["Open in new tab", "Copy link"]]);
  });
});
