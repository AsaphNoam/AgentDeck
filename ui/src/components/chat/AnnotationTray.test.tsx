import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { useAgentStore } from "../../store/agentStore";
import { useAnnotationStore } from "../../store/annotationStore";
import { AnnotationTray } from "./AnnotationTray";

afterEach(() => {
  cleanup();
  useAgentStore.setState({ agents: {} });
  useAnnotationStore.setState({ bySource: {}, overallBySource: {}, editedAt: {} });
});

describe("AnnotationTray", () => {
  it("keeps delivery controls outside the scrolling multi-draft body", () => {
    useAgentStore.setState({
      agents: {
        source: {
          agent_id: "source", name: "Atlas", role: "implementer", project: "app", backend: "claude", model: "sonnet",
          interface: "chat", created_at: "2026-07-26T00:00:00Z", running: true, state: "idle", detail: "", context_pct: 0, updated_at: 1,
        },
      },
    });
    useAnnotationStore.setState({
      bySource: {
        source: [1, 2, 3].map((seq) => ({ seq, excerpt: `Excerpt ${seq}`, instruction: `Instruction ${seq}` })),
      },
      overallBySource: {}, editedAt: { source: Date.now() },
    });
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    const { container } = render(
      <QueryClientProvider client={client}><AnnotationTray sourceId="source" sourceActive /></QueryClientProvider>,
    );

    const body = container.querySelector(".annotation-tray-body");
    const footer = container.querySelector(".annotation-tray-footer");
    expect(body).not.toBeNull();
    expect(footer).not.toBeNull();
    expect(within(body as HTMLElement).getAllByText(/Excerpt/)).toHaveLength(3);
    expect(within(footer as HTMLElement).getByRole("button", { name: "Send annotations" })).toBeInTheDocument();
    expect(screen.getByText("3/20")).toBeInTheDocument();
  });
});
