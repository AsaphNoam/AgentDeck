import { useState } from "react";
import { decidePermission } from "../../../api/client";
import type { PermissionResolution, TranscriptEvent } from "../../../api/types";
import { useTranscriptStore } from "../../../store/transcriptStore";

export function PermissionPrompt({ agentId, event }: { agentId: string; event: TranscriptEvent }) {
  const resolve = useTranscriptStore((state) => state.resolvePermission);
  const toolCallId = String(event.tool_call_id ?? "");
  const resolved = event.resolved as PermissionResolution | undefined;
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
    const presentation = resolutionPresentation[resolved];
    return (
      <article
        className="permission-prompt resolved"
        data-ui="permission-prompt"
        data-state={resolved === "approve" ? "approved" : resolved === "deny" ? "denied" : resolved === "cancelled" ? "cancelled" : "timed-out"}
      >
        <strong data-slot="title">{label}</strong>
        <span className="permission-chip" data-slot="resolution">{presentation.label}</span>
      </article>
    );
  }
  return (
    <article className="permission-prompt" data-ui="permission-prompt" data-state={error ? "error" : "pending"}>
      <strong data-slot="title">{label}</strong>
      <p data-slot="reason">{String(event.reason ?? "")}</p>
      {error && <p className="permission-error" data-slot="error">{error}</p>}
      <div data-slot="actions">
        <button type="button" onClick={() => void decide("approve")}>Approve</button>
        <button type="button" onClick={() => void decide("deny")}>Deny</button>
      </div>
    </article>
  );
}

const resolutionPresentation: Record<PermissionResolution, { label: string }> = {
  approve: { label: "Approved" },
  deny: { label: "Denied" },
  cancelled: { label: "Cancelled" },
  timeout: { label: "Timed out" },
};
