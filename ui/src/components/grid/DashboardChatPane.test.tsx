import React from "react";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { DashboardChatPane } from "./DashboardChatPane";
import type { FileLink } from "../chat/renderers/filePath";

const mocks = vi.hoisted(() => ({ teardown: vi.fn(), register: vi.fn() }));
mocks.register.mockImplementation(() => mocks.teardown);

vi.mock("../../api/sse", () => ({ sseClient: { registerOpenAgent: mocks.register } }));
vi.mock("../chat/TranscriptView", () => ({
  TranscriptView: ({ openFile, onOpenFile }: { openFile?: FileLink | null; onOpenFile?: (link: FileLink | null) => void }) => (
    <div>
      Transcript surface
      <span data-testid="pane-open-file">{openFile ? openFile.path : "none"}</span>
      <button type="button" onClick={() => onOpenFile?.({ path: "internal/state/messages.go", line: 12 })}>Pane file link</button>
    </div>
  ),
}));
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
    // A file link in the pane routes to the agent screen (FS-03.R53), so the
    // pane is a router-scoped component like every other dashboard surface.
    const view = render(<MemoryRouter><DashboardChatPane agent={{
      agent_id: "a_1", name: "Atlas", role: "implementer", project: "my-app",
      backend: "claude", model: "sonnet", interface: "chat", state: "idle",
      detail: "", running: true, context_pct: 0,
    }} /></MemoryRouter>);

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

// FS-03.A36 (R53) — the pane is one grid column wide and deliberately carries
// less than the agent screen, so no viewer opens here: a file link takes the
// pane's existing route to the full surface, with the file already open.
describe("a file link inside the expanded dashboard chat pane", () => {
  function LocationProbe() {
    const location = useLocation();
    return <span data-testid="location">{location.pathname + location.search}</span>;
  }

  it("navigates to the agent screen with that file open instead of opening a panel", () => {
    render(
      <MemoryRouter initialEntries={["/"]}>
        <Routes>
          <Route path="/" element={<DashboardChatPane agent={{
            agent_id: "a_1", name: "Atlas", role: "implementer", project: "my-app",
            backend: "claude", model: "sonnet", interface: "chat", state: "idle",
            detail: "", running: true, context_pct: 0,
          } as never} />} />
          <Route path="/agent/:id" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.getByTestId("pane-open-file")).toHaveTextContent("none");
    fireEvent.click(screen.getByRole("button", { name: "Pane file link" }));

    expect(screen.getByTestId("location")).toHaveTextContent(
      "/agent/a_1?file=internal%2Fstate%2Fmessages.go&fileLine=12",
    );
  });
});
