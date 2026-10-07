import React from "react";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, it, expect, vi } from "vitest";
import { useAgentStore } from "../../store/agentStore";
import { useAnnotationStore } from "../../store/annotationStore";
import { useHeldStore } from "../../store/heldStore";
import { useTranscriptStore } from "../../store/transcriptStore";
import { useUiStore } from "../../store/uiStore";
import { ChatPanel, initialTab } from "./ChatPanel";

const mocks = vi.hoisted(() => ({
  getTranscript: vi.fn(async (id: string) => ({ agent_id: id, events: [] })),
  getHeldPrompt: vi.fn(async (id: string) => ({ agent_id: id, text: "", after_seq: 0 })),
  setSessionConfig: vi.fn(),
  switchRuntime: vi.fn(),
  getFileContent: vi.fn(async (agent_id: string, path: string) => ({
    agent_id, path, size: 14, mod_time: "2026-09-10T10:00:00Z",
    line_count: 1, content: "package state\n", truncated: false, language: "go",
  })),
  useBackends: vi.fn(),
  useProjects: vi.fn(),
}));

vi.mock("../../api/client", () => ({
  getTranscript: mocks.getTranscript,
  getHeldPrompt: mocks.getHeldPrompt,
  setSessionConfig: mocks.setSessionConfig,
  switchRuntime: mocks.switchRuntime,
  getFileContent: mocks.getFileContent,
}));

vi.mock("../../api/config", async (importOriginal) => ({
  ...await importOriginal<typeof import("../../api/config")>(),
  useBackends: mocks.useBackends,
  useProjects: mocks.useProjects,
}));

vi.mock("../../api/sse", () => ({
  sseClient: { registerOpenAgent: vi.fn(() => vi.fn()) },
}));

beforeEach(() => {
  mocks.getHeldPrompt.mockImplementation(async (id: string) => ({ agent_id: id, text: "", after_seq: 0 }));
  mocks.useBackends.mockReturnValue({ data: undefined });
  mocks.useProjects.mockReturnValue({ data: undefined });
});

afterEach(() => {
  cleanup();
  mocks.switchRuntime.mockReset();
  mocks.getHeldPrompt.mockReset();
  mocks.setSessionConfig.mockReset();
  mocks.useBackends.mockReset();
  mocks.useProjects.mockReset();
  useAgentStore.setState({ agents: {}, order: [], hydrated: false, hydrating: false });
  useAnnotationStore.setState({ bySource: {}, overallBySource: {}, editedAt: {} });
  useHeldStore.setState({ byAgent: {}, afterSeqByAgent: {} });
  useTranscriptStore.setState({ byAgent: {}, rawByAgent: {}, pending: {} });
  useUiStore.setState({ toasts: [] });
});

// initialTab drives which tab a chat panel opens on. The load-bearing case for
// the Finding 9 secondary fix: a terminal-interface agent must default to the
// Terminal tab so a WS attaches after launch (chat agents stay on transcript).
describe("initialTab", () => {
  it("defaults a terminal-interface agent to the Terminal tab", () => {
    expect(initialTab(null, "terminal")).toBe("terminal");
  });

  it("defaults a chat-interface agent to the transcript tab", () => {
    expect(initialTab(null, "acp")).toBe("transcript");
    expect(initialTab(null, undefined)).toBe("transcript");
  });

  it("honors an explicit ?tab= over the interface default", () => {
    expect(initialTab("terminal", "acp")).toBe("terminal");
    expect(initialTab("files", "terminal")).toBe("files");
  });

  // FS-03.R80: an old ?tab=commands link opens Transcript, never an empty panel.
  it("maps the retired Commands tab to Transcript", () => {
    expect(initialTab("commands", "acp")).toBe("transcript");
    expect(initialTab("commands", "terminal")).toBe("transcript");
  });
});

function renderPanel(id: string, search = "") {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: 0 } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[`/agent/${id}${search}`]}>
        <Routes><Route path="/agent/:id" element={<ChatPanel />} /></Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function liveAgent(id: string) {
  return {
    agent_id: id, name: "Nova", role: "implementer", project: "app", backend: "claude",
    model: "sonnet", interface: "chat", running: true, state: "idle", context_pct: 12,
    created_at: "2026-07-26T00:00:00Z",
  } as never;
}

const backends = {
  version: 2 as const,
  backends: {
    claude: {
      name: "Claude", type: "claude-acp" as const, default_model: "sonnet",
      models: { sonnet: { name: "Sonnet", model: "sonnet" } },
    },
    codex: {
      name: "Codex", type: "codex-acp" as const, default_model: "gpt-5",
      models: { "gpt-5": { name: "GPT-5", model: "gpt-5", efforts: ["low", "high"], default_effort: "high", fast: true } },
    },
  },
};

// FS-03.A61: the agent conversation offers no Commands tab, and an old
// ?tab=commands link lands on a working Transcript.
it("offers no Commands tab and opens Transcript for ?tab=commands", () => {
  useAgentStore.setState({ agents: { a_tabs: liveAgent("a_tabs") }, order: ["a_tabs"], hydrated: true, hydrating: false });
  renderPanel("a_tabs", "?tab=commands");

  const tabs = screen.getAllByRole("tab").map((tab) => tab.textContent);
  expect(tabs).toEqual(["Transcript", "Files"]);
  expect(screen.getByRole("tab", { name: "Transcript" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByRole("tabpanel")).toBeInTheDocument();
});

// FS-03.R66/A47: the full agent screen's context meter renders the shared
// exact used/total figure beside the percentage when both raw counts are known.
it("shows the exact used/total token figure when both raw counts are known", () => {
  const agent = { ...liveAgent("a_exact"), context_pct: 0.06, context_used: 12345, context_size: 200000 };
  useAgentStore.setState({ agents: { a_exact: agent }, order: ["a_exact"], hydrated: true, hydrating: false });
  mocks.useBackends.mockReturnValue({ data: backends });
  renderPanel("a_exact");

  expect(screen.getByText("12,345 / 200,000 tokens · 6% context used")).toBeInTheDocument();
});

// A state with no raw pair keeps the existing percentage-only label.
it("keeps the existing percentage-only label when raw counts are unavailable", () => {
  const agent = { ...liveAgent("a_pctonly"), context_pct: 0.12 };
  useAgentStore.setState({ agents: { a_pctonly: agent }, order: ["a_pctonly"], hydrated: true, hydrating: false });
  mocks.useBackends.mockReturnValue({ data: backends });
  renderPanel("a_pctonly");

  expect(screen.getByText("12% context used")).toBeInTheDocument();
});

// FS-12.A28: unsupported live controls leave no empty settings band, while a
// staged target with effort controls still exposes its existing settings.
it("omits empty session settings and shows them for a capable staged model", async () => {
  const agent = liveAgent("a_settings");
  useAgentStore.setState({ agents: { a_settings: agent }, order: ["a_settings"], hydrated: true, hydrating: false });
  mocks.useBackends.mockReturnValue({ data: backends });
  renderPanel("a_settings");

  expect(screen.getByRole("group", { name: "Runtime" })).toBeInTheDocument();
  expect(screen.queryByRole("group", { name: "Session settings" })).not.toBeInTheDocument();
  fireEvent.change(screen.getByLabelText("Backend"), { target: { value: "codex" } });
  expect(await screen.findByRole("group", { name: "Session settings" })).toBeInTheDocument();
  expect(screen.getByLabelText("Effort")).toBeInTheDocument();
});

// FS-13.R16/A8: the missing-source recovery is destructive, so it may only claim
// a source is gone once agent hydration has actually looked for it.
describe("ChatPanel missing-agent recovery", () => {
  it("lets a retained annotation tray be discarded when its source is gone", () => {
    useAgentStore.setState({ agents: {}, order: [], hydrated: true, hydrating: false });
    useAnnotationStore.getState().add("a_gone", { seq: 3, excerpt: "line", instruction: "check" });

    renderPanel("a_gone");

    expect(screen.getByText("1 pending annotation cannot be sent because the source agent no longer exists.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Discard pending annotations" }));
    expect(useAnnotationStore.getState().bySource.a_gone).toBeUndefined();
  });

  it("does not offer to discard drafts before the first hydration completes", () => {
    useAgentStore.setState({ agents: {}, order: [], hydrated: false, hydrating: true });
    useAnnotationStore.getState().add("a_pending", { seq: 3, excerpt: "line", instruction: "check" });

    renderPanel("a_pending");

    expect(screen.queryByText("Agent not found")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Discard pending annotations" })).not.toBeInTheDocument();
    expect(screen.getByText("Loading agent…")).toBeInTheDocument();
    expect(useAnnotationStore.getState().bySource.a_pending).toHaveLength(1);
  });

  it("shows the live workspace when hydration delivers the source", () => {
    useAgentStore.setState({ agents: { a_live: liveAgent("a_live") }, order: ["a_live"], hydrated: true, hydrating: false });
    useAnnotationStore.getState().add("a_live", { seq: 3, excerpt: "line", instruction: "check" });

    renderPanel("a_live");

    expect(screen.getByRole("heading", { name: "Nova" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Discard pending annotations" })).not.toBeInTheDocument();
    expect(useAnnotationStore.getState().bySource.a_live).toHaveLength(1);
  });

  // FS-12.A14 (R38) — the pane work added a focus-cycling binding to the card
  // grid only. On /agent/:id there is nothing to cycle to, and the agent screen's
  // composer must keep every key it had: Ctrl+Alt+Arrow reaches no handler here
  // and moves focus nowhere.
  it("leaves the agent screen composer untouched by the pane focus-cycling keys", () => {
    useAgentStore.setState({ agents: { a_live: liveAgent("a_live") }, order: ["a_live"], hydrated: true, hydrating: false });

    renderPanel("a_live");

    const composer = document.querySelector(".composer textarea") as HTMLTextAreaElement;
    expect(composer).not.toBeNull();
    composer.focus();
    for (const key of ["ArrowDown", "ArrowUp"]) {
      fireEvent.keyDown(composer, { key, ctrlKey: true, altKey: true });
      expect(composer).toHaveFocus();
    }
    expect(document.querySelectorAll("[data-agent-pane]")).toHaveLength(0);
  });
});

// FS-03.A12 (R27) — the chat header Back link targets the agent's project
// dashboard only when that project is a current, non-archived catalog member;
// otherwise it falls back to the projects home so a removed/archived project
// never strands the user on a dead-end route.
describe("ChatPanel back target", () => {
  it("targets the project dashboard for an active catalog project", () => {
    useAgentStore.setState({ agents: { a_live: liveAgent("a_live") }, order: ["a_live"], hydrated: true, hydrating: false });
    mocks.useProjects.mockReturnValue({ data: { app: { title: "App", archived: false } } });

    renderPanel("a_live");

    expect(screen.getByRole("link", { name: "Back" })).toHaveAttribute("href", "/project/app");
  });

  it("falls back to the projects home when the project is absent from the catalog", () => {
    useAgentStore.setState({ agents: { a_live: liveAgent("a_live") }, order: ["a_live"], hydrated: true, hydrating: false });
    mocks.useProjects.mockReturnValue({ data: {} });

    renderPanel("a_live");

    expect(screen.getByRole("link", { name: "Back" })).toHaveAttribute("href", "/");
  });

  it("falls back to the projects home when the project is archived", () => {
    useAgentStore.setState({ agents: { a_live: liveAgent("a_live") }, order: ["a_live"], hydrated: true, hydrating: false });
    mocks.useProjects.mockReturnValue({ data: { app: { title: "App", archived: true } } });

    renderPanel("a_live");

    expect(screen.getByRole("link", { name: "Back" })).toHaveAttribute("href", "/");
  });
});

// FS-03.A9 — the chat header is a second, explicit entry point to the existing
// runtime switch. It shares backend/model/effort reset behavior with launch and
// the dashboard switch dialog.
describe("ChatPanel runtime picker", () => {
  it("copies the stable thread identity from the header context menu", () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    useAgentStore.setState({ agents: { a_live: liveAgent("a_live") }, order: ["a_live"], hydrated: true, hydrating: false });

    const { container } = renderPanel("a_live");
    fireEvent.contextMenu(container.querySelector(".chat-header") as HTMLElement, { clientX: 20, clientY: 30 });
    fireEvent.click(screen.getByRole("button", { name: "Copy thread identity" }));

    expect(writeText).toHaveBeenCalledWith("a_live");
  });

  it("surfaces a clipboard refusal from the header action", async () => {
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText: vi.fn().mockRejectedValue(new Error("clipboard denied")) } });
    useAgentStore.setState({ agents: { a_live: liveAgent("a_live") }, order: ["a_live"], hydrated: true, hydrating: false });

    const { container } = renderPanel("a_live");
    fireEvent.contextMenu(container.querySelector(".chat-header") as HTMLElement, { clientX: 20, clientY: 30 });
    fireEvent.click(screen.getByRole("button", { name: "Copy thread identity" }));

    await waitFor(() => expect(useUiStore.getState().toasts.some((toast) => toast.title === "Copy failed" && toast.body === "clipboard denied")).toBe(true));
  });

  it("resets the model and effort, then switches a running chat agent", async () => {
    mocks.useBackends.mockReturnValue({ data: backends });
    mocks.switchRuntime.mockResolvedValue({ history_handoff: "native_resume" });
    useAgentStore.setState({ agents: { a_live: liveAgent("a_live") }, order: ["a_live"], hydrated: true, hydrating: false });

    renderPanel("a_live");

    const backend = await screen.findByLabelText("Backend");
    expect((screen.getByLabelText("Model") as HTMLSelectElement).value).toBe("sonnet");
    expect(screen.queryByLabelText("Effort")).not.toBeInTheDocument();
    fireEvent.change(backend, { target: { value: "codex" } });

    expect((screen.getByLabelText("Model") as HTMLSelectElement).value).toBe("gpt-5");
    expect((screen.getByLabelText("Effort") as HTMLSelectElement).value).toBe("high");
    fireEvent.click(screen.getByRole("button", { name: "Switch" }));

    await waitFor(() => expect(mocks.switchRuntime).toHaveBeenCalledWith("a_live", { backend: "codex", model: "gpt-5", effort: "high" }));
  });

  it("restores the current runtime and explains a rejected switch", async () => {
    mocks.useBackends.mockReturnValue({ data: backends });
    mocks.switchRuntime.mockRejectedValue(new Error("no runtime change requested"));
    useAgentStore.setState({ agents: { a_live: liveAgent("a_live") }, order: ["a_live"], hydrated: true, hydrating: false });

    renderPanel("a_live");

    fireEvent.change(await screen.findByLabelText("Backend"), { target: { value: "codex" } });
    fireEvent.click(screen.getByRole("button", { name: "Switch" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("no runtime change requested");
    expect((screen.getByLabelText("Backend") as HTMLSelectElement).value).toBe("claude");
    expect(screen.queryByRole("button", { name: "Switch" })).not.toBeInTheDocument();
  });

  it("keeps the active model and shows the provider's policy reason when a model is refused (FS-09.A48)", async () => {
    const reason = 'switch failed, rolled back to previous runtime: runtime: provider rejected the setting: model "opus": a model-switch policy hook blocked it: Opus is reserved for release work';
    const claude = backends.backends.claude;
    mocks.useBackends.mockReturnValue({ data: { ...backends, backends: { ...backends.backends, claude: {
      ...claude, models: { ...claude.models, opus: { name: "Opus", model: "opus" } },
    } } } });
    mocks.switchRuntime.mockRejectedValue(new Error(reason));
    useAgentStore.setState({ agents: { a_live: liveAgent("a_live") }, order: ["a_live"], hydrated: true, hydrating: false });

    renderPanel("a_live");

    fireEvent.change(await screen.findByLabelText("Model"), { target: { value: "opus" } });
    fireEvent.click(screen.getByRole("button", { name: "Switch" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("Opus is reserved for release work");
    expect((screen.getByLabelText("Model") as HTMLSelectElement).value).toBe("sonnet");
    expect(useAgentStore.getState().agents.a_live.model).toBe("sonnet");
    expect(mocks.switchRuntime).toHaveBeenCalledTimes(1);
  });

  it("keeps stopped agents' runtime identity static", () => {
    mocks.useBackends.mockReturnValue({ data: backends });
    useAgentStore.setState({ agents: { a_stopped: { ...liveAgent("a_stopped"), running: false } }, order: ["a_stopped"], hydrated: true, hydrating: false });

    renderPanel("a_stopped");

    expect(screen.getByText("claude · sonnet")).toBeInTheDocument();
    expect(screen.queryByLabelText("Backend")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Switch" })).not.toBeInTheDocument();
  });

  it("keeps an unavailable current runtime visible until a listed target is chosen", async () => {
    mocks.useBackends.mockReturnValue({ data: { version: 2, backends: { codex: backends.backends.codex } } });
    useAgentStore.setState({ agents: { a_live: liveAgent("a_live") }, order: ["a_live"], hydrated: true, hydrating: false });

    renderPanel("a_live");

    expect((await screen.findByLabelText("Backend") as HTMLSelectElement).value).toBe("claude");
    expect((screen.getByLabelText("Model") as HTMLSelectElement).value).toBe("sonnet");
    expect(screen.queryByRole("button", { name: "Switch" })).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Backend"), { target: { value: "codex" } });
    expect(screen.getByRole("button", { name: "Switch" })).toBeEnabled();
  });

  it("applies effort immediately without staging a runtime switch", async () => {
    mocks.useBackends.mockReturnValue({ data: backends });
    mocks.setSessionConfig.mockResolvedValue({ ...liveAgent("a_live"), backend: "codex", model: "gpt-5", effort: "low" });
    useAgentStore.setState({ agents: { a_live: { ...liveAgent("a_live"), backend: "codex", model: "gpt-5", effort: "high" } }, order: ["a_live"], hydrated: true, hydrating: false });

    renderPanel("a_live");
    fireEvent.change(await screen.findByLabelText("Effort"), { target: { value: "low" } });

    await waitFor(() => expect(mocks.setSessionConfig).toHaveBeenCalledWith("a_live", { effort: "low" }));
    expect(mocks.switchRuntime).not.toHaveBeenCalled();
    expect((screen.getByLabelText("Effort") as HTMLSelectElement).value).toBe("low");
  });

  it("shows and applies fast mode only for a capable current model", async () => {
    mocks.useBackends.mockReturnValue({ data: backends });
    mocks.setSessionConfig.mockResolvedValue({ ...liveAgent("a_live"), backend: "codex", model: "gpt-5", effort: "high", fast: true });
    useAgentStore.setState({ agents: { a_live: { ...liveAgent("a_live"), backend: "codex", model: "gpt-5", effort: "high", fast: false, fast_available: true } }, order: ["a_live"], hydrated: true, hydrating: false });

    renderPanel("a_live");
    const fast = await screen.findByRole("checkbox", { name: /Fast mode/ });
    fireEvent.click(fast);

    await waitFor(() => expect(mocks.setSessionConfig).toHaveBeenCalledWith("a_live", { fast: true }));
    expect(fast).toBeChecked();
  });

  // FS-03.A29 / R46: a launch may ask for fast mode on a catalog-capable model
  // and not get it, because only the live session knows whether its current model
  // really offers the speed tier (FS-09.R55). The header must say so rather than
  // render an ordinary enabled off toggle that silently springs back, which left
  // the person unable to tell an unavailable tier from their own choice.
  it("explains fast mode the live session does not offer instead of offering a dead toggle", async () => {
    mocks.useBackends.mockReturnValue({ data: backends });
    useAgentStore.setState({ agents: { a_live: { ...liveAgent("a_live"), backend: "codex", model: "gpt-5", effort: "high", fast: false, fast_available: false } }, order: ["a_live"], hydrated: true, hydrating: false });

    renderPanel("a_live");

    const fast = await screen.findByRole("checkbox", { name: /Fast mode/ });
    expect(fast).toBeDisabled();
    expect(fast).not.toBeChecked();
    expect(screen.getByText(/this model does not offer it/)).toBeInTheDocument();

    fireEvent.click(fast);
    expect(mocks.setSessionConfig).not.toHaveBeenCalled();
  });

  // FS-03.A29: a stopped chat agent renders its settings as static text, so the
  // unavailable-fast explanation never leaks into a surface with no live session.
  it("renders a stopped agent's fast mode as static text", () => {
    mocks.useBackends.mockReturnValue({ data: backends });
    useAgentStore.setState({ agents: { a_live: { ...liveAgent("a_live"), backend: "codex", model: "gpt-5", running: false, fast: true, fast_available: false } }, order: ["a_live"], hydrated: true, hydrating: false });

    renderPanel("a_live");

    expect(screen.getByText("Fast mode")).toBeInTheDocument();
    expect(screen.queryByRole("checkbox", { name: /Fast mode/ })).toBeNull();
  });
});

// FS-03.A31/A33 — the panel decides when a queued message stops being pending
// and whether the Steer control exists at all, both from the live session's own
// report rather than from anything the client inferred.
describe("ChatPanel queued follow-up and steering", () => {
  it("clears the pending message only when the server's own event delivers it", async () => {
    useAgentStore.setState({ agents: { a_live: { ...liveAgent("a_live"), state: "busy" } }, order: ["a_live"], hydrated: true, hydrating: false });
    mocks.getHeldPrompt.mockResolvedValue({ agent_id: "a_live", text: "then run the tests", after_seq: 10 });
    useTranscriptStore.setState({ byAgent: { a_live: [
      { kind: "user_text", seq: 4, text: "then run the tests" },
      { kind: "user_text", text: "then run the tests" },
    ] } });

    renderPanel("a_live");

    // The local echo carries no seq, so it is not evidence of delivery.
    expect(await screen.findByText("Queued — sends when this turn ends")).toBeInTheDocument();
    expect(useHeldStore.getState().byAgent.a_live).toBe("then run the tests");

    useTranscriptStore.setState({ byAgent: { a_live: [{ kind: "user_text", seq: 12, text: "then run the tests" }] } });
    await waitFor(() => expect(useHeldStore.getState().byAgent.a_live).toBeUndefined());
  });

  it("rehydrates a live held message after the panel remounts", async () => {
    useAgentStore.setState({ agents: { a_live: { ...liveAgent("a_live"), state: "busy" } }, order: ["a_live"], hydrated: true, hydrating: false });
    mocks.getHeldPrompt.mockResolvedValue({ agent_id: "a_live", text: "still queued", after_seq: 21 });

    renderPanel("a_live");

    expect(await screen.findByText("still queued")).toBeInTheDocument();
    expect(useHeldStore.getState().afterSeqByAgent.a_live).toBe(21);
  });

  it("renders no Steer control for a session that does not advertise it", async () => {
    useAgentStore.setState({ agents: { a_live: { ...liveAgent("a_live"), state: "busy", steering_available: false } }, order: ["a_live"], hydrated: true, hydrating: false });

    renderPanel("a_live");

    expect(await screen.findByRole("button", { name: "Send" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Steer" })).toBeNull();
  });

  it("renders Steer beside Send for a session that advertises it", async () => {
    useAgentStore.setState({ agents: { a_live: { ...liveAgent("a_live"), state: "busy", steering_available: true } }, order: ["a_live"], hydrated: true, hydrating: false });

    renderPanel("a_live");

    expect(await screen.findByRole("button", { name: "Steer" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Send" })).toBeInTheDocument();
  });
});

// FS-03.A36 (R53, R54) — the address is the open file's single source of truth,
// so a reload reopens it and Close clears it, and the panel carries the state
// attribute the relaxed content width keys off.
describe("the open file on the agent route", () => {
  it("reopens the file named by ?file= and clears it on Close", async () => {
    useAgentStore.setState({ agents: { a_1: liveAgent("a_1") }, order: ["a_1"], hydrated: true });
    renderPanel("a_1", "?file=internal/state/messages.go&fileLine=12");

    // Reopening from the address is the reload case: the viewer reads the file
    // itself, so its header is the witness that the parameter was honored.
    expect(await screen.findByText("internal/state/messages.go")).toBeInTheDocument();
    expect(document.querySelector('[data-ui="agent-workspace"]')).toHaveAttribute("data-file-open", "true");

    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    await waitFor(() => expect(screen.queryByText("internal/state/messages.go")).not.toBeInTheDocument());
    expect(document.querySelector('[data-ui="agent-workspace"]')).not.toHaveAttribute("data-file-open");
  });

  it("opens no viewer without ?file=", () => {
    useAgentStore.setState({ agents: { a_1: liveAgent("a_1") }, order: ["a_1"], hydrated: true });
    renderPanel("a_1");

    expect(screen.queryByRole("button", { name: "Reload" })).not.toBeInTheDocument();
    expect(document.querySelector('[data-ui="agent-workspace"]')).not.toHaveAttribute("data-file-open");
  });
});

// FS-03.A52, FS-21.R45: a member's chat keeps its ordinary header and shows a
// compact room cue plus a Think Tank tab with the goal, members and links —
// also between turns, when the agent is idle.
it("shows a compact room cue and a Think Tank tab for room membership", async () => {
  const room = {
    version: 1, room_id: "tt_1", title: "Cache choice", goal: "A long goal about caches", origin_project: "app",
    phase: "discussion", control: "running", participants: ["Nova", "Ari"], revision: 2, created_at: "2026-10-07T00:00:00Z",
    updated_at: "2026-10-07T00:00:00Z", active_attempts: [], total_remaining: 3, judge_enabled: false,
    roster: [
      { agent_id: "a_room", name: "Nova", project: "app", role: "participant", state: "active", limit: 2, completed: 1, remaining: 1, exists: true },
      { agent_id: "a_ari", name: "Ari", project: "app", role: "participant", state: "active", limit: 2, completed: 0, remaining: 2, exists: true },
    ],
  };
  const fetchSpy = vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
    if (String(input).startsWith("/api/think-tanks?agent_id=a_room")) return new Response(JSON.stringify({ version: 1, rooms: [room] }));
    return new Response("{}", { status: 404 });
  });
  useAgentStore.setState({ agents: { a_room: liveAgent("a_room") }, order: ["a_room"], hydrated: true, hydrating: false });
  mocks.useBackends.mockReturnValue({ data: backends });
  renderPanel("a_room");

  const cue = await screen.findByRole("link", { name: "Cache choice" });
  expect(cue.getAttribute("href")).toBe("/think-tank/tt_1");
  expect(screen.getByRole("heading", { name: "Nova" })).toBeInTheDocument();
  expect(screen.queryByText("A long goal about caches")).toBeNull();
  const tab = screen.getByRole("tab", { name: "Think Tank" });
  fireEvent.mouseDown(tab);
  fireEvent.click(tab);
  expect(await screen.findByText("1 of 2 turns left", { exact: false })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "Ari" }).getAttribute("href")).toBe("/agent/a_ari");
  expect(screen.getByText("Nova (this agent)")).toBeInTheDocument();
  expect(screen.queryByText(/most recent Think Tanks/)).toBeNull();
  fetchSpy.mockRestore();
});

// TS-14.R27: a clipped membership list says so in the Think Tank tab.
it("reports a clipped membership list in the Think Tank tab", async () => {
  const room = {
    version: 1, room_id: "tt_new", title: "Newest room", goal: "g", origin_project: "app",
    phase: "ended", control: "running", participants: ["Nova"], revision: 2, created_at: "2026-10-07T00:00:00Z",
    updated_at: "2026-10-07T00:00:00Z", active_attempts: [], total_remaining: 0, judge_enabled: false,
    roster: [{ agent_id: "a_room", name: "Nova", project: "app", role: "participant", state: "active", limit: 2, completed: 2, remaining: 0, exists: true }],
  };
  const fetchSpy = vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
    if (String(input).startsWith("/api/think-tanks?agent_id=a_room")) return new Response(JSON.stringify({ version: 1, rooms: [room], clipped: true }));
    return new Response("{}", { status: 404 });
  });
  useAgentStore.setState({ agents: { a_room: liveAgent("a_room") }, order: ["a_room"], hydrated: true, hydrating: false });
  mocks.useBackends.mockReturnValue({ data: backends });
  renderPanel("a_room");

  const tab = await screen.findByRole("tab", { name: "Think Tank" });
  fireEvent.mouseDown(tab);
  fireEvent.click(tab);
  expect(await screen.findByText("Showing the 1 most recent Think Tanks; older ones are not listed here.")).toBeInTheDocument();
  fetchSpy.mockRestore();
});
