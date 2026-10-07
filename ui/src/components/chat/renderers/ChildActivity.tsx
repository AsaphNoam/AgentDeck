import { useState, type ReactNode } from "react";
import type { TranscriptEvent } from "../../../api/types";
import { hasPendingPermission, type ChildNode, type ChildState } from "../runtimeActivity";
import { ThoughtScopeContext, useThoughtScope } from "../TurnList";

const STATE_LABEL: Record<ChildState, string> = {
  active: "Running",
  completed: "Completed",
  failed: "Failed",
  stopped: "Stopped",
  // Read-only history whose outcome the provider could not prove (FS-03.R58).
  disconnected: "Outcome unknown",
};

// A native child as a restrained disclosure at its causal position (TS-08.R59).
// It stays open while running or waiting on a permission, and its contents use
// the ordinary transcript renderers. Indentation stops after two levels while
// the label keeps the full ancestry.
export function ChildActivity({ node, ancestry, depth, renderEvents }: {
  node: ChildNode;
  ancestry: string[];
  depth: number;
  renderEvents: (events: TranscriptEvent[], ancestry: string[], depth: number) => ReactNode;
}) {
  const [chosen, setChosen] = useState<boolean | null>(null);
  // The first terminal state drops any open choice once, so the child and its
  // nested tool/thought detail close; a later manual reopen stays (TS-08.R102).
  const terminal = node.state !== "active";
  const [sawTerminal, setSawTerminal] = useState(terminal);
  if (terminal && !sawTerminal) {
    setSawTerminal(true);
    setChosen(null);
  }
  const scope = useThoughtScope();
  const open = chosen ?? (node.state === "active" || hasPendingPermission(node));
  const path = [...ancestry, node.name || "Subagent"];
  return (
    <section
      className={depth <= 2 ? "runtime-child runtime-child-indent" : "runtime-child"}
      data-ui="runtime-activity"
      data-slot="child"
      data-state={node.state}
    >
      <button type="button" className="tool-toggle" aria-expanded={open} onClick={() => setChosen(!open)}>
        {open ? "▾" : "▸"} {path.join(" › ")} · {STATE_LABEL[node.state]}
      </button>
      {open && (
        <div className="runtime-child-content">
          {node.task && <p className="runtime-child-task">{node.task}</p>}
          {/* A child's terminal outcome ends its live thoughts (FS-03.R73). */}
          <ThoughtScopeContext.Provider value={scope && { ...scope, live: scope.live && node.state === "active" }}>
            {renderEvents(node.items, path, depth + 1)}
          </ThoughtScopeContext.Provider>
        </div>
      )}
    </section>
  );
}
