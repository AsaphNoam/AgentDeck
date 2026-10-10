import { CSS } from "@dnd-kit/utilities";
import { useSortable } from "@dnd-kit/sortable";
import { Link, useNavigate } from "react-router-dom";
import type { CSSProperties, MouseEvent, ReactNode } from "react";
import type { AgentState } from "../../api/types";
import { ContextBar } from "./ContextBar";
import { StateBadge } from "./StateBadge";
import { useUiStore } from "../../store/uiStore";
import { IconButton } from "../ui";
import { CollapseIcon, ExpandIcon, GripIcon } from "../ui/icons";

// The Figma session-card composition (FS-12.R65, TS-08.R116): one head region carrying
// state, identity, runtime, context, preview and footer, with the chat pane below it
// once expanded. The head is the expanded card's activation target (FS-02.R52).
export function AgentCard({ agent, lastLine, projectTitle, showProject = true, expanded = false, onToggle, onUse, children }: { agent: AgentState; lastLine?: string; projectTitle?: string; showProject?: boolean; expanded?: boolean; onToggle?: () => void; onUse?: () => void; children?: ReactNode }) {
  const navigate = useNavigate();
  const openContextMenu = useUiStore((state) => state.openContextMenu);
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: agent.agent_id, disabled: expanded });
  const style: CSSProperties = { transform: CSS.Transform.toString(transform), transition };
  const preview = agent.detail || lastLine || "";
  const projectLabel = projectTitle?.trim() || agent.project;
  const expandable = agent.interface === "chat" && Boolean(onToggle);
  const menu = (event: MouseEvent) => {
    event.preventDefault();
    openContextMenu(agent.agent_id, event.clientX, event.clientY);
  };
  const stop = (event: MouseEvent) => event.stopPropagation();

  return (
    <article
      ref={setNodeRef}
      className={`agent-card ${agent.running ? "" : "stopped"} ${isDragging ? "dragging" : ""}`}
      data-ui="agent-card"
      data-state={agent.running ? agent.state : "stopped"}
      data-variant={expanded ? "expanded" : isDragging ? "dragging" : "default"}
      style={style}
      onClick={expanded ? undefined : () => expandable ? onToggle?.() : navigate(`/agent/${agent.agent_id}`)}
      onContextMenu={expanded ? undefined : menu}
      onFocusCapture={expanded ? onUse : undefined}
      onPointerDownCapture={expanded ? onUse : undefined}
    >
      <div className="agent-card-head" data-slot="header" onClick={expanded ? onToggle : undefined} onContextMenu={expanded ? menu : undefined}>
        <div className="agent-card-topline">
          {agent.running ? <StateBadge state={agent.state} /> : <span className="stopped-label">Stopped</span>}
          {!expanded && <button
            type="button"
            className="drag-handle"
            aria-label={`Reorder ${agent.name}`}
            title="Drag to reorder"
            onClick={stop}
            onPointerDown={(event) => event.stopPropagation()}
            {...attributes}
            {...listeners}
          >
            <GripIcon />
          </button>}
        </div>
        <div className="agent-card-title-row">
          <Link className="agent-card-name-link" data-slot="identity" to={`/agent/${agent.agent_id}`} onClick={stop}>
            {agent.name}
          </Link>
          {expandable && <IconButton
            className="agent-card-toggle"
            data-slot="collapse-control"
            size="small"
            type="button"
            aria-expanded={expanded}
            aria-label={expanded ? "Collapse" : "Expand"}
            title={expanded ? "Collapse" : "Expand"}
            onClick={(event) => {
              event.stopPropagation();
              onToggle?.();
            }}
          >
            {expanded ? <CollapseIcon /> : <ExpandIcon />}
          </IconButton>}
        </div>
        {/* Backend, model and resolved effort, omitting only an empty effort (FS-02.R63). */}
        <div className="agent-card-runtime" data-slot="metadata">
          {agent.backend && <span>{agent.backend}</span>}
          {agent.model && <strong>{agent.model}</strong>}
          {agent.effort && <span className="agent-card-effort">{agent.effort}</span>}
        </div>
        <div className="agent-card-context" data-slot="context"><ContextBar value={agent.context_pct} used={agent.context_used} size={agent.context_size} /></div>
        {preview && <p className="agent-preview" data-slot="preview">{preview}</p>}
        <div className="agent-card-footer">
          <span className="agent-subtitle">{showProject ? `${agent.role} · ${projectLabel}` : agent.role}</span>
          {agent.pipeline && (
            <Link className="pipeline-association" to={`/pipelines/runs/${encodeURIComponent(agent.pipeline.run_id)}`} onClick={stop}>
              {agent.pipeline.run_name} · {agent.pipeline.stage_id} · attempt {agent.pipeline.attempt_no}
            </Link>
          )}
          {agent.interface === "terminal" && <span className="terminal-pill">terminal{agent.driver ? ` · ${agent.driver}` : ""}</span>}
          <span className="message-indicators" data-slot="indicators" aria-label="Message indicators">
            {agent.unread_messages ? <span className="mail-badge">Mail {agent.unread_messages}</span> : null}
            {agent.last_sent_at ? <span className="sent-pulse">Sent</span> : null}
          </span>
        </div>
      </div>
      {expanded && children}
    </article>
  );
}
