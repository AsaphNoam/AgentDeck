import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
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
	archived: false,
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
	archived: true,
};

const server = setupServer(
  http.get("/api/archive", ({ request }) => {
    const url = new URL(request.url);
    const q = url.searchParams.get("q") ?? "";
    if (q === "noresult") {
      return HttpResponse.json({ query: q, search_mode: "full_text", total: 0, limit: 50, offset: 0, results: [] });
    }
    if (q === "atlas") {
      return HttpResponse.json({
        query: q, search_mode: "full_text", total: 1, limit: 50, offset: 0,
        results: [{ ...mockActive, matched_in: ["metadata"], snippet: "" }],
      });
    }
    if (q === "snippet test") {
      return HttpResponse.json({
        query: q, search_mode: "full_text", total: 1, limit: 50, offset: 0,
        results: [{ ...mockActive, matched_in: ["transcript"], snippet: "a snippet here" }],
      });
    }
    return HttpResponse.json({ query: "", search_mode: "full_text", total: 2, limit: 50, offset: 0, results: [mockActive, mockInactive] });
  }),
  http.get("/api/archive/projects/:project", ({ params, request }) => {
    const query = new URL(request.url).searchParams.get("q");
    if (query === "atlas") return HttpResponse.json({ search_mode: "full_text", total: 1, limit: 50, offset: 0, results: [{ ...mockActive, matched_in: ["metadata"] }] });
    if (query === "snippet test") return HttpResponse.json({ search_mode: "full_text", total: 1, limit: 50, offset: 0, results: [{ ...mockActive, matched_in: ["transcript"], snippet: "a snippet here" }] });
    const result = params.project === "my-app" ? mockActive : mockInactive;
    return HttpResponse.json({ search_mode: "full_text", total: 1, limit: 50, offset: 0, results: [result] });
  }),
);

beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterEach(() => { cleanup(); server.resetHandlers(); });
afterAll(() => server.close());

function renderArchive() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/archive"]}>
        <ArchivePage />
      </MemoryRouter>
    </QueryClientProvider>,
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

  // FS-05.R36 / INV §8/§10: a group whose independent page fails keeps a
  // visible recovery path instead of presenting an empty archive as success.
  it("shows a retry control when a project archive page fails", async () => {
    let attempts = 0;
    const group = {
      project: "my-app", title: "My app", color: [1, 2, 3], project_status: "active",
      archived_agent_count: 1, results: [],
    };
    server.use(
      http.get("/api/archive", () => HttpResponse.json({
        query: "", search_mode: "full_text", total: 1, limit: 50, offset: 0, results: [group],
      })),
      http.get("/api/archive/projects/my-app", () => {
        attempts += 1;
        if (attempts === 1) return HttpResponse.error();
        return HttpResponse.json({ search_mode: "full_text", total: 1, limit: 50, offset: 0, results: [mockInactive] });
      }),
    );

    renderArchive();
    expect(await screen.findByText("Could not load this project's archived agents.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByText("Morpheus")).toBeInTheDocument();
    expect(screen.queryByText("Could not load this project's archived agents.")).not.toBeInTheDocument();
    expect(attempts).toBe(2);
  });

  // FS-05.R36 / INV §1: a delayed project page from the previous search must
  // not overwrite the rows loaded for the current search.
  it("keeps project rows from the newest search request", async () => {
    let markOldStarted!: () => void;
    const oldStarted = new Promise<void>((resolve) => { markOldStarted = resolve; });
    let releaseOld!: () => void;
    const oldRelease = new Promise<void>((resolve) => { releaseOld = resolve; });
    let markOldFinished!: () => void;
    const oldFinished = new Promise<void>((resolve) => { markOldFinished = resolve; });
    const group = {
      project: "my-app", title: "My app", color: [1, 2, 3], project_status: "active",
      archived_agent_count: 0, results: [],
    };
    server.use(
      http.get("/api/archive", ({ request }) => {
        const query = new URL(request.url).searchParams.get("q") ?? "";
        return HttpResponse.json({ query, search_mode: "full_text", total: 1, limit: 50, offset: 0, results: [group] });
      }),
      http.get("/api/archive/projects/my-app", async ({ request }) => {
        const query = new URL(request.url).searchParams.get("q") ?? "";
        if (query === "old") {
          markOldStarted();
          await oldRelease;
          markOldFinished();
          return HttpResponse.json({
            search_mode: "full_text", total: 1, limit: 50, offset: 0,
            results: [{ ...mockActive, agent_id: "a_old", name: "Old result" }],
          });
        }
        return HttpResponse.json({
          search_mode: "full_text", total: 1, limit: 50, offset: 0,
          results: [{ ...mockActive, agent_id: "a_new", name: query === "new" ? "New result" : "Initial result" }],
        });
      }),
    );

    renderArchive();
    const input = screen.getByRole("searchbox");
    fireEvent.change(input, { target: { value: "old" } });
    await oldStarted;
    fireEvent.change(input, { target: { value: "new" } });

    expect(await screen.findByText("New result")).toBeInTheDocument();
    releaseOld();
    await oldFinished;
    await waitFor(() => expect(screen.queryByText("Old result")).not.toBeInTheDocument());
    expect(screen.getByText("New result")).toBeInTheDocument();
  });

  it("shows no-results message when query has no matches", async () => {
    renderArchive();
    const input = screen.getByRole("searchbox");
    fireEvent.change(input, { target: { value: "noresult" } });
    await screen.findByText(/No results for/);
    expect(screen.getByText(/does not combine text across turns/)).toBeInTheDocument();
    expect(input).toHaveAttribute("aria-describedby", "archive-search-scope");
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

  it("loads the next result page using the rendered count as offset", async () => {
    const offsets: string[] = [];
    const pages = [
      { project: "page-one", title: "Project one", color: [1, 2, 3] as [number, number, number], project_status: "active" as const, archived_agent_count: 1, results: [{ ...mockActive, agent_id: "a_page_1", name: "Page one" }] },
      { project: "page-two", title: "Project two", color: [1, 2, 3] as [number, number, number], project_status: "active" as const, archived_agent_count: 1, results: [{ ...mockInactive, agent_id: "a_page_2", name: "Page two" }] },
    ];
    server.use(
      http.get("/api/archive", ({ request }) => {
        const offset = new URL(request.url).searchParams.get("offset") ?? "0";
        offsets.push(offset);
        return HttpResponse.json({ query: "", search_mode: "full_text", total: 2, limit: 1, offset: Number(offset), results: [pages[Number(offset)]] });
      }),
      http.get("/api/archive/projects/:project", ({ params }) => {
        const page = pages.find((entry) => entry.project === params.project);
        return HttpResponse.json({ search_mode: "full_text", total: 1, limit: 50, offset: 0, results: page?.results ?? [] });
      }),
    );

    renderArchive();
    expect(await screen.findByText("Page one")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Load more" }));

    expect(await screen.findByText("Page two")).toBeInTheDocument();
    expect(screen.getByText("Page one")).toBeInTheDocument();
    expect(offsets).toEqual(["0", "1"]);
    expect(screen.queryByRole("button", { name: "Load more" })).not.toBeInTheDocument();
  });

  // FS-05.A19: project groups and their agent rows page independently. A
  // mutable updated_at can repeat a boundary row, so the per-project pager
  // refetches its head and merges it before deciding the corpus is complete.
  it("reaches every agent when a per-project page is reordered", async () => {
    const sessions = Array.from({ length: 51 }, (_, index) => ({
      ...mockInactive,
      agent_id: `a_seq_${index + 1}`,
      name: `Session ${index + 1}`,
      updated_at: new Date(Date.UTC(2026, 5, 1, 0, 51 - index)).toISOString(),
    }));
    let order = [...sessions];
    let requests = 0;
    server.use(http.get("/api/archive", () => HttpResponse.json({
      query: "", search_mode: "full_text", total: 1, limit: 50, offset: 0,
      results: [{ project: "auth-lib", title: "Auth lib", color: [1, 2, 3], project_status: "active", archived_agent_count: 0, results: sessions.slice(0, 50) }],
    })), http.get("/api/archive/projects/auth-lib", ({ request }) => {
      const url = new URL(request.url);
      const offset = Number(url.searchParams.get("offset") ?? "0");
      const limit = Number(url.searchParams.get("limit") ?? "50");
      const page = order.slice(offset, offset + limit);
      requests += 1;
      // After the first page is rendered, the last session is resumed: it moves
      // to the front and shifts every rendered row one position later.
      if (requests === 1) order = [order[50], ...order.slice(0, 50)];
      return HttpResponse.json({ search_mode: "full_text", total: order.length, limit, offset, results: page });
    }));

    renderArchive();
    await screen.findByText("Session 1");
    expect(screen.queryByText("Session 51")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Load more agents" }));

    await waitFor(() => expect(screen.getByText("Session 51")).toBeInTheDocument());
    await waitFor(() => expect(screen.queryByRole("button", { name: "Load more agents" })).not.toBeInTheDocument());
    const rendered = Array.from(document.querySelectorAll(".archive-name")).map((node) => node.textContent);
    expect(new Set(rendered).size).toBe(rendered.length);
    expect(rendered).toHaveLength(51);
    for (const session of sessions) expect(rendered).toContain(session.name);
  });

  it("does not advertise transcript search in metadata-only mode", async () => {
    server.use(http.get("/api/archive", () => HttpResponse.json({
      query: "", search_mode: "metadata", total: 0, limit: 50, offset: 0, results: [],
    })));

    renderArchive();
    const input = await screen.findByPlaceholderText("Search agents, roles, projects, backends…");
    expect(input).not.toHaveAttribute("placeholder", expect.stringContaining("transcript"));
    expect(screen.getByText(/searches session metadata only/)).toBeInTheDocument();
  });
});
