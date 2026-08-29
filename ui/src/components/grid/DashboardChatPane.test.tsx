import React from "react";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { DashboardChatPane } from "./DashboardChatPane";

const mocks = vi.hoisted(() => ({ teardown: vi.fn(), register: vi.fn() }));
mocks.register.mockImplementation(() => mocks.teardown);

vi.mock("../../api/sse", () => ({ sseClient: { registerOpenAgent: mocks.register } }));
vi.mock("../chat/TranscriptView", () => ({ TranscriptView: () => <div>Transcript surface</div> }));
vi.mock("../chat/Composer", () => ({ Composer: () => <textarea aria-label="Pane composer" /> }));

afterEach(() => {
  cleanup();
  mocks.register.mockClear();
  mocks.teardown.mockClear();
});

describe("DashboardChatPane", () => {
  // FS-03.A23 — the pane composes the shared transcript and composer and exposes
  // no Files, Commands, Terminal, or runtime picker. A23's multi-open delivery and
  // stale-response clauses are pinned in sse.test.ts and transcriptStore.test.ts.
  it("composes only the shared transcript and composer and releases registration", () => {
    const view = render(<DashboardChatPane agent={{
      agent_id: "a_1", name: "Atlas", role: "implementer", project: "my-app",
      backend: "claude", model: "sonnet", interface: "chat", state: "idle",
      detail: "", running: true, context_pct: 0,
    }} />);

    expect(screen.getByText("Transcript surface")).toBeInTheDocument();
    expect(screen.getByLabelText("Pane composer")).toBeInTheDocument();
    expect(screen.queryByText("Files")).not.toBeInTheDocument();
    expect(screen.queryByText("Commands")).not.toBeInTheDocument();
    expect(screen.queryByText("Terminal")).not.toBeInTheDocument();
    expect(mocks.register).toHaveBeenCalledWith("a_1");
    view.unmount();
    expect(mocks.teardown).toHaveBeenCalledOnce();
  });
});
