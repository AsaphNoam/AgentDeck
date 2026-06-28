import React from "react";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { PermissionPrompt } from "./PermissionPrompt";

describe("PermissionPrompt", () => {
  it("posts approve decisions", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response("{}", { status: 200 }));
    render(<PermissionPrompt agentId="a_1" event={{ kind: "permission_request", tool_call_id: "tc_1", name: "Bash" }} />);
    fireEvent.click(screen.getByText("Approve"));
    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    expect(fetchMock.mock.calls[0][0]).toBe("/api/sessions/a_1/permission");
    expect(fetchMock.mock.calls[0][1]).toMatchObject({ method: "POST" });
    fetchMock.mockRestore();
  });
});
