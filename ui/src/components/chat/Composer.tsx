import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { cancelTurn, getAvailableCommands, searchSessionFiles, sendPrompt } from "../../api/client";
import type { AvailableCommand } from "../../api/types";
import { useTranscriptStore } from "../../store/transcriptStore";
import { discardChatDraft, getChatDraft, setChatDraft } from "./drafts";

// Composer owns one reusable keyboard-operated autocomplete picker for `@` files
// and `#` ACP commands (FS-03.R30–R34). Both insert ordinary composer text — no
// structured attachment, no command persistence — so submission (R6) is unchanged.

type Trigger = { type: "@" | "#"; start: number; query: string };

// A picker entry: `label` is shown, `insert` is the text that replaces the typed
// trigger token (already including its trailing space), and the optional
// description/hint annotate a command entry.
type PickerItem = { label: string; insert: string; description?: string; hint?: string };

// detectTrigger finds the active `@`/`#` token ending at the cursor. A trigger is
// active only at a word boundary (start of input or after whitespace) with no
// whitespace between it and the cursor, so `user@example.com` never opens it.
function detectTrigger(text: string, cursor: number): Trigger | null {
  for (let i = cursor - 1; i >= 0; i--) {
    const ch = text[i];
    if (ch === " " || ch === "\n" || ch === "\t") return null;
    if (ch === "@" || ch === "#") {
      const before = i === 0 ? "" : text[i - 1];
      if (before === "" || before === " " || before === "\n" || before === "\t") {
        return { type: ch, start: i, query: text.slice(i + 1, cursor) };
      }
      return null;
    }
  }
  return null;
}

// filterCommands keeps commands whose name or description contains the query
// (case-insensitive); an empty query keeps the whole advertised list (R33).
function filterCommands(commands: AvailableCommand[], query: string): PickerItem[] {
  const q = query.toLowerCase();
  return commands
    .filter((c) => q === "" || c.name.toLowerCase().includes(q) || c.description.toLowerCase().includes(q))
    .map((c) => ({
      // ACP advertises names without the invocation slash, so insert `/<name>`;
      // a Codex `$skill` therefore becomes `/$skill` (R33).
      label: `/${c.name}`,
      insert: `/${c.name} `,
      description: c.description,
      hint: c.input_hint,
    }));
}

export function Composer({ agentId, busy }: { agentId: string; busy: boolean }) {
  const [text, setText] = useState(() => getChatDraft(agentId));
  const [error, setError] = useState<string | null>(null);
  const [trigger, setTrigger] = useState<Trigger | null>(null);
  const [items, setItems] = useState<PickerItem[]>([]);
  const [highlight, setHighlight] = useState(0);
  // dismissedStart holds the offset of a trigger token dismissed with Escape, so
  // the picker stays closed for that token until a fresh `@`/`#` is typed (R32).
  const [dismissedStart, setDismissedStart] = useState<number | null>(null);
  const append = useTranscriptStore((state) => state.appendMessage);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const reqRef = useRef(0);
  const pendingCursor = useRef<number | null>(null);
  const currentAgentId = useRef(agentId);
  currentAgentId.current = agentId;

  useEffect(() => {
    setText(getChatDraft(agentId));
    setError(null);
    setTrigger(null);
    setItems([]);
    setHighlight(0);
    setDismissedStart(null);
  }, [agentId]);

  const open = trigger !== null && trigger.start !== dismissedStart;
  const hasSuggestions = open && items.length > 0;

  // Fetch suggestions whenever the active trigger or its query changes. A per-request
  // token discards stale answers (INV §1); any failure clears items so the picker
  // shows an empty state and never blocks the composer (R32).
  useEffect(() => {
    if (!open || !trigger) {
      setItems([]);
      return;
    }
    const token = ++reqRef.current;
    setHighlight(0);
    if (trigger.type === "@") {
      searchSessionFiles(agentId, trigger.query)
        .then((res) => {
          if (token === reqRef.current) setItems((res.files ?? []).map((f) => ({ label: f, insert: `@${f} ` })));
        })
        .catch(() => {
          if (token === reqRef.current) setItems([]);
        });
    } else {
      getAvailableCommands(agentId)
        .then((res) => {
          if (token === reqRef.current) setItems(filterCommands(res.commands ?? [], trigger.query));
        })
        .catch(() => {
          if (token === reqRef.current) setItems([]);
        });
    }
    // trigger.start is intentionally excluded: only the type and query change what
    // is fetched; start changes are covered by dismissedStart/open.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [agentId, open, trigger?.type, trigger?.query]);

  // Restore the caret after a programmatic insert (React resets it on value change).
  useLayoutEffect(() => {
    if (pendingCursor.current !== null && textareaRef.current) {
      const pos = pendingCursor.current;
      pendingCursor.current = null;
      textareaRef.current.setSelectionRange(pos, pos);
    }
  }, [text]);

  const syncTrigger = (el: HTMLTextAreaElement) => {
    const next = detectTrigger(el.value, el.selectionStart ?? el.value.length);
    setTrigger(next);
    // Leaving the token (whitespace/caret move) clears an Escape dismissal so the
    // next `@`/`#` reopens the picker.
    if (!next) setDismissedStart(null);
  };

  const accept = (item: PickerItem) => {
    if (!trigger) return;
    const el = textareaRef.current;
    const cursor = el?.selectionStart ?? text.length;
    const next = text.slice(0, trigger.start) + item.insert + text.slice(cursor);
    const caret = trigger.start + item.insert.length;
    pendingCursor.current = caret;
    setText(next);
    setChatDraft(agentId, next);
    setTrigger(null);
    setItems([]);
    setDismissedStart(null);
  };

  const submit = async () => {
    // Reject whitespace-only drafts (R19) but send the text exactly as displayed:
    // a picker insertion's trailing space is part of the prompt and must reach both
    // the optimistic event and POST /prompt verbatim (FS-03.R31/R33, INV §1).
    const prompt = text;
    if (!prompt.trim()) return;
    setError(null);
    append(agentId, { kind: "user_text", text: prompt, message_id: `local-${Date.now()}` });
    setText("");
    setTrigger(null);
    try {
      // A stopped chat agent is woken by this same request (FS-03.R35), so the
      // composer submits normally and only the reply takes longer to start.
      await sendPrompt(agentId, prompt);
      discardChatDraft(agentId);
    } catch (err) {
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

  const onKeyDown = (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (open) {
      if (event.key === "Escape") {
        event.preventDefault();
        setDismissedStart(trigger?.start ?? null);
        return;
      }
      if (hasSuggestions) {
        if (event.key === "ArrowDown") {
          event.preventDefault();
          setHighlight((h) => (h + 1) % items.length);
          return;
        }
        if (event.key === "ArrowUp") {
          event.preventDefault();
          setHighlight((h) => (h - 1 + items.length) % items.length);
          return;
        }
        if (event.key === "Enter" || event.key === "Tab") {
          event.preventDefault();
          accept(items[highlight]);
          return;
        }
      }
    }
    // Picker closed (or showing an empty state): Enter submits, Shift+Enter newlines.
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      void submit();
    }
  };

  return (
    <form className="composer" onSubmit={(event) => { event.preventDefault(); void submit(); }}>
      <div className="composer-input">
        <textarea
          ref={textareaRef}
          value={text}
          onChange={(event) => {
            const next = event.target.value;
            setText(next);
            setChatDraft(agentId, next);
            syncTrigger(event.target);
          }}
          onKeyUp={(event) => syncTrigger(event.currentTarget)}
          onClick={(event) => syncTrigger(event.currentTarget)}
          onKeyDown={onKeyDown}
        />
        {open && (
          <ul className="composer-picker" role="listbox" aria-label={trigger?.type === "@" ? "Files" : "Commands"}>
            {items.length === 0 ? (
              <li className="composer-picker-empty" aria-disabled="true">
                {trigger?.type === "@" ? "No matching files" : "No commands available"}
              </li>
            ) : (
              items.map((item, i) => (
                <li
                  key={item.label + i}
                  role="option"
                  aria-selected={i === highlight}
                  className={i === highlight ? "composer-picker-item is-active" : "composer-picker-item"}
                  onMouseDown={(event) => { event.preventDefault(); accept(item); }}
                >
                  <span className="composer-picker-label">{item.label}</span>
                  {item.description && <span className="composer-picker-desc">{item.description}</span>}
                  {item.hint && <span className="composer-picker-hint">{item.hint}</span>}
                </li>
              ))
            )}
          </ul>
        )}
      </div>
      {busy ? (
        <button
          type="button"
          onClick={() => {
            setError(null);
            cancelTurn(agentId).catch(() => setError("Failed to cancel — the turn may have already finished."));
          }}
        >
          Cancel
        </button>
      ) : (
        <button type="submit">Send</button>
      )}
      {error && <p className="composer-error">{error}</p>}
    </form>
  );
}
