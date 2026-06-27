import { useEffect, useMemo, useState } from "react";
import "./App.css";

type AgentState = "busy" | "idle" | "waiting_input" | "done";
type Runtime = "chat" | "terminal";
type PrimaryPanel = "config" | "dashboard" | "chat" | "messaging" | "inspect" | "archive";

interface Health {
  status: string;
  version: string;
}

interface WalkthroughStep {
  id: string;
  title: string;
  demoAction: string;
  userGoal: string;
  behindScenes: string;
  primaryPanel: PrimaryPanel;
}

interface DemoFlags {
  launched: boolean;
  sseUpdated: boolean;
  permissionApproved: boolean;
  messageSent: boolean;
  runtimeSwitched: boolean;
  resumed: boolean;
}

interface DemoAgent {
  id: string;
  name: string;
  role: string;
  project: string;
  backend: string;
  model: string;
  runtime: Runtime;
  group: string;
  state: AgentState;
  contextPct: number;
  detail: string;
  preview: string;
  messages: number;
  files: string[];
  commands: string[];
}

const steps: WalkthroughStep[] = [
  {
    id: "configure",
    title: "Configure the local workspace",
    demoAction: "Review the project, role, and backend that will be composed into a launch.",
    userGoal: "Set up enough local config to make future launches one-click or one CLI command.",
    behindScenes:
      "AgentDeck stores roles, projects, backends, and defaults as JSON under ~/.agentdeck. Launching combines project cwd, project context, role prompt, backend env, and model settings.",
    primaryPanel: "config",
  },
  {
    id: "launch",
    title: "Launch implementer@agentdeck",
    demoAction: "Create a running implementer card and select it.",
    userGoal: "Start a named session without manually wiring prompts, cwd, model flags, or MCP registration.",
    behindScenes:
      "The server writes agents/{agent_id}.json for stable identity and running/{agent_id}.json for the active process. The runtime receives the composed config and starts the CLI.",
    primaryPanel: "dashboard",
  },
  {
    id: "dashboard",
    title: "Watch live dashboard state",
    demoAction: "Simulate a status update as the agent edits files and consumes context.",
    userGoal: "Understand which agents are busy, idle, waiting, or done without opening every session.",
    behindScenes:
      "Hooks or the chat runtime write status/{agent_id}.json. A file watcher notices the change, recomputes state, and emits an SSE state_update event to every browser client.",
    primaryPanel: "dashboard",
  },
  {
    id: "chat",
    title: "Open chat and approve a tool",
    demoAction: "Inspect the transcript, tool call, diff preview, and inline permission request.",
    userGoal: "Intervene at the exact point an agent needs a decision, then let the turn continue.",
    behindScenes:
      "Chat runtime speaks ACP over stdio. Assistant text, tool calls, tool results, diffs, and permission prompts stream into the server, then out to the UI as SSE events.",
    primaryPanel: "chat",
  },
  {
    id: "messaging",
    title: "Ask a reviewer agent for help",
    demoAction: "Send a review request from the implementer to a reviewer.",
    userGoal: "Coordinate agents directly instead of copying text between terminal tabs.",
    behindScenes:
      "The MCP messaging server exposes list_agents, send_message, and check_messages. Messages are mailbox files in messages/{recipient_id}/, and the nudger wakes idle recipients.",
    primaryPanel: "messaging",
  },
  {
    id: "inspect",
    title: "Inspect files, commands, and state",
    demoAction: "Review what the selected agent edited, ran, and wrote to local state.",
    userGoal: "Audit an agent's activity before merging, stopping, or assigning follow-up work.",
    behindScenes:
      "Tool calls and hooks populate file and command trails. The UI is a view over plain local state, so the user can inspect or back up ~/.agentdeck without a database.",
    primaryPanel: "inspect",
  },
  {
    id: "archive",
    title: "Search archive and resume",
    demoAction: "Find a past session and resume it with history intact.",
    userGoal: "Recover useful work records and continue the same logical session later.",
    behindScenes:
      "Stable agent_id is separate from the provider runtime session id. Resuming reads sessions/{agent_id}/ and relaunches a runtime with the previous transcript and config.",
    primaryPanel: "archive",
  },
];

const archivedSessions = [
  "dashboard-prd implementer changed card state rendering",
  "reviewer found missing SSE reconnect tests",
  "codex backend endpoint override verification",
  "terminal runtime iTerm2 launch spike",
  "activity map ambient visualization notes",
];

const initialFlags: DemoFlags = {
  launched: false,
  sseUpdated: false,
  permissionApproved: false,
  messageSent: false,
  runtimeSwitched: false,
  resumed: false,
};

function flagsForStep(index: number): DemoFlags {
  return {
    launched: index >= 1,
    sseUpdated: index >= 2,
    permissionApproved: index >= 3,
    messageSent: index >= 4,
    runtimeSwitched: index >= 5,
    resumed: false,
  };
}

export default function App() {
  const [health, setHealth] = useState<Health | null>(null);
  const [activeStepIndex, setActiveStepIndex] = useState(0);
  const [flags, setFlags] = useState<DemoFlags>(initialFlags);
  const [selectedAgentId, setSelectedAgentId] = useState("a_impl");
  const [collapsedGroups, setCollapsedGroups] = useState<string[]>([]);
  const [density, setDensity] = useState<"compact" | "comfortable">("comfortable");
  const [archiveQuery, setArchiveQuery] = useState("dashboard");
  const [promptText, setPromptText] = useState("Ask Vega to review the dashboard card state changes.");

  useEffect(() => {
    fetch("/api/health")
      .then((response) => (response.ok ? (response.json() as Promise<Health>) : null))
      .then(setHealth)
      .catch(() => setHealth(null));
  }, []);

  const activeStep = steps[activeStepIndex];
  const agents = useMemo(() => buildAgents(flags), [flags]);
  const selectedAgent = agents.find((agent) => agent.id === selectedAgentId) ?? agents[0];
  const groups = useMemo(() => Array.from(new Set(agents.map((agent) => agent.group))), [agents]);
  const archiveMatches = archivedSessions.filter((session) =>
    session.toLowerCase().includes(archiveQuery.toLowerCase()),
  );

  const setFlag = (partial: Partial<DemoFlags>) => setFlags((current) => ({ ...current, ...partial }));

  const moveToStep = (nextIndex: number) => {
    const boundedIndex = Math.max(0, Math.min(steps.length - 1, nextIndex));
    setActiveStepIndex(boundedIndex);
    setFlags(flagsForStep(boundedIndex));
    setSelectedAgentId(boundedIndex >= 4 && boundedIndex < 6 ? "a_review" : "a_impl");
    if (boundedIndex === 6) setArchiveQuery("dashboard");
  };

  const launchAgent = () => {
    setFlag({ launched: true });
    setSelectedAgentId("a_impl");
    if (activeStepIndex < 1) setActiveStepIndex(1);
  };

  const simulateSseUpdate = () => {
    setFlag({ launched: true, sseUpdated: true });
    setSelectedAgentId("a_impl");
    if (activeStepIndex < 2) setActiveStepIndex(2);
  };

  const approvePermission = (approved: boolean) => {
    setFlag({ launched: true, sseUpdated: true, permissionApproved: approved });
    setSelectedAgentId("a_impl");
    if (activeStepIndex < 3) setActiveStepIndex(3);
  };

  const sendReviewerMessage = () => {
    setFlag({ launched: true, sseUpdated: true, permissionApproved: true, messageSent: true });
    setSelectedAgentId("a_review");
    if (activeStepIndex < 4) setActiveStepIndex(4);
  };

  const switchRuntime = () => {
    setFlag({ launched: true, sseUpdated: true, permissionApproved: true, runtimeSwitched: true });
    setSelectedAgentId("a_impl");
    if (activeStepIndex < 5) setActiveStepIndex(5);
  };

  const resumeSession = () => {
    setFlag({ launched: true, sseUpdated: true, permissionApproved: true, messageSent: true, resumed: true });
    setSelectedAgentId("a_archive");
    setActiveStepIndex(6);
  };

  const toggleGroup = (group: string) => {
    setCollapsedGroups((current) =>
      current.includes(group) ? current.filter((item) => item !== group) : [...current, group],
    );
  };

  return (
    <main className="app-shell">
      <section className="walkthrough-hero" aria-labelledby="page-title">
        <nav className="topbar" aria-label="AgentDeck walkthrough overview">
          <div className="brand-mark">AD</div>
          <div>
            <strong>AgentDeck</strong>
            <span>guided product demo</span>
          </div>
          <div className="server-pill">
            <span className={health ? "live-dot" : "muted-dot"} />
            {health ? `server ${health.status} · v${health.version}` : "static walkthrough mode"}
          </div>
        </nav>
        <div className="hero-intro">
          <p className="eyebrow">Walkthrough meets docs</p>
          <h1 id="page-title">Launch an agent, supervise the work, and learn what AgentDeck is doing underneath.</h1>
          <p>
            Follow the scenario from first config to archive resume. Each step changes the demo UI and explains the
            local files, runtimes, REST/SSE events, and MCP messaging that make the experience work.
          </p>
        </div>
      </section>

      <section className="walkthrough-layout" aria-label="AgentDeck guided walkthrough">
        <WalkthroughRail
          activeStepIndex={activeStepIndex}
          onBack={() => moveToStep(activeStepIndex - 1)}
          onNext={() => moveToStep(activeStepIndex + 1)}
          onStepChange={moveToStep}
        />
        <section className="demo-stage">
          <div className="stage-header">
            <div>
              <p className="section-kicker">Step {activeStepIndex + 1} of 7</p>
              <h2>{activeStep.title}</h2>
              <p>{activeStep.userGoal}</p>
            </div>
            <DemoActions
              activePanel={activeStep.primaryPanel}
              flags={flags}
              selectedAgent={selectedAgent}
              onApprovePermission={approvePermission}
              onLaunch={launchAgent}
              onResume={resumeSession}
              onSendMessage={sendReviewerMessage}
              onSseUpdate={simulateSseUpdate}
              onSwitchRuntime={switchRuntime}
            />
          </div>
          <div className="product-grid">
            <DashboardPreview
              activePanel={activeStep.primaryPanel}
              agents={agents}
              collapsedGroups={collapsedGroups}
              density={density}
              groups={groups}
              selectedAgentId={selectedAgent.id}
              onDensityChange={setDensity}
              onSelect={setSelectedAgentId}
              onToggleGroup={toggleGroup}
            />
            <PrimaryPanel
              activePanel={activeStep.primaryPanel}
              archiveMatches={archiveMatches}
              archiveQuery={archiveQuery}
              flags={flags}
              promptText={promptText}
              selectedAgent={selectedAgent}
              onApprovePermission={approvePermission}
              onArchiveQueryChange={setArchiveQuery}
              onPromptChange={setPromptText}
              onResume={resumeSession}
              onSendMessage={sendReviewerMessage}
              onSwitchRuntime={switchRuntime}
            />
          </div>
        </section>
        <BehindScenesPanel flags={flags} selectedAgent={selectedAgent} step={activeStep} />
      </section>

      <ArchitectureStrip />
    </main>
  );
}

function buildAgents(flags: DemoFlags): DemoAgent[] {
  const agents: DemoAgent[] = [
    {
      id: "a_impl",
      name: "Atlas",
      role: "implementer",
      project: "agentdeck",
      backend: flags.runtimeSwitched ? "Codex" : "Claude",
      model: flags.runtimeSwitched ? "GPT-5.5" : "Sonnet 4.6",
      runtime: flags.runtimeSwitched ? "terminal" : "chat",
      group: "dashboard-prd",
      state: flags.permissionApproved ? "idle" : flags.sseUpdated ? "waiting_input" : flags.launched ? "busy" : "idle",
      contextPct: flags.sseUpdated ? 61 : flags.launched ? 22 : 8,
      detail: flags.permissionApproved
        ? "Permission approved"
        : flags.sseUpdated
          ? "Permission required"
          : flags.launched
            ? "Starting runtime"
            : "Ready to launch",
      preview: flags.permissionApproved
        ? "Tool execution continued and the turn is waiting for review."
        : flags.sseUpdated
          ? "Approve switching runtime with the same stable agent id?"
          : flags.launched
            ? "Composing role, project, backend, model, and MCP config."
            : "Launch this implementer to start the scenario.",
      messages: flags.messageSent ? 1 : 0,
      files: ["ui/src/App.tsx", "ui/src/App.css", "docs/agent-dashboard-prd.md"],
      commands: ["npm run build", "make test"],
    },
  ];

  if (flags.messageSent || flags.resumed) {
    agents.push({
      id: "a_review",
      name: "Vega",
      role: "reviewer",
      project: "agentdeck",
      backend: "Codex",
      model: "GPT-5.5",
      runtime: "chat",
      group: "dashboard-prd",
      state: flags.messageSent ? "busy" : "idle",
      contextPct: 34,
      detail: flags.messageSent ? "Nudged by mailbox" : "Waiting",
      preview: flags.messageSent
        ? "Received review request through MCP and started checking the diff."
        : "Available for review requests.",
      messages: flags.messageSent ? 1 : 0,
      files: ["ui/src/App.tsx", "internal/server/events.go"],
      commands: ['rg "state_update" ui internal'],
    });
  }

  if (flags.resumed) {
    agents.push({
      id: "a_archive",
      name: "Quill",
      role: "pm",
      project: "agentdeck",
      backend: "Claude",
      model: "Sonnet 4.6",
      runtime: "chat",
      group: "resumed-session",
      state: "done",
      contextPct: 48,
      detail: "Resumed from archive",
      preview: "Loaded prior transcript and restored the same logical session.",
      messages: 0,
      files: ["sessions/a_archive/transcript.jsonl", "agents/a_archive.json"],
      commands: ["agentdeck dashboard open"],
    });
  }

  return agents;
}

function WalkthroughRail({
  activeStepIndex,
  onStepChange,
  onNext,
  onBack,
}: {
  activeStepIndex: number;
  onStepChange: (index: number) => void;
  onNext: () => void;
  onBack: () => void;
}) {
  return (
    <aside className="walkthrough-rail" aria-label="Walkthrough steps">
      <p className="section-kicker">Scenario</p>
      <div className="step-list">
        {steps.map((step, index) => (
          <button
            key={step.id}
            type="button"
            className={index === activeStepIndex ? "active" : index < activeStepIndex ? "complete" : ""}
            onClick={() => onStepChange(index)}
          >
            <span>{index + 1}</span>
            <strong>{step.title}</strong>
          </button>
        ))}
      </div>
      <div className="rail-actions">
        <button type="button" className="secondary" disabled={activeStepIndex === 0} onClick={onBack}>
          Back
        </button>
        <button type="button" disabled={activeStepIndex === steps.length - 1} onClick={onNext}>
          Next
        </button>
      </div>
    </aside>
  );
}

function DemoActions({
  activePanel,
  flags,
  selectedAgent,
  onLaunch,
  onSseUpdate,
  onApprovePermission,
  onSendMessage,
  onSwitchRuntime,
  onResume,
}: {
  activePanel: PrimaryPanel;
  flags: DemoFlags;
  selectedAgent: DemoAgent;
  onLaunch: () => void;
  onSseUpdate: () => void;
  onApprovePermission: (approved: boolean) => void;
  onSendMessage: () => void;
  onSwitchRuntime: () => void;
  onResume: () => void;
}) {
  if (activePanel === "config") return <div className="demo-actions"><button type="button" onClick={onLaunch}>Launch Atlas</button></div>;
  if (activePanel === "dashboard") {
    return (
      <div className="demo-actions">
        <button type="button" onClick={flags.launched ? onSseUpdate : onLaunch}>
          {flags.launched ? "Simulate SSE Update" : "Launch Atlas"}
        </button>
      </div>
    );
  }
  if (activePanel === "chat") return <div className="demo-actions"><button type="button" onClick={() => onApprovePermission(true)}>{flags.permissionApproved ? "Permission Approved" : "Approve Tool"}</button></div>;
  if (activePanel === "messaging") return <div className="demo-actions"><button type="button" onClick={onSendMessage}>Send Review Request</button></div>;
  if (activePanel === "inspect") return <div className="demo-actions"><button type="button" onClick={onSwitchRuntime}>{selectedAgent.runtime === "terminal" ? "Runtime Switched" : "Switch Runtime"}</button></div>;
  return <div className="demo-actions"><button type="button" onClick={onResume}>Resume Session</button></div>;
}

function DashboardPreview({
  agents,
  groups,
  selectedAgentId,
  collapsedGroups,
  density,
  activePanel,
  onSelect,
  onToggleGroup,
  onDensityChange,
}: {
  agents: DemoAgent[];
  groups: string[];
  selectedAgentId: string;
  collapsedGroups: string[];
  density: "compact" | "comfortable";
  activePanel: PrimaryPanel;
  onSelect: (id: string) => void;
  onToggleGroup: (group: string) => void;
  onDensityChange: (density: "compact" | "comfortable") => void;
}) {
  return (
    <section className={`dashboard-preview ${activePanel === "dashboard" ? "spotlight" : ""}`}>
      <div className="panel-header">
        <div>
          <p className="section-kicker">Live dashboard</p>
          <h2>Agent cards</h2>
        </div>
        <div className="segmented">
          <button type="button" className={density === "compact" ? "active" : ""} onClick={() => onDensityChange("compact")}>
            Compact
          </button>
          <button
            type="button"
            className={density === "comfortable" ? "active" : ""}
            onClick={() => onDensityChange("comfortable")}
          >
            Roomy
          </button>
        </div>
      </div>
      <div className={`agent-board ${density}`}>
        {groups.map((group) => {
          const groupAgents = agents.filter((agent) => agent.group === group);
          const isCollapsed = collapsedGroups.includes(group);
          return (
            <div className="agent-group" key={group}>
              <button type="button" className="group-header" onClick={() => onToggleGroup(group)}>
                <span>{isCollapsed ? "+" : "-"}</span>
                {group}
                <em>{groupAgents.length} agents</em>
              </button>
              {!isCollapsed && (
                <div className="agent-card-grid">
                  {groupAgents.map((agent) => (
                    <button
                      type="button"
                      key={agent.id}
                      className={`agent-card ${agent.state} ${selectedAgentId === agent.id ? "selected" : ""}`}
                      onClick={() => onSelect(agent.id)}
                    >
                      <span className="card-status">{formatState(agent.state)}</span>
                      <strong>{agent.name}</strong>
                      <small>
                        {agent.role}@{agent.project}
                      </small>
                      <div className="card-meta">
                        <span>{agent.backend}</span>
                        <span>{agent.model}</span>
                        <span>{agent.runtime}</span>
                      </div>
                      <div className="context-bar" aria-label={`${agent.contextPct}% context used`}>
                        <span style={{ width: `${agent.contextPct}%` }} />
                      </div>
                      <p>{agent.preview}</p>
                      <div className="card-footer">
                        <span>{agent.detail}</span>
                        {agent.messages > 0 && <b>{agent.messages} mail</b>}
                      </div>
                    </button>
                  ))}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </section>
  );
}

function PrimaryPanel({
  activePanel,
  flags,
  selectedAgent,
  archiveQuery,
  archiveMatches,
  promptText,
  onArchiveQueryChange,
  onPromptChange,
  onApprovePermission,
  onSendMessage,
  onSwitchRuntime,
  onResume,
}: {
  activePanel: PrimaryPanel;
  flags: DemoFlags;
  selectedAgent: DemoAgent;
  archiveQuery: string;
  archiveMatches: string[];
  promptText: string;
  onArchiveQueryChange: (value: string) => void;
  onPromptChange: (value: string) => void;
  onApprovePermission: (approved: boolean) => void;
  onSendMessage: () => void;
  onSwitchRuntime: () => void;
  onResume: () => void;
}) {
  if (activePanel === "config") return <ConfigPanel />;
  if (activePanel === "chat") {
    return (
      <ChatPanel
        agent={selectedAgent}
        permissionApproved={flags.permissionApproved}
        promptText={promptText}
        onPermissionDecision={onApprovePermission}
        onPromptChange={onPromptChange}
      />
    );
  }
  if (activePanel === "messaging") return <MessagingPanel messageSent={flags.messageSent} onSendMessage={onSendMessage} />;
  if (activePanel === "inspect") return <InspectPanel runtimeSwitched={flags.runtimeSwitched} selectedAgent={selectedAgent} onSwitchRuntime={onSwitchRuntime} />;
  if (activePanel === "archive") {
    return (
      <ArchivePanel
        archiveMatches={archiveMatches}
        archiveQuery={archiveQuery}
        resumed={flags.resumed}
        onArchiveQueryChange={onArchiveQueryChange}
        onResume={onResume}
      />
    );
  }
  return <DashboardFocusPanel selectedAgent={selectedAgent} sseUpdated={flags.sseUpdated} />;
}

function ConfigPanel() {
  return (
    <section className="primary-panel">
      <p className="section-kicker">Launch config</p>
      <h2>Reusable inputs</h2>
      <div className="config-list">
        <ConfigRow label="Project" value="agentdeck · cwd ~/Projects/AgentDeck · context prompt included" accent="#43b883" />
        <ConfigRow label="Role" value="implementer · product-aware coding agent · permissions inherited" accent="#f3b63f" />
        <ConfigRow label="Backend" value="Claude · Sonnet 4.6 · chat runtime by default" accent="#7c9cff" />
      </div>
      <code className="cli-preview">agentdeck implementer@agentdeck --backend claude --model sonnet-4-6</code>
    </section>
  );
}

function DashboardFocusPanel({ selectedAgent, sseUpdated }: { selectedAgent: DemoAgent; sseUpdated: boolean }) {
  return (
    <section className="primary-panel">
      <p className="section-kicker">State stream</p>
      <h2>{selectedAgent.name} is {formatState(selectedAgent.state)}</h2>
      <div className="state-sample">
        <code>{`status/${selectedAgent.id}.json`}</code>
        <pre>{JSON.stringify(
          {
            agent_id: selectedAgent.id,
            state: selectedAgent.state,
            detail: selectedAgent.detail,
            last_trace: sseUpdated ? "PreToolUse: SwitchRuntime" : "SessionStart",
            context_pct: selectedAgent.contextPct / 100,
          },
          null,
          2,
        )}</pre>
      </div>
    </section>
  );
}

function ChatPanel({
  agent,
  promptText,
  permissionApproved,
  onPromptChange,
  onPermissionDecision,
}: {
  agent: DemoAgent;
  promptText: string;
  permissionApproved: boolean;
  onPromptChange: (value: string) => void;
  onPermissionDecision: (value: boolean) => void;
}) {
  return (
    <section className="primary-panel chat-panel">
      <div className="panel-header">
        <div>
          <p className="section-kicker">Streaming chat</p>
          <h2>{agent.name}</h2>
        </div>
        <button type="button" className="danger-button">Cancel turn</button>
      </div>
      <div className="transcript">
        <article className="message user-message">
          <strong>User</strong>
          <p>Implement the walkthrough page and explain the local state behind each step.</p>
        </article>
        <article className="message assistant-message">
          <strong>{agent.name}</strong>
          <p>I am editing the UI, then I will run the build and test suite before handing it back.</p>
        </article>
        <article className="tool-call">
          <div>
            <strong>Tool call</strong>
            <span>Edit ui/src/App.tsx</span>
          </div>
          <code>+ &lt;WalkthroughRail steps={`{steps}`} /&gt;</code>
        </article>
        <article className="permission-box">
          <div>
            <strong>Permission request</strong>
            <p>Allow this agent to switch runtime and resume the same stable AgentDeck session?</p>
          </div>
          <div className="permission-actions">
            <button type="button" onClick={() => onPermissionDecision(true)}>Approve</button>
            <button type="button" className="secondary" onClick={() => onPermissionDecision(false)}>Deny</button>
          </div>
          <small>{permissionApproved ? "Approved: ACP stream continues." : "Waiting: tool execution is gated."}</small>
        </article>
      </div>
      <label className="prompt-box">
        <span>Send prompt</span>
        <textarea value={promptText} onChange={(event) => onPromptChange(event.target.value)} rows={3} />
      </label>
    </section>
  );
}

function MessagingPanel({ messageSent, onSendMessage }: { messageSent: boolean; onSendMessage: () => void }) {
  return (
    <section className="primary-panel">
      <p className="section-kicker">MCP messaging</p>
      <h2>Implementer asks reviewer</h2>
      <div className="message-route">
        <FlowNode title="Atlas" detail="send_message(to: Vega)" />
        <FlowConnector />
        <FlowNode title="Mailbox file" detail="messages/a_review/msg_001.json" />
        <FlowConnector />
        <FlowNode title="Vega" detail={messageSent ? "nudged and reviewing" : "idle until message lands"} />
      </div>
      <button type="button" onClick={onSendMessage}>{messageSent ? "Message Sent" : "Send Review Request"}</button>
    </section>
  );
}

function InspectPanel({
  selectedAgent,
  runtimeSwitched,
  onSwitchRuntime,
}: {
  selectedAgent: DemoAgent;
  runtimeSwitched: boolean;
  onSwitchRuntime: () => void;
}) {
  return (
    <section className="primary-panel">
      <p className="section-kicker">Audit trail</p>
      <h2>{selectedAgent.name} trail</h2>
      <div className="trail-grid">
        <div>
          <strong>Edited files</strong>
          {selectedAgent.files.map((file) => <code key={file}>{file}</code>)}
        </div>
        <div>
          <strong>Commands</strong>
          {selectedAgent.commands.map((command) => <code key={command}>{command}</code>)}
        </div>
      </div>
      <button type="button" onClick={onSwitchRuntime}>{runtimeSwitched ? "Runtime switched to terminal" : "Switch runtime"}</button>
    </section>
  );
}

function ArchivePanel({
  archiveQuery,
  archiveMatches,
  resumed,
  onArchiveQueryChange,
  onResume,
}: {
  archiveQuery: string;
  archiveMatches: string[];
  resumed: boolean;
  onArchiveQueryChange: (value: string) => void;
  onResume: () => void;
}) {
  return (
    <section className="primary-panel">
      <p className="section-kicker">Archive search</p>
      <h2>Resume durable sessions</h2>
      <input value={archiveQuery} onChange={(event) => onArchiveQueryChange(event.target.value)} aria-label="Search archive" />
      <div className="archive-results">
        {(archiveMatches.length > 0 ? archiveMatches : ["No matches in the demo archive"]).map((session) => (
          <button type="button" key={session} onClick={onResume}>
            {session}
            <span>{resumed ? "Resumed" : "Resume"}</span>
          </button>
        ))}
      </div>
    </section>
  );
}

function BehindScenesPanel({
  step,
  selectedAgent,
  flags,
}: {
  step: WalkthroughStep;
  selectedAgent: DemoAgent;
  flags: DemoFlags;
}) {
  return (
    <aside className="behind-panel" aria-live="polite">
      <p className="section-kicker">Behind the scenes</p>
      <h2>{step.title}</h2>
      <p>{step.behindScenes}</p>
      <div className="mini-doc">
        <strong>What just happened</strong>
        <span>{step.demoAction}</span>
      </div>
      <div className="mini-doc">
        <strong>Current local signal</strong>
        <code>{currentSignal(step.primaryPanel, selectedAgent, flags)}</code>
      </div>
    </aside>
  );
}

function ArchitectureStrip() {
  return (
    <section className="architecture-band">
      <div>
        <p className="section-kicker">System map</p>
        <h2>Three local layers, one dashboard.</h2>
        <p>
          The browser issues REST commands and receives SSE updates. The Go server manages runtimes, watches files, and
          hosts the MCP messaging bridge. The agent CLI does the work while writing local state.
        </p>
      </div>
      <div className="architecture-flow">
        <FlowNode title="React UI" detail="walkthrough, cards, chat, archive" />
        <FlowConnector />
        <FlowNode title="Go server" detail="REST, SSE, watcher, registry" />
        <FlowConnector />
        <FlowNode title="Agent CLI" detail="ACP stream, hooks, tools" />
        <FlowConnector />
        <FlowNode title="~/.agentdeck" detail="agents, running, status, messages, sessions" />
      </div>
    </section>
  );
}

function ConfigRow({ label, value, accent }: { label: string; value: string; accent: string }) {
  return (
    <div className="config-row" style={{ "--accent": accent } as React.CSSProperties}>
      <span />
      <div>
        <strong>{label}</strong>
        <p>{value}</p>
      </div>
    </div>
  );
}

function FlowNode({ title, detail }: { title: string; detail: string }) {
  return (
    <div className="flow-node">
      <strong>{title}</strong>
      <span>{detail}</span>
    </div>
  );
}

function FlowConnector() {
  return <span className="flow-connector" aria-hidden="true" />;
}

function currentSignal(panel: PrimaryPanel, agent: DemoAgent, flags: DemoFlags) {
  const signals: Record<PrimaryPanel, string> = {
    config: "roles/implementer.json + projects/agentdeck.json + backends.json",
    dashboard: `SSE state_update -> ${agent.state}`,
    chat: flags.permissionApproved ? "notification: permission approved" : "notification: permission_required",
    messaging: flags.messageSent ? "messages/a_review/msg_001.json" : "MCP tools registered",
    inspect: flags.runtimeSwitched ? "running/a_impl.json interface=terminal" : "files + commands captured",
    archive: flags.resumed ? "sessions/a_archive/ loaded" : "GET /api/archive?q=dashboard",
  };
  return signals[panel];
}

function formatState(state: AgentState) {
  return state.replace("_", " ");
}
