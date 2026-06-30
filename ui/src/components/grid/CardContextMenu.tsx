import { useEffect } from "react";
import { createPortal } from "react-dom";
import { useNavigate } from "react-router-dom";
import { renameAgent, stopAgent, switchRuntime, updateAgentIdentity } from "../../api/client";
import { useAgentStore } from "../../store/agentStore";
import { useUiStore } from "../../store/uiStore";

export function CardContextMenu() {
  const menu = useUiStore((state) => state.contextMenu);
  const close = useUiStore((state) => state.closeContextMenu);
  const pushError = useUiStore((state) => state.pushError);
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
      <button
        type="button"
        disabled={!agent.running}
        title={agent.running ? "Switch interface/backend/model" : "Agent must be running"}
        onClick={() => {
          const iface = window.prompt("Interface (chat or terminal)", agent.interface) || agent.interface;
          const backend = window.prompt("Backend", agent.backend) || agent.backend;
          const model = window.prompt("Model", agent.model) || agent.model;
          switchRuntime(agent.agent_id, { interface: iface, backend, model }).catch((err) =>
            pushError("Switch runtime failed", err instanceof Error ? err.message : String(err)),
          );
          close();
        }}
      >
        Switch runtime
      </button>
      <button type="button" disabled title="Available in Phase 3">
        Clone
      </button>
      <button
        type="button"
        onClick={() => {
          const group = window.prompt("Move to group (blank removes group)", agent.group ?? "");
          if (group !== null)
            updateAgentIdentity(agent.agent_id, { group }).catch((err) =>
              pushError("Move to group failed", err instanceof Error ? err.message : String(err)),
            );
          close();
        }}
      >
        Move to group
      </button>
      {agent.interface === "terminal" && (
        <button type="button" onClick={() => { navigate(`/agent/${agent.agent_id}?tab=terminal`); close(); }}>
          Reveal terminal
        </button>
      )}
    </div>,
    document.body,
  );
}
