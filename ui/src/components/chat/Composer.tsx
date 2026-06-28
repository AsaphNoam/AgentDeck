import { useState } from "react";
import { cancelTurn, sendPrompt } from "../../api/client";
import { useTranscriptStore } from "../../store/transcriptStore";

export function Composer({ agentId, busy }: { agentId: string; busy: boolean }) {
  const [text, setText] = useState("");
  const [error, setError] = useState<string | null>(null);
  const append = useTranscriptStore((state) => state.appendMessage);
  const submit = async () => {
    const trimmed = text.trim();
    if (!trimmed) return;
    setError(null);
    append(agentId, { kind: "user_text", text: trimmed, message_id: `local-${Date.now()}` });
    setText("");
    try {
      await sendPrompt(agentId, trimmed);
    } catch {
      // Surface the failure and restore the draft so the user can retry; the
      // optimistic bubble stays, but the error makes clear it was not delivered.
      setError("Failed to send — the agent may have stopped. Your message was restored.");
      setText(trimmed);
    }
  };
  return (
    <form className="composer" onSubmit={(event) => { event.preventDefault(); void submit(); }}>
      <textarea
        value={text}
        onChange={(event) => setText(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === "Enter" && !event.shiftKey) {
            event.preventDefault();
            void submit();
          }
        }}
      />
      {busy ? (
        <button type="button" onClick={() => void cancelTurn(agentId)}>Cancel</button>
      ) : (
        <button type="submit">Send</button>
      )}
      {error && <p className="composer-error">{error}</p>}
    </form>
  );
}
