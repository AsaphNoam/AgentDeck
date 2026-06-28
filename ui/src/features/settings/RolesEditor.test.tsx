import React from "react";
import { describe, it, expect, beforeAll, afterAll, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { RolesEditor } from "./RolesEditor";

// Minimal MSW server for roles API.
const server = setupServer(
  http.get("/api/roles", () =>
    HttpResponse.json({
      implementer: { title: "Implementer", system_prompt: "", skip_permissions: null },
    }),
  ),
  http.post("/api/roles", async ({ request }) => {
    const body = (await request.json()) as Record<string, unknown>;
    return HttpResponse.json({ role: body.role, ...body }, { status: 201 });
  }),
  http.delete("/api/roles/:id", () => new HttpResponse(null, { status: 204 })),
);

beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterEach(() => {
  cleanup();
  server.resetHandlers();
});
afterAll(() => server.close());

function renderWithQuery(ui: React.ReactElement) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

describe("RolesEditor", () => {
  it("renders existing roles from GET /api/roles", async () => {
    renderWithQuery(<RolesEditor />);
    expect(await screen.findByText("Implementer")).toBeInTheDocument();
  });

  it("create role invalidates query so new role appears", async () => {
    let calls = 0;
    server.use(
      http.get("/api/roles", () => {
        calls++;
        const roles: Record<string, unknown> =
          calls === 1
            ? { implementer: { title: "Implementer", system_prompt: "" } }
            : {
                implementer: { title: "Implementer", system_prompt: "" },
                "new-role": { title: "New Role", system_prompt: "desc" },
              };
        return HttpResponse.json(roles);
      }),
    );

    renderWithQuery(<RolesEditor />);
    // Wait for initial render.
    expect(await screen.findByText("Implementer")).toBeInTheDocument();

    // Open the create dialog.
    fireEvent.click(screen.getByText("New role"));

    // Fill in the form.
    const slugInput = screen.getByPlaceholderText("e.g. security-reviewer");
    const titleInput = screen.getByPlaceholderText("e.g. Security Reviewer");
    fireEvent.change(slugInput, { target: { value: "new-role" } });
    fireEvent.change(titleInput, { target: { value: "New Role" } });

    // Submit.
    fireEvent.click(screen.getByText("Create"));

    // After create, query invalidates and the new role appears.
    await waitFor(() => expect(calls).toBeGreaterThan(1));
    expect(await screen.findByText("New Role")).toBeInTheDocument();
  });
});
