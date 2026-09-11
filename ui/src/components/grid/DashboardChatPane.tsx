import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { sseClient } from "../../api/sse";
import type { AgentState } from "../../api/types";
import { useTranscriptStore } from "../../store/transcriptStore";
import { Composer } from "../chat/Composer";
import { TranscriptView } from "../chat/TranscriptView";
import type { FileLink } from "../chat/renderers/filePath";
import { writeFileLinkParams } from "../../lib/fileLinkParams";

export function DashboardChatPane({ agent }: { agent: AgentState }) {
  const events = useTranscriptStore((state) => state.byAgent[agent.agent_id] ?? []);
  const navigate = useNavigate();

  useEffect(() => sseClient.registerOpenAgent(agent.agent_id), [agent.agent_id]);

  // The pane is one grid column wide and deliberately carries less than the agent
  // screen, so no viewer opens here: a file link takes the pane's existing route
  // to the full surface, with the file already open (FS-03.R53).
  const openOnAgentScreen = (link: FileLink | null) => {
    if (!link) return;
    const query = writeFileLinkParams(new URLSearchParams(), link);
    navigate(`/agent/${agent.agent_id}?${query}`);
  };

  return (
    <div className="dashboard-chat-pane" data-slot="chat-pane" data-agent-pane={agent.agent_id}>
      <TranscriptView
        agentId={agent.agent_id}
        events={events}
        sourceActive={agent.running && agent.state === "idle"}
        busy={agent.state === "busy"}
        onOpenFile={openOnAgentScreen}
      />
      <div className="dashboard-chat-composer">
        <Composer agentId={agent.agent_id} busy={agent.state === "busy" || agent.state === "waiting_input"} />
      </div>
    </div>
  );
}
