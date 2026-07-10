import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { LaunchStep } from "./LaunchStep";

// Both the seeded my-app (bad cwd) and the user's freshly-created project exist.
const roles = { implementer: { title: "Implementer", system_prompt: "" } };
const projects = {
  "my-app": { title: "My App", color: [1, 2, 3], cwd: "~/Projects/my-app", add_dirs: [], context_prompt: "" },
  "user-proj": { title: "User Proj", color: [1, 2, 3], cwd: "/tmp/user-proj", add_dirs: [], context_prompt: "" },
};
const backends = {
  version: 2,
  backends: { claude: { name: "Claude", type: "claude-acp", default: true, default_model: "s", models: { s: { name: "S", model: "s" } } } },
};

let launchBody: { project?: string } | null = null;

const server = setupServer(
  http.get("/api/roles", () => HttpResponse.json(roles)),
  http.get("/api/projects", () => HttpResponse.json(projects)),
  http.get("/api/backends", () => HttpResponse.json(backends)),
  http.post("/api/sessions", async ({ request }) => {
    launchBody = (await request.json()) as { project?: string };
    return HttpResponse.json({ agent: { agent_id: "a_1", name: "Atlas" } });
  }),
  http.put("/api/config", () => HttpResponse.json({})),
);

beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterEach(() => {
  cleanup();
  launchBody = null;
  server.resetHandlers();
});
afterAll(() => server.close());

function renderWithQuery(ui: React.ReactElement) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: 0 }, mutations: { retry: 0 } } });
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

describe("LaunchStep", () => {
  it("launches the just-created project, not the seeded my-app (J2 onboarding-completion blocker)", async () => {
    renderWithQuery(<LaunchStep onDone={() => {}} initialProject="user-proj" />);

    // Wait for role/project data to populate the selects.
    expect(await screen.findByText("User Proj (user-proj)")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Launch"));

    await waitFor(() => expect(launchBody).not.toBeNull());
    expect(launchBody?.project).toBe("user-proj");
  });
});
