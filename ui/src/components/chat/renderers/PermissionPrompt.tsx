import { decidePermission } from "../../../api/client";
import type { TranscriptEvent } from "../../../api/types";
import { useTranscriptStore } from "../../../store/transcriptStore";

export function PermissionPrompt({ agentId, event }: { agentId: string; event: TranscriptEvent }) {
  const resolve = useTranscriptStore((state) => state.resolvePermission);
  const toolCallId = String(event.tool_call_id ?? "");
  const resolved = event.resolved as "approve" | "deny" | undefined;
  const decide = async (decision: "approve" | "deny") => {
    resolve(agentId, toolCallId, decision);
    await decidePermission(agentId, toolCallId, decision);
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
      <button type="button" onClick={() => void decide("approve")}>Approve</button>
      <button type="button" onClick={() => void decide("deny")}>Deny</button>
    </article>
  );
}
