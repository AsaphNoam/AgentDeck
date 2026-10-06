import { AutoGrowTextarea } from "../ui";
import { useEffect, useRef, useState } from "react";
import { cancelTurn, sendPrompt, steerPrompt } from "../../api/client";
import { withdrawHeldMessage } from "../../lib/heldMessage";
import { useHeldStore } from "../../store/heldStore";
import { useTranscriptStore } from "../../store/transcriptStore";
import { chatDraftRevision, discardChatDraft, getChatDraft, setChatDraft } from "./drafts";
import { useAutocomplete } from "./autocomplete";

// Composer sends to one agent and owns its `@` file / `#` command picker through
// the shared autocomplete hook (FS-03.R30–R34).

export function Composer({ agentId, busy, running = true, steerable = false, variant }: {
  agentId: string;
  busy: boolean;
  running?: boolean;
  steerable?: boolean;
  variant?: "dashboard";
}) {
  const [text, setText] = useState(() => getChatDraft(agentId));
  const [error, setError] = useState<string | null>(null);
  // Steer's outcome is best-effort by nature and the two outcomes mean different
  // things, so it is reported rather than left to be inferred (FS-03.R50).
  const [notice, setNotice] = useState<string | null>(null);
  const [steering, setSteering] = useState(false);
  const held = useHeldStore((state) => state.byAgent[agentId]);
  const hold = useHeldStore((state) => state.hold);
  const releaseHeld = useHeldStore((state) => state.release);
  const append = useTranscriptStore((state) => state.appendMessage);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const currentAgentId = useRef(agentId);
  const steeringAgents = useRef(new Set<string>());
  currentAgentId.current = agentId;
  const autocomplete = useAutocomplete({
    sourceId: agentId,
    text,
    textareaRef,
    onText: (next) => {
      setText(next);
      setChatDraft(agentId, next);
    },
  });

  useEffect(() => {
    setText(getChatDraft(agentId));
    setError(null);
    setNotice(null);
    autocomplete.reset();
    setSteering(steeringAgents.current.has(agentId));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [agentId]);

  // Stop, crash, or archive leaves no next turn for a held message, so it comes
  // back as the person's own text — but only into an empty composer, never over
  // text typed since, which is R36's newer-draft rule applied to the same store
  // (FS-03.R49, TS-08.R56).
  useEffect(() => {
    if (running || !held) return;
    if (text === "") {
      setText(held);
      setChatDraft(agentId, held);
    }
    releaseHeld(agentId);
    // `text` is read, not depended on: this runs on the lifecycle boundary, and
    // re-running it on every keystroke would race the person's own typing.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [agentId, running, held, releaseHeld]);

  const submit = async () => {
    // Reject whitespace-only drafts (R19) but send the text exactly as displayed:
    // a picker insertion's trailing space is part of the prompt and must reach both
    // the optimistic event and POST /prompt verbatim (FS-03.R31/R33, INV §1).
    const prompt = text;
    if (!prompt.trim()) return;
    setError(null);
    setNotice(null);
    // A busy agent queues the message, and a queued message is not in the
    // transcript until the server actually sends it — so there is nothing to
    // echo yet, and the pending tail carries it instead (FS-03.R48, TS-08.R56).
    // Capture the draft generation this send owns. If the person edits the draft
    // for this agent while the request is in flight, the completion must leave
    // that newer draft alone instead of discarding or overwriting it (INV §1/§5).
    const draftRev = chatDraftRevision(agentId);
    setText("");
    autocomplete.clearTrigger();
    try {
      // A stopped chat agent is woken by this same request (FS-03.R35), so the
      // composer submits normally and only the reply takes longer to start.
      const result = await sendPrompt(agentId, prompt);
      // The server's answer decides whether this ran or is waiting, so the
      // pending state is rendered from what happened rather than inferred from
      // agent status (TS-03.R38).
      if (result.delivery === "held") hold(agentId, prompt, result.after_seq);
      else append(agentId, { kind: "user_text", text: prompt, message_id: `local-${Date.now()}` });
      // Only clear the draft we actually sent; a newer same-agent draft survives.
      if (chatDraftRevision(agentId) === draftRev) discardChatDraft(agentId);
    } catch (err) {
      // A newer same-agent draft typed while the request was pending wins: leave
      // it and its live composer untouched rather than restoring the sent text.
      if (chatDraftRevision(agentId) !== draftRev) return;
      // Surface the server's own typed error — a rejected wake included — and
      // restore the draft so the user can retry; the optimistic bubble stays,
      // but the error makes clear it was not delivered.
      const reason = err instanceof Error && err.message ? err.message : "the agent may have stopped";
      setChatDraft(agentId, prompt);
      // The request belongs to the chat that started it. Do not overwrite a
      // different chat the person opened while this request was in flight.
      if (currentAgentId.current === agentId) {
        setError(`Failed to send — ${reason}. Your message was restored.`);
        setText(prompt);
      }
    }
  };

  // Steer delivers into the turn that is running instead of queueing behind it.
  // With the composer empty and a message already held, it delivers that one, so
  // a person who queued and then changed their mind about waiting does not
  // retype it (FS-03.R50).
  const steer = async () => {
    const typed = text.trim() ? text : "";
    if (!typed && !held) return;
    if (steeringAgents.current.has(agentId)) return;
    const steeringAgentId = agentId;
    steeringAgents.current.add(steeringAgentId);
    setSteering(true);
    setError(null);
    setNotice(null);
    try {
      const result = await steerPrompt(steeringAgentId, typed);
      if (typed) {
        setText("");
        autocomplete.clearTrigger();
        discardChatDraft(agentId);
      } else {
        releaseHeld(agentId);
      }
      setNotice(result.outcome === "steered"
        ? "Delivered into the running turn."
        : "That turn had already ended — sent as a new turn.");
    } catch (err) {
      // A refused message is not downgraded to a queue and not discarded: the
      // composer keeps exactly what the person typed, with the reason (INV §8).
      const reason = err instanceof Error && err.message ? err.message : "the agent may have stopped";
      setError(`Could not steer — ${reason}.`);
    } finally {
      steeringAgents.current.delete(steeringAgentId);
      if (currentAgentId.current === steeringAgentId) setSteering(false);
    }
  };

  const withdraw = async () => {
    setError(null);
    setNotice(null);
    try {
      await withdrawHeldMessage(agentId);
    } catch (err) {
      const reason = err instanceof Error && err.message ? err.message : "the agent may have stopped";
      setError(`Could not withdraw — ${reason}. It may already have been sent.`);
    }
  };

  const onKeyDown = (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (autocomplete.onKeyDown(event)) return;
    // Picker closed (or showing an empty state): Enter submits, Shift+Enter newlines.
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      void submit();
    }
  };

  return (
    <form className="composer" data-ui="composer" data-variant={variant} onSubmit={(event) => { event.preventDefault(); void submit(); }}>
      <div className="composer-input">
        <AutoGrowTextarea
          ref={textareaRef}
          maxHeight="40vh"
          value={text}
          onChange={(event) => {
            const next = event.target.value;
            setText(next);
            setChatDraft(agentId, next);
            autocomplete.syncTrigger(event.target);
          }}
          onKeyUp={(event) => autocomplete.syncTrigger(event.currentTarget)}
          onClick={(event) => autocomplete.syncTrigger(event.currentTarget)}
          onKeyDown={onKeyDown}
        />
        {autocomplete.picker}
      </div>
      {/* Send is always present and is the safe default; Steer appears only where
          the live session advertises it, and never as a disabled control where it
          does not (FS-03.R50, FS-09.R26). */}
      <div className="composer-actions">
        <button type="submit">Send</button>
        {busy && steerable && (
          <button type="button" className="composer-steer" disabled={steering} onClick={() => void steer()}>Steer</button>
        )}
        {held && (
          <button type="button" className="composer-withdraw" onClick={() => void withdraw()}>Withdraw queued</button>
        )}
        {busy && (
          <button
            type="button"
            className="composer-cancel"
            onClick={() => {
              setError(null);
              setNotice(null);
              cancelTurn(agentId).catch(() => setError("Failed to cancel — the turn may have already finished."));
            }}
          >
            Cancel
          </button>
        )}
      </div>
      {notice && <p className="composer-notice" role="status">{notice}</p>}
      {error && <p className="composer-error">{error}</p>}
    </form>
  );
}
