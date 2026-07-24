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
    <aside className="annotation-tray" aria-label="Pending annotations">
      <header className="annotation-tray-header">
        <div><strong>Pending annotations</strong><span>{drafts.length}/20</span></div>
        <button type="button" className="annotation-link" onClick={() => discard(sourceId)} disabled={sending}>Discard all</button>
      </header>
      <ol className="annotation-drafts">
        {drafts.map((draft, index) => <AnnotationDraftRow key={`${draft.seq}-${index}`} draft={draft} index={index} sourceId={sourceId} onRemove={remove} onUpdate={updateInstruction} disabled={sending} />)}
      </ol>
      <label className="annotation-overall">
        Overall instruction (optional)
        <textarea value={overall} maxLength={2000} onChange={(event) => setOverall(sourceId, event.target.value)} disabled={sending} />
      </label>
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
      <div className="annotation-draft-head"><strong>{anchor}</strong><button type="button" className="annotation-link" onClick={() => onRemove(sourceId, index)} disabled={disabled}>Remove</button></div>
      <blockquote>{draft.excerpt}</blockquote>
      <label>Instruction<textarea value={draft.instruction} maxLength={2000} onChange={(event) => onUpdate(sourceId, index, event.target.value)} disabled={disabled} /></label>
    </li>
  );
}
