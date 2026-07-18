import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { getTranscript, resumeAgent } from "../../api/client";
import { useTranscriptStore } from "../../store/transcriptStore";
import { TranscriptView } from "../../components/chat/TranscriptView";

export function ArchiveAgentPage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const events = useTranscriptStore((state) => state.byAgent[id] ?? []);
  const setTranscript = useTranscriptStore((state) => state.setTranscript);
  const [resuming, setResuming] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    void getTranscript(id).then((r) => setTranscript(r.agent_id, r.events)).catch(() => {});
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
          <h1>Archived session</h1>
          <span className="archive-readonly-label">read-only</span>
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
      <div data-slot="content"><TranscriptView agentId={id} events={events} /></div>
      {/* No Composer — read-only view */}
      <div />
    </section>
  );
}
