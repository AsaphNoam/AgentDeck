import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { QUERY_KEYS } from "../../api/config";
import { OnboardingGate } from "./OnboardingGate";
import { OnboardingWizard } from "./OnboardingWizard";

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

const readyBackends = {
  version: 2,
  backends: {
    claude: {
      name: "Claude", type: "claude-acp", default: true, default_model: "sonnet",
      models: { sonnet: { name: "Sonnet", model: "sonnet" } },
    },
  },
};

const wizardSteps = {
  backend: { done: true, detail: "ok" },
  project: { done: true, detail: "ok" },
  role: { done: true, detail: "ok" },
};

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
);

beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterEach(() => {
  cleanup();
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

  // FS-04.R32/A13: someone who cannot finish setup now must be able to reach the
  // dashboard. The escape hatch marks onboarding complete and touches nothing
  // else — no project, no backend/catalog write, no launch.
  it("Set up later reveals the dashboard without any other side effect", async () => {
    let configPut: unknown = null;
    const touched: string[] = [];
    server.use(
      http.put("/api/config", async ({ request }) => {
        configPut = await request.json();
        return HttpResponse.json({ ...satisfiedConfig, onboarding_complete: true });
      }),
      http.put("/api/backends", () => {
        touched.push("PUT /api/backends");
        return HttpResponse.json({ version: 2, backends: {}, credentials: {} });
      }),
      http.post("/api/projects", () => {
        touched.push("POST /api/projects");
        return HttpResponse.json({}, { status: 201 });
      }),
      http.post("/api/sessions", () => {
        touched.push("POST /api/sessions");
        return HttpResponse.json({}, { status: 201 });
      }),
    );

    renderWithQuery(
      <OnboardingGate>
        <div data-testid="dashboard">Dashboard</div>
      </OnboardingGate>,
    );
    fireEvent.click(await screen.findByText("Set up later"));

    expect(await screen.findByTestId("dashboard")).toBeInTheDocument();
    expect(screen.queryByText("Welcome to AgentDeck")).toBeNull();
    expect(configPut).toEqual({ onboarding_complete: true });
    expect(touched).toEqual([]);
  });

  // A completion write that fails must not silently close the wizard: the poll
  // would reopen it and the person would never learn why (INV §8).
  it("keeps the wizard open and explains a failed Set up later", async () => {
    server.use(
      http.put("/api/config", () =>
        HttpResponse.json(
          { error: { code: "internal", message: "disk is read-only" } },
          { status: 500 },
        ),
      ),
    );

    renderWithQuery(
      <OnboardingGate>
        <div data-testid="dashboard">Dashboard</div>
      </OnboardingGate>,
    );
    fireEvent.click(await screen.findByText("Set up later"));

    expect(await screen.findByText(/disk is read-only/i)).toBeInTheDocument();
    expect(screen.getByText("Welcome to AgentDeck")).toBeInTheDocument();
    expect(screen.queryByTestId("dashboard")).toBeNull();
    // Still retryable.
    expect(screen.getByText("Set up later")).not.toBeDisabled();
  });

  // FS-04.A13 / INV §5: Set up later and a mounted step mutation share one
  // synchronous claim. The completion write cannot race any of the four
  // mutating onboarding surfaces.
  it("disables Set up later while backend validation is pending", async () => {
    let release!: () => void;
    let completionWrites = 0;
    server.use(
      http.get("/api/backends", () => HttpResponse.json(readyBackends)),
      http.put("/api/backends", async () => {
        await new Promise<void>((resolve) => { release = resolve; });
        return HttpResponse.json({ ...readyBackends, credentials: { claude: { status: "ok", detail: "" } } });
      }),
      http.put("/api/config", () => { completionWrites += 1; return HttpResponse.json({}); }),
    );
    renderWithQuery(<OnboardingWizard steps={{ ...wizardSteps, backend: { done: false, detail: "missing" } }} onComplete={() => {}} />);
    await screen.findByText(/Using this backend's default model/i);
    fireEvent.click(screen.getByText("Validate & Continue"));
    expect(screen.getByText("Set up later")).toBeDisabled();
    fireEvent.click(screen.getByText("Set up later"));
    expect(completionWrites).toBe(0);
    await waitFor(() => expect(typeof release).toBe("function"));
    release();
  });

  it("disables Set up later while project creation is pending", async () => {
    let release!: () => void;
    let completionWrites = 0;
    server.use(
      http.post("/api/projects", async () => {
        await new Promise<void>((resolve) => { release = resolve; });
        return HttpResponse.json({ project: "app", title: "App", color: [1, 2, 3], cwd: "/tmp/app", add_dirs: [], context_prompt: "" }, { status: 201 });
      }),
      http.put("/api/config", () => { completionWrites += 1; return HttpResponse.json({}); }),
    );
    renderWithQuery(<OnboardingWizard steps={{ ...wizardSteps, project: { done: false, detail: "missing" } }} onComplete={() => {}} />);
    fireEvent.change(screen.getByPlaceholderText("e.g. My App"), { target: { value: "App" } });
    fireEvent.change(screen.getByPlaceholderText("~/Projects/my-app"), { target: { value: "/tmp/app" } });
    fireEvent.click(screen.getByText("Create project"));
    expect(screen.getByText("Set up later")).toBeDisabled();
    fireEvent.click(screen.getByText("Set up later"));
    expect(completionWrites).toBe(0);
    await waitFor(() => expect(typeof release).toBe("function"));
    release();
  });

  it("disables Set up later while a configuration-source connection is pending", async () => {
    let release!: () => void;
    let completionWrites = 0;
    server.use(
      http.get("/api/projects", () => HttpResponse.json({ app: { title: "App", cwd: "/tmp/app" } })),
      http.get("/api/config-sources", () => HttpResponse.json({ bindings: [], candidates: [] })),
      http.post("/api/config-sources/preview", async () => {
        await new Promise<void>((resolve) => { release = resolve; });
        return HttpResponse.json({});
      }),
      http.put("/api/config", () => { completionWrites += 1; return HttpResponse.json({}); }),
    );
    renderWithQuery(<OnboardingWizard steps={wizardSteps} onComplete={() => {}} />);
    const connect = await screen.findByText(/^Use my .* configuration$/);
    await waitFor(() => expect(connect).not.toBeDisabled());
    fireEvent.click(connect);
    expect(screen.getByText("Set up later")).toBeDisabled();
    fireEvent.click(screen.getByText("Set up later"));
    expect(completionWrites).toBe(0);
    await waitFor(() => expect(typeof release).toBe("function"));
    release();
  });

  it("disables Set up later while the first agent launch is pending", async () => {
    let release!: () => void;
    let completionWrites = 0;
    server.use(
      http.get("/api/roles", () => HttpResponse.json({ implementer: { title: "Implementer", system_prompt: "" } })),
      http.get("/api/projects", () => HttpResponse.json({ app: { title: "App", cwd: "/tmp/app" } })),
      http.get("/api/backends", () => HttpResponse.json(readyBackends)),
      http.get("/api/config-sources", () => HttpResponse.json({ bindings: [], candidates: [] })),
      http.post("/api/sessions", async () => {
        await new Promise<void>((resolve) => { release = resolve; });
        return HttpResponse.json({ agent: { agent_id: "a_1", name: "Atlas" } });
      }),
      http.put("/api/config", () => { completionWrites += 1; return HttpResponse.json({}); }),
    );
    renderWithQuery(<OnboardingWizard steps={wizardSteps} onComplete={() => {}} />);
    fireEvent.click(await screen.findByText("Continue"));
    await screen.findByText("App (app)");
    const launchButton = screen.getByRole("button", { name: "Launch" });
    await waitFor(() => expect(launchButton).not.toBeDisabled());
    fireEvent.click(launchButton);
    expect(screen.getByText("Set up later")).toBeDisabled();
    fireEvent.click(screen.getByText("Set up later"));
    expect(completionWrites).toBe(0);
    await waitFor(() => expect(typeof release).toBe("function"));
    release();
  });
});
