import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { QUERY_KEYS } from "../../api/config";
import { OnboardingGate } from "./OnboardingGate";

const notSatisfiedConfig = {
  version: 1,
  port: 4317,
  default_project: "",
  default_role: "implementer",
  skip_permissions: false,
  onboarding_complete: false,
  onboarding: {
    satisfied: false,
    steps: {
      backend: { done: false, detail: "no backend" },
      project: { done: false, detail: "no projects" },
      role: { done: true, detail: "4 roles" },
    },
  },
};

const satisfiedConfig = {
  ...notSatisfiedConfig,
  onboarding: {
    satisfied: true,
    steps: {
      backend: { done: true, detail: "ok" },
      project: { done: true, detail: "1 project" },
      role: { done: true, detail: "4 roles" },
    },
  },
};

const backendDoneProjectNotDoneConfig = {
  ...notSatisfiedConfig,
  onboarding: {
    satisfied: false,
    steps: {
      backend: { done: true, detail: "ok" },
      project: { done: false, detail: "no projects" },
      role: { done: true, detail: "4 roles" },
    },
  },
};

let putConfigBody: { onboarding_complete?: boolean } | null = null;

const server = setupServer(
  http.get("/api/config", () => HttpResponse.json(notSatisfiedConfig)),
  http.get("/api/backends", () =>
    HttpResponse.json({ version: 2, backends: {} }),
  ),
  http.get("/api/roles", () => HttpResponse.json({})),
  http.get("/api/projects", () => HttpResponse.json({})),
  http.put("/api/backends", () =>
    HttpResponse.json({
      version: 2,
      backends: {},
      credentials: {},
    }),
  ),
  http.post("/api/projects", () =>
    HttpResponse.json({ project: "test", title: "Test", color: [128, 128, 128], cwd: "/tmp", add_dirs: [], context_prompt: "" }, { status: 201 }),
  ),
  http.put("/api/config", async ({ request }) => {
    putConfigBody = (await request.json()) as { onboarding_complete?: boolean };
    return HttpResponse.json(satisfiedConfig);
  }),
);

beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterEach(() => {
  cleanup();
  putConfigBody = null;
  server.resetHandlers();
});
afterAll(() => server.close());

function renderWithQuery(ui: React.ReactElement) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
  return {
    ...render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>),
    queryClient: qc,
  };
}

describe("OnboardingGate", () => {
  it("renders wizard when satisfied is false, blocking dashboard", async () => {
    renderWithQuery(
      <OnboardingGate>
        <div data-testid="dashboard">Dashboard</div>
      </OnboardingGate>,
    );
    expect(await screen.findByText("Welcome to AgentDeck")).toBeInTheDocument();
  });

  it("renders dashboard children when satisfied is true (no wizard)", async () => {
    server.use(http.get("/api/config", () => HttpResponse.json(satisfiedConfig)));
    renderWithQuery(
      <OnboardingGate>
        <div data-testid="dashboard">Dashboard</div>
      </OnboardingGate>,
    );
    expect(await screen.findByTestId("dashboard")).toBeInTheDocument();
    expect(screen.queryByText("Welcome to AgentDeck")).toBeNull();
  });

  it("wizard shows Backend step first when backend is not done", async () => {
    renderWithQuery(
      <OnboardingGate>
        <div>Dashboard</div>
      </OnboardingGate>,
    );
    expect(await screen.findByText("Configure your AI backend")).toBeInTheDocument();
  });

  it("wizard resumes on Project step when backend is done but project is not", async () => {
    server.use(
      http.get("/api/config", () => HttpResponse.json(backendDoneProjectNotDoneConfig)),
    );
    renderWithQuery(
      <OnboardingGate>
        <div>Dashboard</div>
      </OnboardingGate>,
    );
    expect(await screen.findByText("Create your first project")).toBeInTheDocument();
    expect(screen.queryByText("Configure your AI backend")).toBeNull();
  });

  it("keeps an open wizard on Project when a config refresh becomes satisfied", async () => {
    let currentConfig = backendDoneProjectNotDoneConfig;
    server.use(http.get("/api/config", () => HttpResponse.json(currentConfig)));

    const { queryClient } = renderWithQuery(
      <OnboardingGate>
        <div data-testid="dashboard">Dashboard</div>
      </OnboardingGate>,
    );
    expect(await screen.findByText("Create your first project")).toBeInTheDocument();

    currentConfig = satisfiedConfig;
    await queryClient.invalidateQueries({ queryKey: QUERY_KEYS.config });

    await waitFor(() => {
      expect(screen.getByText("Create your first project")).toBeInTheDocument();
    });
    expect(screen.queryByTestId("dashboard")).toBeNull();
  });

  it("Esc key does not dismiss the wizard", async () => {
    renderWithQuery(
      <OnboardingGate>
        <div>Dashboard</div>
      </OnboardingGate>,
    );
    expect(await screen.findByText("Welcome to AgentDeck")).toBeInTheDocument();

    fireEvent.keyDown(document, { key: "Escape" });

    // Wizard still visible
    expect(screen.getByText("Welcome to AgentDeck")).toBeInTheDocument();
  });

  it("Skip setup marks onboarding complete and reveals the dashboard without a launch (FS-04.R32)", async () => {
    renderWithQuery(
      <OnboardingGate>
        <div data-testid="dashboard">Dashboard</div>
      </OnboardingGate>,
    );
    fireEvent.click(await screen.findByText("Skip setup"));

    expect(await screen.findByTestId("dashboard")).toBeInTheDocument();
    expect(screen.queryByText("Welcome to AgentDeck")).toBeNull();
    expect(putConfigBody?.onboarding_complete).toBe(true);
  });

  it("Skip setup keeps the wizard open when the config write fails (FS-04.R32)", async () => {
    server.use(
      http.put("/api/config", () =>
        HttpResponse.json({ error: "Server error", message: "disk write failed" }, { status: 500 }),
      ),
    );
    renderWithQuery(
      <OnboardingGate>
        <div data-testid="dashboard">Dashboard</div>
      </OnboardingGate>,
    );
    fireEvent.click(await screen.findByText("Skip setup"));

    // Give the failing mutation time to settle; wizard must stay, dashboard hidden.
    await new Promise((r) => setTimeout(r, 100));
    expect(screen.getByText("Welcome to AgentDeck")).toBeInTheDocument();
    expect(screen.queryByTestId("dashboard")).toBeNull();
  });

  it("clicking overlay does not dismiss the wizard", async () => {
    renderWithQuery(
      <OnboardingGate>
        <div>Dashboard</div>
      </OnboardingGate>,
    );
    const overlay = await screen.findByRole("dialog").then(
      () => document.querySelector(".dialog-overlay"),
    );
    if (overlay) {
      fireEvent.click(overlay);
    }
    expect(screen.getByText("Welcome to AgentDeck")).toBeInTheDocument();
  });
});
