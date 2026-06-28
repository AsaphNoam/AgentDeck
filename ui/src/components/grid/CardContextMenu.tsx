import { useEffect } from "react";
import { createPortal } from "react-dom";
import { useNavigate } from "react-router-dom";
import { renameAgent, stopAgent } from "../../api/client";
import { useAgentStore } from "../../store/agentStore";
import { useUiStore } from "../../store/uiStore";

export function CardContextMenu() {
  const menu = useUiStore((state) => state.contextMenu);
  const close = useUiStore((state) => state.closeContextMenu);
  const agent = useAgentStore((state) => (menu ? state.agents[menu.agentId] : null));
  const navigate = useNavigate();

  useEffect(() => {
    if (!menu) return;
    const onPointerDown = (event: MouseEvent) => {
      if (!(event.target as HTMLElement)?.closest(".context-menu")) close();
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") close();
    };
    window.addEventListener("mousedown", onPointerDown);
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("mousedown", onPointerDown);
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [menu, close]);

  if (!menu || !agent) return null;

  return createPortal(
    <div className="context-menu" style={{ left: menu.x, top: menu.y }} role="menu">
      <button type="button" onClick={() => { navigate(`/agent/${agent.agent_id}`); close(); }}>
        Open chat
      </button>
      <button
        type="button"
        onClick={() => {
          const name = window.prompt("Rename agent", agent.name);
          if (name) void renameAgent(agent.agent_id, name);
          close();
        }}
      >
        Rename
      </button>
      <button
        type="button"
        disabled={!agent.running}
        onClick={() => {
          if (window.confirm(`Stop ${agent.name}?`)) void stopAgent(agent.agent_id);
          close();
        }}
      >
        Stop
      </button>
      <hr />
      <button type="button" disabled title="Available in Phase 6">
        Switch runtime
      </button>
      <button type="button" disabled title="Available in Phase 3">
        Clone
      </button>
      <button type="button" disabled title="Available in Phase 6">
        Move to group
      </button>
    </div>,
    document.body,
  );
}
