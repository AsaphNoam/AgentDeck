import type { AgentStatus } from "../../api/types";

export function StateBadge({ state }: { state: AgentStatus }) {
  return (
    <span className={`state-badge ${state}`} data-testid="state-badge">
      <span />
      {state === "waiting_input" ? "Waiting" : state === "unknown" ? "-" : state.replace("_", " ")}
    </span>
  );
}
