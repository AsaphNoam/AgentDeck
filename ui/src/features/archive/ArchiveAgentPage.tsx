import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { getTranscript, resumeAgent } from "../../api/client";
import { useTranscriptStore } from "../../store/transcriptStore";
import { useAgentStore } from "../../store/agentStore";
import { TranscriptView } from "../../components/chat/TranscriptView";

export function ArchiveAgentPage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const events = useTranscriptStore((state) => state.byAgent[id] ?? []);
  const setTranscript = useTranscriptStore((state) => state.setTranscript);
  const agent = useAgentStore((state) => state.agents[id]);
  const [resuming, setResuming] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const metadata = events.find((event) => (event.kind ?? event.type) === "session_meta");
  const archivedName = textField(metadata?.name) || "Archived session";
  const project = textField(metadata?.project);
  const backend = textField(metadata?.backend);
  const model = textField(metadata?.model);
  const createdAt = textField(metadata?.created_at);

  useEffect(() => {
    void getTranscript(id, true).then((r) => setTranscript(r.agent_id, r.events)).catch(() => {});
  }, [id, setTranscript]);

  const doResume = async () => {
    setResuming(true);
    setError(null);
    try {
      await resumeAgent(id);
      navigate(`/agent/${id}`);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Resume failed");
      setResuming(false);
    }
  };

  return (
    <section className="chat-panel" data-ui="agent-workspace" data-state="archived" data-variant="chat">
      <header className="chat-header" data-slot="header">
        <Link to="/archive">Back to Archive</Link>
        <div data-slot="identity">
          <h1>{archivedName}</h1>
          {(project || backend || model) && <span>{[project, [backend, model].filter(Boolean).join(" · ")].filter(Boolean).join(" · ")}</span>}
          {(project || backend || model) && <br />}
          <span className="archive-readonly-label">
            Archived · read-only{createdAt && <> · <time dateTime={createdAt}>{formatTimestamp(createdAt)}</time></>}
          </span>
        </div>
        <button
          type="button"
          className="resume-btn"
          disabled={resuming}
          onClick={() => void doResume()}
        >
          {resuming ? "Resuming…" : "Resume"}
        </button>
      </header>
      {error && <p className="archive-error">{error}</p>}
      <div data-slot="content"><TranscriptView agentId={id} events={events} sourceActive={false} annotationsEnabled={agent?.interface !== "terminal"} /></div>
      {/* No Composer — read-only view */}
      <div />
    </section>
  );
}

function textField(value: unknown) {
  return typeof value === "string" ? value : "";
}

function formatTimestamp(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}
