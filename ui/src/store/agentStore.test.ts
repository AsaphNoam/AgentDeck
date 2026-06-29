import { describe, expect, it, beforeEach } from "vitest";
import { useAgentStore } from "./agentStore";

const agent = {
  agent_id: "a_1",
  name: "Atlas",
  role: "implementer",
  project: "agentdeck",
  backend: "claude",
  model: "sonnet",
  interface: "chat",
  created_at: "2026-06-28T00:00:00Z",
  running: true,
  state: "idle" as const,
  detail: "",
  context_pct: 0,
  updated_at: 1,
};

beforeEach(() => {
  useAgentStore.setState({ agents: {}, order: [], hydrating: false });
});

describe("agentStore", () => {
  it("upserts agents and appends order once", () => {
    useAgentStore.getState().applyStateUpdate(agent);
    useAgentStore.getState().applyStateUpdate({ ...agent, state: "busy" });
    expect(useAgentStore.getState().agents.a_1.state).toBe("busy");
    expect(useAgentStore.getState().order).toEqual(["a_1"]);
  });

  it("removes stale agents after hydration completes", () => {
    useAgentStore.getState().applyStateUpdate(agent);
    useAgentStore.getState().applyStateUpdate({ ...agent, agent_id: "a_2" });
    useAgentStore.getState().hydrateComplete(["a_2"]);
    expect(Object.keys(useAgentStore.getState().agents)).toEqual(["a_2"]);
    expect(useAgentStore.getState().order).toEqual(["a_2"]);
  });

  it("clears last_sent_at only when the timestamp still matches", () => {
    useAgentStore.getState().applyStateUpdate({ ...agent, last_sent_at: "t1" });
    useAgentStore.getState().clearLastSentAt("a_1", "old");
    expect(useAgentStore.getState().agents.a_1.last_sent_at).toBe("t1");
    useAgentStore.getState().clearLastSentAt("a_1", "t1");
    expect(useAgentStore.getState().agents.a_1.last_sent_at).toBeUndefined();
  });
});
