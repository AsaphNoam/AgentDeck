import { useEffect, useMemo, useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { createPortal } from "react-dom";
import { useNavigate } from "react-router-dom";
import { archiveAgent, getCapabilities, launchAgent, renameAgent, resumeAgent, stopAgent, switchRuntime, updateAgentIdentity } from "../../api/client";
import { useBackends } from "../../api/config";
import type { AgentState } from "../../api/types";
import { terminalSupported } from "../../lib/backendTypes";
import { useAgentStore } from "../../store/agentStore";
import { useUiStore } from "../../store/uiStore";
import { ConfirmDialog } from "../ui";

type DialogKind = "rename" | "stop" | "switch" | "group";
type ActiveDialog = { kind: DialogKind; agent: AgentState };

export function CardContextMenu() {
  const menu = useUiStore((state) => state.contextMenu);
  const close = useUiStore((state) => state.closeContextMenu);
  const pushError = useUiStore((state) => state.pushError);
  const agents = useAgentStore((state) => state.agents);
  const agent = menu ? agents[menu.agentId] : null;
  const navigate = useNavigate();
  const { data: backends } = useBackends();
  const [dialog, setDialog] = useState<ActiveDialog | null>(null);
  const [name, setName] = useState("");
  const [group, setGroup] = useState("");
  const [runtime, setRuntime] = useState({ interface: "chat", backend: "", model: "", effort: "" });
  const [dialogError, setDialogError] = useState("");
  const [terminalAvailable, setTerminalAvailable] = useState(true);

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

  useEffect(() => {
    if (dialog?.kind !== "switch") return;
    void getCapabilities().then((caps) => setTerminalAvailable(caps.terminal.available)).catch(() => setTerminalAvailable(false));
  }, [dialog?.kind]);

  useEffect(() => {
    if (!dialog) return;
    setDialogError("");
    setName(dialog.agent.name);
    setGroup(dialog.agent.group ?? "");
    setRuntime({ interface: dialog.agent.interface, backend: dialog.agent.backend, model: dialog.agent.model, effort: dialog.agent.effort ?? "" });
  }, [dialog]);

  const groupLabels = useMemo(
    () => [...new Set(Object.values(agents).map((item) => item.group?.trim()).filter((label): label is string => !!label))].sort(),
    [agents],
  );
  const selectedBackend = backends?.backends[runtime.backend];
  const modelEntries = Object.entries(selectedBackend?.models ?? {});
  const terminalOK = terminalAvailable && !!selectedBackend && terminalSupported(selectedBackend.type);

  useEffect(() => {
    if (runtime.interface === "terminal" && selectedBackend && !terminalOK) setRuntime((current) => ({ ...current, interface: "chat" }));
  }, [runtime.interface, selectedBackend, terminalOK]);

  const openDialog = (kind: DialogKind) => {
    if (!agent) return;
    setDialog({ kind, agent });
    close();
  };
  const closeDialog = () => {
    setDialog(null);
    setDialogError("");
  };
  const reportError = (title: string, err: unknown) => {
    const message = err instanceof Error ? err.message : String(err);
    setDialogError(message);
    pushError(title, message);
  };

  const submitRename = () => {
    if (!dialog || !name.trim()) {
      setDialogError("Name is required.");
      return;
    }
    renameAgent(dialog.agent.agent_id, name.trim()).then(closeDialog).catch((err) => reportError("Rename failed", err));
  };
  const submitMoveGroup = () => {
    if (!dialog) return;
    updateAgentIdentity(dialog.agent.agent_id, { group: group.trim() }).then(closeDialog).catch((err) => reportError("Move to group failed", err));
  };
  const submitSwitch = () => {
    if (!dialog) return;
    if (runtime.interface === dialog.agent.interface && runtime.backend === dialog.agent.backend && runtime.model === dialog.agent.model && runtime.effort === (dialog.agent.effort ?? "")) {
		setDialogError("Choose an interface, backend, model, or effort that differs from the current runtime.");
      return;
    }
    switchRuntime(dialog.agent.agent_id, runtime).then(closeDialog).catch((err) => reportError("Switch runtime failed", err));
  };

  const menuView = menu && agent ? createPortal(
    <div className="context-menu" data-ui="context-menu" style={{ left: menu.x, top: menu.y }} role="menu">
      <button type="button" data-slot="item" onClick={() => { navigate(`/agent/${agent.agent_id}`); close(); }}>
        Open chat
      </button>
      <button type="button" data-slot="item" onClick={() => openDialog("rename")}>Rename</button>
      <button type="button" data-slot="item" disabled={!agent.running} onClick={() => openDialog("stop")}>Stop</button>
      {!agent.running && (
        <button type="button" data-slot="item" onClick={() => {
          resumeAgent(agent.agent_id).catch((err) => pushError("Resume failed", err instanceof Error ? err.message : String(err)));
          close();
        }}>
          Resume
        </button>
      )}
      <hr />
      <button type="button" data-slot="item" disabled={!agent.running} title={agent.running ? "Switch interface/backend/model" : "Agent must be running"} onClick={() => openDialog("switch")}>
        Switch runtime
      </button>
      <button type="button" data-slot="item" title="Launch a new agent with this one's role, project, backend, and model" onClick={() => {
        launchAgent({ role: agent.role, project: agent.project, backend: agent.backend, model: agent.model, effort: agent.effort, interface: agent.interface, group: agent.group }).catch((err) => pushError("Clone failed", err instanceof Error ? err.message : String(err)));
        close();
      }}>
        Clone
      </button>
      <button type="button" data-slot="item" onClick={() => openDialog("group")}>Move to group</button>
      <button type="button" data-slot="item" onClick={() => {
        archiveAgent(agent.agent_id).catch((err) => pushError("Archive failed", err instanceof Error ? err.message : String(err)));
        close();
      }}>
        Archive
      </button>
      {agent.interface === "terminal" && (
        <button type="button" data-slot="item" onClick={() => { navigate(`/agent/${agent.agent_id}?tab=terminal`); close(); }}>
          Reveal terminal
        </button>
      )}
    </div>,
    document.body,
  ) : null;

  return (
    <>
      {menuView}
      {dialog?.kind === "stop" && (
        <ConfirmDialog
          open
          title={`Stop ${dialog.agent.name}?`}
          confirmLabel="Stop agent"
          destructive
          onCancel={closeDialog}
          onConfirm={() => stopAgent(dialog.agent.agent_id).then(closeDialog).catch((err) => reportError("Stop failed", err))}
        >
          <p>This stops the running agent. Its session remains available to resume later.</p>
          {dialogError && <p className="form-error">{dialogError}</p>}
        </ConfirmDialog>
      )}
      {dialog && dialog.kind !== "stop" && (
        <Dialog.Root open onOpenChange={(open) => { if (!open) closeDialog(); }}>
          <Dialog.Portal>
            <Dialog.Overlay className="dialog-overlay" data-ui="dialog" data-slot="overlay" />
            <Dialog.Content className="dialog-content" data-ui="dialog" data-slot="content" data-variant="default">
              <Dialog.Title data-slot="title">
                {dialog.kind === "rename" ? "Rename agent" : dialog.kind === "switch" ? "Switch runtime" : "Move to group"}
              </Dialog.Title>
              {dialog.kind === "rename" && (
                <form className="config-form" onSubmit={(event) => { event.preventDefault(); submitRename(); }}>
                  <div className="form-field"><label htmlFor="agent-name">Name</label><input id="agent-name" value={name} onChange={(event) => setName(event.target.value)} /></div>
                  {dialogError && <p className="form-error">{dialogError}</p>}
                  <div className="form-actions" data-slot="actions"><button type="button" onClick={closeDialog}>Cancel</button><button type="submit">Rename</button></div>
                </form>
              )}
              {dialog.kind === "group" && (
                <form className="config-form" onSubmit={(event) => { event.preventDefault(); submitMoveGroup(); }}>
                  <div className="form-field"><label htmlFor="agent-group">Group</label><input id="agent-group" list="agent-group-suggestions" value={group} onChange={(event) => setGroup(event.target.value)} /><datalist id="agent-group-suggestions">{groupLabels.map((label) => <option key={label} value={label} />)}</datalist><span className="form-hint">Leave blank to remove this agent from its group.</span></div>
                  {dialogError && <p className="form-error">{dialogError}</p>}
                  <div className="form-actions" data-slot="actions"><button type="button" onClick={closeDialog}>Cancel</button><button type="submit">Move</button></div>
                </form>
              )}
              {dialog.kind === "switch" && (
                <form className="config-form" onSubmit={(event) => { event.preventDefault(); submitSwitch(); }}>
                  <div className="form-field"><label htmlFor="runtime-interface">Interface</label><select id="runtime-interface" value={runtime.interface} onChange={(event) => setRuntime((current) => ({ ...current, interface: event.target.value }))}><option value="chat">Chat</option><option value="terminal" disabled={!terminalOK}>Terminal</option></select></div>
                  <div className="form-field"><label htmlFor="runtime-backend">Backend</label><select id="runtime-backend" value={runtime.backend} onChange={(event) => { const backend = event.target.value; const next = backends?.backends[backend]; const model = next?.models[next?.default_model ?? ""]; setRuntime((current) => ({ ...current, backend, model: next?.default_model ?? Object.keys(next?.models ?? {})[0] ?? "", effort: model?.default_effort ?? "" })); }}>{Object.entries(backends?.backends ?? {}).map(([id, backend]) => <option key={id} value={id}>{backend.name} ({id})</option>)}</select></div>
                  <div className="form-field"><label htmlFor="runtime-model">Model</label><select id="runtime-model" value={runtime.model} onChange={(event) => { const model = selectedBackend?.models[event.target.value]; setRuntime((current) => ({ ...current, model: event.target.value, effort: model?.default_effort ?? "" })); }}>{modelEntries.map(([id, model]) => <option key={id} value={id}>{model.name} ({id})</option>)}</select></div>
                  {(selectedBackend?.models[runtime.model]?.efforts ?? []).length > 0 && <div className="form-field"><label htmlFor="runtime-effort">Effort</label><select id="runtime-effort" value={runtime.effort} onChange={(event) => setRuntime((current) => ({ ...current, effort: event.target.value }))}>{(selectedBackend?.models[runtime.model]?.efforts ?? []).map((level) => <option key={level} value={level}>{level}</option>)}</select></div>}
                  {dialogError && <p className="form-error">{dialogError}</p>}
                  <div className="form-actions" data-slot="actions"><button type="button" onClick={closeDialog}>Cancel</button><button type="submit" disabled={!runtime.backend || !runtime.model}>Switch runtime</button></div>
                </form>
              )}
            </Dialog.Content>
          </Dialog.Portal>
        </Dialog.Root>
      )}
    </>
  );
}
