import { useState, type ReactNode } from "react";
import type { TranscriptEvent } from "../../../api/types";
import { hasPendingPermission, type ChildNode, type ChildState } from "../runtimeActivity";

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
          {renderEvents(node.items, path, depth + 1)}
        </div>
      )}
    </section>
  );
}
