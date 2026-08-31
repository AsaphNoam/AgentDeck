import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { Badge, Button, PageHeader, ProjectColorPicker, Surface } from "../components/ui";
import { ContextBar } from "../components/grid/ContextBar";
import { StateBadge } from "../components/grid/StateBadge";
import { EmptyState } from "../components/grid/EmptyState";
import { AssistantText } from "../components/chat/renderers/AssistantText";
import { DiffBlock } from "../components/chat/renderers/DiffBlock";
import { ToolCall } from "../components/chat/renderers/ToolCall";
import { ToolResult } from "../components/chat/renderers/ToolResult";
import type { AgentStatus } from "../api/types";
import { applyAppearance } from "../features/appearance/appearance";
import { PROJECT_COLOR_PRESETS } from "../lib/projectColors";
import { ProjectNav } from "../components/shell/ActiveProjectNav";
import "./contract-fixture.css";

const agentStates: AgentStatus[] = ["busy", "idle", "waiting_input", "done", "error", "unknown"];

export function VisualMatrix() {
  const [highVariance, setHighVariance] = useState(false);
  const [appearance, setAppearance] = useState<"core" | "sky-grove">("core");
  const initialSkin = useRef(document.documentElement.getAttribute("data-skin"));

  useEffect(() => {
    applyAppearance(appearance === "sky-grove" ? "sky-grove" : "");
  }, [appearance]);

  useEffect(() => () => {
    const previous = initialSkin.current;
    if (previous) document.documentElement.dataset.skin = previous;
    else document.documentElement.removeAttribute("data-skin");
  }, []);

  return (
    <div className={`visual-matrix ${highVariance ? "visual-matrix-high-variance" : ""}`}>
      <PageHeader
        eyebrow="Development fixture"
        title="Presentation matrix"
        description="Deterministic AgentDeck core surfaces for visual contract review."
        actions={(
          <div className="visual-matrix-controls">
            <label className="visual-matrix-toggle">
              Appearance
              <select
                aria-label="Fixture appearance"
                value={appearance}
                onChange={(event) => setAppearance(event.target.value as "core" | "sky-grove")}
              >
                <option value="core">AgentDeck Core</option>
                <option value="sky-grove">Sky & Grove</option>
              </select>
            </label>
            <label className="visual-matrix-toggle">
              <input
                type="checkbox"
                checked={highVariance}
                onChange={(event) => setHighVariance(event.target.checked)}
              />
              High-variance contract
            </label>
          </div>
        )}
      />

      <nav className="visual-matrix-routes" aria-label="Fixture route links">
        <Link to="/">Dashboard</Link>
        <Link to="/archive">Archive</Link>
        <Link to="/settings">Settings</Link>
      </nav>

      <section className="visual-matrix-section">
        <h2>Active-project shell navigation</h2>
        <div className="visual-matrix-shell-row">
          <ShellNavFixture visible={[]} overflow={[]} />
          <ShellNavFixture visible={fixtureProjects.slice(0, 1)} overflow={[]} currentProjectID="alpha" />
          <ShellNavFixture visible={[...fixtureProjects.slice(0, 4), fixtureProjects[5]]} overflow={[fixtureProjects[4]]} currentProjectID="foxtrot" />
        </div>
      </section>

      <section className="visual-matrix-section">
        <h2>Shared construction</h2>
        <div className="visual-matrix-row">
          <Button variant="primary">Primary action</Button>
          <Button variant="secondary">Secondary action</Button>
          <Button variant="ghost">Quiet action</Button>
          <Button variant="danger">Destructive action</Button>
          <Button disabled>Disabled action</Button>
        </div>
        <div className="visual-matrix-row">
          <Badge variant="neutral">Neutral</Badge>
          <Badge variant="info">Information</Badge>
          <Badge variant="success">Success</Badge>
          <Badge variant="warning">Warning</Badge>
          <Badge variant="danger">Error</Badge>
          <Badge variant="technical">Technical</Badge>
        </div>
        <Surface className="visual-matrix-surface">
          <label className="form-field">
            <span>Field label</span>
            <input defaultValue="Deterministic value" />
          </label>
          <p className="form-warning">A bounded warning stays attached to its field.</p>
          <p className="form-error">A mutation error remains visible.</p>
        </Surface>
      </section>

      <section className="visual-matrix-section" data-ui="dashboard">
        <h2>Dashboard states</h2>
        <div className="visual-matrix-agent-grid" data-slot="groups">
          {agentStates.map((state, index) => (
            <article
              className="agent-card"
              data-ui="agent-card"
              data-state={state}
              data-variant="default"
              key={state}
              style={{ "--ad-project-accent": `rgb(${PROJECT_COLOR_PRESETS[index].color.join(",")})` } as React.CSSProperties}
            >
              <div className="agent-card-top" data-slot="header">
                <button className="drag-handle" aria-label={`Reorder ${state}`} type="button">::</button>
                <strong data-slot="identity">{state === "waiting_input" ? "Needs a decision" : `${state} agent`}</strong>
                <StateBadge state={state} />
              </div>
              <p className="agent-subtitle" data-slot="metadata">builder · AgentDeck demo</p>
              <span className="model-pill">codex · gpt-fixture-{index + 1}</span>
              <div className="message-indicators" data-slot="indicators">
                {index === 2 && <span className="mail-badge">Mail 3</span>}
                {index === 3 && <span className="sent-pulse">Sent</span>}
              </div>
              <div data-slot="context"><ContextBar value={index / 5} /></div>
              <p className="agent-preview" data-slot="preview">Long operational detail remains bounded inside the card surface.</p>
            </article>
          ))}
          <article className="agent-card stopped" data-ui="agent-card" data-state="stopped" data-variant="default" style={{ "--ad-project-accent": `rgb(${PROJECT_COLOR_PRESETS[0].color.join(",")})` } as React.CSSProperties}>
            <div className="agent-card-top" data-slot="header">
              <button className="drag-handle" aria-label="Reorder stopped" type="button">::</button>
              <strong data-slot="identity">Stopped agent</strong>
              <StateBadge state="done" />
            </div>
            <p className="agent-subtitle" data-slot="metadata">reviewer · AgentDeck demo</p>
            <span className="terminal-pill">terminal · xterm</span>
            <small className="stopped-label">stopped</small>
          </article>
        </div>
        <EmptyState onNewAgent={() => undefined} />
        <div className="context-menu" data-ui="context-menu" role="menu">
          <div className="context-menu-color" role="menuitem">
            <span>Project color picker</span>
            <ProjectColorPicker value={PROJECT_COLOR_PRESETS[0].color} onChange={() => undefined} />
          </div>
        </div>
      </section>

      <section className="visual-matrix-section">
        <h2>Transcript and technical surfaces</h2>
        <div className="visual-matrix-transcript" data-ui="transcript">
          <article className="message user-message" data-slot="event" data-variant="user">
            Preserve product behavior while changing presentation.
          </article>
          <article className="message assistant-message" data-slot="event" data-variant="assistant">
            <h3>Assistant response</h3>
            <p>Readable Markdown uses a deliberate measure and technical detail stays distinct.</p>
            <code>inline_code()</code>
          </article>
          <AssistantText event={{ seq: 17, kind: "assistant_text", text: "```ts\nconst appearance = 'sky-grove';\n```" }} />
          <AssistantText event={{ seq: 19, kind: "assistant_text", text: "```mermaid\ngraph TD;\n  Deck-->Agent;\n  Agent-->Task;\n```" }} />
          <DiffBlock
            event={{ seq: 18, kind: "diff", path: "ui/src/styles/skins/sky-grove.css", old_text: "--surface: core;", new_text: "--surface: sky;" }}
            onAnnotate={() => undefined}
          />
          <article className="tool-run" data-ui="tool-run" data-state="collapsed">
            <button className="tool-toggle" data-slot="trigger" type="button">▸ Ran 2 tools</button>
          </article>
          <article className="tool-run" data-ui="tool-run" data-state="expanded">
            <button className="tool-toggle" data-slot="trigger" type="button">▾ Ran 2 tools</button>
            <div className="tool-run-content" data-slot="content">
              <ToolCall event={{ kind: "tool_call", name: "inspect_workspace", args: { path: "ui/src" } }} />
              <ToolCall event={{ kind: "tool_call", name: "search_files" }} />
              <ToolResult event={{ kind: "tool_result", status: "completed", content: "Representative tool output" }} />
              <ToolResult event={{ kind: "tool_result", status: "failed", error: "Representative tool error output" }} />
            </div>
          </article>
          <article className="permission-prompt" data-ui="permission-prompt" data-state="pending">
            <strong data-slot="title">Permission required</strong>
            <p data-slot="reason">Run a bounded local verification command.</p>
            <div data-slot="actions"><button type="button">Approve</button><button type="button">Deny</button></div>
          </article>
          <div className="terminal-panel" data-ui="terminal">
            <pre className="visual-matrix-terminal" data-slot="viewport">$ make test{"\n"}all checks passed</pre>
          </div>
        </div>
      </section>

      <section className="visual-matrix-section visual-matrix-columns">
        <article className="pipeline-run-page" data-ui="pipeline-run">
          <section className="pipeline-run-hero" data-slot="live">
            <div className="pipeline-run-kicker"><code>run_release</code><Badge variant="warning">needs attention</Badge></div>
            <div className="pipeline-run-title"><div><h2>Release verification</h2><p>Compare Core and Sky & Grove against the same deterministic fixture.</p></div><div className="pipeline-live-stage"><small>Current stage</small><strong>Visual review</strong><span>Visit 2 · attempt 4</span></div></div>
          </section>
          <section className="pipeline-timeline" data-slot="timeline">
            <ol><li className="pipeline-timeline-item pipeline-timeline-current" data-slot="attempt"><span className="pipeline-timeline-line" aria-hidden="true" /><details open><summary><span className="pipeline-stage-number">04</span><span className="pipeline-attempt-identity"><strong>Visual review</strong><small>Claude · Sonnet</small></span><span className="pipeline-state pipeline-state-paused">paused</span><span className="pipeline-disclosure-chevron">⌄</span></summary><div className="pipeline-attempt-body"><p className="pipeline-result-summary">One focused attempt owns the main reading column.</p><div className="pipeline-agent-grid" data-slot="agents"><div className="pipeline-agent-card"><span className="pipeline-agent-dot pipeline-agent-dot-idle" /><span><strong>Reviewer</strong><small>Stage agent · waiting</small><em>Review ready for approval</em></span><span>↗</span></div></div></div></details></li></ol>
          </section>
        </article>
        <div className="dialog-content onboarding-wizard" data-ui="dialog" data-slot="content" data-variant="onboarding">
          <div className="onboarding-flow" data-ui="onboarding" data-state="current">
            <div className="wizard-progress" data-slot="progress">Step 2 of 4</div>
            <div className="onboarding-step" data-slot="content">
              <h2>Choose a project</h2>
              <p>Onboarding keeps its existing structure and behavior in both appearances.</p>
            </div>
            <div className="onboarding-actions" data-slot="actions"><Button>Back</Button><Button variant="primary">Continue</Button></div>
          </div>
        </div>
      </section>

      <section className="visual-matrix-section visual-matrix-columns">
        <div data-ui="archive">
          <h2 data-slot="header">Archive result</h2>
          <div className="archive-row" data-slot="result" data-state="inactive">
            <div className="archive-row-top" data-slot="metadata"><span className="archive-name">Interface redesign</span><Badge variant="success">inactive</Badge></div>
            <p className="archive-snippet" data-slot="snippet">…semantic tokens and stable hooks…</p>
            <div className="archive-row-meta" data-slot="metadata"><span>12 turns</span><span>8 files</span></div>
          </div>
        </div>
        <div data-ui="settings">
          <h2 data-slot="header">Settings editor</h2>
          <div className="backend-card" data-ui="config-editor" data-variant="backends" data-slot="item">
            <div className="backend-card-header"><strong>Codex</strong><Badge variant="success">ready</Badge></div>
            <div className="model-row"><code className="config-slug">gpt-fixture</code><span className="config-badge">default</span></div>
            <div className="source-panel" data-ui="config-source" data-state="ok"><span className="source-health source-health-ok" data-slot="status">ok</span><code className="source-root" data-slot="root">~/.codex</code></div>
          </div>
        </div>
      </section>

      <section className="visual-matrix-section">
        <h2>Overlay and feedback</h2>
        <div className="visual-matrix-overlay-sample">
          <div className="visual-matrix-dialog" data-ui="dialog" data-slot="content" data-variant="default">
            <h3 data-slot="title">New agent</h3>
            <p data-slot="body">Dialog geometry, fields, and actions share the core construction.</p>
            <div className="form-actions" data-slot="actions"><Button>Cancel</Button><Button variant="primary">Launch</Button></div>
          </div>
          <button className="toast done" data-ui="toast" data-state="done" type="button">
            <strong data-slot="title">Agent finished</strong><span data-slot="body">The fixture task completed.</span>
          </button>
        </div>
      </section>
    </div>
  );
}

const fixtureProjects = ["Alpha", "Bravo workspace", "Charlie", "Delta", "Echo", "Foxtrot"].map((title, index) => ({
  id: title.split(" ")[0].toLowerCase(),
  title,
  color: PROJECT_COLOR_PRESETS[index].color,
}));

function ShellNavFixture({ visible, overflow, currentProjectID }: Parameters<typeof ProjectNav>[0]) {
  return (
    <div className="app-header visual-matrix-shell">
      <div className="app-logo">AgentDeck</div>
      <nav className="app-nav" aria-label="Fixture primary navigation"><a href="#dashboard">Dashboard</a><a href="#tasks">Tasks</a><a href="#pipelines">Pipelines</a><a href="#archive">Archive</a><a href="#settings">Settings</a></nav>
      <ProjectNav visible={visible} overflow={overflow} currentProjectID={currentProjectID} />
      <div className="app-connection"><span className="connection-status">Connected</span></div>
    </div>
  );
}
