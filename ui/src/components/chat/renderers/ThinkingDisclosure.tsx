import { useState } from "react";

// One compact, collapsed-by-default disclosure per live reasoning span
// (FS-03.R57, TS-08.R59). Opening it shows the text received so far and keeps
// streaming in place; it is never persisted and reserves no space after reload.
export function ThinkingDisclosure({ text }: { text: string }) {
  const [open, setOpen] = useState(false);
  return (
    <section className="thinking" data-ui="runtime-activity" data-slot="thinking">
      <button type="button" className="tool-toggle" aria-expanded={open} onClick={() => setOpen((value) => !value)}>
        {open ? "▾" : "▸"} Thinking
      </button>
      {open && <p className="thinking-text">{text}</p>}
    </section>
  );
}
