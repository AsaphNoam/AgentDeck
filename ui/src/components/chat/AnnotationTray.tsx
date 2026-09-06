import { useState } from "react";
import { sendAnnotations } from "../../api/client";
import type { AnnotationDraft } from "../../api/types";
import { useAgentStore } from "../../store/agentStore";
import { useAnnotationStore } from "../../store/annotationStore";
import { NewAgentModal } from "../../features/launch/NewAgentModal";

export function AnnotationTray({ sourceId, sourceActive }: { sourceId: string; sourceActive: boolean }) {
  const drafts = useAnnotationStore((state) => state.bySource[sourceId] ?? []);
  const overall = useAnnotationStore((state) => state.overallBySource[sourceId] ?? "");
  const updateInstruction = useAnnotationStore((state) => state.updateInstruction);
  const remove = useAnnotationStore((state) => state.remove);
  const discard = useAnnotationStore((state) => state.discard);
  const setOverall = useAnnotationStore((state) => state.setOverall);
  const collapsed = useAnnotationStore((state) => state.collapsedBySource[sourceId] ?? false);
  const setCollapsed = useAnnotationStore((state) => state.setCollapsed);
  const agents = useAgentStore((state) => state.agents);
  const source = agents[sourceId];
  const recipients = Object.values(agents).filter((agent) => agent.agent_id !== sourceId && agent.running && agent.interface === "chat");
  const [target, setTarget] = useState<"self" | "agent" | "new">("self");
  const [recipientId, setRecipientId] = useState("");
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showLaunch, setShowLaunch] = useState(false);

  if (drafts.length === 0) return null;

  const batch = (agentId?: string) => ({
    annotations: drafts,
    overall_instruction: overall || undefined,
    target: agentId ? { kind: "agent" as const, agent_id: agentId } : { kind: "self" as const },
  });

  const send = async () => {
    setError(null);
    if (target === "new") {
      if (!source) setError("The source agent is no longer available for a prefilled launch.");
      else setShowLaunch(true);
      return;
    }
    const agentId = target === "agent" ? recipientId : undefined;
    if (target === "agent" && !agentId) {
      setError("Choose a running chat agent.");
      return;
    }
    setSending(true);
    try {
      await sendAnnotations(sourceId, batch(agentId));
      discard(sourceId);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Sending annotations failed.");
    } finally {
      setSending(false);
    }
  };

  const onLaunched = (agentId: string) => {
    setShowLaunch(false);
    setSending(true);
    setError(null);
    void sendAnnotations(sourceId, batch(agentId))
      .then(() => discard(sourceId))
      .catch((err: unknown) => setError(err instanceof Error ? err.message : "The new agent launched, but annotation delivery failed."))
      .finally(() => setSending(false));
  };

  return (
    // Docked column or floating overlay is decided by the transcript region's own
    // width in CSS, so this renders one tray in one shape either way. The collapse
    // control and the collapsed strip are likewise CSS states of that same markup:
    // the overlay hides the control, so narrowing the window can never strand a
    // person with a collapsed tray and no way to open it (FS-13.R20–R21).
    <aside className="annotation-tray" data-ui="annotation-tray" data-state={collapsed ? "collapsed" : "expanded"} aria-label="Pending annotations">
      <header className="annotation-tray-header">
        <div><strong className="annotation-tray-title">Pending annotations</strong><span>{drafts.length}/20</span></div>
        <div>
          <button
            type="button"
            className="annotation-tray-collapse"
            aria-expanded={!collapsed}
            aria-label={collapsed ? "Expand pending annotations" : "Collapse pending annotations"}
            onClick={() => setCollapsed(sourceId, !collapsed)}
          >
            {collapsed ? "‹" : "›"}
          </button>
          <button type="button" className="annotation-link annotation-tray-discard" onClick={() => discard(sourceId)} disabled={sending}>Discard all</button>
        </div>
      </header>
      <div className="annotation-tray-body">
        <ol className="annotation-drafts">
          {drafts.map((draft, index) => <AnnotationDraftRow key={`${draft.seq}-${index}`} draft={draft} index={index} sourceId={sourceId} onRemove={remove} onUpdate={updateInstruction} disabled={sending} />)}
        </ol>
        <label className="annotation-overall">
          Overall instruction (optional)
          <textarea value={overall} maxLength={2000} onChange={(event) => setOverall(sourceId, event.target.value)} disabled={sending} />
        </label>
      </div>
      <footer className="annotation-tray-footer">
        <div className="annotation-targets">
          <label><input type="radio" checked={target === "self"} disabled={!sourceActive || sending} onChange={() => setTarget("self")} /> Current agent</label>
          <label><input type="radio" checked={target === "agent"} disabled={sending} onChange={() => setTarget("agent")} /> Another agent</label>
          <label><input type="radio" checked={target === "new"} disabled={!source || sending} onChange={() => setTarget("new")} /> New task</label>
          {target === "agent" && (
            <select value={recipientId} onChange={(event) => setRecipientId(event.target.value)} disabled={sending} aria-label="Annotation recipient">
              <option value="">Choose an agent</option>
              {recipients.map((agent) => <option key={agent.agent_id} value={agent.agent_id}>{agent.name} ({agent.role}@{agent.project})</option>)}
            </select>
          )}
        </div>
        {error && <p className="annotation-error">{error}</p>}
        <button type="button" className="annotation-send" onClick={() => void send()} disabled={sending}>{sending ? "Sending…" : target === "new" ? "Continue to launch" : "Send annotations"}</button>
      </footer>
      <NewAgentModal open={showLaunch} onClose={() => setShowLaunch(false)} initialRole={source?.role} initialProject={source?.project} onLaunched={onLaunched} />
    </aside>
  );
}

function AnnotationDraftRow({ draft, index, sourceId, onRemove, onUpdate, disabled }: {
  draft: AnnotationDraft;
  index: number;
  sourceId: string;
  onRemove: (sourceId: string, index: number) => void;
  onUpdate: (sourceId: string, index: number, instruction: string) => void;
  disabled: boolean;
}) {
  const anchor = draft.path ? `${draft.path}:${draft.start_line}${draft.end_line && draft.end_line !== draft.start_line ? `–${draft.end_line}` : ""}` : `Event ${draft.seq}`;
  return (
    <li className="annotation-draft">
      {/* The anchor is what the reader scans for, so it is the row's heading
          rather than bold text sharing a line with a control (FS-13.R22). */}
      <div className="annotation-draft-head"><h3 className="annotation-draft-anchor">{anchor}</h3><button type="button" className="annotation-link" onClick={() => onRemove(sourceId, index)} disabled={disabled}>Remove</button></div>
      <blockquote>{draft.excerpt}</blockquote>
      <label>Instruction<textarea value={draft.instruction} maxLength={2000} onChange={(event) => onUpdate(sourceId, index, event.target.value)} disabled={disabled} /></label>
    </li>
  );
}
