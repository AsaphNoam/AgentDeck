import { useEffect, useMemo, useRef, useState } from "react";

const RESERVED_GROUP = "_ungrouped";

export function GroupPicker({
  value,
  groups,
  onChange,
  id = "group-picker",
  disabled = false,
}: {
  value: string;
  groups: string[];
  onChange: (value: string) => void;
  id?: string;
  disabled?: boolean;
}) {
  const [query, setQuery] = useState(value);
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const [filtering, setFiltering] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const labels = useMemo(() => [...new Set(groups.map((group) => group.trim()).filter((group) => group && group !== RESERVED_GROUP))].sort(), [groups]);
  const filtered = useMemo(() => {
    const needle = filtering ? query.trim().toLowerCase() : "";
    return labels.filter((label) => label.toLowerCase().includes(needle));
  }, [labels, query, filtering]);
  const trimmed = query.trim();
  const exact = labels.find((label) => label === trimmed);
  const reserved = trimmed === RESERVED_GROUP;
  const choices = [
    { kind: "ungrouped" as const, label: "Ungrouped", value: "" },
    ...filtered.map((label) => ({ kind: "existing" as const, label, value: label })),
    ...(trimmed && !exact && !reserved ? [{ kind: "create" as const, label: `Create group “${trimmed}”`, value: trimmed }] : []),
  ];

  useEffect(() => setQuery(value), [value]);

  const choose = (next: string) => {
    onChange(next);
    setQuery(next);
    setOpen(false);
  };

  return (
    <div className="group-picker" ref={rootRef}>
      <input
        id={id}
        role="combobox"
        aria-label="Group"
        aria-expanded={open}
        aria-controls={`${id}-options`}
        aria-activedescendant={open ? `${id}-option-${active}` : undefined}
        aria-autocomplete="list"
        value={query}
        disabled={disabled}
        onFocus={() => { setOpen(true); setActive(0); setFiltering(false); }}
        onBlur={() => window.setTimeout(() => {
          if (!rootRef.current?.contains(document.activeElement)) { setQuery(value); setOpen(false); }
        }, 0)}
        onChange={(event) => { setQuery(event.target.value); setOpen(true); setActive(0); setFiltering(true); }}
        onKeyDown={(event) => {
          if (event.key === "Escape") { setQuery(value); setOpen(false); return; }
          if (event.key === "ArrowDown") { event.preventDefault(); setOpen(true); setActive((index) => Math.min(index + 1, choices.length - 1)); return; }
          if (event.key === "ArrowUp") { event.preventDefault(); setActive((index) => Math.max(index - 1, 0)); return; }
          if (event.key === "Enter" && open && choices[active] !== undefined) { event.preventDefault(); if (!reserved || choices[active].kind === "ungrouped") choose(choices[active].value); }
        }}
      />
      {open && (
        <ul id={`${id}-options`} role="listbox" className="group-picker-options">
          <li id={`${id}-option-0`} role="option" aria-selected={active === 0}>
            <button type="button" onMouseDown={(event) => event.preventDefault()} onClick={() => choose("")}>Ungrouped</button>
          </li>
          {filtered.map((label, index) => (
            <li id={`${id}-option-${index + 1}`} key={label} role="option" aria-selected={active === index + 1}>
              <button type="button" onMouseDown={(event) => event.preventDefault()} onClick={() => choose(label)}>{label}</button>
            </li>
          ))}
          {trimmed && !exact && !reserved && (
            <li id={`${id}-option-${filtered.length + 1}`} role="option" aria-selected={active === filtered.length + 1}>
              <button type="button" onMouseDown={(event) => event.preventDefault()} onClick={() => choose(trimmed)}>Create group “{trimmed}”</button>
            </li>
          )}
          {reserved && <li className="form-error" role="status">“_ungrouped” is reserved.</li>}
        </ul>
      )}
    </div>
  );
}

export { RESERVED_GROUP };
