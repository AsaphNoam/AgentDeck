import { CSS } from "@dnd-kit/utilities";
import { useSortable } from "@dnd-kit/sortable";
import { useNavigate } from "react-router-dom";
import type { CSSProperties } from "react";
import type { AgentState } from "../../api/types";
import { ContextBar } from "./ContextBar";
import { StateBadge } from "./StateBadge";
import { useUiStore } from "../../store/uiStore";

export function AgentCard({ agent, lastLine, projectColor }: { agent: AgentState; lastLine?: string; projectColor?: [number, number, number] }) {
  const navigate = useNavigate();
  const openContextMenu = useUiStore((state) => state.openContextMenu);
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: agent.agent_id });
  const style: CSSProperties & { "--ad-project-accent"?: string } = {
    transform: CSS.Transform.toString(transform),
    transition,
    ...(projectColor ? { "--ad-project-accent": `rgb(${projectColor.join(",")})` } : {}),
  };
  const preview = agent.detail || lastLine || "";

  return (
    <article
      ref={setNodeRef}
      className={`agent-card ${agent.running ? "" : "stopped"} ${isDragging ? "dragging" : ""}`}
      data-ui="agent-card"
      data-state={agent.running ? agent.state : "stopped"}
      data-variant={isDragging ? "dragging" : "default"}
      style={style}
      onClick={() => navigate(`/agent/${agent.agent_id}`)}
      onContextMenu={(event) => {
        event.preventDefault();
        openContextMenu(agent.agent_id, event.clientX, event.clientY);
      }}
    >
      <div className="agent-card-top" data-slot="header">
        <button
          type="button"
          className="drag-handle"
          aria-label={`Reorder ${agent.name}`}
          onClick={(event) => event.stopPropagation()}
          onPointerDown={(event) => event.stopPropagation()}
          {...attributes}
          {...listeners}
        >
          ::
        </button>
        <strong data-slot="identity">{agent.name}</strong>
        <StateBadge state={agent.state} />
      </div>
      <p className="agent-subtitle" data-slot="metadata">
        {agent.role} · {agent.project}
      </p>
      <span className="model-pill">
        {agent.backend} · {agent.model}
      </span>
      {agent.interface === "terminal" && <span className="terminal-pill">terminal{agent.driver ? ` · ${agent.driver}` : ""}</span>}
      <div className="message-indicators" data-slot="indicators" aria-label="Message indicators">
        {agent.unread_messages ? <span className="mail-badge">Mail {agent.unread_messages}</span> : null}
        {agent.last_sent_at ? <span className="sent-pulse">Sent</span> : null}
      </div>
      <div data-slot="context"><ContextBar value={agent.context_pct} /></div>
      {preview && <p className="agent-preview" data-slot="preview">{preview}</p>}
      {!agent.running && <small className="stopped-label">stopped</small>}
    </article>
  );
}
