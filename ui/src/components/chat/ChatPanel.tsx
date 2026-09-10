import { useEffect, useRef, useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import * as Tabs from "@radix-ui/react-tabs";
import { getHeldPrompt, setSessionConfig, switchRuntime } from "../../api/client";
import { useBackends, useProjects } from "../../api/config";
import type { AgentState } from "../../api/types";
import { sseClient } from "../../api/sse";
import { useAgentStore } from "../../store/agentStore";
import { useAnnotationStore } from "../../store/annotationStore";
import { useHeldStore } from "../../store/heldStore";
import { useTranscriptStore } from "../../store/transcriptStore";
import { ContextBar } from "../grid/ContextBar";
import { Composer } from "./Composer";
import { TranscriptView } from "./TranscriptView";
import { FilesTab } from "./FilesTab";
import { CommandsTab } from "./CommandsTab";
import { TerminalTab } from "./TerminalTab";
import { resetRuntimeForBackend, resetRuntimeForModel, type RuntimeSelection } from "../../lib/runtimeSelection";

function runtimeSelection(agent: AgentState): RuntimeSelection {
  return { backend: agent.backend, model: agent.model, effort: agent.effort ?? "" };
}

function runtimeErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

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
  const agentsHydrated = useAgentStore((state) => state.hydrated);
  const pendingAnnotations = useAnnotationStore((state) => state.bySource[id]?.length ?? 0);
  const discardAnnotations = useAnnotationStore((state) => state.discard);
  const events = useTranscriptStore((state) => state.byAgent[id] ?? []);
  const held = useHeldStore((state) => state.byAgent[id]);
  const heldAfterSeq = useHeldStore((state) => state.afterSeqByAgent[id] ?? 0);
  const hold = useHeldStore((state) => state.hold);
  const releaseHeld = useHeldStore((state) => state.release);
  const { data: backends } = useBackends();
  const { data: projects } = useProjects();
  const [tab, setTab] = useState(() => initialTab(params.get("tab"), agent?.interface));
  const [runtime, setRuntime] = useState<RuntimeSelection>(() => agent ? runtimeSelection(agent) : { backend: "", model: "", effort: "" });
  const [switchError, setSwitchError] = useState<string | null>(null);
  const [switching, setSwitching] = useState(false);
  const [applyingSetting, setApplyingSetting] = useState<"effort" | "fast" | null>(null);
  const [liveFast, setLiveFast] = useState(agent?.fast ?? false);

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

  // A successful switch is published as an updated agent record. Re-seed from
  // that authoritative identity instead of keeping the former pending choice.
  useEffect(() => {
    if (!agent) return;
    setRuntime(runtimeSelection(agent));
    setLiveFast(agent.fast ?? false);
    setSwitchError(null);
  }, [agent?.backend, agent?.model, agent?.effort, agent?.fast]);

  useEffect(() => {
    return sseClient.registerOpenAgent(id);
  }, [id]);

  useEffect(() => {
    if (!agent?.running || agent.interface !== "chat") return;
    let current = true;
    getHeldPrompt(id).then((snapshot) => {
      if (!current) return;
      if (snapshot.text) hold(id, snapshot.text, snapshot.after_seq);
      else releaseHeld(id);
    }).catch(() => { /* SSE remains the primary agent-state path. */ });
    return () => { current = false; };
  }, [agent?.running, agent?.interface, id, hold, releaseHeld]);

  // A held message stops being pending the moment the server actually sends it,
  // which is when it enters the durable transcript as an ordinary user prompt.
  // Matching the sequenced server event rather than the local echo is what keeps
  // the pending tail from clearing before delivery (FS-03.R48, TS-08.R56).
  useEffect(() => {
    if (!held) return;
    const delivered = events.some((event) =>
      (event.kind ?? event.type) === "user_text" && event.seq != null && event.seq > heldAfterSeq && String(event.text ?? "") === held);
    if (delivered) releaseHeld(id);
  }, [events, held, heldAfterSeq, id, releaseHeld]);

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

  // A deep link or hard reload renders before the first agent hydration lands, so
  // an absent agent is not yet evidence the source is gone (FS-13.R16). Claiming
  // it early would offer a destructive discard for drafts whose live source is
  // still arriving, so the missing-source recovery waits for hydration.
  if (!agent && !agentsHydrated) {
    return (
      <section className="placeholder-view">
        <h1>Loading agent…</h1>
        <Link to="/">Back</Link>
      </section>
    );
  }

  if (!agent) {
    return (
      <section className="placeholder-view">
        <h1>Agent not found</h1>
        {pendingAnnotations > 0 && (
          <div>
            <p>{pendingAnnotations} pending annotation{pendingAnnotations === 1 ? "" : "s"} cannot be sent because the source agent no longer exists.</p>
            <button type="button" onClick={() => discardAnnotations(id)}>Discard pending annotations</button>
          </div>
        )}
        <Link to="/">Back</Link>
      </section>
    );
  }

  // Back targets the agent's project dashboard only when that project is a current,
  // non-archived catalog member; otherwise it falls back to the projects home so a
  // removed/archived project never strands the user on a dead-end route (FS-03.R27).
  const projectActive = !!agent.project && !!projects?.[agent.project] && !projects[agent.project].archived;
  const backTarget = projectActive ? `/project/${agent.project}` : "/";
  const selectedBackend = backends?.backends[runtime.backend];
  const selectedModel = selectedBackend?.models[runtime.model];
  const currentRuntime = runtimeSelection(agent);
  const runtimeChanged = runtime.backend !== currentRuntime.backend || runtime.model !== currentRuntime.model;
  const runtimeListed = !!selectedBackend && !!selectedModel;
  const editableRuntime = agent.running && agent.interface === "chat";
  const currentModel = backends?.backends[agent.backend]?.models[agent.model];
  const stagedRuntime = runtime.backend !== agent.backend || runtime.model !== agent.model;

  const applySetting = async (body: { effort?: string; fast?: boolean }, kind: "effort" | "fast") => {
    if (applyingSetting) return;
    setApplyingSetting(kind);
    setSwitchError(null);
    try {
      const applied = await setSessionConfig(agent.agent_id, body);
      if (kind === "effort" && applied.effort !== undefined) setRuntime((current) => ({ ...current, effort: applied.effort ?? "" }));
      if (kind === "fast") setLiveFast(applied.fast);
    } catch (error) {
      setRuntime(currentRuntime);
      setLiveFast(agent.fast);
      setSwitchError(runtimeErrorMessage(error));
    } finally {
      setApplyingSetting(null);
    }
  };

  const submitRuntimeSwitch = async () => {
    if (!runtimeChanged || !runtimeListed || switching) return;
    setSwitching(true);
    setSwitchError(null);
    try {
      await switchRuntime(agent.agent_id, runtime);
    } catch (error) {
      setRuntime(currentRuntime);
      setSwitchError(runtimeErrorMessage(error));
    } finally {
      setSwitching(false);
    }
  };

  return (
    <section className="chat-panel" data-ui="agent-workspace" data-state="active" data-variant={agent.interface === "terminal" ? "terminal" : "chat"}>
      <header className="chat-header" data-slot="header">
        <Link to={backTarget}>Back</Link>
        <div data-slot="identity">
          <h1>{agent.name}</h1>
          {editableRuntime ? (
            <div className="chat-runtime-picker">
              <fieldset className="chat-runtime-staged">
                <legend>Runtime</legend>
              <div className="form-field">
                <label htmlFor="chat-runtime-backend">Backend</label>
                <select id="chat-runtime-backend" value={runtime.backend} disabled={!backends || switching} onChange={(event) => setRuntime(resetRuntimeForBackend(backends, event.target.value))}>
                  {!backends?.backends[runtime.backend] && runtime.backend && <option value={runtime.backend}>{runtime.backend}</option>}
                  {Object.entries(backends?.backends ?? {}).map(([id, backend]) => <option key={id} value={id}>{backend.name} ({id})</option>)}
                </select>
              </div>
              <div className="form-field">
                <label htmlFor="chat-runtime-model">Model</label>
                <select id="chat-runtime-model" value={runtime.model} disabled={!selectedBackend || switching} onChange={(event) => setRuntime(resetRuntimeForModel(backends, runtime.backend, event.target.value))}>
                  {!selectedModel && runtime.model && <option value={runtime.model}>{runtime.model}</option>}
                  {Object.entries(selectedBackend?.models ?? {}).map(([id, model]) => <option key={id} value={id}>{model.name} ({id})</option>)}
                </select>
              </div>
              {runtimeChanged && <button className="chat-runtime-switch" type="button" disabled={!runtimeListed || switching} onClick={() => void submitRuntimeSwitch()}>{switching ? "Switching…" : "Switch"}</button>}
              </fieldset>
              <fieldset className="chat-session-settings">
                <legend>Session settings</legend>
              {(selectedModel?.efforts ?? []).length > 0 && (
                <div className="form-field">
                  <label htmlFor="chat-runtime-effort">Effort</label>
                  <select id="chat-runtime-effort" value={runtime.effort} disabled={switching || !!applyingSetting} onChange={(event) => stagedRuntime ? setRuntime((current) => ({ ...current, effort: event.target.value })) : void applySetting({ effort: event.target.value }, "effort")}>
                    {selectedModel!.efforts!.map((effort) => <option key={effort} value={effort}>{effort}</option>)}
                  </select>
                </div>
              )}
              {currentModel?.fast && (
                // The catalog says this model can go fast, but only the live
                // session knows whether it really offers the speed tier — a
                // launch that asked for it and did not get it is a case the
                // feature deliberately allows (FS-09.R55). Showing an ordinary
                // enabled off toggle there would let the person flip a control
                // that silently springs back, so the unavailable case names its
                // reason and disables the control instead (FS-03.R46, INV §8).
                <label className="form-field">
                  <span>Speed</span>
                  {agent.fast_available ? (
                    <span><input type="checkbox" checked={liveFast} disabled={switching || !!applyingSetting || stagedRuntime} onChange={(event) => void applySetting({ fast: event.target.checked }, "fast")} /> {applyingSetting === "fast" ? "Applying…" : "Fast mode — higher provider usage"}</span>
                  ) : (
                    <span><input type="checkbox" checked={false} disabled /> Fast mode — this model does not offer it</span>
                  )}
                </label>
              )}
              </fieldset>
              {switchError && <p className="form-error" role="alert">{switchError}</p>}
            </div>
          ) : (
            <><span>{[agent.backend, agent.model, agent.effort].filter(Boolean).join(" · ")}</span><span>{agent.fast ? "Fast mode" : "Normal speed"}</span></>
          )}
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
          <TranscriptView agentId={id} events={events} sourceActive={agent.running && agent.state === "idle"} annotationsEnabled={agent.interface === "chat"} busy={agent.state === "busy"} />
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
        <div data-slot="composer">
          <Composer
            agentId={id}
            busy={agent.state === "busy" || agent.state === "waiting_input"}
            running={agent.running}
            steerable={agent.steering_available}
          />
        </div>
      )}
    </section>
  );
}
