import type { AgentStatus } from "../../api/types";
import { Badge } from "../ui";

export function StateBadge({ state }: { state: AgentStatus }) {
  return (
    <Badge
      className={`state-badge ${state}`}
      variant={state === "done" ? "success" : state === "error" ? "danger" : state === "busy" ? "warning" : state === "waiting_input" ? "info" : "neutral"}
      state={state}
      indicator
      data-testid="state-badge"
    >
      {state === "waiting_input" ? "Waiting" : state === "unknown" ? "-" : state.replace("_", " ")}
    </Badge>
  );
}
