import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import * as client from "../../api/client";
import { useAgentStore } from "../../store/agentStore";
import { useAnnotationStore } from "../../store/annotationStore";
import { useHeldStore } from "../../store/heldStore";
import { foldTranscript, useTranscriptStore } from "../../store/transcriptStore";
import { annotationBlockSentinel } from "../../lib/annotations";
import { TranscriptView } from "./TranscriptView";

const mswServer = setupServer();
beforeAll(() => mswServer.listen({ onUnhandledRequest: "bypass" }));
afterAll(() => mswServer.close());

afterEach(() => {
  cleanup();
  window.getSelection()?.removeAllRanges();
  useAnnotationStore.setState({ bySource: {}, overallBySource: {}, editedAt: {}, collapsedBySource: {} });
  useTranscriptStore.setState({ byAgent: {}, rawByAgent: {}, pending: {} });
  useHeldStore.setState({ byAgent: {}, afterSeqByAgent: {} });
  useAgentStore.setState({ agents: {} });
  mswServer.resetHandlers();
});

const events = [{ kind: "assistant_text", seq: 7, text: "First line\nSecond line" }];

function renderTranscript(annotationsEnabled = true, transcriptEvents = events, busy = false) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
  return render(
    <QueryClientProvider client={client}>
      <TranscriptView agentId="a1" events={transcriptEvents} sourceActive annotationsEnabled={annotationsEnabled} busy={busy} />
    </QueryClientProvider>,
  );
}

function selectWithin(node: Node) {
  const range = document.createRange();
  range.selectNodeContents(node);
  const selection = window.getSelection();
  selection?.removeAllRanges();
  selection?.addRange(range);
}

describe("TranscriptView annotation entry point", () => {
  it("shows no standing Annotate control on transcript events", () => {
    renderTranscript();

    expect(screen.queryByRole("button", { name: /annotate/i })).toBeNull();
  });

  it("captures the highlighted text when the selection is right-clicked", () => {
    const { container } = renderTranscript();
    const item = container.querySelector('[data-slot="event"]') as HTMLElement;
    const text = document.createTextNode("Second line");
    item.appendChild(text);
    selectWithin(text);

    fireEvent.contextMenu(item, { clientX: 20, clientY: 30 });
    fireEvent.click(screen.getByRole("button", { name: "Annotate selection" }));

    expect(useAnnotationStore.getState().bySource.a1).toEqual([
      { seq: 7, excerpt: "Second line", instruction: "" },
    ]);
  });

  it("trims surrounding whitespace from an annotated selection", () => {
    const { container } = renderTranscript();
    const item = container.querySelector('[data-slot="event"]') as HTMLElement;
    const text = document.createTextNode("  Second line  ");
    item.appendChild(text);
    selectWithin(text);

    fireEvent.contextMenu(item, { clientX: 20, clientY: 30 });
    fireEvent.click(screen.getByRole("button", { name: "Annotate selection" }));

    expect(useAnnotationStore.getState().bySource.a1).toEqual([
      { seq: 7, excerpt: "Second line", instruction: "" },
    ]);
  });

  it("copies the exact highlighted text without creating an annotation", () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    const { container } = renderTranscript();
    const item = container.querySelector('[data-slot="event"]') as HTMLElement;
    const text = document.createTextNode("  Second line  ");
    item.appendChild(text);
    selectWithin(text);

    fireEvent.contextMenu(item, { clientX: 20, clientY: 30 });

    expect(screen.getByRole("button", { name: "Annotate selection" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Copy selection" }));
    expect(writeText).toHaveBeenCalledWith("  Second line  ");
    expect(useAnnotationStore.getState().bySource.a1).toBeUndefined();
  });

  it("falls back to the whole event when nothing is highlighted", () => {
    const { container } = renderTranscript();
    const item = container.querySelector('[data-slot="event"]') as HTMLElement;

    fireEvent.contextMenu(item, { clientX: 20, clientY: 30 });
    fireEvent.click(screen.getByRole("button", { name: "Annotate whole event" }));

    expect(useAnnotationStore.getState().bySource.a1).toEqual([
      { seq: 7, excerpt: "First line\nSecond line", instruction: "" },
    ]);
  });

  it("leaves the native menu alone where annotations are disabled", () => {
    const { container } = renderTranscript(false);
    const item = container.querySelector('[data-slot="event"]') as HTMLElement;

    const opened = fireEvent.contextMenu(item, { clientX: 20, clientY: 30 });

    expect(opened).toBe(true); // preventDefault was not called
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("collapses uninterrupted tool activity and omits successful no-payload outcomes", () => {
    renderTranscript(false, [
      { kind: "tool_call", seq: 1, name: "Read", args: { path: "a.ts" } },
      { kind: "tool_result", seq: 2, status: "completed", content: null },
      { kind: "tool_call", seq: 3, name: "Search" },
      { kind: "tool_result", seq: 4, status: "completed", content: "Found one match." },
    ]);

    const summary = screen.getByRole("button", { name: "Ran 2 tools" });
    expect(screen.queryByText(/Tool call:/)).toBeNull();
    expect(screen.queryByText("Completed")).toBeNull();

    fireEvent.click(summary);

    expect(screen.getAllByText(/Tool call:/)).toHaveLength(2);
    expect(screen.getByText("Found one match.")).toBeInTheDocument();
    expect(screen.queryByText("Completed")).toBeNull();
  });

  it("starts a new tool run after non-tool activity", () => {
    renderTranscript(false, [
      { kind: "tool_call", seq: 1, name: "Read" },
      { kind: "tool_result", seq: 2, status: "completed", content: null },
      { kind: "assistant_text", seq: 3, text: "I found the file." },
      { kind: "tool_call", seq: 4, name: "Edit" },
      { kind: "tool_result", seq: 5, status: "completed", content: null },
    ]);

    expect(screen.getAllByRole("button", { name: "Ran 1 tool" })).toHaveLength(2);
    expect(screen.getByText("I found the file.")).toBeInTheDocument();
  });

  it("shows a waiting indicator only while the agent is busy", () => {
    const { rerender } = renderTranscript(false, events, false);
    expect(screen.queryByText("Working…")).toBeNull();

    rerender(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: 0 } } })}>
        <TranscriptView agentId="a1" events={events} sourceActive annotationsEnabled={false} busy />
      </QueryClientProvider>,
    );

    expect(screen.getByText("Working…")).toBeInTheDocument();
  });

  it("keeps expanded tool calls individually annotatable", () => {
    const { container } = renderTranscript(true, [
      { kind: "tool_call", seq: 1, name: "Read" },
      { kind: "tool_result", seq: 2, status: "completed", content: null },
    ]);

    fireEvent.click(screen.getByRole("button", { name: "Ran 1 tool" }));
    fireEvent.contextMenu(container.querySelector('[data-seq="1"]') as HTMLElement, { clientX: 20, clientY: 30 });
    fireEvent.click(screen.getByRole("button", { name: "Annotate whole event" }));

    expect(useAnnotationStore.getState().bySource.a1).toMatchObject([{ seq: 1 }]);
  });
});

// FS-13.A14 (R23). Sending a batch to the current agent starts an ordinary
// prompt turn whose text is the machine block, so the transcript would otherwise
// show the same excerpts twice: once as the card, once as a wall of generated
// prose the person never wrote.
describe("TranscriptView self-annotation prompt", () => {
  const block = `${annotationBlockSentinel}\n\n1. Transcript event 7\nExcerpt:\n---\nFirst line\n---\nInstruction: tighten this\n`;
  const annotationEvent = {
    type: "annotation",
    seq: 8,
    data: { annotations: [{ seq: 7, excerpt: "First line", instruction: "tighten this" }], target: { kind: "self" } },
  };
  const promptEvent = { type: "user_text", seq: 9, data: { text: block } };

  function expectQuieted() {
    expect(screen.getByText("Annotations assigned")).toBeInTheDocument();
    expect(screen.getByText("tighten this")).toBeInTheDocument();
    expect(screen.queryByText(annotationBlockSentinel, { exact: false })).toBeNull();
  }

  it("draws the card alone when the prompt arrives live", () => {
    const append = useTranscriptStore.getState().appendMessage;
    append("a1", annotationEvent);
    append("a1", promptEvent);

    renderTranscript(true, useTranscriptStore.getState().byAgent.a1);

    expectQuieted();
  });

  // The same list on replay: a rule that lived on only one of the two paths
  // would draw an event that the next reload then removes (INV §2).
  it("draws the same list when the transcript is replayed", () => {
    renderTranscript(true, foldTranscript([annotationEvent, promptEvent]));

    expectQuieted();
    expect(foldTranscript([annotationEvent, promptEvent])).toHaveLength(1);
  });

  // The three conditions are load-bearing. A batch assigned to another agent
  // leaves the source free to keep talking, and that next message is ordinary
  // prose that must still appear.
  it("keeps a prompt that follows a batch assigned to another agent", () => {
    const assigned = { ...annotationEvent, data: { ...annotationEvent.data, target: { kind: "agent", agent_id: "a2" } } };
    renderTranscript(true, foldTranscript([assigned, { type: "user_text", seq: 9, data: { text: "carry on without me" } }]));

    expect(screen.getByText("carry on without me")).toBeInTheDocument();
  });

  it("keeps a prompt that does not immediately follow its annotation event", () => {
    const rendered = foldTranscript([
      annotationEvent,
      { type: "assistant_text", seq: 9, data: { text: "on it" } },
      { ...promptEvent, seq: 10 },
    ]);
    renderTranscript(true, rendered);

    expect(screen.getByText(annotationBlockSentinel, { exact: false })).toBeInTheDocument();
  });
});

// TS-08.R56 — the queued follow-up is a transcript-tail affordance, not a
// transcript event: it renders beside the list the fold builds, reads as
// not-yet-sent, and carries its own withdraw control.
describe("TranscriptView pending follow-up", () => {
  it("renders nothing when no message is held", () => {
    renderTranscript(true, events, true);

    expect(screen.queryByText(/^Queued/)).toBeNull();
  });

  it("shows a held message as queued and outside the folded event list", () => {
    useHeldStore.getState().hold("a1", "also update the docs");
    renderTranscript(true, events, true);

    expect(screen.getByText("Queued — sends when this turn ends")).toBeInTheDocument();
    expect(screen.getByText("also update the docs")).toBeInTheDocument();
    // It is beside the list, not an item in it — a live render and a reload
    // cannot disagree about something the fold never sees.
    expect(document.querySelector('[data-variant="held"]')?.closest(".transcript-item")).toBeNull();
    expect(useTranscriptStore.getState().byAgent.a1).toBeUndefined();
  });

  it("keeps the message visible when withdrawing it fails", async () => {
    useHeldStore.getState().hold("a1", "still queued");
    const failing = vi.spyOn(client, "withdrawPrompt").mockRejectedValue(new Error("gone"));
    renderTranscript(true, events, true);

    fireEvent.click(screen.getByRole("button", { name: "Withdraw" }));
    expect(await screen.findByText("Could not withdraw — it may already have been sent.")).toBeInTheDocument();
    expect(useHeldStore.getState().byAgent.a1).toBe("still queued");
    failing.mockRestore();
  });
});

// FS-03.A46, FS-13.A16 (J13): the file viewer's own selection menu is reached
// through TranscriptView's shared annotate-selection entry point, and the
// draft it hands the store must carry the file's exact anchor.
describe("TranscriptView file annotation integration (J13)", () => {
  function selectBetween(start: Element, end: Element) {
    const range = document.createRange();
    range.setStart(start, 0);
    range.setEnd(end, end.childNodes.length);
    const selection = window.getSelection();
    selection?.removeAllRanges();
    selection?.addRange(range);
  }

  it("captures the file's exact anchor when a selection is annotated through the menu", async () => {
    mswServer.use(
      http.get("/api/sessions/:id/file", () =>
        HttpResponse.json({
          agent_id: "a1",
          path: "internal/state/messages.go",
          size: 30,
          mod_time: "2026-09-10T10:00:00Z",
          line_count: 3,
          content: "package state\n\nfunc Read() {}\n",
          truncated: false,
          language: "go",
        }),
      ),
    );
    const onOpenFile = vi.fn();
    const client_ = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    render(
      <QueryClientProvider client={client_}>
        <TranscriptView
          agentId="a1"
          events={events}
          sourceActive
          annotationsEnabled
          openFile={{ path: "internal/state/messages.go" }}
          onOpenFile={onOpenFile}
        />
      </QueryClientProvider>,
    );

    await screen.findByText("internal/state/messages.go");
    const first = await waitFor(() => document.querySelector('[data-file-line="1"]') as HTMLElement);
    const second = document.querySelector('[data-file-line="2"]') as HTMLElement;
    selectBetween(first, second);
    fireEvent.contextMenu(document.querySelector(".file-viewer-body") as HTMLElement, { clientX: 5, clientY: 6 });
    fireEvent.click(await screen.findByRole("button", { name: "Annotate selection" }));

    expect(useAnnotationStore.getState().bySource.a1).toEqual([
      expect.objectContaining({ anchor_kind: "file", path: "internal/state/messages.go", start_line: 1, end_line: 2, instruction: "" }),
    ]);
  });
});

// FS-13.A16, TS-02.R38, TS-03.R49, TS-08.R80 (J13): the tray holds a
// transcript-anchored and a file-anchored draft together, survives being
// mounted fresh (what a reload restores from the persisted store), and can
// still be edited, have one draft removed, and send the exact remaining
// batch as the wire payload.
describe("TranscriptView mixed annotation tray (J13)", () => {
  function seedMixedTray() {
    useAnnotationStore.setState({
      bySource: {
        a1: [
          { seq: 7, excerpt: "transcript excerpt", instruction: "initial instruction" },
          { anchor_kind: "file", path: "src/a.go", start_line: 2, end_line: 4, excerpt: "file excerpt", instruction: "" },
        ],
      },
      overallBySource: {},
      editedAt: { a1: Date.now() },
      collapsedBySource: {},
    });
  }

  // A page reload: keep only what persist wrote to storage, then rehydrate.
  async function reloadTrays() {
    const key = useAnnotationStore.persist.getOptions().name as string;
    const saved = localStorage.getItem(key) as string;
    useAnnotationStore.setState({ bySource: {}, overallBySource: {}, editedAt: {}, collapsedBySource: {} });
    localStorage.setItem(key, saved);
    await useAnnotationStore.persist.rehydrate();
  }

  function renderMixed() {
    const client_ = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    return render(
      <QueryClientProvider client={client_}>
        <TranscriptView agentId="a1" events={events} sourceActive annotationsEnabled />
      </QueryClientProvider>,
    );
  }

  it("preserves both drafts across a fresh mount, then edits, removes, and sends the exact remaining batch", async () => {
    seedMixedTray();
    await reloadTrays();
    renderMixed();

    // What a reload restores: both anchors are still present and readable.
    expect(screen.getByText("transcript excerpt")).toBeInTheDocument();
    expect(screen.getByText("file excerpt")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "File src/a.go:2–4" })).toBeInTheDocument();

    // Edit the file draft's instruction.
    const fileRow = screen.getByRole("heading", { name: "File src/a.go:2–4" }).closest(".annotation-draft") as HTMLElement;
    fireEvent.change(within(fileRow).getByLabelText("Instruction"), { target: { value: "tighten this excerpt" } });
    expect(useAnnotationStore.getState().bySource.a1[1].instruction).toBe("tighten this excerpt");

    // Remove the transcript draft, leaving only the edited file draft.
    const transcriptRow = screen.getByText("transcript excerpt").closest(".annotation-draft") as HTMLElement;
    fireEvent.click(within(transcriptRow).getByRole("button", { name: "Remove" }));
    expect(useAnnotationStore.getState().bySource.a1).toHaveLength(1);

    let sentBody: unknown = null;
    mswServer.use(
      http.post("/api/sessions/a1/annotations", async ({ request }) => {
        sentBody = await request.json();
        return HttpResponse.json({ accepted: true, seq: 99 }, { status: 202 });
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Send annotations" }));

    await waitFor(() => expect(useAnnotationStore.getState().bySource.a1).toBeUndefined());
    expect(sentBody).toEqual({
      annotations: [
        { anchor_kind: "file", path: "src/a.go", start_line: 2, end_line: 4, excerpt: "file excerpt", instruction: "tighten this excerpt" },
      ],
      target: { kind: "self" },
    });
  });

  // A file annotation persisted with an anchor the server would reject (a
  // start_line with no end_line) must stay in the tray rather than being
  // silently dropped on the next mount — only Remove or Send should end it.
  it("preserves a persisted file annotation with an invalid anchor rather than dropping it", async () => {
    useAnnotationStore.setState({
      bySource: {
        a1: [
          { anchor_kind: "file", path: "src/broken.go", start_line: 5, excerpt: "broken anchor", instruction: "note" },
        ],
      },
      overallBySource: {},
      editedAt: { a1: Date.now() },
      collapsedBySource: {},
    });
    await reloadTrays();
    renderMixed();

    expect(screen.getByText("broken anchor")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "File src/broken.go:5" })).toBeInTheDocument();
    expect(useAnnotationStore.getState().bySource.a1).toHaveLength(1);
  });
});
