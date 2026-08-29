import { useEffect } from "react";
import { sseClient } from "../../api/sse";
import type { AgentState } from "../../api/types";
import { useTranscriptStore } from "../../store/transcriptStore";
import { Composer } from "../chat/Composer";
import { TranscriptView } from "../chat/TranscriptView";

export function DashboardChatPane({ agent }: { agent: AgentState }) {
  const events = useTranscriptStore((state) => state.byAgent[agent.agent_id] ?? []);

  useEffect(() => sseClient.registerOpenAgent(agent.agent_id), [agent.agent_id]);

  return (
    <div className="dashboard-chat-pane" data-slot="chat-pane" data-agent-pane={agent.agent_id}>
      <TranscriptView
        agentId={agent.agent_id}
        events={events}
        sourceActive={agent.running && agent.state === "idle"}
        busy={agent.state === "busy"}
      />
      <div className="dashboard-chat-composer">
        <Composer agentId={agent.agent_id} busy={agent.state === "busy" || agent.state === "waiting_input"} />
      </div>
    </div>
  );
}
