import { useMemo, useRef, useState } from "react";
import { newCommandID, useThinkTankMessage } from "../../api/thinkTanks";
import { useAutocomplete, type MentionTarget } from "../../components/chat/autocomplete";
import { AutoGrowTextarea, VisuallyHidden } from "../../components/ui";
import { RoomIcon, SendIcon } from "../../components/ui/icons";
import type { ThinkTankDetail } from "../../schemas/thinkTank";

/** A selected mention over the draft, in UTF-16 offsets. */
type Mention = { agentId: string; start: number; end: number };

/** shiftMentions keeps mentions outside an edit, moving those after it, and
 *  drops any the edit touched: an edited mention is no longer an address
 *  (TS-14.R25). */
export function shiftMentions(mentions: Mention[], prev: string, next: string): Mention[] {
  let head = 0;
  while (head < prev.length && head < next.length && prev[head] === next[head]) head++;
  let tail = 0;
  while (tail < prev.length - head && tail < next.length - head && prev[prev.length - 1 - tail] === next[next.length - 1 - tail]) tail++;
  const editEnd = prev.length - tail;
  const delta = next.length - prev.length;
  return mentions.flatMap((m) => {
    if (m.end <= head) return [m];
    if (m.start >= editEnd) return [{ ...m, start: m.start + delta, end: m.end + delta }];
    return [];
  });
}

const utf8Length = (s: string) => new TextEncoder().encode(s).length;

/** RoomComposer is the room's shared input, built like the ordinary chat
 *  composer and anchored beneath the discussion: Enter sends, Shift+Enter
 *  adds a line, and an open picker takes its keys first. `@` offers
 *  participants to address and a participant's files; `#` their commands.
 *  A mention is shared with everyone and identifies that participant on its
 *  next room turn; it never sends privately or reorders turns
 *  (FS-21.R46, TS-08.R97). */
export function RoomComposer({ room }: { room: ThinkTankDetail }) {
  const [text, setText] = useState("");
  const [mentions, setMentions] = useState<Mention[]>([]);
  const [commandID, setCommandID] = useState(newCommandID);
  const [error, setError] = useState("");
  const send = useThinkTankMessage(room.room_id);
  const ended = room.phase === "ended";
  const held = !!room.active || room.active_attempts.length > 0 || room.phase === "openings" || room.phase === "setup";
  const sources = room.members.filter((m) => m.exists);
  const [sourceId, setSourceId] = useState(sources[0]?.agent_id ?? "");
  const selectedSource = sources.find((m) => m.agent_id === sourceId) ?? sources[0];
  const sourceName = selectedSource?.name ?? "";
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Live participants may be addressed; duplicate names carry their project.
  const targets: MentionTarget[] = useMemo(() => {
    const live = room.members.filter((m) => m.role === "participant" && m.state !== "departed" && m.exists);
    return live.map((m) => ({
    agentId: m.agent_id,
    label: live.filter((o) => o.name === m.name).length > 1
      ? `${m.name} (${m.project}${live.filter((o) => o.name === m.name && o.project === m.project).length > 1 ? ` · ${m.agent_id}` : ""})`
      : m.name,
    note: m.completed >= m.limit ? "No turns left" : `${m.limit - m.completed} turns left`,
    }));
  }, [room.members]);

  const changeText = (next: string) => {
    setMentions((current) => shiftMentions(current, text, next));
    setText(next);
    setCommandID(newCommandID());
  };
  const autocomplete = useAutocomplete({
    sourceId: selectedSource?.agent_id ?? "",
    text,
    textareaRef,
    onText: changeText,
    qualify: (item) => `${item.insert.trimEnd()} (${sourceName}) `,
    mentions: ended ? undefined : targets,
    fileGroup: `Files from ${sourceName}`,
    onMention: (agentId, start, end) => setMentions((current) => [...current, { agentId, start, end }]),
  });

  const submit = () => {
    if (!text.trim() || ended || send.isPending) return;
    setError("");
    const wire = mentions.map((m) => ({ agent_id: m.agentId, start: utf8Length(text.slice(0, m.start)), end: utf8Length(text.slice(0, m.end)) }));
    send.mutate({ command_id: commandID, body: text, mentions: wire }, {
      onSuccess: () => {
        setText("");
        setMentions([]);
        setCommandID(newCommandID());
      },
      // A refused message keeps the draft and its mentions (INV §8).
      onError: (err) => setError(err instanceof Error ? err.message : "The message was not sent."),
    });
  };
  const addressed = [...new Set(mentions.map((m) => targets.find((t) => t.agentId === m.agentId)?.label ?? room.members.find((member) => member.agent_id === m.agentId)?.name ?? "an unavailable participant"))];

  // The toolbar's @ and # type the trigger at the caret, as the agent
  // composer does (FS-12.R64, TS-08.R115).
  const insertTrigger = (trigger: "@" | "#") => {
    const textarea = textareaRef.current;
    const start = textarea?.selectionStart ?? text.length;
    const end = textarea?.selectionEnd ?? start;
    const token = (start > 0 && !/\s/.test(text[start - 1]) ? " " : "") + trigger;
    changeText(text.slice(0, start) + token + text.slice(end));
    requestAnimationFrame(() => {
      if (!textarea) return;
      textarea.focus();
      textarea.setSelectionRange(start + token.length, start + token.length);
      autocomplete.syncTrigger(textarea);
    });
  };

  if (ended) {
    return (
      <div className="think-tank-composer" data-ui="think-tank" data-slot="composer">
        <div className="think-tank-readonly">
          <RoomIcon />
          <div>
            <strong>This room is read-only</strong>
            <p>Room messages are closed. Annotate an entry to follow up with an agent; participant conversations stay available from their cards.</p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="think-tank-composer" data-ui="think-tank" data-slot="composer">
      <form className="composer" data-ui="composer" data-variant="room" onSubmit={(event) => { event.preventDefault(); submit(); }}>
        <div className="composer-input">
          <VisuallyHidden><label htmlFor="think-tank-message">Message the room</label></VisuallyHidden>
          <AutoGrowTextarea
            id="think-tank-message"
            ref={textareaRef}
            maxHeight="40vh"
            value={text}
            disabled={send.isPending}
            placeholder="Add a thought, question, or direction to the room…"
            onChange={(event) => { changeText(event.target.value); autocomplete.syncTrigger(event.target); }}
            onKeyUp={(event) => autocomplete.syncTrigger(event.currentTarget)}
            onClick={(event) => autocomplete.syncTrigger(event.currentTarget)}
            onKeyDown={(event) => {
              if (autocomplete.onKeyDown(event)) return;
              if (event.key === "Enter" && !event.shiftKey && !event.nativeEvent.isComposing) {
                event.preventDefault();
                submit();
              }
            }}
          />
          {autocomplete.picker}
        </div>
        <div className="composer-toolbar">
          <div className="composer-references">
            <button type="button" className="ad-button-icon" aria-label="Address a participant or reference a file" title="Address a participant or reference a file" onClick={() => insertTrigger("@")}>@</button>
            <button type="button" className="ad-button-icon" aria-label="Insert a command" title="Insert a command" onClick={() => insertTrigger("#")}>#</button>
            {sources.length > 0 && (
              <select className="think-tank-composer-source" aria-label="Files and commands from" title="Files and commands from" value={selectedSource?.agent_id ?? ""} onChange={(event) => { setSourceId(event.target.value); autocomplete.reset(); }}>
                {sources.map((m) => <option key={m.agent_id} value={m.agent_id}>{m.name}</option>)}
              </select>
            )}
          </div>
          <div className="composer-actions">
            <button type="submit" className="composer-icon ad-button-icon-primary" aria-label="Send to room" title="Send to room" disabled={!text.trim() || send.isPending}><span className="composer-send-label">Send to room</span><SendIcon /></button>
          </div>
        </div>
        {addressed.length > 0 && (
          <p className="composer-notice" role="status">Addressed to {addressed.join(", ")} on their next room turn.</p>
        )}
        {error && <p className="composer-error" role="alert">{error}</p>}
      </form>
      <p className="think-tank-composer-help">
        <span>{held ? "Held until the current turn finishes, then shared with the room." : "Shared with every participant, not private agent history."}</span>
        <span>Enter to send · Shift+Enter for a new line</span>
      </p>
    </div>
  );
}
