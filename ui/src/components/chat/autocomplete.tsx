import { Fragment, useEffect, useLayoutEffect, useRef, useState, type KeyboardEvent, type RefObject } from "react";
import { getAvailableCommands, searchSessionFiles } from "../../api/client";
import type { AvailableCommand } from "../../api/types";

// The one keyboard-operated autocomplete picker for `@` files and `#` ACP
// commands (FS-03.R30–R34). Both insert ordinary composer text — no structured
// attachment, no command persistence — so submission is unchanged. The agent
// composer and the Think Tank room composer share it; the room names which
// participant's workspace and session the suggestions come from (FS-21.R35).

type Trigger = { type: "@" | "#"; start: number; query: string };

// A picker entry: `label` is shown, `insert` is the text that replaces the typed
// trigger token (already including its trailing space), and the optional
// description/hint annotate a command entry.
// A Think Tank room picker also offers participant mentions: `mention` marks
// such an item, and `group` labels the participant and file sections
// (FS-21.R46, TS-08.R97).
export type PickerItem = { label: string; insert: string; description?: string; hint?: string; group?: string; mention?: string };

/** A participant a room message may address. */
export type MentionTarget = { agentId: string; label: string; note?: string };

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

/** useAutocomplete owns trigger detection, suggestion fetching for one source
 *  agent, keyboard navigation and insertion. `qualify` may rewrite an inserted
 *  token, e.g. to name the participant whose workspace it came from. */
export function useAutocomplete({ sourceId, text, onText, textareaRef, qualify, mentions, fileGroup, onMention }: {
  sourceId: string;
  text: string;
  onText: (next: string) => void;
  textareaRef: RefObject<HTMLTextAreaElement | null>;
  qualify?: (item: PickerItem) => string;
  /** Participants offered before files on `@`; selecting one reports its
   *  inserted UTF-16 range through onMention. */
  mentions?: MentionTarget[];
  fileGroup?: string;
  onMention?: (agentId: string, start: number, end: number) => void;
}) {
  const [trigger, setTrigger] = useState<Trigger | null>(null);
  const [items, setItems] = useState<PickerItem[]>([]);
  const [highlight, setHighlight] = useState(0);
  // dismissedStart holds the offset of a trigger token dismissed with Escape, so
  // the picker stays closed for that token until a fresh `@`/`#` is typed (R32).
  const [dismissedStart, setDismissedStart] = useState<number | null>(null);
  const reqRef = useRef(0);
  const pendingCursor = useRef<number | null>(null);

  const open = trigger !== null && trigger.start !== dismissedStart && sourceId !== "";
  const hasSuggestions = open && items.length > 0;

  const reset = () => {
    reqRef.current++;
    setTrigger(null);
    setItems([]);
    setHighlight(0);
    setDismissedStart(null);
  };

  // Fetch suggestions whenever the active trigger, its query or the source
  // changes. A per-request token discards stale answers (INV §1); any failure
  // clears items so the picker shows an empty state and never blocks typing.
  useEffect(() => {
    const token = ++reqRef.current;
    if (!open || !trigger) {
      setItems([]);
      return;
    }
    setHighlight(0);
    if (trigger.type === "@") {
      const q = trigger.query.toLowerCase();
      const people: PickerItem[] = (mentions ?? [])
        .filter((m) => m.label.toLowerCase().includes(q))
        .map((m) => ({ label: `@${m.label}`, insert: `@${m.label} `, description: m.note, group: "Participants", mention: m.agentId }));
      setItems(people);
      searchSessionFiles(sourceId, trigger.query)
        .then((res) => {
          if (token === reqRef.current) setItems([...people, ...(res.files ?? []).map((f) => ({ label: f, insert: `@${f} `, group: mentions ? fileGroup : undefined }))]);
        })
        .catch(() => {
          if (token === reqRef.current) setItems(people);
        });
    } else {
      getAvailableCommands(sourceId)
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
    return () => { reqRef.current++; };
  }, [sourceId, open, trigger?.type, trigger?.query, mentions, fileGroup]);

  // Restore the caret after a programmatic insert (React resets it on value change).
  useLayoutEffect(() => {
    if (pendingCursor.current !== null && textareaRef.current) {
      const pos = pendingCursor.current;
      pendingCursor.current = null;
      textareaRef.current.setSelectionRange(pos, pos);
    }
  }, [text, textareaRef]);

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
    const insert = item.mention || !qualify ? item.insert : qualify(item);
    const next = text.slice(0, trigger.start) + insert + text.slice(cursor);
    pendingCursor.current = trigger.start + insert.length;
    onText(next);
    if (item.mention) onMention?.(item.mention, trigger.start, trigger.start + insert.trimEnd().length);
    setTrigger(null);
    setItems([]);
    setDismissedStart(null);
  };

  /** onKeyDown handles picker keys and reports whether it consumed the event. */
  const onKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>): boolean => {
    if (event.nativeEvent.isComposing || (event.key === "Enter" && event.shiftKey)) return false;
    if (!open) return false;
    if (event.key === "Escape") {
      event.preventDefault();
      setDismissedStart(trigger?.start ?? null);
      return true;
    }
    if (!hasSuggestions) return false;
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setHighlight((h) => (h + 1) % items.length);
      return true;
    }
    if (event.key === "ArrowUp") {
      event.preventDefault();
      setHighlight((h) => (h - 1 + items.length) % items.length);
      return true;
    }
    if (event.key === "Enter" || event.key === "Tab") {
      event.preventDefault();
      accept(items[highlight]);
      return true;
    }
    return false;
  };

  const picker = open ? (
    <ul className="composer-picker" role="listbox" aria-label={trigger?.type === "@" ? (mentions ? "Participants and files" : "Files") : "Commands"}>
      {items.length === 0 ? (
        <li className="composer-picker-empty" aria-disabled="true">
          {trigger?.type === "@" ? (mentions ? "No matching participants or files" : "No matching files") : "No commands available"}
        </li>
      ) : (
        items.map((item, i) => (
          <Fragment key={item.label + i}>
            {item.group && item.group !== items[i - 1]?.group && (
              <li className="composer-picker-group" role="presentation">{item.group}</li>
            )}
            <li
              role="option"
              aria-selected={i === highlight}
              className={i === highlight ? "composer-picker-item is-active" : "composer-picker-item"}
              onMouseDown={(event) => { event.preventDefault(); accept(item); }}
            >
              <span className="composer-picker-label">{item.label}</span>
              {item.description && <span className="composer-picker-desc">{item.description}</span>}
              {item.hint && <span className="composer-picker-hint">{item.hint}</span>}
            </li>
          </Fragment>
        ))
      )}
    </ul>
  ) : null;

  return { syncTrigger, onKeyDown, picker, reset, clearTrigger: () => setTrigger(null) };
}
