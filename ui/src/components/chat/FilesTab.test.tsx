import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { FilesTab } from "./FilesTab";
import type { TrackedFile } from "../../api/types";

const mockFiles: TrackedFile[] = [
  {
    path: "src/auth.ts",
    edit_count: 3,
    first_seq: 1,
    last_seq: 10,
    first_ts: "2026-06-28T10:00:01Z",
    last_ts: "2026-06-28T10:00:10Z",
    has_diff: true,
    diff_refs: [{ seq: 5, tool_call_id: "tc_5" }],
  },
  {
    path: "src/db.ts",
    edit_count: 1,
    first_seq: 3,
    last_seq: 3,
    first_ts: "2026-06-28T10:00:03Z",
    last_ts: "2026-06-28T10:00:03Z",
    has_diff: false,
    diff_refs: [],
  },
];

const server = setupServer(
  http.get("/api/sessions/:id/files", () =>
    HttpResponse.json({ agent_id: "a_1", files: mockFiles }),
  ),
);

beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterEach(() => { cleanup(); server.resetHandlers(); });
afterAll(() => server.close());

describe("FilesTab", () => {
  it("renders file rows with path and edit count", async () => {
    render(<FilesTab agentId="a_1" />);
    expect(await screen.findByText("src/auth.ts")).toBeInTheDocument();
    expect(screen.getByText("src/db.ts")).toBeInTheDocument();
    expect(screen.getByText("3 edits")).toBeInTheDocument();
    expect(screen.getByText("1 edit")).toBeInTheDocument();
  });

  it("shows has-diff indicator and Diff button for files with diffs", async () => {
    render(<FilesTab agentId="a_1" />);
    await screen.findByText("src/auth.ts");
    expect(screen.getByText("has diff")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Diff/i })).toBeInTheDocument();
  });

  it("filter narrows the list", async () => {
    render(<FilesTab agentId="a_1" />);
    await screen.findByText("src/auth.ts");
    const input = screen.getByRole("searchbox");
    fireEvent.change(input, { target: { value: "auth" } });
    await waitFor(() => expect(screen.queryByText("src/db.ts")).not.toBeInTheDocument());
    expect(screen.getByText("src/auth.ts")).toBeInTheDocument();
  });

  it("shows count label", async () => {
    render(<FilesTab agentId="a_1" />);
    await screen.findByText("2 files");
  });

  it("copy button is present for each file", async () => {
    render(<FilesTab agentId="a_1" />);
    await screen.findByText("src/auth.ts");
    const copyBtns = screen.getAllByRole("button", { name: /Copy/i });
    expect(copyBtns.length).toBeGreaterThanOrEqual(2);
  });

  it("shows empty state when no files", async () => {
    server.use(
      http.get("/api/sessions/:id/files", () =>
        HttpResponse.json({ agent_id: "a_1", files: [] }),
      ),
    );
    render(<FilesTab agentId="a_1" />);
    await screen.findByText(/No files tracked yet/);
  });
});
