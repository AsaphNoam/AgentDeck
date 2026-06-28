import { useEffect } from "react";
import { Link, useParams } from "react-router-dom";
import { getTranscript } from "../../api/client";
import { sseClient } from "../../api/sse";
import { useAgentStore } from "../../store/agentStore";
import { useTranscriptStore } from "../../store/transcriptStore";
import { ContextBar } from "../grid/ContextBar";
import { Composer } from "./Composer";
import { TranscriptView } from "./TranscriptView";

export function ChatPanel() {
  const { id = "" } = useParams();
  const agent = useAgentStore((state) => state.agents[id]);
  const events = useTranscriptStore((state) => state.byAgent[id] ?? []);
  const setTranscript = useTranscriptStore((state) => state.setTranscript);

  useEffect(() => {
    sseClient.setOpenAgent(id);
    void getTranscript(id).then((result) => setTranscript(result.agent_id, result.events)).catch(() => {});
    return () => sseClient.setOpenAgent(null);
  }, [id, setTranscript]);

  if (!agent) {
    return (
      <section className="placeholder-view">
        <h1>Agent not found</h1>
        <Link to="/">Back</Link>
      </section>
    );
  }

  return (
    <section className="chat-panel">
      <header className="chat-header">
        <Link to="/">Back</Link>
        <div>
          <h1>{agent.name}</h1>
          <span>{agent.backend} · {agent.model}</span>
        </div>
        <ContextBar value={agent.context_pct} />
      </header>
      <TranscriptView agentId={id} events={events} />
      <Composer agentId={id} busy={agent.state === "busy" || agent.state === "waiting_input"} />
    </section>
  );
}
