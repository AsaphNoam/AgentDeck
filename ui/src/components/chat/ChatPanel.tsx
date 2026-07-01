import { useEffect, useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import * as Tabs from "@radix-ui/react-tabs";
import { getTranscript } from "../../api/client";
import { sseClient } from "../../api/sse";
import { useAgentStore } from "../../store/agentStore";
import { useTranscriptStore } from "../../store/transcriptStore";
import { ContextBar } from "../grid/ContextBar";
import { Composer } from "./Composer";
import { TranscriptView } from "./TranscriptView";
import { FilesTab } from "./FilesTab";
import { CommandsTab } from "./CommandsTab";
import { TerminalTab } from "./TerminalTab";

export function ChatPanel() {
  const { id = "" } = useParams();
  const [params] = useSearchParams();
  const agent = useAgentStore((state) => state.agents[id]);
  const events = useTranscriptStore((state) => state.byAgent[id] ?? []);
  const setTranscript = useTranscriptStore((state) => state.setTranscript);
  const [tab, setTab] = useState(params.get("tab") === "terminal" ? "terminal" : "transcript");

  useEffect(() => {
    sseClient.setOpenAgent(id);
    void getTranscript(id).then((result) => setTranscript(result.agent_id, result.events)).catch(() => {});
    return () => sseClient.setOpenAgent(null);
  }, [id, setTranscript]);

  // Reveal a transcript event from the Files tab's "Diff" action: switch to the
  // transcript tab (its content is unmounted while another tab is active), then
  // scroll to the [data-seq] node once it has mounted.
  const revealInTranscript = (seq: number) => {
    setTab("transcript");
    requestAnimationFrame(() =>
      requestAnimationFrame(() => {
        const el = document.querySelector(`[data-seq="${seq}"]`);
        if (el) el.scrollIntoView({ behavior: "smooth", block: "center" });
      }),
    );
  };

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
      <Tabs.Root value={tab} onValueChange={setTab} className="chat-tabs">
        <Tabs.List className="chat-tabs-list">
          <Tabs.Trigger value="transcript">Transcript</Tabs.Trigger>
          <Tabs.Trigger value="files">Files</Tabs.Trigger>
          <Tabs.Trigger value="commands">Commands</Tabs.Trigger>
          {agent.interface === "terminal" && <Tabs.Trigger value="terminal">Terminal</Tabs.Trigger>}
        </Tabs.List>
        <Tabs.Content value="transcript" className="chat-tab-content">
          <TranscriptView agentId={id} events={events} />
        </Tabs.Content>
        <Tabs.Content value="files" className="chat-tab-content">
          <FilesTab agentId={id} onReveal={revealInTranscript} />
        </Tabs.Content>
        <Tabs.Content value="commands" className="chat-tab-content">
          <CommandsTab agentId={id} />
        </Tabs.Content>
        {agent.interface === "terminal" && (
          <Tabs.Content value="terminal" className="chat-tab-content">
            <TerminalTab agentId={id} />
          </Tabs.Content>
        )}
      </Tabs.Root>
      {agent.interface === "terminal" ? (
        <p className="terminal-readonly">Terminal agents receive input in the terminal tab.</p>
      ) : (
        <Composer agentId={id} busy={agent.state === "busy" || agent.state === "waiting_input"} />
      )}
    </section>
  );
}
