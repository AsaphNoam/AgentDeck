import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import type { PipelineTemplate } from "../../schemas/pipeline";
import { TemplateEditor } from "./TemplateEditor";

const stages = Array.from({ length: 32 }, (_, index) => ({
  id: `stage-${index + 1}`, title: `Stage ${index + 1}`, role: "implementer", instruction: `Instruction ${index + 1}`,
  inputs: [], outputs: [], max_visits: 1,
  transitions: {
    success: { stage: "", final: "success", approval: "automatic" as const },
    failure: { stage: "", final: "failure", approval: "required" as const },
  },
}));
const template: PipelineTemplate = { version: 1, title: "Maximum delivery", inputs: [], stages };
const server = setupServer(
  http.get("/api/pipelines", () => HttpResponse.json([{ id: "maximum", template, valid: true, diagnostics: [] }])),
  http.get("/api/roles", () => HttpResponse.json({ implementer: { title: "Implementer", system_prompt: "", skip_permissions: false } })),
);

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
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
});
