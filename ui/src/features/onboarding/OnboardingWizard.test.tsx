import React from "react";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { delay, http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { OnboardingWizard } from "./OnboardingWizard";

const server = setupServer(
  http.get("/api/backends", () => HttpResponse.json({
    version: 2,
    backends: {
      codex: { name: "Codex", type: "codex-acp", default: true, default_model: "gpt", models: { gpt: { name: "GPT", model: "gpt" } } },
      opencode: { name: "OpenCode", type: "opencode-acp", default: false, default_model: "open", models: { open: { name: "Open", model: "open" } } },
    },
  })),
  http.get("/api/roles", () => HttpResponse.json({ implementer: { title: "Implementer", system_prompt: "", skip_permissions: null } })),
  http.get("/api/projects", () => HttpResponse.json({ app: { title: "App", cwd: "/tmp/app" } })),
  http.get("/api/config-sources", () => HttpResponse.json({ bindings: [], candidates: [] })),
);

beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterEach(() => {
  cleanup();
  server.resetHandlers();
});
afterAll(() => server.close());

function renderWizard(projectDone = true) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
  const steps = {
    backend: { done: true, detail: null },
    project: { done: projectDone, detail: null },
    role: { done: true, detail: null },
  };
  return render(<QueryClientProvider client={queryClient}>
    <OnboardingWizard steps={steps} onComplete={() => {}} />
  </QueryClientProvider>);
}

describe("OnboardingWizard", () => {
  it("resumes with Config for the configured Codex backend", async () => {
    renderWizard();
    expect(await screen.findByRole("button", { name: "Use my Codex configuration" })).toBeInTheDocument();
    expect(screen.getByText("Config")).toHaveAttribute("data-state", "current");
    expect(screen.getByText("Launch")).toHaveAttribute("data-state", "upcoming");
  });

  it("skips Config and its progress item for an unsupported configured backend", async () => {
    server.use(http.get("/api/backends", () => HttpResponse.json({
      version: 2,
      backends: {
        opencode: { name: "OpenCode", type: "opencode-acp", default: true, default_model: "open", models: { open: { name: "Open", model: "open" } } },
      },
    })));
    renderWizard();
    expect(await screen.findByText("Launch your first agent")).toBeInTheDocument();
    expect(screen.queryByText("Config")).not.toBeInTheDocument();
    expect(screen.getByText("Launch", { selector: ".wizard-step-indicator" })).toHaveAttribute("data-state", "current");
  });

  // Resuming at Project must use the catalog that arrives after first render,
  // not a Claude fallback seeded at mount (FS-04.R49).
  function completeProject() {
    fireEvent.change(screen.getByPlaceholderText("e.g. My App"), { target: { value: "App" } });
    fireEvent.change(screen.getByPlaceholderText("~/Projects/my-app"), { target: { value: "/tmp/app" } });
    fireEvent.click(screen.getByRole("button", { name: "Create project" }));
  }

  function serveLateCatalog(type: string) {
    server.use(
      http.get("/api/backends", async () => {
        await delay(50);
        return HttpResponse.json({
          version: 2,
          backends: { chosen: { name: "Chosen", type, default: true, default_model: "m", models: { m: { name: "M", model: "m" } } } },
        });
      }),
      http.post("/api/projects", () => HttpResponse.json({
        project: "app", title: "App", color: [1, 2, 3], cwd: "/tmp/app", add_dirs: [], context_prompt: "",
      })),
    );
  }

  it("offers Codex linking after Project when resumed with a late Codex catalog", async () => {
    serveLateCatalog("codex-acp");
    renderWizard(false);
    completeProject();
    expect(await screen.findByRole("button", { name: "Use my Codex configuration" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Use my Claude Code configuration" })).not.toBeInTheDocument();
  });

  it("advances to Launch after Project when resumed with a late OpenCode catalog", async () => {
    serveLateCatalog("opencode-acp");
    renderWizard(false);
    completeProject();
    expect(await screen.findByText("Launch your first agent")).toBeInTheDocument();
    expect(screen.queryByText("Config")).not.toBeInTheDocument();
  });
});
