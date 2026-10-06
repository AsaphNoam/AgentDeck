import { CSS } from "@dnd-kit/utilities";
import { useSortable } from "@dnd-kit/sortable";
import { Link, useNavigate } from "react-router-dom";
import type { CSSProperties, ReactNode } from "react";
import type { AgentState } from "../../api/types";
import { ContextBar } from "./ContextBar";
import { StateBadge } from "./StateBadge";
import { useUiStore } from "../../store/uiStore";
import { IconButton } from "../ui";
import { CollapseIcon } from "../ui/icons";

export function AgentCard({ agent, lastLine, projectColor, projectTitle, showProject = true, expanded = false, onToggle, onUse, children }: { agent: AgentState; lastLine?: string; projectColor?: [number, number, number]; projectTitle?: string; showProject?: boolean; expanded?: boolean; onToggle?: () => void; onUse?: () => void; children?: ReactNode }) {
  const navigate = useNavigate();
  const openContextMenu = useUiStore((state) => state.openContextMenu);
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: agent.agent_id, disabled: expanded });
  const style: CSSProperties & { "--ad-project-accent"?: string } = {
    transform: CSS.Transform.toString(transform),
    transition,
    ...(projectColor ? { "--ad-project-accent": `rgb(${projectColor.join(",")})` } : {}),
  };
  const preview = agent.detail || lastLine || "";
  const projectLabel = projectTitle?.trim() || agent.project;
  // Collapsed and expanded cards show the same runtime identity string (FS-02.R63,
  // TS-08.R81), so it is derived once rather than re-joined at each call site (INV §2).
  const runtimeIdentity = [agent.backend, agent.model, agent.effort].filter(Boolean).join(" · ");

  return (
    <article
      ref={setNodeRef}
      className={`agent-card ${agent.running ? "" : "stopped"} ${isDragging ? "dragging" : ""}`}
      data-ui="agent-card"
      data-state={agent.running ? agent.state : "stopped"}
      data-variant={expanded ? "expanded" : isDragging ? "dragging" : "default"}
      style={style}
      onClick={expanded ? undefined : () => agent.interface === "chat" && onToggle ? onToggle() : navigate(`/agent/${agent.agent_id}`)}
      onContextMenu={expanded ? undefined : (event) => {
        event.preventDefault();
        openContextMenu(agent.agent_id, event.clientX, event.clientY);
      }}
      onFocusCapture={expanded ? onUse : undefined}
      onPointerDownCapture={expanded ? onUse : undefined}
    >
      <div
        className="agent-card-top"
        data-slot="header"
        onClick={expanded ? onToggle : undefined}
        onContextMenu={expanded ? (event) => {
          event.preventDefault();
          openContextMenu(agent.agent_id, event.clientX, event.clientY);
        } : undefined}
      >
        {!expanded && <button
          type="button"
          className="drag-handle"
          aria-label={`Reorder ${agent.name}`}
          onClick={(event) => event.stopPropagation()}
          onPointerDown={(event) => event.stopPropagation()}
          {...attributes}
          {...listeners}
        >
          ::
        </button>}
        {expanded ? (
          <div className="agent-card-header-content">
            <Link className="agent-card-name-link" data-slot="identity" to={`/agent/${agent.agent_id}`} onClick={(event) => event.stopPropagation()}>
              {agent.name}
            </Link>
            {runtimeIdentity && <span className="model-pill" data-slot="metadata">{runtimeIdentity}</span>}
          </div>
        ) : <strong data-slot="identity">{agent.name}</strong>}
        {expanded ? (
          <div className="agent-card-header-actions">
            <StateBadge state={agent.state} />
            <IconButton
              data-slot="collapse-control"
              size="small"
              type="button"
              aria-label="Collapse"
              title="Collapse"
              onClick={(event) => {
                event.stopPropagation();
                onToggle?.();
              }}
            >
              <CollapseIcon />
            </IconButton>
          </div>
        ) : <StateBadge state={agent.state} />}
      </div>
      {/* The meter's label can carry the exact used/total figure, so it takes its own row
          below the identity/badge/Collapse header (FS-02.R68, TS-08.R90). */}
      {expanded && <div className="agent-card-context" data-slot="context"><ContextBar value={agent.context_pct} used={agent.context_used} size={agent.context_size} compact /></div>}
      {expanded ? children : <>
      <p className="agent-subtitle" data-slot="metadata">{showProject ? `${agent.role} · ${projectLabel}` : agent.role}</p>
      <span className="model-pill">{runtimeIdentity}</span>
      {agent.pipeline && (
        <Link
          className="pipeline-association"
          to={`/pipelines/runs/${encodeURIComponent(agent.pipeline.run_id)}`}
          onClick={(event) => event.stopPropagation()}
        >
          {agent.pipeline.run_name} · {agent.pipeline.stage_id} · attempt {agent.pipeline.attempt_no}
        </Link>
      )}
      {agent.interface === "terminal" && <span className="terminal-pill">terminal{agent.driver ? ` · ${agent.driver}` : ""}</span>}
      <div className="message-indicators" data-slot="indicators" aria-label="Message indicators">
        {agent.unread_messages ? <span className="mail-badge">Mail {agent.unread_messages}</span> : null}
        {agent.last_sent_at ? <span className="sent-pulse">Sent</span> : null}
      </div>
      {preview && <p className="agent-preview" data-slot="preview">{preview}</p>}
      {!agent.running && <small className="stopped-label">stopped</small>}
      </>}
    </article>
  );
}
