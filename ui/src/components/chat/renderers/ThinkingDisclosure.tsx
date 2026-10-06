import { useState } from "react";
import { thoughtScope, useReasoningStore } from "../../../store/reasoningStore";
import { useThoughtScope } from "../TurnList";

// One compact disclosure per live reasoning span (FS-03.R57, TS-08.R59). While
// its turn and child scope run it starts open, and a manual collapse holds for
// every later span in that scope and turn (FS-03.R73). Completed activity starts
// closed. It is never persisted and reserves no space after reload.
export function ThinkingDisclosure({ text, activityId }: { text: string; activityId?: string }) {
  const scope = useThoughtScope();
  const live = Boolean(scope?.live);
  const key = scope ? thoughtScope(scope.turnKey, activityId) : "";
  const collapsed = useReasoningStore((state) => (scope ? state.byAgent[scope.agentId]?.collapsed.includes(key) : false));
  const setCollapsed = useReasoningStore((state) => state.setCollapsed);
  const [local, setLocal] = useState(false);
  const open = live ? !collapsed : local;
  const toggle = () => (live && scope ? setCollapsed(scope.agentId, key, open) : setLocal(!local));
  return (
    <section className="thinking" data-ui="runtime-activity" data-slot="thinking">
      <button type="button" className="tool-toggle" aria-expanded={open} onClick={toggle}>
        {open ? "▾" : "▸"} Thinking
      </button>
      {open && <p className="thinking-text">{text}</p>}
    </section>
  );
}
