import { useState } from "react";
import { decidePermission } from "../../../api/client";
import type { TranscriptEvent } from "../../../api/types";
import { useTranscriptStore } from "../../../store/transcriptStore";

export function PermissionPrompt({ agentId, event }: { agentId: string; event: TranscriptEvent }) {
  const resolve = useTranscriptStore((state) => state.resolvePermission);
  const toolCallId = String(event.tool_call_id ?? "");
  const resolved = event.resolved as "approve" | "deny" | undefined;
  const [error, setError] = useState<string | null>(null);
  const decide = async (decision: "approve" | "deny") => {
    setError(null);
    try {
      await decidePermission(agentId, toolCallId, decision);
      resolve(agentId, toolCallId, decision);
    } catch {
      setError("Failed to send decision — the agent may have stopped.");
    }
  };
  const label = String(event.name ?? event.tool ?? "Permission required");
  if (resolved) {
    return (
      <article className="permission-prompt resolved">
        <strong>{label}</strong>
        <span className="permission-chip">{resolved === "approve" ? "Approved" : "Denied"}</span>
      </article>
    );
  }
  return (
    <article className="permission-prompt">
      <strong>{label}</strong>
      <p>{String(event.reason ?? "")}</p>
      {error && <p className="permission-error">{error}</p>}
      <button type="button" onClick={() => void decide("approve")}>Approve</button>
      <button type="button" onClick={() => void decide("deny")}>Deny</button>
    </article>
  );
}
