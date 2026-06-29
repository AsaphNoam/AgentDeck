import { CSS } from "@dnd-kit/utilities";
import { useSortable } from "@dnd-kit/sortable";
import { useNavigate } from "react-router-dom";
import type { AgentState } from "../../api/types";
import { ContextBar } from "./ContextBar";
import { StateBadge } from "./StateBadge";
import { useUiStore } from "../../store/uiStore";

export function AgentCard({ agent, lastLine }: { agent: AgentState; lastLine?: string }) {
  const navigate = useNavigate();
  const openContextMenu = useUiStore((state) => state.openContextMenu);
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: agent.agent_id });
  const style = { transform: CSS.Transform.toString(transform), transition };
  const preview = agent.detail || lastLine || "";

  return (
    <article
      ref={setNodeRef}
      className={`agent-card ${agent.running ? "" : "stopped"} ${isDragging ? "dragging" : ""}`}
      style={style}
      onClick={() => navigate(`/agent/${agent.agent_id}`)}
      onContextMenu={(event) => {
        event.preventDefault();
        openContextMenu(agent.agent_id, event.clientX, event.clientY);
      }}
      {...attributes}
      {...listeners}
    >
      <div className="agent-card-top">
        <strong>{agent.name}</strong>
        <StateBadge state={agent.state} />
      </div>
      <p className="agent-subtitle">
        {agent.role} · {agent.project}
      </p>
      <span className="model-pill">
        {agent.backend} · {agent.model}
      </span>
      <div className="message-indicators" aria-label="Message indicators">
        {agent.unread_messages ? <span className="mail-badge">Mail {agent.unread_messages}</span> : null}
        {agent.last_sent_at ? <span className="sent-pulse">Sent</span> : null}
      </div>
      <ContextBar value={agent.context_pct} />
      {preview && <p className="agent-preview">{preview}</p>}
      {!agent.running && <small className="stopped-label">stopped</small>}
    </article>
  );
}
