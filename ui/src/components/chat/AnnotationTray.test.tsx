import React from "react";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { useAgentStore } from "../../store/agentStore";
import { useAnnotationStore } from "../../store/annotationStore";
import { AnnotationTray } from "./AnnotationTray";

afterEach(() => {
  cleanup();
  useAgentStore.setState({ agents: {} });
  useAnnotationStore.setState({ bySource: {}, overallBySource: {}, editedAt: {}, collapsedBySource: {} });
});

const sourceAgent = {
  agent_id: "source", name: "Atlas", role: "implementer", project: "app", backend: "claude", model: "sonnet",
  interface: "chat", created_at: "2026-07-26T00:00:00Z", running: true, state: "idle", detail: "", context_pct: 0, updated_at: 1,
};

function seedTray(count: number) {
  useAgentStore.setState({ agents: { source: sourceAgent } });
  useAnnotationStore.setState({
    bySource: { source: Array.from({ length: count }, (_, i) => ({ seq: i + 1, excerpt: `Excerpt ${i + 1}`, instruction: `Instruction ${i + 1}` })) },
    overallBySource: {}, editedAt: { source: Date.now() }, collapsedBySource: {},
  });
}

function renderTray() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
  return render(<QueryClientProvider client={client}><AnnotationTray sourceId="source" sourceActive /></QueryClientProvider>);
}

function agentStylesheet() {
  return readFileSync(join(process.cwd(), "src/styles/features/agent.css"), "utf8");
}

describe("AnnotationTray", () => {
  it("keeps delivery controls outside the scrolling multi-draft body", () => {
    seedTray(3);
    const { container } = renderTray();

    const body = container.querySelector(".annotation-tray-body");
    const footer = container.querySelector(".annotation-tray-footer");
    expect(body).not.toBeNull();
    expect(footer).not.toBeNull();
    expect(within(body as HTMLElement).getAllByText(/Excerpt/)).toHaveLength(3);
    expect(within(footer as HTMLElement).getByRole("button", { name: "Send annotations" })).toBeInTheDocument();
    expect(screen.getByText("3/20")).toBeInTheDocument();
  });

  // FS-13.A12. jsdom evaluates no CSS and knows nothing of container queries, so
  // the stylesheet is the only witness that a wide transcript region docks the
  // tray and a narrow one keeps the shipped overlay (INV §13). What matters is
  // that the threshold is asked of the transcript region — a viewport query would
  // dock the dashboard's narrow chat pane inside a wide window.
  it("docks through the transcript region's own container query and falls back to the overlay", () => {
    const css = agentStylesheet();
    expect(css).toMatch(/\.transcript-wrap \{[^}]*container: transcript \/ inline-size/);
    expect(css).toMatch(/\.transcript-wrap \{[^}]*grid-template-columns: minmax\(0, 1fr\) auto/);
    // The fallback is the shipped presentation itself, not a second one.
    expect(css).toMatch(/\.annotation-tray \{[^}]*position: absolute/);
    const docked = css.match(/@container transcript \(min-width: 860px\) \{([\s\S]*?)\n\}/);
    expect(docked).not.toBeNull();
    expect(docked![1]).toMatch(/\.annotation-tray \{[^}]*position: static/);
    expect(docked![1]).toMatch(/\.annotation-tray \{[^}]*width: min\(30cqi, 460px\)/);
    // Auto-placement is what puts the transcript in the first column and the
    // docked tray in the second, so every other child of the region has to stay
    // out of flow. Giving the jump button a grid area of its own displaced the
    // transcript into the tray's column — invisible to jsdom, obvious in Chrome.
    expect(css).toMatch(/\.jump-to-latest \{[^}]*position: absolute/);
    expect(css).not.toMatch(/\.jump-to-latest \{[^}]*grid-area/);
    // The collapse control belongs to the docked form only (FS-13.R21).
    expect(css).toMatch(/\.annotation-tray-collapse \{[^}]*display: none/);
    expect(docked![1]).toMatch(/\.annotation-tray-collapse \{[^}]*display: block/);
    // The collapsed strip keeps the count and the control, and drops the rest.
    expect(docked![1]).toMatch(/\.annotation-tray\[data-state="collapsed"\] \.annotation-tray-body/);
    expect(docked![1]).toMatch(/\.annotation-tray\[data-state="collapsed"\] \.annotation-tray-title/);
  });

  // FS-13.A12: the tray is one component in two CSS states, so collapsing is a
  // state on the same markup rather than a second tray. The drafts stay mounted,
  // which is what lets a resize move between the forms without touching them.
  it("collapses and expands the docked tray without disturbing the drafts", () => {
    seedTray(3);
    const { container } = renderTray();

    const tray = container.querySelector(".annotation-tray") as HTMLElement;
    expect(tray).toHaveAttribute("data-state", "expanded");
    fireEvent.click(screen.getByRole("button", { name: "Collapse pending annotations" }));

    expect(container.querySelector(".annotation-tray")).toHaveAttribute("data-state", "collapsed");
    expect(useAnnotationStore.getState().collapsedBySource.source).toBe(true);
    // The strip still names how many drafts are waiting behind it.
    expect(screen.getByText("3/20")).toBeInTheDocument();
    expect(useAnnotationStore.getState().bySource.source).toHaveLength(3);

    fireEvent.click(screen.getByRole("button", { name: "Expand pending annotations" }));
    expect(container.querySelector(".annotation-tray")).toHaveAttribute("data-state", "expanded");
    expect(useAnnotationStore.getState().collapsedBySource.source).toBe(false);
  });

  // FS-13.A13 — the anchor is the row's heading, not bold text sharing a line
  // with the control that deletes the draft.
  it("gives each docked draft an anchor heading separate from its controls", () => {
    seedTray(1);
    renderTray();

    const heading = screen.getByRole("heading", { name: "Event 1" });
    expect(heading).toBeInTheDocument();
    expect(within(heading).queryByRole("button")).toBeNull();
    const row = heading.closest(".annotation-draft") as HTMLElement;
    expect(within(row).getByRole("button", { name: "Remove" })).toBeInTheDocument();
    expect(within(row).getByText("Excerpt 1")).toBeInTheDocument();
    expect(within(row).getByLabelText("Instruction")).toHaveValue("Instruction 1");
  });
});
