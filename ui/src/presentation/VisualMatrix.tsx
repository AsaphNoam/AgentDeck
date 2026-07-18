import { useState } from "react";
import { Link } from "react-router-dom";
import { Badge, Button, PageHeader, Surface } from "../components/ui";
import { ContextBar } from "../components/grid/ContextBar";
import { StateBadge } from "../components/grid/StateBadge";
import type { AgentStatus } from "../api/types";
import "./contract-fixture.css";

const agentStates: AgentStatus[] = ["busy", "idle", "waiting_input", "done", "error", "unknown"];

export function VisualMatrix() {
  const [highVariance, setHighVariance] = useState(false);

  return (
    <div className={`visual-matrix ${highVariance ? "visual-matrix-high-variance" : ""}`}>
      <PageHeader
        eyebrow="Development fixture"
        title="Presentation matrix"
        description="Deterministic AgentDeck core surfaces for visual contract review."
        actions={(
          <label className="visual-matrix-toggle">
            <input
              type="checkbox"
              checked={highVariance}
              onChange={(event) => setHighVariance(event.target.checked)}
            />
            High-variance contract
          </label>
        )}
      />

      <nav className="visual-matrix-routes" aria-label="Fixture route links">
        <Link to="/">Dashboard</Link>
        <Link to="/archive">Archive</Link>
        <Link to="/settings">Settings</Link>
      </nav>

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
            >
              <div className="agent-card-top" data-slot="header">
                <button className="drag-handle" aria-label={`Reorder ${state}`} type="button">::</button>
                <strong data-slot="identity">{state === "waiting_input" ? "Needs a decision" : `${state} agent`}</strong>
                <StateBadge state={state} />
              </div>
              <p className="agent-subtitle" data-slot="metadata">builder · agentdeck</p>
              <span className="model-pill">codex · gpt-fixture-{index + 1}</span>
              <div className="message-indicators" data-slot="indicators">
                {index === 2 && <span className="mail-badge">Mail 3</span>}
                {index === 3 && <span className="sent-pulse">Sent</span>}
              </div>
              <div data-slot="context"><ContextBar value={index / 5} /></div>
              <p className="agent-preview" data-slot="preview">Long operational detail remains bounded inside the card surface.</p>
            </article>
          ))}
          <article className="agent-card stopped" data-ui="agent-card" data-state="stopped" data-variant="default">
            <div className="agent-card-top" data-slot="header">
              <button className="drag-handle" aria-label="Reorder stopped" type="button">::</button>
              <strong data-slot="identity">Stopped agent</strong>
              <StateBadge state="done" />
            </div>
            <p className="agent-subtitle" data-slot="metadata">reviewer · agentdeck</p>
            <span className="terminal-pill">terminal · xterm</span>
            <small className="stopped-label">stopped</small>
          </article>
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
          <article className="tool-block tool-call" data-ui="tool-call" data-state="expanded">
            <button className="tool-toggle" data-slot="trigger" type="button">▾ Tool call: inspect_workspace</button>
            <pre className="tool-args" data-slot="content">{`{"path":"ui/src"}`}</pre>
          </article>
          <article className="tool-block tool-result tool-result-error" data-ui="tool-result" data-state="error">
            <pre data-slot="content">Representative tool error output</pre>
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
