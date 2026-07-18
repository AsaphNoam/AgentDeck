import React from "react";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { useTranscriptStore } from "../../store/transcriptStore";
import { ArchiveAgentPage } from "./ArchiveAgentPage";

const server = setupServer(
  http.get("/api/sessions/a_archive/transcript", () =>
    HttpResponse.json({
      agent_id: "a_archive",
      events: [
        { seq: 1, type: "assistant_text", ts: "t1", data: { delta: "Sure, " } },
        { seq: 2, type: "assistant_text", ts: "t2", data: { delta: "I'll " } },
        { seq: 3, type: "assistant_text", ts: "t3", data: { delta: "do that." } },
      ],
    }),
  ),
);

beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
beforeEach(() => useTranscriptStore.setState({ byAgent: {}, pending: {} }));
afterEach(() => {
  cleanup();
  server.resetHandlers();
});
afterAll(() => server.close());

describe("ArchiveAgentPage", () => {
  it("renders a stored assistant stream as one message", async () => {
    render(
      <MemoryRouter initialEntries={["/archive/a_archive"]}>
        <Routes>
          <Route path="/archive/:id" element={<ArchiveAgentPage />} />
        </Routes>
      </MemoryRouter>,
    );

    expect(await screen.findByText("Sure, I'll do that.")).toBeInTheDocument();
    expect(document.querySelectorAll("article.assistant-message")).toHaveLength(1);
  });
});
