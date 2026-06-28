import { useState } from "react";
import { cancelTurn, sendPrompt } from "../../api/client";
import { useTranscriptStore } from "../../store/transcriptStore";

export function Composer({ agentId, busy }: { agentId: string; busy: boolean }) {
  const [text, setText] = useState("");
  const append = useTranscriptStore((state) => state.appendMessage);
  const submit = async () => {
    const trimmed = text.trim();
    if (!trimmed) return;
    append(agentId, { kind: "user_text", text: trimmed, message_id: `local-${Date.now()}` });
    setText("");
    await sendPrompt(agentId, trimmed);
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
    </form>
  );
}
