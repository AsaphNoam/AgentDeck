import { AutoGrowTextarea } from "../../components/ui";
import { useState } from "react";
import { newCommandID, useThinkTankAnnotations, type RoomAnnotationWire } from "../../api/thinkTanks";
import type { AnnotationDraft } from "../../api/types";
import { useAgentStore } from "../../store/agentStore";
import { useAnnotationStore } from "../../store/annotationStore";
import { NewAgentModal } from "../launch/NewAgentModal";
import type { ThinkTankDetail } from "../../schemas/thinkTank";
import { roomAnnotationSource } from "./roomText";

/** RoomAnnotationTray is the familiar tray with the room's three destinations:
 *  Room (shared input between turns), a selected agent, or New task through the
 *  ordinary New Agent flow (FS-21.R30, FS-13.R27). Drafts stay in the bounded
 *  browser tray until a send succeeds. */
export function RoomAnnotationTray({ room }: { room: ThinkTankDetail }) {
  const sourceId = roomAnnotationSource(room.room_id);
  const drafts = useAnnotationStore((state) => state.bySource[sourceId] ?? []);
  const overall = useAnnotationStore((state) => state.overallBySource[sourceId] ?? "");
  const updateInstruction = useAnnotationStore((state) => state.updateInstruction);
  const remove = useAnnotationStore((state) => state.remove);
  const discard = useAnnotationStore((state) => state.discard);
  const setOverall = useAnnotationStore((state) => state.setOverall);
  const agents = useAgentStore((state) => state.agents);
  const ended = room.phase === "ended";
  const [target, setTarget] = useState<"room" | "agent" | "new">(ended ? "agent" : "room");
  const [recipientId, setRecipientId] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [showLaunch, setShowLaunch] = useState(false);
  const [commandID, setCommandID] = useState(newCommandID);
  const send = useThinkTankAnnotations(room.room_id);

  if (drafts.length === 0) return null;
  const recipients = Object.values(agents).filter((agent) => agent.running && agent.interface === "chat");
  const effectiveTarget = ended && target === "room" ? "agent" : target;

  const deliver = (agentId?: string) => {
    setError(null);
    send.mutate({
      command_id: commandID,
      annotations: drafts.map(toWire),
      overall_instruction: overall || undefined,
      target: agentId ? { kind: "agent", agent_id: agentId } : { kind: "room" },
    }, {
      onSuccess: () => {
        discard(sourceId);
        setCommandID(newCommandID());
      },
      onError: (err) => setError(err instanceof Error ? err.message : "Sending annotations failed."),
    });
  };

  const submit = () => {
    if (effectiveTarget === "new") {
      setShowLaunch(true);
      return;
    }
    if (effectiveTarget === "agent") {
      if (!recipientId) {
        setError("Choose a running chat agent.");
        return;
      }
      deliver(recipientId);
      return;
    }
    deliver();
  };

  return (
    <aside className="annotation-tray" data-ui="annotation-tray" data-state="expanded" aria-label="Pending room annotations">
      <header className="annotation-tray-header">
        <div><strong className="annotation-tray-title">Pending annotations</strong><span>{drafts.length}/20</span></div>
        <div>
          <button type="button" className="annotation-tray-discard ad-button-danger" onClick={() => discard(sourceId)} disabled={send.isPending}>Discard all</button>
        </div>
      </header>
      <div className="annotation-tray-body">
        <ol className="annotation-drafts">
          {drafts.map((draft, index) => (
            <li className="annotation-draft" key={`${draft.room_anchor}-${draft.seq ?? draft.path}-${index}`}>
              <div className="annotation-draft-head">
                <h3 className="annotation-draft-anchor">{roomAnchorLabel(draft)}</h3>
                <button type="button" className="ad-button-danger" onClick={() => remove(sourceId, index)} disabled={send.isPending}>Remove</button>
              </div>
              <blockquote>{draft.excerpt}</blockquote>
              <label>Instruction<AutoGrowTextarea value={draft.instruction} maxLength={2000} onChange={(event) => updateInstruction(sourceId, index, event.target.value)} disabled={send.isPending} /></label>
            </li>
          ))}
        </ol>
        <label className="annotation-overall">
          Overall instruction (optional)
          <AutoGrowTextarea value={overall} maxLength={2000} onChange={(event) => setOverall(sourceId, event.target.value)} disabled={send.isPending} />
        </label>
      </div>
      <footer className="annotation-tray-footer">
        <div className="annotation-targets">
          <label title={ended ? "The discussion has ended, so the room takes no new input." : "Shared with every participant between turns"}>
            <input type="radio" checked={effectiveTarget === "room"} disabled={ended || send.isPending} onChange={() => setTarget("room")} /> Room
          </label>
          <label><input type="radio" checked={effectiveTarget === "agent"} disabled={send.isPending} onChange={() => setTarget("agent")} /> An agent</label>
          <label><input type="radio" checked={effectiveTarget === "new"} disabled={send.isPending} onChange={() => setTarget("new")} /> New task</label>
          {effectiveTarget === "agent" && (
            <select value={recipientId} onChange={(event) => setRecipientId(event.target.value)} disabled={send.isPending} aria-label="Annotation recipient">
              <option value="">Choose an agent</option>
              {recipients.map((agent) => <option key={agent.agent_id} value={agent.agent_id}>{agent.name} ({agent.role}@{agent.project})</option>)}
            </select>
          )}
        </div>
        {error && <p className="annotation-error" role="alert">{error}</p>}
        <button type="button" className="annotation-send ad-button-primary" onClick={submit} disabled={send.isPending}>
          {send.isPending ? "Sending…" : effectiveTarget === "new" ? "Continue to launch" : effectiveTarget === "room" ? "Send to room" : "Send annotations"}
        </button>
      </footer>
      <NewAgentModal
        open={showLaunch}
        groupPicker={false}
        onClose={() => setShowLaunch(false)}
        initialProject={room.origin_project}
        onLaunched={(agentId) => deliver(agentId)}
      />
    </aside>
  );
}

function toWire(draft: AnnotationDraft): RoomAnnotationWire {
  return {
    anchor: draft.room_anchor ?? "entry",
    seq: draft.seq,
    source_id: draft.source_id,
    path: draft.path,
    side: draft.side,
    start_line: draft.start_line,
    end_line: draft.end_line,
    excerpt: draft.excerpt,
    instruction: draft.instruction,
  };
}

export function roomAnchorLabel(draft: AnnotationDraft): string {
  if (draft.room_anchor === "file") {
    const lines = draft.start_line ? `:${draft.start_line}${draft.end_line && draft.end_line !== draft.start_line ? `–${draft.end_line}` : ""}` : "";
    return `File ${draft.path}${lines}`;
  }
  if (draft.room_anchor === "activity") return draft.path ? `${draft.path} (activity ${draft.seq})` : `Activity ${draft.seq}`;
  return `Room entry ${draft.seq}`;
}
