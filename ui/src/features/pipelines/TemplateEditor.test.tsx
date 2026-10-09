import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import type { PipelineTemplate } from "../../schemas/pipeline";
import { TemplateEditor } from "./TemplateEditor";

const stages = Array.from({ length: 32 }, (_, index) => ({
  id: `stage-${index + 1}`, title: `Stage ${index + 1}`, objective: `Instruction ${index + 1}`,
  coordination: "standing" as const, inputs: [], outputs: [],
}));
const template: PipelineTemplate = { version: 2, title: "Maximum delivery", orchestrator_role: "implementer", inputs: [], stages };
const server = setupServer(
  http.get("/api/pipelines", () => HttpResponse.json([{ id: "maximum", template, valid: true, diagnostics: [] }])),
  http.get("/api/roles", () => HttpResponse.json({ implementer: { title: "Implementer", system_prompt: "", skip_permissions: false } })),
);

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => { cleanup(); server.resetHandlers(); });
afterAll(() => server.close());

describe("TemplateEditor focused workspace", () => {
  // FS-14.A20: a maximum-shape template mounts one selected stage form rather
  // than 32 full forms, while local edits survive stage navigation.
  it("edits one selected stage at a time and preserves its unsaved draft", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    const { container } = render(<QueryClientProvider client={client}><MemoryRouter><TemplateEditor seed={{ id: "maximum", template }} /></MemoryRouter></QueryClientProvider>);
    await screen.findByText("Stage 32");
    expect(container.querySelectorAll(".pipeline-stage-card")).toHaveLength(1);
    const title = container.querySelector<HTMLInputElement>(".pipeline-stage-card input")!;
    fireEvent.change(title, { target: { value: "work" } });
    fireEvent.click(screen.getByRole("button", { name: /Stage 32/ }));
    expect(container.querySelectorAll(".pipeline-stage-card")).toHaveLength(1);
    expect(screen.getByDisplayValue("Instruction 32")).toBeInTheDocument();
    const firstStage = [...container.querySelectorAll<HTMLButtonElement>(".pipeline-stage-nav-item")].find((button) => button.querySelector("strong")?.textContent === "Stage 1")!;
    fireEvent.click(firstStage);
    expect(container.querySelector<HTMLInputElement>(".pipeline-stage-card input")).toHaveValue("work");
  });

  // FS-14.R81: switching one stage to Think Tank seeds a two-participant room
  // and judge, and switching back drops only that stage's room config.
  it("switches a stage to Think Tank and back without touching other stages", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    render(<QueryClientProvider client={client}><MemoryRouter><TemplateEditor seed={{ id: "maximum", template }} /></MemoryRouter></QueryClientProvider>);
    await screen.findByText("Stage 32");
    fireEvent.click(screen.getByRole("radio", { name: "Think Tank" }));
    expect(screen.getAllByLabelText("Participant id")).toHaveLength(2);
    expect(screen.getByLabelText("Judge role")).toHaveValue("implementer");
    expect(screen.getByText("Think Tank · 2 participants")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Add participant" }));
    expect(screen.getAllByLabelText("Participant id")).toHaveLength(3);
    fireEvent.click(screen.getByRole("radio", { name: "Standing owner" }));
    expect(screen.queryByLabelText("Participant id")).not.toBeInTheDocument();
    expect(screen.getAllByText("Standing owner").length).toBeGreaterThan(30);
  });

  // FS-14.A53: shared orchestrator instructions are entered once, saved
  // exactly, and a refused oversized value names the field and keeps the draft.
  it("saves shared orchestrator instructions exactly and keeps a refused draft", async () => {
    const guidance = "  Name branches by stage id.\nGroup agents by stage.  ";
    const saved: PipelineTemplate[] = [];
    server.use(http.put("/api/pipelines/maximum", async ({ request }) => {
      const body = await request.json() as PipelineTemplate;
      saved.push(body);
      if (saved.length > 1) {
        return HttpResponse.json({ error: { code: "validation", message: "invalid template", details: { diagnostics: [{ field: "orchestrator_instructions", code: "too_long", message: "must be at most 16000 characters" }] } } }, { status: 422 });
      }
      return HttpResponse.json({ id: "maximum", template: body, valid: true, diagnostics: [] });
    }));
    const client = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
    render(<QueryClientProvider client={client}><MemoryRouter><TemplateEditor seed={{ id: "maximum", template }} /></MemoryRouter></QueryClientProvider>);
    await screen.findByText("Stage 32");
    const field = screen.getByLabelText("Shared orchestrator instructions");
    fireEvent.change(field, { target: { value: guidance } });
    fireEvent.click(screen.getByRole("button", { name: "Save template" }));
    await screen.findByText("Template saved.");
    expect(saved[0].orchestrator_instructions).toBe(guidance);
    expect(saved[0].stages.every((stage) => !stage.objective.includes("Name branches"))).toBe(true);
    expect(screen.getByLabelText("Shared orchestrator instructions")).toHaveValue(guidance);

    const oversized = "x".repeat(16001);
    fireEvent.change(screen.getByLabelText("Shared orchestrator instructions"), { target: { value: oversized } });
    fireEvent.click(screen.getByRole("button", { name: "Save template" }));
    expect(await screen.findByText("orchestrator_instructions")).toBeInTheDocument();
    expect(screen.getByLabelText("Shared orchestrator instructions")).toHaveValue(oversized);
  });
});
