import { useEffect, useRef, useState } from "react";
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

// initialTab picks the tab a chat panel opens on. An explicit ?tab= wins; then a
// terminal-interface agent defaults to its Terminal tab so a WS attaches right
// after launch and the user sees the live session (the server-side always-on PTY
// drain already prevents a stall, but a transcript-first default would hide the
// terminal until the user clicked over). Everything else defaults to transcript.
export function initialTab(tabParam: string | null, agentInterface?: string): string {
  if (tabParam === "terminal") return "terminal";
  if (tabParam) return tabParam;
  if (agentInterface === "terminal") return "terminal";
  return "transcript";
}

export function ChatPanel() {
  const { id = "" } = useParams();
  const [params] = useSearchParams();
  const agent = useAgentStore((state) => state.agents[id]);
  const events = useTranscriptStore((state) => state.byAgent[id] ?? []);
  const setTranscript = useTranscriptStore((state) => state.setTranscript);
  const [tab, setTab] = useState(() => initialTab(params.get("tab"), agent?.interface));

  // The agent often isn't in the store yet at mount (it hydrates over SSE), so
  // the useState initializer above can't see its interface. Once it loads, apply
  // the terminal default exactly once — and never over an explicit ?tab= or a
  // manual switch the user already made.
  const appliedDefault = useRef(false);
  useEffect(() => {
    if (appliedDefault.current || params.get("tab")) return;
    if (agent?.interface === "terminal") {
      appliedDefault.current = true;
      setTab("terminal");
    }
  }, [agent?.interface, params]);

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
    <section className="chat-panel" data-ui="agent-workspace" data-state="active" data-variant={agent.interface === "terminal" ? "terminal" : "chat"}>
      <header className="chat-header" data-slot="header">
        <Link to="/">Back</Link>
        <div data-slot="identity">
          <h1>{agent.name}</h1>
          <span>{agent.backend} · {agent.model}</span>
        </div>
        <div data-slot="context"><ContextBar value={agent.context_pct} /></div>
      </header>
      <Tabs.Root value={tab} onValueChange={setTab} className="chat-tabs" data-slot="tabs">
        <Tabs.List className="chat-tabs-list" data-slot="tabs">
          <Tabs.Trigger value="transcript">Transcript</Tabs.Trigger>
          <Tabs.Trigger value="files">Files</Tabs.Trigger>
          <Tabs.Trigger value="commands">Commands</Tabs.Trigger>
          {agent.interface === "terminal" && <Tabs.Trigger value="terminal">Terminal</Tabs.Trigger>}
        </Tabs.List>
        <Tabs.Content value="transcript" className="chat-tab-content" data-slot="content">
          <TranscriptView agentId={id} events={events} />
        </Tabs.Content>
        <Tabs.Content value="files" className="chat-tab-content" data-slot="content">
          <FilesTab agentId={id} onReveal={revealInTranscript} />
        </Tabs.Content>
        <Tabs.Content value="commands" className="chat-tab-content" data-slot="content">
          <CommandsTab agentId={id} />
        </Tabs.Content>
        {agent.interface === "terminal" && (
          <Tabs.Content value="terminal" className="chat-tab-content" data-slot="content">
            <TerminalTab agentId={id} />
          </Tabs.Content>
        )}
      </Tabs.Root>
      {agent.interface === "terminal" ? (
        <p className="terminal-readonly" data-slot="composer">Terminal agents receive input in the terminal tab.</p>
      ) : (
        <div data-slot="composer"><Composer agentId={id} busy={agent.state === "busy" || agent.state === "waiting_input"} /></div>
      )}
    </section>
  );
}
