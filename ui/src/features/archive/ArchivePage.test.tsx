import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { ArchivePage } from "./ArchivePage";
import type { ArchiveResult } from "../../api/types";

const mockActive: ArchiveResult = {
  agent_id: "a_1",
  name: "Atlas",
  role: "implementer",
  project: "my-app",
  backend: "claude",
  model: "sonnet-4-6",
  interface: "chat",
  created_at: "2026-06-28T10:00:00Z",
  updated_at: "2026-06-28T11:30:00Z",
  turn_count: 5,
  files_touched: 3,
  commands_run: 2,
  active: true,
};

const mockInactive: ArchiveResult = {
  agent_id: "a_2",
  name: "Morpheus",
  role: "reviewer",
  project: "auth-lib",
  backend: "claude",
  model: "opus-4-8",
  interface: "chat",
  created_at: "2026-06-27T09:00:00Z",
  updated_at: "2026-06-27T10:15:00Z",
  turn_count: 2,
  files_touched: 0,
  commands_run: 0,
  active: false,
};

const server = setupServer(
  http.get("/api/archive", ({ request }) => {
    const url = new URL(request.url);
    const q = url.searchParams.get("q") ?? "";
    if (q === "noresult") {
      return HttpResponse.json({ query: q, total: 0, limit: 50, offset: 0, results: [] });
    }
    if (q === "atlas") {
      return HttpResponse.json({
        query: q, total: 1, limit: 50, offset: 0,
        results: [{ ...mockActive, matched_in: ["metadata"], snippet: "" }],
      });
    }
    if (q === "snippet test") {
      return HttpResponse.json({
        query: q, total: 1, limit: 50, offset: 0,
        results: [{ ...mockActive, matched_in: ["transcript"], snippet: "a snippet here" }],
      });
    }
    return HttpResponse.json({ query: "", total: 2, limit: 50, offset: 0, results: [mockActive, mockInactive] });
  }),
);

beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterEach(() => { cleanup(); server.resetHandlers(); });
afterAll(() => server.close());

function renderArchive() {
  return render(
    <MemoryRouter initialEntries={["/archive"]}>
      <ArchivePage />
    </MemoryRouter>,
  );
}

describe("ArchivePage", () => {
  it("lists active and inactive sessions on load", async () => {
    renderArchive();
    expect(await screen.findByText("Atlas")).toBeInTheDocument();
    expect(await screen.findByText("Morpheus")).toBeInTheDocument();
    expect(screen.getByText("active")).toBeInTheDocument();
    expect(screen.getByText("inactive")).toBeInTheDocument();
  });

  it("shows role · project and backend · model for each result", async () => {
    renderArchive();
    await screen.findByText("Atlas");
    expect(screen.getByText("implementer · my-app")).toBeInTheDocument();
    expect(screen.getByText("claude · sonnet-4-6")).toBeInTheDocument();
  });

  it("filters results when search changes", async () => {
    renderArchive();
    await screen.findByText("Atlas");
    const input = screen.getByRole("searchbox");
    fireEvent.change(input, { target: { value: "atlas" } });
    await waitFor(() => expect(screen.queryByText("Morpheus")).not.toBeInTheDocument());
    expect(screen.getByText("Atlas")).toBeInTheDocument();
  });

  it("shows no-results message when query has no matches", async () => {
    renderArchive();
    const input = screen.getByRole("searchbox");
    fireEvent.change(input, { target: { value: "noresult" } });
    await screen.findByText(/No results for/);
  });

  it("renders snippet when present in search result", async () => {
    renderArchive();
    const input = screen.getByRole("searchbox");
    fireEvent.change(input, { target: { value: "snippet test" } });
    await screen.findByText(/a snippet here/);
  });

  it("shows matched_in metadata tag", async () => {
    renderArchive();
    const input = screen.getByRole("searchbox");
    fireEvent.change(input, { target: { value: "atlas" } });
    await screen.findByText("metadata");
  });

  it("shows result count", async () => {
    renderArchive();
    await screen.findByText("2 results");
  });
});
