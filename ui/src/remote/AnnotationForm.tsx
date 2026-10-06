import { AutoGrowTextarea } from "../components/ui";
import { useEffect, useRef, useState } from "react";
import { launchAgent, sendAnnotations } from "../api/client";
import type { AgentState, AnnotationDraft } from "../api/types";
import { annotationAnchor, annotationBatch } from "../lib/annotations";
import { useAnnotationStore } from "../store/annotationStore";
import { useConnection } from "./connection";

type Target = "self" | "agent" | "new";
const noDrafts: AnnotationDraft[] = [];
const errorText = (error: unknown) => (error instanceof Error ? error.message : String(error));

/** The phone-sized FS-13 form for a chat agent's diff-line annotations
 *  (FS-20.R32). Drafts live in the same bounded browser-local tray the desktop
 *  uses and go out through the same batch request; only the presentation and
 *  the new-task launch (the source's role and project) are the phone's own. */
export function PhoneAnnotationForm({ agent }: { agent: AgentState }) {
  const sourceId = agent.agent_id;
  const drafts = useAnnotationStore((state) => state.bySource[sourceId]) ?? noDrafts;
  const overall = useAnnotationStore((state) => state.overallBySource[sourceId] ?? "");
  const updateInstruction = useAnnotationStore((state) => state.updateInstruction);
  const remove = useAnnotationStore((state) => state.remove);
  const discard = useAnnotationStore((state) => state.discard);
  const agents = useConnection((state) => state.agents);
  const offline = useConnection((state) => state.link !== "connected");
  const recipients = Object.values(agents).filter(
    (other) => other.agent_id !== sourceId && other.running && other.interface === "chat" && !other.archived,
  );
  const [target, setTarget] = useState<Target>(agent.running ? "self" : "agent");
  const [recipientId, setRecipientId] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sent, setSent] = useState(false);
  const form = useRef<HTMLElement>(null);
  const seen = useRef(drafts.length);

  // A newly captured range brings the form, and its instruction, into reach.
  useEffect(() => {
    if (drafts.length > seen.current) {
      setSent(false);
      const fields = form.current?.querySelectorAll("textarea");
      fields?.[fields.length - 1]?.focus();
    }
    seen.current = drafts.length;
  }, [drafts.length]);

  if (drafts.length === 0) return sent ? <p className="phone-meta" role="status">Annotations sent.</p> : null;

  const send = async () => {
    setBusy(true);
    setError(null);
    let launched: string | undefined;
    try {
      let recipient = target === "agent" ? recipientId : undefined;
      if (target === "new") {
        launched = (await launchAgent({ role: agent.role, project: agent.project })).agent.agent_id;
        recipient = launched;
      }
      await sendAnnotations(sourceId, annotationBatch(drafts, overall, recipient));
      discard(sourceId);
      setSent(true);
    } catch (err) {
      // A retry after a launched agent's delivery failed goes to that agent
      // rather than launching another one.
      if (launched) {
        setTarget("agent");
        setRecipientId(launched);
      }
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  };

  const ready = !offline && !busy && drafts.every((draft) => draft.instruction.trim()) && (target !== "agent" || !!recipientId);

  return (
    <section ref={form} className="phone-card phone-annotate" aria-label="Annotate and assign">
      <p className="phone-card-kicker">Annotate and assign · {drafts.length}/20</p>
      <ol className="phone-annotate-drafts">
        {drafts.map((draft, index) => (
          <li key={`${draft.seq}-${index}`} className="phone-annotate-draft">
            <div className="phone-annotate-head">
              <h3>{annotationAnchor(draft)}</h3>
              <button type="button" className="phone-link" disabled={busy} onClick={() => remove(sourceId, index)}>
                Remove
              </button>
            </div>
            <pre className="phone-pre">{draft.excerpt}</pre>
            <label className="phone-field">
              Instruction
              <AutoGrowTextarea
                rows={3}
                maxLength={2000}
                value={draft.instruction}
                disabled={busy}
                onChange={(event) => updateInstruction(sourceId, index, event.target.value)}
              />
            </label>
          </li>
        ))}
      </ol>
      <div className="phone-actions" role="radiogroup" aria-label="Send to">
        {(
          [
            ["self", "This agent"],
            ["agent", "Another agent"],
            ["new", "New task"],
          ] as const
        ).map(([value, label]) => (
          <button
            key={value}
            type="button"
            role="radio"
            aria-checked={target === value}
            className={target === value ? "phone-primary" : undefined}
            disabled={busy || (value === "self" && !agent.running)}
            onClick={() => setTarget(value)}
          >
            {label}
          </button>
        ))}
      </div>
      {target === "agent" &&
        (recipients.length > 0 || recipientId ? (
          <label className="phone-field">
            Agent
            <select value={recipientId} disabled={busy} onChange={(event) => setRecipientId(event.target.value)}>
              <option value="">Choose a running chat agent</option>
              {recipients.map((other) => (
                <option key={other.agent_id} value={other.agent_id}>
                  {other.name || `${other.role}@${other.project}`}
                </option>
              ))}
            </select>
          </label>
        ) : (
          <p className="phone-empty">No other chat agent is running.</p>
        ))}
      {target === "new" && (
        <p className="phone-meta">
          Launches a new {agent.role} agent in {agent.project}, then sends these annotations to it.
        </p>
      )}
      {error && <p className="phone-error">{error}</p>}
      <div className="phone-actions">
        <button type="button" className="phone-primary" disabled={!ready} onClick={() => void send()}>
          {busy ? "Sending…" : "Send annotations"}
        </button>
        <button type="button" disabled={busy} onClick={() => discard(sourceId)}>
          Discard
        </button>
      </div>
    </section>
  );
}
