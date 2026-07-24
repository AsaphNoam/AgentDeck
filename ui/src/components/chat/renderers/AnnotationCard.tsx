import type { TranscriptEvent } from "../../../api/types";

export function AnnotationCard({ event }: { event: TranscriptEvent }) {
  const annotations = Array.isArray(event.annotations) ? event.annotations as Array<Record<string, unknown>> : [];
  const target = event.target as Record<string, unknown> | undefined;
  return (
    <article className="annotation-card" data-ui="transcript" data-variant="annotation">
      <header><strong>Annotations assigned</strong><span>{target?.kind === "self" ? "Current agent" : target?.agent_id ? `Agent ${String(target.agent_id)}` : "Agent"}</span></header>
      {annotations.map((annotation, index) => (
        <section key={index}>
          <p>{annotation.path ? `${String(annotation.path)}:${String(annotation.start_line ?? "")}${annotation.end_line && annotation.end_line !== annotation.start_line ? `–${String(annotation.end_line)}` : ""}` : `Event ${String(annotation.seq ?? "")}`}</p>
          <blockquote>{String(annotation.excerpt ?? "")}</blockquote>
          <p><strong>Instruction:</strong> {String(annotation.instruction ?? "")}</p>
        </section>
      ))}
      {event.overall_instruction ? <p className="annotation-card-overall"><strong>Overall:</strong> {String(event.overall_instruction)}</p> : null}
    </article>
  );
}
